package web

import (
	"fmt"
	"net/http"

	"go.starlark.net/starlark"
)

// ErrorResponse represents a standardized error response.
// This structure provides a consistent format for all HTTP error responses
// throughout the web module, including error message, HTTP status code, and description.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

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
