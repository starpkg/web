package web

import (
	"fmt"
	"net/http"

	"go.starlark.net/starlark"
)

// ErrorHandler represents a custom error handler function
type ErrorHandler struct {
	statusCodes []int
	handler     starlark.Callable
}

// ErrorHandlerRegistry manages custom error handlers
type ErrorHandlerRegistry struct {
	handlers map[int]starlark.Callable
}

// NewErrorHandlerRegistry creates a new error handler registry
func NewErrorHandlerRegistry() *ErrorHandlerRegistry {
	return &ErrorHandlerRegistry{
		handlers: make(map[int]starlark.Callable),
	}
}

// RegisterHandler registers an error handler for specific status codes
func (ehr *ErrorHandlerRegistry) RegisterHandler(statusCodes []int, handler starlark.Callable) {
	for _, code := range statusCodes {
		ehr.handlers[code] = handler
	}
}

// GetHandler returns the handler for a specific status code
func (ehr *ErrorHandlerRegistry) GetHandler(statusCode int) starlark.Callable {
	return ehr.handlers[statusCode]
}

// HandleError calls the appropriate error handler or returns a default error response
func (ehr *ErrorHandlerRegistry) HandleError(statusCode int, req *Request) *Response {
	handler := ehr.GetHandler(statusCode)
	if handler == nil {
		// Return default error response
		return &Response{
			StatusCode: statusCode,
			Headers: map[string]string{
				"Content-Type": MIMEApplicationJSON,
			},
			Body: fmt.Sprintf(`{"error":%q,"code":%d}`, http.StatusText(statusCode), statusCode),
		}
	}

	thread := &starlark.Thread{Name: "error_handler"}
	reqWrapper := NewRequestWrapper(req)
	args := starlark.Tuple{reqWrapper}

	result, err := starlark.Call(thread, handler, args, nil)
	if err != nil {
		// If error handler fails, return default error
		return &Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": MIMEApplicationJSON,
			},
			Body: fmt.Sprintf(`{"error":"Error handler failed: %s"}`, err.Error()),
		}
	}

	// Convert result to Response
	if respWrapper, ok := result.(*ResponseWrapper); ok {
		return respWrapper.response
	}

	// If not a response wrapper, return error
	return &Response{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: `{"error":"Error handler must return a response"}`,
	}
}

// Built-in error handlers

// DefaultNotFoundHandler returns a default 404 error response
func DefaultNotFoundHandler(req *Request) *Response {
	return &Response{
		StatusCode: 404,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Not Found","message":"The requested resource %s was not found","code":404}`, req.Path),
	}
}

// DefaultMethodNotAllowedHandler returns a default 405 error response
func DefaultMethodNotAllowedHandler(req *Request) *Response {
	return &Response{
		StatusCode: 405,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Method Not Allowed","message":"Method %s is not allowed for %s","code":405}`, req.Method, req.Path),
	}
}

// DefaultInternalServerErrorHandler returns a default 500 error response
func DefaultInternalServerErrorHandler(req *Request) *Response {
	return &Response{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: `{"error":"Internal Server Error","message":"An internal server error occurred","code":500}`,
	}
}

// DefaultBadRequestHandler returns a default 400 error response
func DefaultBadRequestHandler(req *Request) *Response {
	return &Response{
		StatusCode: 400,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: `{"error":"Bad Request","message":"The request was malformed or invalid","code":400}`,
	}
}

// DefaultUnauthorizedHandler returns a default 401 error response
func DefaultUnauthorizedHandler(req *Request) *Response {
	return &Response{
		StatusCode: 401,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: `{"error":"Unauthorized","message":"Authentication is required","code":401}`,
	}
}

// DefaultForbiddenHandler returns a default 403 error response
func DefaultForbiddenHandler(req *Request) *Response {
	return &Response{
		StatusCode: 403,
		Headers: map[string]string{
			"Content-Type": MIMEApplicationJSON,
		},
		Body: `{"error":"Forbidden","message":"Access to this resource is forbidden","code":403}`,
	}
}

// IsErrorStatus checks if a status code represents an error
func IsErrorStatus(statusCode int) bool {
	return statusCode >= 400
}

// GetDefaultErrorMessage returns a default error message for a status code
func GetDefaultErrorMessage(statusCode int) string {
	return http.StatusText(statusCode)
}
