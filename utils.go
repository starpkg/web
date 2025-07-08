package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// MIME type constants to avoid hardcoding throughout the module
const (
	MIMEApplicationJSON        = "application/json"
	MIMEApplicationOctetStream = "application/octet-stream"
	MIMETextPlain              = "text/plain"
	MIMETextHTML               = "text/html"
	MIMETextCSV                = "text/csv"
	MIMETextCSS                = "text/css"
	MIMETextJavaScript         = "text/javascript"
	MIMEApplicationJavaScript  = "application/javascript"
	MIMEApplicationForm        = "application/x-www-form-urlencoded"
	MIMEMultipartForm          = "multipart/form-data"
	MIMEApplicationXML         = "application/xml"
	MIMETextXML                = "text/xml"
	MIMEApplicationRSSXML      = "application/rss+xml"
	MIMEApplicationAtomXML     = "application/atom+xml"
)

// Common header constants
const (
	HeaderContentType                   = "Content-Type"
	HeaderContentLength                 = "Content-Length"
	HeaderContentDisposition            = "Content-Disposition"
	HeaderAuthorization                 = "Authorization"
	HeaderAPIKey                        = "X-API-Key"
	HeaderServer                        = "Server"
	HeaderLocation                      = "Location"
	HeaderCacheControl                  = "Cache-Control"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderXFrameOptions                 = "X-Frame-Options"
	HeaderXContentTypeOptions           = "X-Content-Type-Options"
	HeaderXXSSProtection                = "X-XSS-Protection"
	HeaderStrictTransportSecurity       = "Strict-Transport-Security"
	HeaderContentSecurityPolicy         = "Content-Security-Policy"
	HeaderReferrerPolicy                = "Referrer-Policy"
	HeaderXResponseTime                 = "X-Response-Time"
	HeaderXRateLimitLimit               = "X-RateLimit-Limit" // Rate limiting headers following IETF draft-polli-ratelimit-headers-02
	HeaderXRateLimitRemaining           = "X-RateLimit-Remaining"
	HeaderXRateLimitReset               = "X-RateLimit-Reset"
	HeaderContentEncoding               = "Content-Encoding"
	HeaderVary                          = "Vary"
	HeaderRetryAfter                    = "Retry-After"
)

// ErrorResponse represents a standardized error response.
// This structure provides a consistent format for all HTTP error responses
// throughout the web module, including error message, HTTP status code, and description.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// canonicalHeader standardizes header key using http.CanonicalHeaderKey
func canonicalHeader(key string) string {
	return http.CanonicalHeaderKey(key)
}

// starlarkListToStringSlice converts a Starlark list to Go string slice
func starlarkListToStringSlice(list *starlark.List) ([]string, error) {
	if list == nil {
		return []string{}, nil
	}

	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		item := list.Index(i)
		if str, ok := item.(starlark.String); ok {
			result[i] = string(str)
		} else {
			return nil, fmt.Errorf("list item at index %d is not a string", i)
		}
	}
	return result, nil
}

// starlarkDictToStringMap converts a Starlark dict to Go string map
func starlarkDictToStringMap(dict *starlark.Dict) (map[string]string, error) {
	if dict == nil {
		return make(map[string]string), nil
	}

	result := make(map[string]string)
	for _, item := range dict.Items() {
		key, keyOk := item[0].(starlark.String)
		value, valueOk := item[1].(starlark.String)
		if !keyOk || !valueOk {
			return nil, fmt.Errorf("dict contains non-string key or value")
		}
		result[string(key)] = string(value)
	}
	return result, nil
}

// createJSONErrorResponse creates a standardized JSON error response
func createJSONErrorResponse(statusCode int, message string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":%q,"code":%d}`, message, statusCode),
	}
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

// Additional utility functions for enhanced middleware support

// starlarkIntToInt converts a Starlark integer to Go int
func starlarkIntToInt(value starlark.Value) (int, error) {
	if intVal, ok := value.(starlark.Int); ok {
		if i, ok := intVal.Int64(); ok {
			return int(i), nil
		}
		return 0, fmt.Errorf("integer value too large")
	}
	return 0, fmt.Errorf("value is not an integer")
}

// starlarkBoolToBool converts a Starlark boolean to Go bool
func starlarkBoolToBool(value starlark.Value) (bool, error) {
	if boolVal, ok := value.(starlark.Bool); ok {
		return bool(boolVal), nil
	}
	return false, fmt.Errorf("value is not a boolean")
}

// starlarkStringToString converts a Starlark string to Go string
func starlarkStringToString(value starlark.Value) (string, error) {
	if strVal, ok := value.(starlark.String); ok {
		return string(strVal), nil
	}
	return "", fmt.Errorf("value is not a string")
}

// starlarkListToIntSlice converts a Starlark list to Go int slice
func starlarkListToIntSlice(list *starlark.List) ([]int, error) {
	if list == nil {
		return []int{}, nil
	}

	result := make([]int, list.Len())
	for i := 0; i < list.Len(); i++ {
		item := list.Index(i)
		if intVal, ok := item.(starlark.Int); ok {
			if val, ok := intVal.Int64(); ok {
				result[i] = int(val)
			} else {
				return nil, fmt.Errorf("list item at index %d is too large", i)
			}
		} else {
			return nil, fmt.Errorf("list item at index %d is not an integer", i)
		}
	}
	return result, nil
}

// isCompressibleContentType checks if a content type should be compressed
func isCompressibleContentType(contentType string) bool {
	compressibleTypes := []string{
		MIMETextPlain,
		MIMETextHTML,
		MIMETextCSV,
		MIMETextCSS,
		MIMETextJavaScript,
		MIMEApplicationJSON,
		MIMEApplicationJavaScript,
		MIMEApplicationForm,
		MIMEApplicationXML,
		MIMETextXML,
		MIMEApplicationRSSXML,
		MIMEApplicationAtomXML,
	}

	mainType := parseContentType(contentType)
	for _, compressible := range compressibleTypes {
		if mainType == compressible {
			return true
		}
	}
	return false
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// isValidHTTPMethod checks if a string is a valid HTTP method
func isValidHTTPMethod(method string) bool {
	validMethods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodHead,
		http.MethodConnect,
		http.MethodTrace,
	}

	for _, valid := range validMethods {
		if method == valid {
			return true
		}
	}
	return false
}

// createRateLimitKey creates a consistent key for rate limiting
func createRateLimitKey(prefix, identifier string) string {
	return fmt.Sprintf("%s:%s", prefix, identifier)
}

// getCurrentTimeWindow returns the current time window for rate limiting
func getCurrentTimeWindow(windowSize int) int64 {
	return time.Now().Unix() / int64(windowSize)
}

// Additional error response builders for middleware

// createTooManyRequestsResponse creates a 429 Too Many Requests response
func createTooManyRequestsResponse(retryAfter int) *Response {
	headers := map[string]string{
		canonicalHeader(HeaderContentType): MIMEApplicationJSON,
	}
	if retryAfter > 0 {
		headers[HeaderRetryAfter] = fmt.Sprintf("%d", retryAfter)
	}

	return &Response{
		StatusCode: http.StatusTooManyRequests,
		Headers:    headers,
		Body:       `{"error":"Too Many Requests","code":429}`,
	}
}

// createRequestEntityTooLargeResponse creates a 413 Request Entity Too Large response
func createRequestEntityTooLargeResponse(maxSize int64) *Response {
	return &Response{
		StatusCode: http.StatusRequestEntityTooLarge,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Request Entity Too Large","code":413,"max_size":"%s"}`, formatBytes(maxSize)),
	}
}

// createURITooLongResponse creates a 414 URI Too Long response
func createURITooLongResponse(maxLength int) *Response {
	return &Response{
		StatusCode: http.StatusRequestURITooLong,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"URI Too Long","code":414,"max_length":%d}`, maxLength),
	}
}

// createNotAcceptableResponse creates a 406 Not Acceptable response
func createNotAcceptableResponse(message string) *Response {
	return &Response{
		StatusCode: http.StatusNotAcceptable,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Not Acceptable","code":406,"message":%q}`, message),
	}
}

// createUnsupportedMediaTypeResponse creates a 415 Unsupported Media Type response
func createUnsupportedMediaTypeResponse(contentType string) *Response {
	return &Response{
		StatusCode: http.StatusUnsupportedMediaType,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Unsupported Media Type","code":415,"content_type":%q}`, contentType),
	}
}
