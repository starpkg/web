package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
			return createJSONErrorResponse(500, fmt.Sprintf("Middleware error: %s", err.Error()))
		}

		// Convert result back to Response
		if respWrapper, ok := result.(*ResponseWrapper); ok {
			return respWrapper.response
		}

		// If not a response wrapper, return error
		return createJSONErrorResponse(500, "Middleware must return a response")
	}
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
		headers = []string{HeaderContentType, HeaderAuthorization}
	}

	return func(req *Request, next NextFunc) *Response {
		// Handle preflight requests
		if req.Method == http.MethodOptions {
			return &Response{
				StatusCode: 204,
				Headers: map[string]string{
					canonicalHeader(HeaderAccessControlAllowOrigin):      strings.Join(origins, ", "),
					canonicalHeader(HeaderAccessControlAllowMethods):     strings.Join(methods, ", "),
					canonicalHeader(HeaderAccessControlAllowHeaders):     strings.Join(headers, ", "),
					canonicalHeader(HeaderAccessControlAllowCredentials): fmt.Sprintf("%t", credentials),
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
		response.Headers[canonicalHeader(HeaderAccessControlAllowOrigin)] = strings.Join(origins, ", ")
		if credentials {
			response.Headers[canonicalHeader(HeaderAccessControlAllowCredentials)] = "true"
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
			response.Headers[canonicalHeader(header)] = value
		}

		return response
	}
}

// timingMiddleware adds response time header
func timingMiddleware(header string) MiddlewareFunc {
	if header == "" {
		header = HeaderXResponseTime
	}

	return func(req *Request, next NextFunc) *Response {
		start := time.Now()

		response := next(req)

		duration := time.Since(start)

		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		response.Headers[canonicalHeader(header)] = fmt.Sprintf("%.3fms", float64(duration)/float64(time.Millisecond))

		return response
	}
}

// jsonMiddleware parses JSON bodies and sets JSON content type
func jsonMiddleware() MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		// Parse JSON if content type is application/json
		if req.Headers[canonicalHeader(HeaderContentType)] == MIMEApplicationJSON && len(req.bodyData) > 0 {
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
			if response.Headers[canonicalHeader(HeaderContentType)] == "" {
				response.Headers[canonicalHeader(HeaderContentType)] = MIMEApplicationJSON
			}
		}

		return response
	}
}

// compressionMiddleware creates a compression middleware with gzip support
func compressionMiddleware(level int, minSize int, types []string) MiddlewareFunc {
	// Validate compression level
	if level < 1 || level > 9 {
		level = 6 // Default compression level
	}

	// Default minimum size
	if minSize <= 0 {
		minSize = 1024 // 1KB default
	}

	// Default compressible types if none provided
	if len(types) == 0 {
		types = []string{
			MIMETextPlain,
			MIMETextHTML,
			MIMEApplicationJSON,
			MIMETextCSS,
			MIMETextJavaScript,
			MIMEApplicationJavaScript,
		}
	}

	return func(req *Request, next NextFunc) *Response {
		// Check if client accepts gzip
		acceptEncoding := req.Headers[canonicalHeader("Accept-Encoding")]
		if !strings.Contains(acceptEncoding, "gzip") {
			return next(req)
		}

		// Process request
		response := next(req)

		// Check if response should be compressed
		if len(response.Body) < minSize {
			return response
		}

		// Check content type
		contentType := response.Headers[canonicalHeader(HeaderContentType)]
		if contentType == "" {
			// Try to determine from body
			if isCompressibleContentType(parseContentType(contentType)) {
				contentType = MIMETextPlain
			} else {
				return response
			}
		}

		shouldCompress := false
		mainContentType := parseContentType(contentType)
		for _, compressibleType := range types {
			if mainContentType == compressibleType {
				shouldCompress = true
				break
			}
		}

		if !shouldCompress {
			return response
		}

		// Compress response body
		var buf bytes.Buffer
		writer, err := gzip.NewWriterLevel(&buf, level)
		if err != nil {
			return response // Return uncompressed on error
		}

		_, err = writer.Write([]byte(response.Body))
		if err != nil {
			return response
		}

		err = writer.Close()
		if err != nil {
			return response
		}

		// Update response
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		response.Headers[canonicalHeader(HeaderContentEncoding)] = "gzip"
		response.Headers[canonicalHeader(HeaderVary)] = "Accept-Encoding"
		response.Headers[canonicalHeader(HeaderContentLength)] = strconv.Itoa(buf.Len())
		response.Body = buf.String()

		return response
	}
}

// RateLimitStorage interface for rate limiting storage backends
type RateLimitStorage interface {
	Get(key string) (int, error)
	Set(key string, value int, ttl time.Duration) error
	Increment(key string, ttl time.Duration) (int, error)
}

// MemoryRateLimitStorage implements in-memory rate limiting storage
type MemoryRateLimitStorage struct {
	data map[string]*rateLimitEntry
	mu   sync.RWMutex
}

type rateLimitEntry struct {
	count   int
	expires time.Time
}

// NewMemoryRateLimitStorage creates a new memory-based rate limit storage
func NewMemoryRateLimitStorage() *MemoryRateLimitStorage {
	storage := &MemoryRateLimitStorage{
		data: make(map[string]*rateLimitEntry),
	}

	// Start cleanup goroutine
	go storage.cleanup()

	return storage
}

func (m *MemoryRateLimitStorage) Get(key string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.data[key]; exists && time.Now().Before(entry.expires) {
		return entry.count, nil
	}
	return 0, nil
}

func (m *MemoryRateLimitStorage) Set(key string, value int, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = &rateLimitEntry{
		count:   value,
		expires: time.Now().Add(ttl),
	}
	return nil
}

func (m *MemoryRateLimitStorage) Increment(key string, ttl time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if entry, exists := m.data[key]; exists && now.Before(entry.expires) {
		entry.count++
		return entry.count, nil
	}

	// Create new entry
	m.data[key] = &rateLimitEntry{
		count:   1,
		expires: now.Add(ttl),
	}
	return 1, nil
}

func (m *MemoryRateLimitStorage) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, entry := range m.data {
			if now.After(entry.expires) {
				delete(m.data, key)
			}
		}
		m.mu.Unlock()
	}
}

// rateLimitMiddleware creates a rate limiting middleware
func rateLimitMiddleware(requests int, window int, keyFunc func(*Request) string, storage RateLimitStorage) MiddlewareFunc {
	if requests <= 0 {
		requests = 100 // Default 100 requests
	}
	if window <= 0 {
		window = 60 // Default 60 seconds
	}
	if keyFunc == nil {
		keyFunc = func(req *Request) string {
			return req.ClientIP
		}
	}
	if storage == nil {
		storage = NewMemoryRateLimitStorage()
	}

	windowDuration := time.Duration(window) * time.Second

	return func(req *Request, next NextFunc) *Response {
		key := createRateLimitKey("rate_limit", keyFunc(req))

		count, err := storage.Increment(key, windowDuration)
		if err != nil {
			// On storage error, allow the request
			return next(req)
		}

		if count > requests {
			return createTooManyRequestsResponse(window)
		}

		// Add rate limit headers to response
		response := next(req)
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}

		response.Headers[HeaderXRateLimitLimit] = strconv.Itoa(requests)
		response.Headers[HeaderXRateLimitRemaining] = strconv.Itoa(max(0, requests-count))
		response.Headers[HeaderXRateLimitReset] = strconv.FormatInt(time.Now().Add(windowDuration).Unix(), 10)

		return response
	}
}

// cacheMiddleware creates a response caching middleware
func cacheMiddleware(maxAge int, private bool, patterns []string, vary []string) MiddlewareFunc {
	if maxAge <= 0 {
		maxAge = 3600 // Default 1 hour
	}

	return func(req *Request, next NextFunc) *Response {
		// Only cache GET requests
		if req.Method != http.MethodGet {
			return next(req)
		}

		// Check if path matches patterns
		if len(patterns) > 0 {
			matched := false
			for _, pattern := range patterns {
				if matchesPattern(req.Path, pattern) {
					matched = true
					break
				}
			}
			if !matched {
				return next(req)
			}
		}

		response := next(req)

		// Add cache headers
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}

		cacheControl := fmt.Sprintf("max-age=%d", maxAge)
		if private {
			cacheControl = "private, " + cacheControl
		} else {
			cacheControl = "public, " + cacheControl
		}
		response.Headers[canonicalHeader(HeaderCacheControl)] = cacheControl

		if len(vary) > 0 {
			response.Headers[canonicalHeader(HeaderVary)] = strings.Join(vary, ", ")
		}

		return response
	}
}

// requestSizeMiddleware creates a middleware that limits request sizes
func requestSizeMiddleware(maxContentLength int64, maxURLLength int, maxHeaders int) MiddlewareFunc {
	if maxContentLength <= 0 {
		maxContentLength = 10 * 1024 * 1024 // Default 10MB
	}
	if maxURLLength <= 0 {
		maxURLLength = 2048 // Default 2KB
	}
	if maxHeaders <= 0 {
		maxHeaders = 100 // Default 100 headers
	}

	return func(req *Request, next NextFunc) *Response {
		// Check URL length (including query parameters)
		requestURI := req.Path
		if len(req.Query) > 0 {
			// Reconstruct the query string
			queryParts := make([]string, 0, len(req.Query))
			for key, value := range req.Query {
				queryParts = append(queryParts, key+"="+value)
			}
			if len(queryParts) > 0 {
				requestURI = requestURI + "?" + strings.Join(queryParts, "&")
			}
		}

		if len(requestURI) > maxURLLength {
			return createURITooLongResponse(maxURLLength)
		}

		// Check number of headers
		if len(req.Headers) > maxHeaders {
			return createNotAcceptableResponse(fmt.Sprintf("Too many headers (max: %d)", maxHeaders))
		}

		// Check content length
		if len(req.bodyData) > int(maxContentLength) {
			return createRequestEntityTooLargeResponse(maxContentLength)
		}

		return next(req)
	}
}

// Helper functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func matchesPattern(path, pattern string) bool {
	// Simple pattern matching - supports * wildcard at end
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}
