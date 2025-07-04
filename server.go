package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ServerConfig holds the configuration for the HTTP server
type ServerConfig struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	MaxBodySize       int64
	EnableCORS        bool
	CORSOrigins       []string
	EnableCompression bool
	StaticCacheMaxAge int
}

// Server represents an HTTP server instance
type Server struct {
	config         *ServerConfig
	httpServer     *http.Server
	router         *Router
	middleware     []MiddlewareFunc
	pathMiddleware map[string][]MiddlewareFunc // Added for path-specific middleware
	errorHandlers  map[int]HandlerFunc         // Added for custom error handlers
	sessionManager *SessionManager
	running        bool
	mu             sync.RWMutex
}

// NewServer creates a new HTTP server instance
func NewServer(config *ServerConfig) *Server {
	server := &Server{
		config:         config,
		router:         NewRouter(),
		middleware:     make([]MiddlewareFunc, 0),
		pathMiddleware: make(map[string][]MiddlewareFunc),
		errorHandlers:  make(map[int]HandlerFunc),
	}

	// Set the router's server reference for error handling
	server.router.SetServer(server)

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      server,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return server
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create request wrapper
	req := NewRequest(r)

	// Build middleware chain starting with the router
	handler := s.router.ServeHTTP

	// Apply path-specific middleware if any matches
	path := r.URL.Path
	for pattern, middlewares := range s.pathMiddleware {
		// Create pattern matcher
		pathPattern, err := newPathPattern(pattern)
		if err == nil && pathPattern.matches(path) {
			// Apply middleware in reverse order (so they execute in the correct order)
			for i := len(middlewares) - 1; i >= 0; i-- {
				// Capture by value to fix closure issue
				middleware := middlewares[i]
				currentHandler := handler
				handler = func(middleware MiddlewareFunc, next func(*Request) *Response) func(*Request) *Response {
					return func(req *Request) *Response {
						return middleware(req, next)
					}
				}(middleware, currentHandler)
			}
		}
	}

	// Apply global middleware in reverse order
	for i := len(s.middleware) - 1; i >= 0; i-- {
		// Capture by value to fix closure issue
		middleware := s.middleware[i]
		currentHandler := handler
		handler = func(middleware MiddlewareFunc, next func(*Request) *Response) func(*Request) *Response {
			return func(req *Request) *Response {
				return middleware(req, next)
			}
		}(middleware, currentHandler)
	}

	// Create a wrapper that can handle error responses
	errorAwareHandler := func(req *Request) *Response {
		response := handler(req)

		// Check if we need to apply custom error handler
		if errorHandler, exists := s.errorHandlers[response.StatusCode]; exists {
			return errorHandler(req)
		}

		return response
	}

	// Execute the middleware chain with error handling
	response := errorAwareHandler(req)

	// Apply session cookies if session exists in context
	if sessionInterface := req.GetContext("session"); sessionInterface != nil {
		if session, ok := sessionInterface.(*Session); ok {
			session.applySessionCookie(response)
		}
	}

	// Write response
	s.writeResponse(w, response)
}

// writeResponse writes a Response to the HTTP response writer
func (s *Server) writeResponse(w http.ResponseWriter, resp *Response) {
	// Set headers
	for name, values := range resp.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Write body
	if resp.JSONData != nil {
		// Handle JSON response - first convert to starlark.Value, then to JSON
		starlarkValue, err := dataconv.Marshal(resp.JSONData)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		jsonStr, err := dataconv.MarshalStarlarkJSON(starlarkValue, 0)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(jsonStr))
	} else if resp.FilePath != "" {
		// Handle file response - create a proper request for ServeFile
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: resp.FilePath},
		}
		http.ServeFile(w, req, resp.FilePath)
	} else {
		// Handle regular body
		w.Write([]byte(resp.Body))
	}
}

// Starlark methods that will be exposed

// Route registers a route with the server
func (s *Server) Route(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		method  starlark.Value
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"method", &method,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	// Handle both string and list of strings for method
	var methods []string
	if methodStr, ok := method.(starlark.String); ok {
		methods = []string{methodStr.GoString()}
	} else if methodList, ok := method.(*starlark.List); ok {
		methods = make([]string, methodList.Len())
		for i := 0; i < methodList.Len(); i++ {
			if methodItem, ok := methodList.Index(i).(starlark.String); ok {
				methods[i] = methodItem.GoString()
			} else {
				return starlark.None, fmt.Errorf("method list must contain strings")
			}
		}
	} else {
		return starlark.None, fmt.Errorf("method must be a string or list of strings")
	}

	// Register route for each method
	for _, m := range methods {
		s.router.AddRoute(strings.ToUpper(m), path.GoString(), handler)
	}

	return starlark.None, nil
}

// Get registers a GET route
func (s *Server) Get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("GET", path.GoString(), handler)
	return starlark.None, nil
}

// Post registers a POST route
func (s *Server) Post(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("POST", path.GoString(), handler)
	return starlark.None, nil
}

// Put registers a PUT route
func (s *Server) Put(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("PUT", path.GoString(), handler)
	return starlark.None, nil
}

// Delete registers a DELETE route
func (s *Server) Delete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("DELETE", path.GoString(), handler)
	return starlark.None, nil
}

// Patch registers a PATCH route
func (s *Server) Patch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("PATCH", path.GoString(), handler)
	return starlark.None, nil
}

// Options registers an OPTIONS route
func (s *Server) Options(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("OPTIONS", path.GoString(), handler)
	return starlark.None, nil
}

// Head registers a HEAD route
func (s *Server) Head(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path    starlark.String
		handler starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddRoute("HEAD", path.GoString(), handler)
	return starlark.None, nil
}

// Use adds global middleware
func (s *Server) Use(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var middlewareFunc starlark.Callable

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"middleware", &middlewareFunc,
	); err != nil {
		return starlark.None, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Wrap Starlark callable as middleware
	middleware := func(req *Request, next NextFunc) *Response {
		// Create next handler wrapper
		nextHandler := starlark.NewBuiltin("next", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var reqArg starlark.Value
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "request", &reqArg); err != nil {
				return starlark.None, err
			}
			resp := next(req)
			result, err := dataconv.Marshal(resp)
			if err != nil {
				return starlark.None, fmt.Errorf("failed to marshal response: %v", err)
			}
			return result, nil
		})

		// Call middleware function
		reqValue, err := dataconv.Marshal(req)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to marshal request: %v", err),
			}
		}

		result, err := starlark.Call(thread, middlewareFunc, starlark.Tuple{reqValue, nextHandler}, nil)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Middleware error: %v", err),
			}
		}

		// Convert result back to Response
		goValue, err := dataconv.Unmarshal(result)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to unmarshal middleware response: %v", err),
			}
		}

		if resp, ok := goValue.(*Response); ok {
			return resp
		}

		return &Response{
			StatusCode: 500,
			Body:       "Invalid middleware response",
		}
	}

	s.middleware = append(s.middleware, middleware)
	return starlark.None, nil
}

// Static serves static files
func (s *Server) Static(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		urlPath   starlark.String
		directory starlark.String
		index     = starlark.String("index.html")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url_path", &urlPath,
		"directory", &directory,
		"index?", &index,
	); err != nil {
		return starlark.None, err
	}

	s.router.AddStaticRoute(urlPath.GoString(), directory.GoString(), index.GoString())
	return starlark.None, nil
}

// Run starts the server and blocks
func (s *Server) Run(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return starlark.None, fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	fmt.Printf("Starting server on %s:%d\n", s.config.Host, s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return starlark.None, fmt.Errorf("server error: %v", err)
	}

	return starlark.None, nil
}

// StartBackground starts the server in the background
func (s *Server) StartBackground(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return starlark.None, fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return starlark.None, nil
}

// Stop stops the server
func (s *Server) Stop(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if !running {
		return starlark.None, fmt.Errorf("server is not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return starlark.None, fmt.Errorf("server shutdown error: %v", err)
	}

	return starlark.None, nil
}

// IsRunning checks if the server is running
func (s *Server) IsRunning(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()
	return starlark.Bool(running), nil
}

// UseFor registers middleware for specific path patterns
func (s *Server) UseFor(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pathPattern    starlark.String
		middlewareFunc starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path_pattern", &pathPattern,
		"middleware", &middlewareFunc,
	); err != nil {
		return starlark.None, err
	}

	// Convert Starlark middleware to Go middleware function
	middleware := wrapStarlarkMiddleware(middlewareFunc)

	// Add middleware to path-specific middleware map
	pattern := pathPattern.GoString()
	s.mu.Lock()
	if _, exists := s.pathMiddleware[pattern]; !exists {
		s.pathMiddleware[pattern] = []MiddlewareFunc{middleware}
	} else {
		s.pathMiddleware[pattern] = append(s.pathMiddleware[pattern], middleware)
	}
	s.mu.Unlock()

	return starlark.None, nil
}

// Group creates a route group with a common prefix
func (s *Server) Group(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var prefix starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "prefix", &prefix); err != nil {
		return starlark.None, err
	}

	// Create a new RouteGroup
	group := NewRouteGroup(s, prefix.GoString())

	// Marshal to Starlark
	result, err := dataconv.Marshal(group)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal route group: %v", err)
	}

	return result, nil
}

// SPA configures a Single Page Application route
func (s *Server) SPA(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		urlPath   starlark.String
		directory starlark.String
		fallback  = starlark.String("index.html")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url_path", &urlPath,
		"directory", &directory,
		"fallback?", &fallback,
	); err != nil {
		return starlark.None, err
	}

	// Add SPA route to router
	s.router.AddSPARoute(urlPath.GoString(), directory.GoString(), fallback.GoString())

	return starlark.None, nil
}

// ErrorHandler registers custom error handlers for specific status codes
func (s *Server) ErrorHandler(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		statusCodes starlark.Value
		handler     starlark.Callable
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"status_codes", &statusCodes,
		"handler", &handler,
	); err != nil {
		return starlark.None, err
	}

	// Wrap the handler
	handlerFunc := func(req *Request) *Response {
		// Call the handler
		reqValue, err := dataconv.Marshal(req)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to marshal request: %v", err),
			}
		}

		result, err := starlark.Call(&starlark.Thread{}, handler, starlark.Tuple{reqValue}, nil)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Error handler error: %v", err),
			}
		}

		// Convert result back to Response
		goValue, err := dataconv.Unmarshal(result)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to unmarshal response: %v", err),
			}
		}

		if resp, ok := goValue.(*Response); ok {
			return resp
		}

		return &Response{
			StatusCode: 500,
			Headers:    make(http.Header),
			Body:       "Invalid handler response",
		}
	}

	// Register handler for each status code
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle both single int and list of ints
	if statusInt, ok := statusCodes.(starlark.Int); ok {
		code, ok := statusInt.Int64()
		if !ok {
			return starlark.None, fmt.Errorf("invalid status code")
		}
		s.errorHandlers[int(code)] = handlerFunc
	} else if statusList, ok := statusCodes.(*starlark.List); ok {
		for i := 0; i < statusList.Len(); i++ {
			if codeVal, ok := statusList.Index(i).(starlark.Int); ok {
				if code, ok := codeVal.Int64(); ok {
					s.errorHandlers[int(code)] = handlerFunc
				} else {
					return starlark.None, fmt.Errorf("invalid status code in list at index %d", i)
				}
			} else {
				return starlark.None, fmt.Errorf("status code at index %d is not an integer", i)
			}
		}
	} else {
		return starlark.None, fmt.Errorf("status_codes must be an int or list of ints")
	}

	return starlark.None, nil
}

// Struct returns a Starlark struct representation of the Server
func (s *Server) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"route":            starlark.NewBuiltin("route", s.Route),
		"get":              starlark.NewBuiltin("get", s.Get),
		"post":             starlark.NewBuiltin("post", s.Post),
		"put":              starlark.NewBuiltin("put", s.Put),
		"delete":           starlark.NewBuiltin("delete", s.Delete),
		"patch":            starlark.NewBuiltin("patch", s.Patch),
		"options":          starlark.NewBuiltin("options", s.Options),
		"head":             starlark.NewBuiltin("head", s.Head),
		"use":              starlark.NewBuiltin("use", s.Use),
		"use_for":          starlark.NewBuiltin("use_for", s.UseFor),
		"group":            starlark.NewBuiltin("group", s.Group),
		"static":           starlark.NewBuiltin("static", s.Static),
		"spa":              starlark.NewBuiltin("spa", s.SPA),
		"error_handler":    starlark.NewBuiltin("error_handler", s.ErrorHandler),
		"run":              starlark.NewBuiltin("run", s.Run),
		"start_background": starlark.NewBuiltin("start_background", s.StartBackground),
		"stop":             starlark.NewBuiltin("stop", s.Stop),
		"is_running":       starlark.NewBuiltin("is_running", s.IsRunning),
	}
	return starlarkstruct.FromStringDict(starlark.String("Server"), sd)
}
