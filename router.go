package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Router handles HTTP routing
type Router struct {
	routes       map[string]*RouteTree
	staticRoutes map[string]*StaticRoute
	paramRegex   *regexp.Regexp
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

// HandlerFunc represents a route handler function
type HandlerFunc func(*Request) *Response

// MiddlewareFunc represents a middleware function
type MiddlewareFunc func(*Request, NextFunc) *Response

// NextFunc represents the next function in middleware chain
type NextFunc func(*Request) *Response

// NewRouter creates a new router instance
func NewRouter() *Router {
	return &Router{
		routes:       make(map[string]*RouteTree),
		staticRoutes: make(map[string]*StaticRoute),
		paramRegex:   regexp.MustCompile(`\{([^}]+)\}`),
	}
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
				Body:       fmt.Sprintf("Handler error: %v", err),
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
		return &Response{
			StatusCode: 404,
			Headers:    make(http.Header),
			Body:       "Not Found",
		}
	}

	handler, params := router.matchRoute(tree, path)
	if handler == nil {
		return &Response{
			StatusCode: 404,
			Headers:    make(http.Header),
			Body:       "Not Found",
		}
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
		return &Response{
			StatusCode: 403,
			Headers:    make(http.Header),
			Body:       "Forbidden",
		}
	}

	// Build full file path
	fullPath := filepath.Join(staticRoute.Directory, filePath)

	// Check if file exists
	if _, err := filepath.Abs(fullPath); err != nil {
		return &Response{
			StatusCode: 404,
			Headers:    make(http.Header),
			Body:       "Not Found",
		}
	}

	// Return file response
	return &Response{
		StatusCode: 200,
		Headers:    make(http.Header),
		FilePath:   fullPath,
	}
}
