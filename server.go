package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/1set/starlight/convert"
	"github.com/gin-gonic/gin"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// Server represents an HTTP server instance
type Server struct {
	host               string
	port               int
	engine             *gin.Engine
	httpServer         *http.Server
	running            bool
	module             *Module // Reference to module for config access
	readTimeout        time.Duration
	writeTimeout       time.Duration
	maxBodySize        int64
	enableCORS         bool
	serverHeader       string
	middleware         []*MiddlewareWrapper
	errorHandlers      *ErrorHandlerRegistry
	ginMiddlewareAdded bool         // Flag to prevent multiple Gin middleware additions
	mu                 sync.RWMutex // Protects running, httpServer fields
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
	serverHeader := module.ext.GetString(configKeyServerHeader)

	// Set gin mode - ensure release mode by default to avoid debug output
	if !debugMode {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create gin engine
	engine := gin.New()
	engine.Use(gin.Recovery())
	if debugMode {
		engine.Use(gin.Logger())
	}

	// Configure method not allowed handler
	engine.HandleMethodNotAllowed = true

	// Create server instance first
	server := &Server{
		host:               host,
		port:               port,
		engine:             engine,
		httpServer:         nil,
		running:            false,
		module:             module,
		readTimeout:        readTimeout,
		writeTimeout:       writeTimeout,
		maxBodySize:        maxBodySize,
		enableCORS:         enableCORS,
		serverHeader:       serverHeader,
		middleware:         make([]*MiddlewareWrapper, 0),
		errorHandlers:      NewErrorHandlerRegistry(),
		ginMiddlewareAdded: false,
	}

	// Now configure NoRoute and NoMethod handlers with access to server
	engine.NoMethod(func(c *gin.Context) {
		// Use custom 405 error handler if registered
		if customHandler := server.errorHandlers.GetHandler(405); customHandler != nil {
			req := createRequestFromGin(c)
			response := server.errorHandlers.HandleError(405, req)
			server.applyResponse(c, response)
		} else {
			sendMethodNotAllowed(c, "Method not allowed")
		}
	})

	engine.NoRoute(func(c *gin.Context) {
		// Use custom 404 error handler if registered
		if customHandler := server.errorHandlers.GetHandler(404); customHandler != nil {
			req := createRequestFromGin(c)
			response := server.errorHandlers.HandleError(404, req)
			server.applyResponse(c, response)
		} else {
			sendNotFound(c, "Not found")
		}
	})

	// Add CORS middleware if enabled
	if enableCORS {
		// Use shared CORS middleware implementation with default settings
		corsHandler := corsMiddleware(
			[]string{"*"}, // origins
			[]string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}, // methods
			[]string{HeaderContentType, HeaderAuthorization},                     // headers
			false, // credentials
		)

		// Convert to Gin middleware
		engine.Use(func(c *gin.Context) {
			// Create request from Gin context
			req := createRequestFromGin(c)

			// Create next function that continues with Gin processing
			next := func(req *Request) *Response {
				// Continue with normal Gin processing
				c.Next()

				// If response was already written (e.g., by another handler), skip
				if c.Writer.Written() {
					return &Response{
						StatusCode: c.Writer.Status(),
						Headers:    make(map[string]string),
						Body:       "",
					}
				}

				// Default response (should not be reached in normal flow)
				return &Response{
					StatusCode: 200,
					Headers:    make(map[string]string),
					Body:       "",
				}
			}

			// Execute CORS middleware
			response := corsHandler(req, next)

			// If CORS middleware returned a response (e.g., for OPTIONS), apply it
			if response.StatusCode != 200 || len(response.Headers) > 0 || response.Body != "" {
				// Apply CORS headers
				for key, value := range response.Headers {
					c.Header(key, value)
				}

				// If it's an OPTIONS request, respond immediately
				if c.Request.Method == "OPTIONS" {
					c.AbortWithStatus(response.StatusCode)
					return
				}
			}
		})
	}

	return server
}

// applyMiddlewareToGin applies custom middleware to the Gin engine
// This is called when middleware is added to handle all requests globally
func (s *Server) applyMiddlewareToGin() {
	// Only add the global middleware once
	if s.ginMiddlewareAdded {
		return
	}
	s.ginMiddlewareAdded = true

	// Add a global middleware to handle all requests with custom middleware
	s.engine.Use(func(c *gin.Context) {
		// Only apply if we have custom middleware
		if len(s.middleware) > 0 {
			// Store original gin context for route handlers
			c.Set("web_server", s)

			// For OPTIONS requests without specific routes, handle with middleware
			if c.Request.Method == http.MethodOptions && !s.hasOptionsRoute(c.Request.URL.Path) {
				// Create a dummy handler that returns 405 by default
				dummyHandler := func(req *Request) *Response {
					return &Response{
						StatusCode: 405,
						Headers:    map[string]string{"Content-Type": "text/plain"},
						Body:       "Method not allowed",
					}
				}

				// Create request object
				req := s.createRequest(c)

				// Execute middleware chain for OPTIONS requests
				var response *Response
				next := dummyHandler
				for i := len(s.middleware) - 1; i >= 0; i-- {
					middleware := s.middleware[i]
					currentNext := next
					next = func(req *Request) *Response {
						return middleware.Execute(req, currentNext)
					}
				}
				response = next(req)

				// Add custom server header
				if s.serverHeader != "" {
					if response.Headers == nil {
						response.Headers = make(map[string]string)
					}
					response.Headers["Server"] = s.serverHeader
				}

				// Apply response and abort further processing
				s.applyResponse(c, response)
				c.Abort()
				return
			}
		}

		// Continue with normal processing
		c.Next()
	})
}

// hasOptionsRoute checks if there's a specific OPTIONS route for the given path
func (s *Server) hasOptionsRoute(path string) bool {
	// This is a simplified check - in a real implementation, you'd check the Gin router
	// For now, we'll assume no specific OPTIONS routes are registered
	return false
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

		// Create the final handler function
		finalHandler := func(req *Request) *Response {
			// Convert to Starlark value using wrapper
			reqValue := &RequestWrapper{request: req}

			// Call the handler
			thread := &starlark.Thread{Name: "web_handler"}
			result, err := starlark.Call(thread, handler, starlark.Tuple{reqValue}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: fmt.Sprintf(`{"error":"Handler error: %s"}`, err.Error()),
				}
			}

			// Handle ResponseWrapper
			if responseWrapper, ok := result.(*ResponseWrapper); ok {
				return responseWrapper.response
			}

			// Fallback: try to convert using starlight
			responseInterface := convert.FromValue(result)
			if response, ok := responseInterface.(*Response); ok {
				return response
			}

			return &Response{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"error":"Invalid response format"}`,
			}
		}

		// Execute middleware chain
		var response *Response
		if len(s.middleware) > 0 {
			// Build middleware chain
			next := finalHandler
			for i := len(s.middleware) - 1; i >= 0; i-- {
				mw := s.middleware[i]
				currentNext := next
				next = func(req *Request) *Response {
					return mw.Execute(req, currentNext)
				}
			}
			response = next(req)
		} else {
			response = finalHandler(req)
		}

		// Add custom server header
		if s.serverHeader != "" {
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers["Server"] = s.serverHeader
		}

		s.applyResponse(c, response)
	}
}

// createRequest creates a Request object from gin context
func (s *Server) createRequest(c *gin.Context) *Request {
	return createRequestFromGin(c)
}

// applyResponse applies a Response object to gin context
func (s *Server) applyResponse(c *gin.Context, response *Response) {
	// Check if this is an error response and if we have a custom error handler
	if response.StatusCode >= 400 {
		if customHandler := s.errorHandlers.GetHandler(response.StatusCode); customHandler != nil {
			// Create request from gin context
			req := createRequestFromGin(c)
			// Use custom error handler
			customResponse := s.errorHandlers.HandleError(response.StatusCode, req)

			// Apply the custom response
			for key, value := range customResponse.Headers {
				c.Header(key, value)
			}

			// Handle file response
			if customResponse.FilePath != "" {
				c.File(customResponse.FilePath)
				return
			}

			// Get content type from response headers
			contentType := customResponse.Headers[canonicalHeader(HeaderContentType)]
			if contentType == "" {
				contentType = MIMEApplicationJSON
			}

			// Handle regular response
			c.Data(customResponse.StatusCode, contentType, []byte(customResponse.Body))
			return
		}
	}

	// Apply original response if no custom error handler
	// Set headers
	for key, value := range response.Headers {
		c.Header(key, value)
	}

	// Handle file response
	if response.FilePath != "" {
		c.File(response.FilePath)
		return
	}

	// Get content type from response headers
	contentType := response.Headers[canonicalHeader(HeaderContentType)]
	if contentType == "" {
		contentType = MIMEApplicationJSON
	}

	// Handle regular response
	c.Data(response.StatusCode, contentType, []byte(response.Body))
}

// Start starts the server in a goroutine.
// This method begins listening for HTTP requests on the configured host and port
// without blocking the current thread, allowing for asynchronous server operation.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
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
	httpServer := s.httpServer // Get a local copy before unlocking
	s.mu.Unlock()

	return httpServer.ListenAndServe()
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	case "use":
		return starlark.NewBuiltin("use", sw.use), nil
	case "use_for":
		return starlark.NewBuiltin("use_for", sw.useFor), nil
	case "error_handler":
		return starlark.NewBuiltin("error_handler", sw.errorHandler), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", sw.Type(), name))
	}
}

func (sw *ServerWrapper) AttrNames() []string {
	return []string{"get", "post", "put", "delete", "patch", "options", "head", "route", "start", "stop", "run", "is_running", "group", "use", "use_for", "error_handler"}
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

// use handles the use() method call for adding global middleware.
func (sw *ServerWrapper) use(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var middleware starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "middleware", &middleware); err != nil {
		return starlark.None, err
	}

	if mw, ok := middleware.(*MiddlewareWrapper); ok {
		sw.server.middleware = append(sw.server.middleware, mw)
	} else if callable, ok := middleware.(starlark.Callable); ok {
		// Convert Starlark function to middleware
		starlarkMW := createStarlarkMiddleware(callable)
		mwWrapper := NewMiddlewareWrapper(starlarkMW)
		sw.server.middleware = append(sw.server.middleware, mwWrapper)
	} else {
		return starlark.None, fmt.Errorf("middleware must be a middleware object or callable")
	}

	// Apply middleware to Gin engine for OPTIONS requests
	sw.server.applyMiddlewareToGin()

	return starlark.None, nil
}

// useFor handles the use_for() method call for adding path-specific middleware.
func (sw *ServerWrapper) useFor(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathPattern string
	var middleware starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path_pattern", &pathPattern, "middleware", &middleware); err != nil {
		return starlark.None, err
	}

	// TODO: Implement path-specific middleware
	// For now, just add to global middleware
	return sw.use(thread, b, starlark.Tuple{middleware}, kwargs)
}

// errorHandler handles the error_handler() method call for registering custom error handlers.
func (sw *ServerWrapper) errorHandler(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var statusCodes starlark.Value
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "status_codes", &statusCodes, "handler", &handler); err != nil {
		return starlark.None, err
	}

	var codes []int
	switch sc := statusCodes.(type) {
	case starlark.Int:
		if code, ok := sc.Int64(); ok {
			codes = []int{int(code)}
		}
	case *starlark.List:
		for i := 0; i < sc.Len(); i++ {
			if codeInt, ok := sc.Index(i).(starlark.Int); ok {
				if code, ok := codeInt.Int64(); ok {
					codes = append(codes, int(code))
				}
			}
		}
	default:
		return starlark.None, fmt.Errorf("status_codes must be an int or list of ints")
	}

	sw.server.errorHandlers.RegisterHandler(codes, handler)
	return starlark.None, nil
}
