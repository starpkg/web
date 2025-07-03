package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
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
	sessionManager *SessionManager
	running        atomic.Bool
	mu             sync.RWMutex
}

// NewServer creates a new HTTP server instance
func NewServer(config *ServerConfig) *Server {
	server := &Server{
		config:     config,
		router:     NewRouter(),
		middleware: make([]MiddlewareFunc, 0),
	}

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

	// Build middleware chain
	handler := s.router.ServeHTTP

	// Apply middleware in reverse order
	for i := len(s.middleware) - 1; i >= 0; i-- {
		middleware := s.middleware[i]
		next := handler
		handler = func(req *Request) *Response {
			return middleware(req, next)
		}
	}

	// Execute the middleware chain
	response := handler(req)

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
		// Handle JSON response
		jsonBytes, err := dataconv.Marshal(resp.JSONData)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if jsonStr, ok := jsonBytes.(starlark.String); ok {
			w.Write([]byte(jsonStr.GoString()))
		} else {
			w.Write([]byte(fmt.Sprintf("%v", jsonBytes)))
		}
	} else if resp.FilePath != "" {
		// Handle file response
		http.ServeFile(w, &http.Request{Method: "GET", URL: &http.URL{Path: resp.FilePath}}, resp.FilePath)
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
			return dataconv.WrapGoValue(next(req)), nil
		})

		// Call middleware function
		result, err := starlark.Call(thread, middlewareFunc, starlark.Tuple{dataconv.WrapGoValue(req), nextHandler}, nil)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Middleware error: %v", err),
			}
		}

		// Convert result back to Response
		if response, ok := result.(*dataconv.GoValue); ok {
			if resp, ok := response.GoValue().(*Response); ok {
				return resp
			}
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
	if s.running.Load() {
		return starlark.None, fmt.Errorf("server is already running")
	}

	s.running.Store(true)
	defer s.running.Store(false)

	fmt.Printf("Starting server on %s:%d\n", s.config.Host, s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return starlark.None, fmt.Errorf("server error: %v", err)
	}

	return starlark.None, nil
}

// StartBackground starts the server in the background
func (s *Server) StartBackground(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if s.running.Load() {
		return starlark.None, fmt.Errorf("server is already running")
	}

	s.running.Store(true)

	go func() {
		defer s.running.Store(false)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return starlark.None, nil
}

// Stop stops the server
func (s *Server) Stop(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !s.running.Load() {
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
	return starlark.Bool(s.running.Load()), nil
}
