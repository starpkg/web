package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/1set/starlight/convert"
	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

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
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
