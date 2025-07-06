package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// MiddlewareFunc represents a middleware function
type MiddlewareFunc func(*Request, NextFunc) *Response

// NextFunc represents the next handler in the middleware chain
type NextFunc func(*Request) *Response

// MiddlewareWrapper wraps a middleware function for Starlark
type MiddlewareWrapper struct {
	middleware MiddlewareFunc
}

// NewMiddlewareWrapper creates a new middleware wrapper
func NewMiddlewareWrapper(middleware MiddlewareFunc) *MiddlewareWrapper {
	return &MiddlewareWrapper{middleware: middleware}
}

// String returns string representation
func (mw *MiddlewareWrapper) String() string {
	return "<web.Middleware>"
}

// Type returns the Starlark type name
func (mw *MiddlewareWrapper) Type() string {
	return "web.Middleware"
}

// Freeze makes the object immutable
func (mw *MiddlewareWrapper) Freeze() {}

// Truth returns the truth value
func (mw *MiddlewareWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash value
func (mw *MiddlewareWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", mw.Type())
}

// Attr returns the named attribute (none for now)
func (mw *MiddlewareWrapper) Attr(name string) (starlark.Value, error) {
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", mw.Type(), name))
}

// AttrNames returns available attributes
func (mw *MiddlewareWrapper) AttrNames() []string {
	return []string{}
}

// Execute runs the middleware function
func (mw *MiddlewareWrapper) Execute(req *Request, next NextFunc) *Response {
	return mw.middleware(req, next)
}

// Built-in middleware functions

// corsMiddleware creates a CORS middleware
func corsMiddleware(origins []string, methods []string, headers []string, credentials bool) MiddlewareFunc {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	}
	if len(headers) == 0 {
		headers = []string{"Content-Type", "Authorization"}
	}

	return func(req *Request, next NextFunc) *Response {
		// Handle preflight requests
		if req.Method == http.MethodOptions {
			return &Response{
				StatusCode: 204,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":      strings.Join(origins, ", "),
					"Access-Control-Allow-Methods":     strings.Join(methods, ", "),
					"Access-Control-Allow-Headers":     strings.Join(headers, ", "),
					"Access-Control-Allow-Credentials": fmt.Sprintf("%t", credentials),
				},
				Body: "",
			}
		}

		// Process normal requests
		response := next(req)

		// Add CORS headers to response
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		response.Headers["Access-Control-Allow-Origin"] = strings.Join(origins, ", ")
		if credentials {
			response.Headers["Access-Control-Allow-Credentials"] = "true"
		}

		return response
	}
}

// loggingMiddleware creates a logging middleware
func loggingMiddleware(format string) MiddlewareFunc {
	if format == "" {
		format = "{method} {path} {status} {duration}ms"
	}

	return func(req *Request, next NextFunc) *Response {
		start := time.Now()

		response := next(req)

		duration := time.Since(start)

		logLine := format
		logLine = strings.ReplaceAll(logLine, "{method}", req.Method)
		logLine = strings.ReplaceAll(logLine, "{path}", req.Path)
		logLine = strings.ReplaceAll(logLine, "{status}", fmt.Sprintf("%d", response.StatusCode))
		logLine = strings.ReplaceAll(logLine, "{duration}", fmt.Sprintf("%.2f", float64(duration.Nanoseconds())/1e6))

		fmt.Println(logLine)

		return response
	}
}

// securityHeadersMiddleware adds security headers
func securityHeadersMiddleware(config map[string]string) MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		response := next(req)

		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}

		// Add security headers
		for header, value := range config {
			response.Headers[header] = value
		}

		return response
	}
}

// timingMiddleware adds response time header
func timingMiddleware(header string) MiddlewareFunc {
	if header == "" {
		header = "X-Response-Time"
	}

	return func(req *Request, next NextFunc) *Response {
		start := time.Now()

		response := next(req)

		duration := time.Since(start)

		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		response.Headers[header] = fmt.Sprintf("%.3fms", float64(duration)/float64(time.Millisecond))

		return response
	}
}

// jsonMiddleware parses JSON bodies and sets JSON content type
func jsonMiddleware() MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		// Parse JSON if content type is application/json
		if req.Headers["Content-Type"] == "application/json" && len(req.bodyData) > 0 {
			// Try to parse JSON and store in context
			if jsonValue, err := dataconv.DecodeStarlarkJSON(req.bodyData); err == nil {
				if jsonData, err := dataconv.Unmarshal(jsonValue); err == nil {
					req.Context["json_data"] = jsonData
				}
			}
		}

		response := next(req)

		// Set JSON content type for JSON responses
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}

		// Check if response body looks like JSON, and set content type if so
		body := strings.TrimSpace(response.Body)
		if (strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}")) ||
			(strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]")) {
			if response.Headers["Content-Type"] == "" {
				response.Headers["Content-Type"] = "application/json"
			}
		}

		return response
	}
}

// createStarlarkMiddleware creates a middleware from a Starlark function
func createStarlarkMiddleware(middlewareFunc starlark.Callable) MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		// Create request wrapper
		reqWrapper := NewRequestWrapper(req)

		// Create next function wrapper
		nextWrapper := starlark.NewBuiltin("next", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var requestArg starlark.Value
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "request", &requestArg); err != nil {
				return nil, err
			}

			// Call the actual next function
			response := next(req)

			// Return response wrapper
			return NewResponseWrapper(response), nil
		})

		// Call middleware function
		thread := &starlark.Thread{Name: "middleware"}
		args := starlark.Tuple{reqWrapper, nextWrapper}
		result, err := starlark.Call(thread, middlewareFunc, args, nil)
		if err != nil {
			// Return error response
			return &Response{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: fmt.Sprintf(`{"error":"Middleware error: %s"}`, err.Error()),
			}
		}

		// Convert result back to Response
		if respWrapper, ok := result.(*ResponseWrapper); ok {
			return respWrapper.response
		}

		// If not a response wrapper, return error
		return &Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error":"Middleware must return a response"}`,
		}
	}
}
