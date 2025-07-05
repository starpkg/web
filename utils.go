package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a standardized error response.
// This structure provides a consistent format for all HTTP error responses
// throughout the web module, including error message, HTTP status code, and description.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// sendErrorResponse sends a consistent error response.
// This function creates a standardized JSON error response with the specified
// status code and message, ensuring all errors follow the same format.
func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	})
}

// sendBadRequest sends a 400 Bad Request response
func sendBadRequest(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusBadRequest, message)
}

// sendUnauthorized sends a 401 Unauthorized response
func sendUnauthorized(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusUnauthorized, message)
}

// sendForbidden sends a 403 Forbidden response
func sendForbidden(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusForbidden, message)
}

// sendNotFound sends a 404 Not Found response
func sendNotFound(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusNotFound, message)
}

// sendMethodNotAllowed sends a 405 Method Not Allowed response
func sendMethodNotAllowed(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusMethodNotAllowed, message)
}

// sendInternalServerError sends a 500 Internal Server Error response
func sendInternalServerError(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusInternalServerError, message)
}

// convertPathParams converts path parameters from {param} to :param format for Gin.
// This function transforms Flask-style path parameters to Gin's expected format,
// enabling seamless route parameter extraction in handlers.
func convertPathParams(path string) string {
	result := ""
	inParam := false

	for _, char := range path {
		switch char {
		case '{':
			if !inParam {
				result += ":"
				inParam = true
			} else {
				result += string(char)
			}
		case '}':
			if inParam {
				inParam = false
			} else {
				result += string(char)
			}
		default:
			result += string(char)
		}
	}

	return result
}

// safeString safely converts a value to string, handling nil cases
func safeString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// getClientIP extracts the client IP from the request, checking various headers.
// This function checks multiple common headers used by proxies and load balancers
// to determine the real client IP address, falling back to the direct connection IP.
func getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header first
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i, char := range xff {
			if char == ',' || char == ' ' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Check X-Forwarded header
	if xf := c.GetHeader("X-Forwarded"); xf != "" {
		return xf
	}

	// Check X-Cluster-Client-IP header
	if xcci := c.GetHeader("X-Cluster-Client-IP"); xcci != "" {
		return xcci
	}

	// Fall back to RemoteAddr
	return c.ClientIP()
}

// validateContentType checks if the content type is valid for the given types
func validateContentType(contentType string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true
	}

	for _, allowed := range allowedTypes {
		if contentType == allowed {
			return true
		}
	}

	return false
}

// parseContentType extracts the main content type from a Content-Type header
func parseContentType(contentType string) string {
	// Split by semicolon to remove parameters like charset
	for i, char := range contentType {
		if char == ';' {
			return contentType[:i]
		}
	}
	return contentType
}
