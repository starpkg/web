package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// MIME types
const (
	MIMEApplicationJSON        = "application/json"
	MIMEApplicationJavaScript  = "application/javascript"
	MIMETextHTML               = "text/html"
	MIMETextPlain              = "text/plain"
	MIMETextCSS                = "text/css"
	MIMETextJavaScript         = "text/javascript"
	MIMEMultipartForm          = "multipart/form-data"
	MIMEApplicationForm        = "application/x-www-form-urlencoded"
	MIMEApplicationOctetStream = "application/octet-stream"
)

// Header names
const (
	HeaderContentType        = "Content-Type"
	HeaderContentLength      = "Content-Length"
	HeaderContentDisposition = "Content-Disposition"
	HeaderContentEncoding    = "Content-Encoding"
	HeaderAuthorization      = "Authorization"
	HeaderWWWAuthenticate    = "WWW-Authenticate"
	HeaderAPIKey             = "X-API-Key"
	HeaderCacheControl       = "Cache-Control"
	HeaderServer             = "Server"
	HeaderLocation           = "Location"
	HeaderVary               = "Vary"
	HeaderRetryAfter         = "Retry-After"

	// CORS headers
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"

	// Response time header
	HeaderXResponseTime = "X-Response-Time"

	// Rate limiting headers
	HeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderXRateLimitReset     = "X-RateLimit-Reset"
)

// readerCloser wraps a strings.Reader to make it closeable
type readerCloser struct {
	*strings.Reader
}

func (rc *readerCloser) Close() error {
	return nil
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

// createBasicAuthChallengeResponse creates a 401 response with WWW-Authenticate header
// for Basic Authentication. This triggers the browser to show the authentication dialog.
func createBasicAuthChallengeResponse(statusCode int, message string, realm string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType):     MIMEApplicationJSON,
			canonicalHeader(HeaderWWWAuthenticate): fmt.Sprintf("Basic realm=\"%s\"", realm),
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

// sendNotFound sends a 404 Not Found response
func sendNotFound(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusNotFound, message)
}

// sendMethodNotAllowed sends a 405 Method Not Allowed response
func sendMethodNotAllowed(c *gin.Context, message string) {
	sendErrorResponse(c, http.StatusMethodNotAllowed, message)
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
				return nil, fmt.Errorf("integer value at index %d too large", i)
			}
		} else {
			return nil, fmt.Errorf("list item at index %d is not an integer", i)
		}
	}
	return result, nil
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// isCompressibleContentType checks if a content type should be compressed
func isCompressibleContentType(contentType string) bool {
	compressibleTypes := []string{
		"text/",
		"application/json",
		"application/javascript",
		"application/xml",
		"application/atom+xml",
		"application/rss+xml",
		"application/svg+xml",
		"image/svg+xml",
	}

	mainType := parseContentType(contentType)
	for _, compressible := range compressibleTypes {
		if strings.HasPrefix(mainType, compressible) {
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

// createRateLimitKey creates a key for rate limiting
func createRateLimitKey(prefix, identifier string) string {
	return fmt.Sprintf("%s:%s", prefix, identifier)
}

// getCurrentTimeWindow returns the current time window for rate limiting
func getCurrentTimeWindow(windowSize int) int64 {
	return time.Now().Unix() / int64(windowSize)
}

// createTooManyRequestsResponse creates a 429 Too Many Requests response
func createTooManyRequestsResponse(retryAfter int) *Response {
	return &Response{
		StatusCode: http.StatusTooManyRequests,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
			canonicalHeader(HeaderRetryAfter):  fmt.Sprintf("%d", retryAfter),
		},
		Body: fmt.Sprintf(`{"error":"Rate limit exceeded","code":%d}`, http.StatusTooManyRequests),
	}
}

// createRequestEntityTooLargeResponse creates a 413 Request Entity Too Large response
func createRequestEntityTooLargeResponse(maxSize int64) *Response {
	return &Response{
		StatusCode: http.StatusRequestEntityTooLarge,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Request body too large","max_size":%d,"code":%d}`, maxSize, http.StatusRequestEntityTooLarge),
	}
}

// createURITooLongResponse creates a 414 URI Too Long response
func createURITooLongResponse(maxLength int) *Response {
	return &Response{
		StatusCode: http.StatusRequestURITooLong,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"URI too long","max_length":%d,"code":%d}`, maxLength, http.StatusRequestURITooLong),
	}
}

// createNotAcceptableResponse creates a 406 Not Acceptable response
func createNotAcceptableResponse(message string) *Response {
	return &Response{
		StatusCode: http.StatusNotAcceptable,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Not acceptable","message":%q,"code":%d}`, message, http.StatusNotAcceptable),
	}
}

// createUnsupportedMediaTypeResponse creates a 415 Unsupported Media Type response
func createUnsupportedMediaTypeResponse(contentType string) *Response {
	return &Response{
		StatusCode: http.StatusUnsupportedMediaType,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMEApplicationJSON,
		},
		Body: fmt.Sprintf(`{"error":"Unsupported media type","content_type":%q,"code":%d}`, contentType, http.StatusUnsupportedMediaType),
	}
}
