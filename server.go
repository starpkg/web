package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/1set/starlight/convert"
	"github.com/gin-gonic/gin"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// Server represents an HTTP server instance
type Server struct {
	host         string
	port         int
	engine       *gin.Engine
	httpServer   *http.Server
	running      bool
	module       *Module // Reference to module for config access
	readTimeout  time.Duration
	writeTimeout time.Duration
	maxBodySize  int64
	enableCORS   bool
}

// newServer creates a new Server instance with module configuration
func newServer(module *Module, host string, port int) *Server {
	// Get configuration values from module
	readTimeout := time.Duration(module.ext.GetInt(configKeyReadTimeout)) * time.Second
	writeTimeout := time.Duration(module.ext.GetInt(configKeyWriteTimeout)) * time.Second

	// For int64 values, we need to use GetConfigValue directly
	maxBodySize, err := base.GetConfigValue[int64](module.cfgMod, configKeyMaxBodySize)
	if err != nil {
		maxBodySize = int64(32 << 20) // Default 32MB if config fails
	}

	enableCORS := module.ext.GetBool(configKeyEnableCORS)
	debugMode := module.ext.GetBool(configKeyDebugMode)

	// Set gin mode
	if !debugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create gin engine
	engine := gin.New()
	engine.Use(gin.Recovery())
	if debugMode {
		engine.Use(gin.Logger())
	}

	// Configure method not allowed handler
	engine.HandleMethodNotAllowed = true
	engine.NoMethod(func(c *gin.Context) {
		sendMethodNotAllowed(c, "Method not allowed")
	})

	// Add CORS middleware if enabled
	if enableCORS {
		engine.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}

			c.Next()
		})
	}

	return &Server{
		host:         host,
		port:         port,
		engine:       engine,
		httpServer:   nil,
		running:      false,
		module:       module,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		maxBodySize:  maxBodySize,
		enableCORS:   enableCORS,
	}
}

// Server methods for Starlark integration

// Get adds a GET route to the server.
// This method registers a handler function for HTTP GET requests to the specified path.
// Path parameters can be specified using {param} syntax (converted to Gin's :param format).
func (s *Server) Get(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodGet, path, handler)
}

// Post adds a POST route
func (s *Server) Post(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodPost, path, handler)
}

// Put adds a PUT route
func (s *Server) Put(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodPut, path, handler)
}

// Delete adds a DELETE route
func (s *Server) Delete(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodDelete, path, handler)
}

// Patch adds a PATCH route
func (s *Server) Patch(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodPatch, path, handler)
}

// Options adds an OPTIONS route
func (s *Server) Options(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodOptions, path, handler)
}

// Head adds a HEAD route
func (s *Server) Head(path string, handler starlark.Callable) error {
	return s.addRoute(http.MethodHead, path, handler)
}

// Route adds a route with specific method(s).
// This method accepts either a single method string or a list of method strings,
// allowing the same handler to respond to multiple HTTP methods on the same path.
func (s *Server) Route(methods interface{}, path string, handler starlark.Callable) error {
	switch m := methods.(type) {
	case starlark.String:
		return s.addRoute(m.GoString(), path, handler)
	case *starlark.List:
		for i := 0; i < m.Len(); i++ {
			method, ok := m.Index(i).(starlark.String)
			if !ok {
				return fmt.Errorf("method must be a string")
			}
			if err := s.addRoute(string(method), path, handler); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("methods must be a string or list of strings")
	}
}

// addRoute adds a route to the gin engine
func (s *Server) addRoute(method, path string, handler starlark.Callable) error {
	// Convert {param} style to :param style for Gin
	ginPath := convertPathParams(path)
	ginHandler := s.wrapHandler(handler)

	switch method {
	case http.MethodGet:
		s.engine.GET(ginPath, ginHandler)
	case http.MethodPost:
		s.engine.POST(ginPath, ginHandler)
	case http.MethodPut:
		s.engine.PUT(ginPath, ginHandler)
	case http.MethodDelete:
		s.engine.DELETE(ginPath, ginHandler)
	case http.MethodPatch:
		s.engine.PATCH(ginPath, ginHandler)
	case http.MethodOptions:
		s.engine.OPTIONS(ginPath, ginHandler)
	case http.MethodHead:
		s.engine.HEAD(ginPath, ginHandler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// wrapHandler wraps a Starlark callable as a gin handler.
// This function creates a bridge between Gin's HTTP handling and Starlark functions,
// converting HTTP requests to Starlark objects and handling response conversion.
func (s *Server) wrapHandler(handler starlark.Callable) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check request body size if configured
		if s.maxBodySize > 0 && c.Request.ContentLength > s.maxBodySize {
			sendBadRequest(c, "Request body too large")
			return
		}

		// Create request object
		req := s.createRequest(c)

		// Convert to Starlark value using wrapper
		reqValue := &RequestWrapper{request: req}

		// Call the handler
		thread := &starlark.Thread{Name: "web_handler"}
		result, err := starlark.Call(thread, handler, starlark.Tuple{reqValue}, nil)
		if err != nil {
			sendInternalServerError(c, err.Error())
			return
		}

		// Handle ResponseWrapper
		if responseWrapper, ok := result.(*ResponseWrapper); ok {
			s.applyResponse(c, responseWrapper.response)
			return
		}

		// Fallback: try to convert using starlight
		responseInterface := convert.FromValue(result)
		if response, ok := responseInterface.(*Response); ok {
			// Convert old Response to Response
			httpResp := &Response{
				StatusCode: response.StatusCode,
				Headers:    response.Headers,
				Body:       response.Body,
				FilePath:   response.FilePath,
			}
			s.applyResponse(c, httpResp)
			return
		}

		sendInternalServerError(c, "Invalid response format")
	}
}

// createRequest creates a Request object from gin context
func (s *Server) createRequest(c *gin.Context) *Request {
	// Parse query parameters
	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Parse headers
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &Request{
		Method:   c.Request.Method,
		URL:      c.Request.URL.String(),
		Path:     c.Request.URL.Path,
		Host:     c.Request.Host,
		Remote:   c.Request.RemoteAddr,
		ClientIP: c.ClientIP(),
		Proto:    c.Request.Proto,
		Headers:  headers,
		Query:    queryParams,
		Context:  make(map[string]interface{}),
		ginCtx:   c,
	}
}

// applyResponse applies a Response object to gin context
func (s *Server) applyResponse(c *gin.Context, response *Response) {
	// Set headers
	for key, value := range response.Headers {
		c.Header(key, value)
	}

	// Handle file response
	if response.FilePath != "" {
		c.File(response.FilePath)
		return
	}

	// Handle regular response
	c.Data(response.StatusCode, c.GetHeader("Content-Type"), []byte(response.Body))
}

// Start starts the server in a goroutine.
// This method begins listening for HTTP requests on the configured host and port
// without blocking the current thread, allowing for asynchronous server operation.
func (s *Server) Start() error {
	if s.running {
		return fmt.Errorf("server is already running")
	}

	addr := s.host + ":" + strconv.Itoa(s.port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error or handle it appropriately
		}
	}()

	s.running = true

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	if !s.running {
		return fmt.Errorf("server is not running")
	}

	if s.httpServer == nil {
		return fmt.Errorf("server not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	s.running = false

	return err
}

// Run starts the server and blocks.
// This method starts the HTTP server and blocks the current thread until
// the server is stopped or encounters an error, suitable for simple applications.
func (s *Server) Run() error {
	if s.running {
		return fmt.Errorf("server is already running")
	}

	addr := s.host + ":" + strconv.Itoa(s.port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	s.running = true
	return s.httpServer.ListenAndServe()
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	return s.running
}

// Group creates a route group.
// This method creates a new route group with the specified prefix, allowing
// for organized route registration and middleware application to related endpoints.
func (s *Server) Group(prefix string) *RouteGroup {
	ginGroup := s.engine.Group(prefix)
	return &RouteGroup{
		server:   s,
		ginGroup: ginGroup,
		prefix:   prefix,
	}
}

// Starlark-compatible methods for Server (these delegate to the actual methods)

// StarlarkGet adds a GET route (Starlark-compatible wrapper)
func (s *Server) StarlarkGet(path string, handler starlark.Callable) error {
	return s.Get(path, handler)
}

// StarlarkPost adds a POST route (Starlark-compatible wrapper)
func (s *Server) StarlarkPost(path string, handler starlark.Callable) error {
	return s.Post(path, handler)
}

// StarlarkPut adds a PUT route (Starlark-compatible wrapper)
func (s *Server) StarlarkPut(path string, handler starlark.Callable) error {
	return s.Put(path, handler)
}

// StarlarkDelete adds a DELETE route (Starlark-compatible wrapper)
func (s *Server) StarlarkDelete(path string, handler starlark.Callable) error {
	return s.Delete(path, handler)
}

// StarlarkPatch adds a PATCH route (Starlark-compatible wrapper)
func (s *Server) StarlarkPatch(path string, handler starlark.Callable) error {
	return s.Patch(path, handler)
}

// StarlarkOptions adds an OPTIONS route (Starlark-compatible wrapper)
func (s *Server) StarlarkOptions(path string, handler starlark.Callable) error {
	return s.Options(path, handler)
}

// StarlarkHead adds a HEAD route (Starlark-compatible wrapper)
func (s *Server) StarlarkHead(path string, handler starlark.Callable) error {
	return s.Head(path, handler)
}

// StarlarkRoute adds a route with specific method(s) (Starlark-compatible wrapper)
func (s *Server) StarlarkRoute(methods interface{}, path string, handler starlark.Callable) error {
	return s.Route(methods, path, handler)
}

// StarlarkStart starts the server (Starlark-compatible wrapper)
func (s *Server) StarlarkStart() error {
	return s.Start()
}

// StarlarkStop stops the server (Starlark-compatible wrapper)
func (s *Server) StarlarkStop() error {
	return s.Stop()
}

// StarlarkRun starts the server and blocks (Starlark-compatible wrapper)
func (s *Server) StarlarkRun() error {
	return s.Run()
}

// StarlarkIsRunning returns whether the server is running (Starlark-compatible wrapper)
func (s *Server) StarlarkIsRunning() bool {
	return s.IsRunning()
}

// StarlarkGroup creates a route group (Starlark-compatible wrapper)
func (s *Server) StarlarkGroup(prefix string) *RouteGroup {
	return s.Group(prefix)
}

// ServerWrapper wraps the Server struct to provide Starlark-compatible method names
type ServerWrapper struct {
	server *Server
}

func (sw *ServerWrapper) String() string {
	return fmt.Sprintf("<web.Server host=%s port=%d>", sw.server.host, sw.server.port)
}

func (sw *ServerWrapper) Type() string {
	return "web.Server"
}

func (sw *ServerWrapper) Freeze() {
	// Server is immutable after creation
}

func (sw *ServerWrapper) Truth() starlark.Bool {
	return starlark.True
}

func (sw *ServerWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", sw.Type())
}

func (sw *ServerWrapper) Attr(name string) (starlark.Value, error) {
	switch name {
	case "get":
		return starlark.NewBuiltin("get", sw.get), nil
	case "post":
		return starlark.NewBuiltin("post", sw.post), nil
	case "put":
		return starlark.NewBuiltin("put", sw.put), nil
	case "delete":
		return starlark.NewBuiltin("delete", sw.delete), nil
	case "patch":
		return starlark.NewBuiltin("patch", sw.patch), nil
	case "options":
		return starlark.NewBuiltin("options", sw.options), nil
	case "head":
		return starlark.NewBuiltin("head", sw.head), nil
	case "route":
		return starlark.NewBuiltin("route", sw.route), nil
	case "start":
		return starlark.NewBuiltin("start", sw.start), nil
	case "stop":
		return starlark.NewBuiltin("stop", sw.stop), nil
	case "run":
		return starlark.NewBuiltin("run", sw.run), nil
	case "is_running":
		return starlark.NewBuiltin("is_running", sw.isRunning), nil
	case "group":
		return starlark.NewBuiltin("group", sw.group), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", sw.Type(), name))
	}
}

func (sw *ServerWrapper) AttrNames() []string {
	return []string{"get", "post", "put", "delete", "patch", "options", "head", "route", "start", "stop", "run", "is_running", "group"}
}

// Starlark builtin methods

func (sw *ServerWrapper) get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Get(path, handler)
}

func (sw *ServerWrapper) post(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Post(path, handler)
}

func (sw *ServerWrapper) put(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Put(path, handler)
}

func (sw *ServerWrapper) delete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Delete(path, handler)
}

func (sw *ServerWrapper) patch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Patch(path, handler)
}

func (sw *ServerWrapper) options(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Options(path, handler)
}

func (sw *ServerWrapper) head(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Head(path, handler)
}

func (sw *ServerWrapper) route(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var methods starlark.Value
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "methods", &methods, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, sw.server.Route(methods, path, handler)
}

func (sw *ServerWrapper) start(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, sw.server.Start()
}

func (sw *ServerWrapper) stop(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, sw.server.Stop()
}

func (sw *ServerWrapper) run(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, sw.server.Run()
}

func (sw *ServerWrapper) isRunning(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Bool(sw.server.IsRunning()), nil
}

func (sw *ServerWrapper) group(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var prefix string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "prefix", &prefix); err != nil {
		return nil, err
	}
	group := sw.server.Group(prefix)
	return &RouteGroupWrapper{group: group}, nil
}
