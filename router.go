package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlight/convert"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Router handles HTTP routing
type Router struct {
	routes       map[string]*RouteTree
	staticRoutes map[string]*StaticRoute
	paramRegex   *regexp.Regexp
	server       *Server // Add reference to server for error handling
}

// RouteTree represents a route tree for efficient matching
type RouteTree struct {
	exact     map[string]HandlerFunc
	param     map[string]*RouteTree
	wildcard  HandlerFunc
	paramName string
}

// StaticRoute represents a static file route
type StaticRoute struct {
	URLPath   string
	Directory string
	Index     string
}

// SPARoute represents a Single Page Application route
type SPARoute struct {
	URLPath   string
	Directory string
	Fallback  string
}

// HandlerFunc represents a route handler function
type HandlerFunc func(*Request) *Response

// MiddlewareFunc represents a middleware function
type MiddlewareFunc func(*Request, NextFunc) *Response

// NextFunc represents the next function in middleware chain
type NextFunc func(*Request) *Response

// RouteGroup represents a group of routes with a common prefix
type RouteGroup struct {
	server *Server
	prefix string
}

// NewRouter creates a new router instance
func NewRouter() *Router {
	return &Router{
		routes:       make(map[string]*RouteTree),
		staticRoutes: make(map[string]*StaticRoute),
		paramRegex:   regexp.MustCompile(`\{([^}]+)\}`),
		server:       nil, // Will be set by server
	}
}

// SetServer sets the server reference for error handling
func (router *Router) SetServer(server *Server) {
	router.server = server
}

// AddRoute registers a new route
func (router *Router) AddRoute(method, path string, handler starlark.Callable) {
	// Get or create route tree for method
	tree, exists := router.routes[method]
	if !exists {
		tree = &RouteTree{
			exact: make(map[string]HandlerFunc),
			param: make(map[string]*RouteTree),
		}
		router.routes[method] = tree
	}

	// Wrap Starlark callable as HandlerFunc
	handlerFunc := func(req *Request) *Response {
		// Call the handler
		reqValue, err := convert.ToValue(req)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to convert request: %v", err),
			}
		}

		result, err := starlark.Call(&starlark.Thread{}, handler, starlark.Tuple{reqValue}, nil)
		if err != nil {
			return &Response{
				StatusCode: 500,
				Body:       fmt.Sprintf("Handler error: %v", err),
			}
		}

		// Convert result to Go value
		resp, err := ResponseFromStarlarkStruct(result)
		if err != nil {
			// If it's not a Response struct, try normal unmarshaling
			goValue, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Failed to unmarshal response: %v", err),
				}
			}

			// Check if the unmarshaled value is a Response
			if r, ok := goValue.(*Response); ok {
				resp = r
			} else {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       "Handler did not return a Response object",
				}
			}
		}

		return resp
	}

	// Add route to tree
	router.addToTree(tree, path, handlerFunc)
}

// AddStaticRoute registers a static file route
func (router *Router) AddStaticRoute(urlPath, directory, index string) {
	router.staticRoutes[urlPath] = &StaticRoute{
		URLPath:   urlPath,
		Directory: directory,
		Index:     index,
	}
}

// AddSPARoute registers a Single Page Application route
func (router *Router) AddSPARoute(urlPath, directory, fallback string) {
	// Add handler for the SPA route
	handlerFunc := func(req *Request) *Response {
		// Get path relative to the URL path
		path := req.Request.URL.Path

		// Strip the URL path prefix
		if strings.HasPrefix(path, urlPath) {
			path = strings.TrimPrefix(path, urlPath)
		}

		// Ensure path starts with /
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		// Try to serve the file
		filePath := filepath.Join(directory, path)
		if _, err := os.Stat(filePath); err == nil {
			// File exists, serve it
			return &Response{
				StatusCode: http.StatusOK,
				Headers:    make(http.Header),
				FilePath:   filePath,
			}
		}

		// File not found, serve the fallback
		fallbackPath := filepath.Join(directory, fallback)
		return &Response{
			StatusCode: http.StatusOK,
			Headers:    make(http.Header),
			FilePath:   fallbackPath,
		}
	}

	// Add a wildcard route for the SPA
	tree, exists := router.routes["GET"]
	if !exists {
		tree = &RouteTree{
			exact: make(map[string]HandlerFunc),
			param: make(map[string]*RouteTree),
		}
		router.routes["GET"] = tree
	}

	// Add the SPA handler for the URL path and all sub-paths
	spaPath := strings.TrimSuffix(urlPath, "/") + "/{*path}"
	router.addToTree(tree, spaPath, handlerFunc)
}

// addToTree adds a route to the route tree
func (router *Router) addToTree(tree *RouteTree, path string, handler HandlerFunc) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = []string{} // Root path
	}

	current := tree
	for i, segment := range segments {
		if segment == "" {
			continue
		}

		// Check for parameter
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			paramName := segment[1 : len(segment)-1]

			// Handle wildcard parameters (e.g., {*filepath})
			if strings.HasPrefix(paramName, "*") {
				if i == len(segments)-1 { // Must be last segment
					current.wildcard = handler
					return
				}
				panic("Wildcard parameter must be the last segment")
			}

			// Regular parameter
			if current.param[paramName] == nil {
				current.param[paramName] = &RouteTree{
					exact:     make(map[string]HandlerFunc),
					param:     make(map[string]*RouteTree),
					paramName: paramName,
				}
			}
			current = current.param[paramName]
		} else {
			// Exact match segment
			if current.param[segment] == nil {
				current.param[segment] = &RouteTree{
					exact: make(map[string]HandlerFunc),
					param: make(map[string]*RouteTree),
				}
			}
			current = current.param[segment]
		}
	}

	// Set handler at final node
	current.exact[""] = handler
}

// ServeHTTP implements the routing logic
func (router *Router) ServeHTTP(req *Request) *Response {
	method := req.Request.Method
	path := req.Request.URL.Path

	// Check static routes first
	for urlPath, staticRoute := range router.staticRoutes {
		if strings.HasPrefix(path, urlPath) {
			return router.serveStatic(req, staticRoute, path)
		}
	}

	// Check dynamic routes
	tree, exists := router.routes[method]
	if !exists {
		return router.createErrorResponse(req, 404, "Not Found")
	}

	handler, params := router.matchRoute(tree, path)
	if handler == nil {
		return router.createErrorResponse(req, 404, "Not Found")
	}

	// Set path parameters
	for name, value := range params {
		req.SetParam(name, value)
	}

	return handler(req)
}

// matchRoute finds a matching route and extracts parameters
func (router *Router) matchRoute(tree *RouteTree, path string) (HandlerFunc, map[string]string) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = []string{} // Root path
	}

	params := make(map[string]string)
	return router.matchSegments(tree, segments, 0, params)
}

// matchSegments recursively matches path segments
func (router *Router) matchSegments(tree *RouteTree, segments []string, index int, params map[string]string) (HandlerFunc, map[string]string) {
	// If we've consumed all segments, check for exact match
	if index >= len(segments) {
		if handler, exists := tree.exact[""]; exists {
			return handler, params
		}
		return nil, nil
	}

	segment := segments[index]

	// Try exact match first
	if nextTree, exists := tree.param[segment]; exists {
		if handler, foundParams := router.matchSegments(nextTree, segments, index+1, params); handler != nil {
			return handler, foundParams
		}
	}

	// Try parameter matches
	for paramName, nextTree := range tree.param {
		if nextTree.paramName != "" {
			// Regular parameter
			newParams := make(map[string]string)
			for k, v := range params {
				newParams[k] = v
			}
			newParams[paramName] = segment

			if handler, foundParams := router.matchSegments(nextTree, segments, index+1, newParams); handler != nil {
				return handler, foundParams
			}
		}
	}

	// Try wildcard match
	if tree.wildcard != nil {
		// Collect remaining segments for wildcard
		remaining := strings.Join(segments[index:], "/")
		if tree.paramName != "" {
			params[tree.paramName] = remaining
		}
		return tree.wildcard, params
	}

	return nil, nil
}

// serveStatic serves static files
func (router *Router) serveStatic(req *Request, staticRoute *StaticRoute, requestPath string) *Response {
	// Remove URL prefix to get file path
	filePath := strings.TrimPrefix(requestPath, staticRoute.URLPath)
	if filePath == "" || filePath == "/" {
		filePath = staticRoute.Index
	}

	// Prevent directory traversal
	if strings.Contains(filePath, "..") {
		return router.createErrorResponse(req, 403, "Forbidden")
	}

	// Build full file path
	fullPath := filepath.Join(staticRoute.Directory, filePath)

	// Check if file exists and is readable
	if stat, err := os.Stat(fullPath); err != nil {
		// File doesn't exist or can't be accessed
		return router.createErrorResponse(req, 404, "Not Found")
	} else if stat.IsDir() {
		// If it's a directory, try to serve index file
		indexPath := filepath.Join(fullPath, staticRoute.Index)
		if _, err := os.Stat(indexPath); err != nil {
			return router.createErrorResponse(req, 404, "Not Found")
		}
		fullPath = indexPath
	}

	// Return file response
	return &Response{
		StatusCode: 200,
		Headers:    make(http.Header),
		FilePath:   fullPath,
	}
}

// NewRouteGroup creates a new route group
func NewRouteGroup(server *Server, prefix string) *RouteGroup {
	// Ensure prefix starts with a / and doesn't end with one
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimSuffix(prefix, "/")

	return &RouteGroup{
		server: server,
		prefix: prefix,
	}
}

// Struct returns a Starlark struct representation of the RouteGroup
func (rg *RouteGroup) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"route":   starlark.NewBuiltin("route", rg.Route),
		"get":     starlark.NewBuiltin("get", rg.Get),
		"post":    starlark.NewBuiltin("post", rg.Post),
		"put":     starlark.NewBuiltin("put", rg.Put),
		"delete":  starlark.NewBuiltin("delete", rg.Delete),
		"patch":   starlark.NewBuiltin("patch", rg.Patch),
		"options": starlark.NewBuiltin("options", rg.Options),
		"head":    starlark.NewBuiltin("head", rg.Head),
		"use":     starlark.NewBuiltin("use", rg.Use),
		"static":  starlark.NewBuiltin("static", rg.Static),
	}
	return starlarkstruct.FromStringDict(starlark.String("RouteGroup"), sd)
}

// Route registers a route with the group
func (rg *RouteGroup) Route(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Route method
	_, err := rg.server.Route(thread, b, starlark.Tuple{method, fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Get registers a GET route
func (rg *RouteGroup) Get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Get method
	_, err := rg.server.Get(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Post registers a POST route
func (rg *RouteGroup) Post(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Post method
	_, err := rg.server.Post(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Put registers a PUT route
func (rg *RouteGroup) Put(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Put method
	_, err := rg.server.Put(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Delete registers a DELETE route
func (rg *RouteGroup) Delete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Delete method
	_, err := rg.server.Delete(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Patch registers a PATCH route
func (rg *RouteGroup) Patch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Patch method
	_, err := rg.server.Patch(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Options registers an OPTIONS route
func (rg *RouteGroup) Options(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Options method
	_, err := rg.server.Options(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Head registers a HEAD route
func (rg *RouteGroup) Head(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(path.GoString(), "/"))

	// Call the server's Head method
	_, err := rg.server.Head(thread, b, starlark.Tuple{fullPath, handler}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Use adds middleware to the route group
func (rg *RouteGroup) Use(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var middlewareFunc starlark.Callable

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "middleware", &middlewareFunc); err != nil {
		return starlark.None, err
	}

	// Create path pattern for this group
	pattern := rg.prefix + "/*"

	// Call server's UseFor method
	pathPattern := starlark.String(pattern)
	_, err := rg.server.UseFor(thread, b, starlark.Tuple{pathPattern, middlewareFunc}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Static adds a static file route
func (rg *RouteGroup) Static(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	// Prepend prefix to path
	fullPath := starlark.String(rg.prefix + "/" + strings.TrimPrefix(urlPath.GoString(), "/"))

	// Call server's Static method
	_, err := rg.server.Static(thread, b, starlark.Tuple{fullPath, directory, index}, nil)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// createErrorResponse creates an error response, using custom error handlers if available
func (router *Router) createErrorResponse(req *Request, statusCode int, defaultMessage string) *Response {
	// If we have a server reference and it has custom error handlers, use them
	if router.server != nil {
		if errorHandler, exists := router.server.errorHandlers[statusCode]; exists {
			return errorHandler(req)
		}
	}

	// Fall back to default error response
	return &Response{
		StatusCode: statusCode,
		Headers:    make(http.Header),
		Body:       defaultMessage,
	}
}
