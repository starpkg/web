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

// Get adds a GET route
func (s *Server) Get(path string, handler starlark.Callable) error {
	return s.addRoute("GET", path, handler)
}

// Post adds a POST route
func (s *Server) Post(path string, handler starlark.Callable) error {
	return s.addRoute("POST", path, handler)
}

// Put adds a PUT route
func (s *Server) Put(path string, handler starlark.Callable) error {
	return s.addRoute("PUT", path, handler)
}

// Delete adds a DELETE route
func (s *Server) Delete(path string, handler starlark.Callable) error {
	return s.addRoute("DELETE", path, handler)
}

// Patch adds a PATCH route
func (s *Server) Patch(path string, handler starlark.Callable) error {
	return s.addRoute("PATCH", path, handler)
}

// Options adds an OPTIONS route
func (s *Server) Options(path string, handler starlark.Callable) error {
	return s.addRoute("OPTIONS", path, handler)
}

// Head adds a HEAD route
func (s *Server) Head(path string, handler starlark.Callable) error {
	return s.addRoute("HEAD", path, handler)
}

// Route adds a route with specific method(s)
func (s *Server) Route(methods interface{}, path string, handler starlark.Callable) error {
	switch m := methods.(type) {
	case string:
		return s.addRoute(m, path, handler)
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
	ginHandler := s.wrapHandler(handler)

	switch method {
	case "GET":
		s.engine.GET(path, ginHandler)
	case "POST":
		s.engine.POST(path, ginHandler)
	case "PUT":
		s.engine.PUT(path, ginHandler)
	case "DELETE":
		s.engine.DELETE(path, ginHandler)
	case "PATCH":
		s.engine.PATCH(path, ginHandler)
	case "OPTIONS":
		s.engine.OPTIONS(path, ginHandler)
	case "HEAD":
		s.engine.HEAD(path, ginHandler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// wrapHandler wraps a Starlark callable as a gin handler
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
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Convert result back to Go
		responseInterface := convert.FromValue(result)
		response, ok := responseInterface.(*Response)
		if !ok {
			c.JSON(500, gin.H{"error": "Invalid response format"})
			return
		}

		// Apply the response
		s.applyResponse(c, response)
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

// Start starts the server in a goroutine
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

// Run starts the server and blocks
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

// Group creates a route group
func (s *Server) Group(prefix string) *RouteGroup {
	ginGroup := s.engine.Group(prefix)
	return &RouteGroup{
		server:   s,
		ginGroup: ginGroup,
		prefix:   prefix,
	}
}

// RouteGroup represents a group of routes with a common prefix
type RouteGroup struct {
	server   *Server
	ginGroup *gin.RouterGroup
	prefix   string
}

// Get adds a GET route to the group
func (rg *RouteGroup) Get(path string, handler starlark.Callable) error {
	return rg.addRoute("GET", path, handler)
}

// Post adds a POST route to the group
func (rg *RouteGroup) Post(path string, handler starlark.Callable) error {
	return rg.addRoute("POST", path, handler)
}

// Put adds a PUT route to the group
func (rg *RouteGroup) Put(path string, handler starlark.Callable) error {
	return rg.addRoute("PUT", path, handler)
}

// Delete adds a DELETE route to the group
func (rg *RouteGroup) Delete(path string, handler starlark.Callable) error {
	return rg.addRoute("DELETE", path, handler)
}

// Patch adds a PATCH route to the group
func (rg *RouteGroup) Patch(path string, handler starlark.Callable) error {
	return rg.addRoute("PATCH", path, handler)
}

// Options adds an OPTIONS route to the group
func (rg *RouteGroup) Options(path string, handler starlark.Callable) error {
	return rg.addRoute("OPTIONS", path, handler)
}

// Head adds a HEAD route to the group
func (rg *RouteGroup) Head(path string, handler starlark.Callable) error {
	return rg.addRoute("HEAD", path, handler)
}

// addRoute adds a route to the gin group
func (rg *RouteGroup) addRoute(method, path string, handler starlark.Callable) error {
	ginHandler := rg.server.wrapHandler(handler)

	switch method {
	case "GET":
		rg.ginGroup.GET(path, ginHandler)
	case "POST":
		rg.ginGroup.POST(path, ginHandler)
	case "PUT":
		rg.ginGroup.PUT(path, ginHandler)
	case "DELETE":
		rg.ginGroup.DELETE(path, ginHandler)
	case "PATCH":
		rg.ginGroup.PATCH(path, ginHandler)
	case "OPTIONS":
		rg.ginGroup.OPTIONS(path, ginHandler)
	case "HEAD":
		rg.ginGroup.HEAD(path, ginHandler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}
