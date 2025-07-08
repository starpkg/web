package web

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// Ensure RouteGroupWrapper implements the required Starlark interfaces
var (
	_ starlark.Value    = (*RouteGroupWrapper)(nil)
	_ starlark.HasAttrs = (*RouteGroupWrapper)(nil)
)

// HTTPMethod represents the supported HTTP methods
type HTTPMethod string

const (
	MethodGet     HTTPMethod = http.MethodGet
	MethodPost    HTTPMethod = http.MethodPost
	MethodPut     HTTPMethod = http.MethodPut
	MethodDelete  HTTPMethod = http.MethodDelete
	MethodPatch   HTTPMethod = http.MethodPatch
	MethodOptions HTTPMethod = http.MethodOptions
	MethodHead    HTTPMethod = http.MethodHead
)

// RouteRegistrar defines the interface for registering routes
type RouteRegistrar interface {
	RegisterRoute(method HTTPMethod, path string, handler gin.HandlerFunc) error
}

// RouteGroup represents a group of routes with a common prefix.
// This structure provides a way to organize related routes together
// and apply common middleware or configuration to them.
type RouteGroup struct {
	prefix   string
	server   *Server
	ginGroup *gin.RouterGroup
}

// RegisterRoute implements RouteRegistrar for RouteGroup
func (rg *RouteGroup) RegisterRoute(method HTTPMethod, path string, handler gin.HandlerFunc) error {
	ginPath := convertPathParams(path)

	switch method {
	case MethodGet:
		rg.ginGroup.GET(ginPath, handler)
	case MethodPost:
		rg.ginGroup.POST(ginPath, handler)
	case MethodPut:
		rg.ginGroup.PUT(ginPath, handler)
	case MethodDelete:
		rg.ginGroup.DELETE(ginPath, handler)
	case MethodPatch:
		rg.ginGroup.PATCH(ginPath, handler)
	case MethodOptions:
		rg.ginGroup.OPTIONS(ginPath, handler)
	case MethodHead:
		rg.ginGroup.HEAD(ginPath, handler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// addRoute adds a route to the gin group using the server's handler wrapper
func (rg *RouteGroup) addRoute(method HTTPMethod, path string, handler starlark.Callable) error {
	ginHandler := rg.server.wrapHandler(handler)
	return rg.RegisterRoute(method, path, ginHandler)
}

// HTTP method handlers for RouteGroup
func (rg *RouteGroup) Get(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodGet, path, handler)
}

func (rg *RouteGroup) Post(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodPost, path, handler)
}

func (rg *RouteGroup) Put(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodPut, path, handler)
}

func (rg *RouteGroup) Delete(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodDelete, path, handler)
}

func (rg *RouteGroup) Patch(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodPatch, path, handler)
}

func (rg *RouteGroup) Options(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodOptions, path, handler)
}

func (rg *RouteGroup) Head(path string, handler starlark.Callable) error {
	return rg.addRoute(MethodHead, path, handler)
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
	group       *RouteGroup
	methodMap   map[string]httpMethodInfo
	methodNames []string
}

// NewRouteGroupWrapper creates a new RouteGroupWrapper with initialized method map
func NewRouteGroupWrapper(group *RouteGroup) *RouteGroupWrapper {
	methods := []httpMethodInfo{
		{"get", MethodGet, group.Get},
		{"post", MethodPost, group.Post},
		{"put", MethodPut, group.Put},
		{"delete", MethodDelete, group.Delete},
		{"patch", MethodPatch, group.Patch},
		{"options", MethodOptions, group.Options},
		{"head", MethodHead, group.Head},
	}

	methodMap := make(map[string]httpMethodInfo, len(methods))
	methodNames := make([]string, len(methods))

	for i, method := range methods {
		methodMap[method.name] = method
		methodNames[i] = method.name
	}

	return &RouteGroupWrapper{
		group:       group,
		methodMap:   methodMap,
		methodNames: methodNames,
	}
}

// String returns a string representation of the RouteGroupWrapper.
func (rgw *RouteGroupWrapper) String() string {
	return fmt.Sprintf("<web.RouteGroup prefix=%s>", rgw.group.prefix)
}

// Type returns the Starlark type name for this object.
func (rgw *RouteGroupWrapper) Type() string {
	return "web.RouteGroup"
}

// Freeze makes this object immutable.
func (rgw *RouteGroupWrapper) Freeze() {
	// RouteGroup is immutable after creation
}

// Truth returns the truth value of this object.
func (rgw *RouteGroupWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash value for this object.
func (rgw *RouteGroupWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", rgw.Type())
}

// httpMethodInfo holds information about HTTP methods for dynamic registration
type httpMethodInfo struct {
	name    string
	method  HTTPMethod
	handler func(path string, handler starlark.Callable) error
}

// createHTTPMethodBuiltin creates a Starlark builtin for an HTTP method
func (rgw *RouteGroupWrapper) createHTTPMethodBuiltin(methodInfo httpMethodInfo) starlark.Value {
	return starlark.NewBuiltin(methodInfo.name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path string
		var handler starlark.Callable
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "handler", &handler); err != nil {
			return nil, err
		}
		return starlark.None, methodInfo.handler(path, handler)
	})
}

// Attr returns the value of the named attribute using efficient map lookup.
func (rgw *RouteGroupWrapper) Attr(name string) (starlark.Value, error) {
	// Check for HTTP method attributes using map lookup
	if methodInfo, exists := rgw.methodMap[name]; exists {
		return rgw.createHTTPMethodBuiltin(methodInfo), nil
	}

	return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", rgw.Type(), name))
}

// AttrNames returns the names of all attributes.
func (rgw *RouteGroupWrapper) AttrNames() []string {
	return rgw.methodNames
}
