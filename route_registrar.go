package web

import (
	"strings"

	"go.starlark.net/starlark"
)

// RouteRegistrar defines the interface for route registration functionality
// This interface is implemented by both Server and RouteGroup to provide
// a consistent API for registering routes with different HTTP methods
type RouteRegistrar interface {
	// registerRoute is the core method that handles route registration
	// It takes the HTTP method, path, and handler function
	registerRoute(method, path string, handler starlark.Callable) error
}

// HTTPMethod represents the supported HTTP methods for route registration
type HTTPMethod string

// Supported HTTP methods as constants for better type safety and consistency
const (
	MethodGet     HTTPMethod = "GET"
	MethodPost    HTTPMethod = "POST"
	MethodPut     HTTPMethod = "PUT"
	MethodDelete  HTTPMethod = "DELETE"
	MethodPatch   HTTPMethod = "PATCH"
	MethodOptions HTTPMethod = "OPTIONS"
	MethodHead    HTTPMethod = "HEAD"
)

// httpMethodBuiltinFunc represents a standardized function signature for HTTP method handlers
// This type helps eliminate repetitive code across different HTTP method implementations
type httpMethodBuiltinFunc func(registrar RouteRegistrar, thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// createHTTPMethodHandler creates a standardized handler for a specific HTTP method
// This function eliminates the need to write repetitive code for each HTTP method
// by providing a generic implementation that can be customized per method
func createHTTPMethodHandler(method HTTPMethod) httpMethodBuiltinFunc {
	return func(registrar RouteRegistrar, thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			path    starlark.String
			handler starlark.Callable
		)

		// Unpack arguments using consistent parameter names across all methods
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"path", &path,
			"handler", &handler,
		); err != nil {
			return starlark.None, err
		}

		// Delegate to the registrar's core registration method
		if err := registrar.registerRoute(string(method), path.GoString(), handler); err != nil {
			return starlark.None, err
		}

		return starlark.None, nil
	}
}

// HTTP method handlers created using the generic factory function
// These eliminate repetitive implementations across Server and RouteGroup
var (
	handleGet     = createHTTPMethodHandler(MethodGet)
	handlePost    = createHTTPMethodHandler(MethodPost)
	handlePut     = createHTTPMethodHandler(MethodPut)
	handleDelete  = createHTTPMethodHandler(MethodDelete)
	handlePatch   = createHTTPMethodHandler(MethodPatch)
	handleOptions = createHTTPMethodHandler(MethodOptions)
	handleHead    = createHTTPMethodHandler(MethodHead)
)

// pathTransformer is a function type for transforming paths before registration
// This allows RouteGroup to add prefixes while Server uses paths as-is
type pathTransformer func(path string) string

// identityPathTransformer returns the path unchanged
// Used by Server which doesn't need path transformation
func identityPathTransformer(path string) string {
	return path
}

// prefixPathTransformer creates a path transformer that adds a prefix
// Used by RouteGroup to prepend the group prefix to all routes
func prefixPathTransformer(prefix string) pathTransformer {
	return func(path string) string {
		// Ensure prefix starts with / and doesn't end with one
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		prefix = strings.TrimSuffix(prefix, "/")

		// Clean up the path and combine with prefix
		cleanPath := "/" + strings.TrimPrefix(path, "/")
		if prefix == "" {
			return cleanPath
		}
		return prefix + cleanPath
	}
}

// routeRegistrarImpl provides a base implementation for route registration
// This struct can be embedded in both Server and RouteGroup to share common functionality
type routeRegistrarImpl struct {
	pathTransformer pathTransformer
	router          *Router
}

// newRouteRegistrarImpl creates a new route registrar implementation
func newRouteRegistrarImpl(router *Router, transformer pathTransformer) *routeRegistrarImpl {
	return &routeRegistrarImpl{
		pathTransformer: transformer,
		router:          router,
	}
}

// registerRoute implements the core route registration logic
func (r *routeRegistrarImpl) registerRoute(method, path string, handler starlark.Callable) error {
	// Transform the path using the configured transformer
	transformedPath := r.pathTransformer(path)

	// Register the route with the underlying router
	r.router.AddRoute(strings.ToUpper(method), transformedPath, handler)
	return nil
}

// createRouteBuiltins creates a consistent set of route registration builtins
// This function eliminates duplication in Struct() methods between Server and RouteGroup
func createRouteBuiltins(registrar RouteRegistrar) map[string]*starlark.Builtin {
	return map[string]*starlark.Builtin{
		"get": starlark.NewBuiltin("get", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handleGet(registrar, thread, b, args, kwargs)
		}),
		"post": starlark.NewBuiltin("post", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handlePost(registrar, thread, b, args, kwargs)
		}),
		"put": starlark.NewBuiltin("put", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handlePut(registrar, thread, b, args, kwargs)
		}),
		"delete": starlark.NewBuiltin("delete", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handleDelete(registrar, thread, b, args, kwargs)
		}),
		"patch": starlark.NewBuiltin("patch", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handlePatch(registrar, thread, b, args, kwargs)
		}),
		"options": starlark.NewBuiltin("options", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handleOptions(registrar, thread, b, args, kwargs)
		}),
		"head": starlark.NewBuiltin("head", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return handleHead(registrar, thread, b, args, kwargs)
		}),
	}
}
