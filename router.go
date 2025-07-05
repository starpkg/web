package web

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// RouteGroup represents a group of routes with a common prefix.
// This structure provides a way to organize related routes together
// and apply common middleware or configuration to them.
type RouteGroup struct {
	prefix   string
	server   *Server
	ginGroup *gin.RouterGroup
}

// Get registers a GET route for this route group.
// This method adds a GET handler to the specified path within the group's prefix.
func (rg *RouteGroup) Get(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodGet, path, handler)
}

// Post registers a POST route for this route group.
// This method adds a POST handler to the specified path within the group's prefix.
func (rg *RouteGroup) Post(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodPost, path, handler)
}

// Put registers a PUT route for this route group.
// This method adds a PUT handler to the specified path within the group's prefix.
func (rg *RouteGroup) Put(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodPut, path, handler)
}

// Delete registers a DELETE route for this route group.
// This method adds a DELETE handler to the specified path within the group's prefix.
func (rg *RouteGroup) Delete(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodDelete, path, handler)
}

// Patch registers a PATCH route for this route group.
// This method adds a PATCH handler to the specified path within the group's prefix.
func (rg *RouteGroup) Patch(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodPatch, path, handler)
}

// Options registers an OPTIONS route for this route group.
// This method adds an OPTIONS handler to the specified path within the group's prefix.
func (rg *RouteGroup) Options(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodOptions, path, handler)
}

// Head registers a HEAD route for this route group.
// This method adds a HEAD handler to the specified path within the group's prefix.
func (rg *RouteGroup) Head(path string, handler starlark.Callable) error {
	return rg.addRoute(http.MethodHead, path, handler)
}

// addRoute adds a route to the gin group
func (rg *RouteGroup) addRoute(method, path string, handler starlark.Callable) error {
	// Convert {param} style to :param style for Gin
	ginPath := convertPathParams(path)
	ginHandler := rg.server.wrapHandler(handler)

	switch method {
	case http.MethodGet:
		rg.ginGroup.GET(ginPath, ginHandler)
	case http.MethodPost:
		rg.ginGroup.POST(ginPath, ginHandler)
	case http.MethodPut:
		rg.ginGroup.PUT(ginPath, ginHandler)
	case http.MethodDelete:
		rg.ginGroup.DELETE(ginPath, ginHandler)
	case http.MethodPatch:
		rg.ginGroup.PATCH(ginPath, ginHandler)
	case http.MethodOptions:
		rg.ginGroup.OPTIONS(ginPath, ginHandler)
	case http.MethodHead:
		rg.ginGroup.HEAD(ginPath, ginHandler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// convertPathParams converts path parameters from {param} format to :param format.
// This function transforms Flask-style path parameters to Gin-compatible format.
func convertPathParams(path string) string {
	// Use regex to replace {param} with :param
	re := regexp.MustCompile(`\{([^}]+)\}`)
	return re.ReplaceAllString(path, ":$1")
}

// RouteGroupWrapper wraps the RouteGroup struct to provide Starlark-compatible method names.
// This wrapper exposes route group methods to Starlark scripts with lowercase names
// that match the expected API conventions.
type RouteGroupWrapper struct {
	group *RouteGroup
}

// String returns a string representation of the RouteGroupWrapper.
// This method provides a human-readable description of the route group.
func (rgw *RouteGroupWrapper) String() string {
	return fmt.Sprintf("<web.RouteGroup prefix=%s>", rgw.group.prefix)
}

// Type returns the Starlark type name for this object.
// This method identifies the object type for Starlark's type system.
func (rgw *RouteGroupWrapper) Type() string {
	return "web.RouteGroup"
}

// Freeze makes this object immutable.
// This method is called by Starlark to freeze the object state.
func (rgw *RouteGroupWrapper) Freeze() {
	// RouteGroup is immutable after creation
}

// Truth returns the truth value of this object.
// This method determines how the object behaves in boolean contexts.
func (rgw *RouteGroupWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash value for this object.
// This method is required for objects that may be used as dictionary keys.
func (rgw *RouteGroupWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", rgw.Type())
}

// Attr returns the value of the named attribute.
// This method provides access to route group methods from Starlark scripts.
func (rgw *RouteGroupWrapper) Attr(name string) (starlark.Value, error) {
	switch name {
	case "get":
		return starlark.NewBuiltin("get", rgw.get), nil
	case "post":
		return starlark.NewBuiltin("post", rgw.post), nil
	case "put":
		return starlark.NewBuiltin("put", rgw.put), nil
	case "delete":
		return starlark.NewBuiltin("delete", rgw.delete), nil
	case "patch":
		return starlark.NewBuiltin("patch", rgw.patch), nil
	case "options":
		return starlark.NewBuiltin("options", rgw.options), nil
	case "head":
		return starlark.NewBuiltin("head", rgw.head), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", rgw.Type(), name))
	}
}

// AttrNames returns the names of all attributes.
// This method provides a list of available attributes for introspection.
func (rgw *RouteGroupWrapper) AttrNames() []string {
	return []string{"get", "post", "put", "delete", "patch", "options", "head"}
}

// get handles the get() method call for route groups.
func (rgw *RouteGroupWrapper) get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Get(path, handler)
}

// post handles the post() method call for route groups.
func (rgw *RouteGroupWrapper) post(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Post(path, handler)
}

// put handles the put() method call for route groups.
func (rgw *RouteGroupWrapper) put(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Put(path, handler)
}

// delete handles the delete() method call for route groups.
func (rgw *RouteGroupWrapper) delete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Delete(path, handler)
}

// patch handles the patch() method call for route groups.
func (rgw *RouteGroupWrapper) patch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Patch(path, handler)
}

// options handles the options() method call for route groups.
func (rgw *RouteGroupWrapper) options(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Options(path, handler)
}

// head handles the head() method call for route groups.
func (rgw *RouteGroupWrapper) head(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var handler starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
		return nil, err
	}
	return starlark.None, rgw.group.Head(path, handler)
}
