package web

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Built-in middleware implementations

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct {
	Origins     []string
	Methods     []string
	Headers     []string
	Credentials bool
	MaxAge      int
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(origins, methods, headers []string, credentials bool, maxAge int) *CORSMiddleware {
	return &CORSMiddleware{
		Origins:     origins,
		Methods:     methods,
		Headers:     headers,
		Credentials: credentials,
		MaxAge:      maxAge,
	}
}

// Middleware returns the CORS middleware function
func (c *CORSMiddleware) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		origin := req.Request.Header.Get("Origin")

		// Handle preflight requests
		if req.Request.Method == "OPTIONS" {
			response := &Response{
				StatusCode: 200,
				Headers:    make(http.Header),
				Body:       "",
			}

			// Set CORS headers
			c.setCORSHeaders(response, origin)
			return response
		}

		// Process normal requests
		response := next(req)

		// Add CORS headers to response
		c.setCORSHeaders(response, origin)

		return response
	}

	return dataconv.WrapGoValue(middleware), nil
}

// setCORSHeaders sets CORS headers on the response
func (c *CORSMiddleware) setCORSHeaders(response *Response, origin string) {
	// Check if origin is allowed
	if c.isOriginAllowed(origin) {
		response.Headers.Set("Access-Control-Allow-Origin", origin)
	} else if len(c.Origins) == 1 && c.Origins[0] == "*" {
		response.Headers.Set("Access-Control-Allow-Origin", "*")
	}

	if len(c.Methods) > 0 {
		response.Headers.Set("Access-Control-Allow-Methods", strings.Join(c.Methods, ", "))
	}

	if len(c.Headers) > 0 {
		response.Headers.Set("Access-Control-Allow-Headers", strings.Join(c.Headers, ", "))
	}

	if c.Credentials {
		response.Headers.Set("Access-Control-Allow-Credentials", "true")
	}

	if c.MaxAge > 0 {
		response.Headers.Set("Access-Control-Max-Age", fmt.Sprintf("%d", c.MaxAge))
	}
}

// isOriginAllowed checks if the origin is in the allowed list
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	for _, allowed := range c.Origins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// LoggingMiddleware logs HTTP requests
type LoggingMiddleware struct {
	Format      string
	SkipPaths   []string
	IncludeBody bool
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(format string, skipPaths []string, includeBody bool) *LoggingMiddleware {
	return &LoggingMiddleware{
		Format:      format,
		SkipPaths:   skipPaths,
		IncludeBody: includeBody,
	}
}

// Middleware returns the logging middleware function
func (l *LoggingMiddleware) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Skip logging for certain paths
		for _, skipPath := range l.SkipPaths {
			if req.Request.URL.Path == skipPath {
				return next(req)
			}
		}

		start := time.Now()

		// Process request
		response := next(req)

		duration := time.Since(start)

		// Format log message
		logMessage := l.formatLogMessage(req, response, duration)
		log.Println(logMessage)

		return response
	}

	return dataconv.WrapGoValue(middleware), nil
}

// formatLogMessage formats the log message
func (l *LoggingMiddleware) formatLogMessage(req *Request, resp *Response, duration time.Duration) string {
	format := l.Format
	format = strings.ReplaceAll(format, "{method}", req.Request.Method)
	format = strings.ReplaceAll(format, "{path}", req.Request.URL.Path)
	format = strings.ReplaceAll(format, "{status}", fmt.Sprintf("%d", resp.StatusCode))
	format = strings.ReplaceAll(format, "{duration}", fmt.Sprintf("%.2fms", float64(duration.Nanoseconds())/1e6))
	format = strings.ReplaceAll(format, "{ip}", req.ClientIP().GoString())
	format = strings.ReplaceAll(format, "{user-agent}", req.Request.Header.Get("User-Agent"))

	return format
}

// TimingMiddleware adds response time headers
type TimingMiddleware struct {
	Header    string
	Precision int
}

// NewTimingMiddleware creates a new timing middleware
func NewTimingMiddleware(header string, precision int) *TimingMiddleware {
	return &TimingMiddleware{
		Header:    header,
		Precision: precision,
	}
}

// Middleware returns the timing middleware function
func (t *TimingMiddleware) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		start := time.Now()

		// Process request
		response := next(req)

		duration := time.Since(start)

		// Add timing header
		timing := fmt.Sprintf("%.*fms", t.Precision, float64(duration.Nanoseconds())/1e6)
		response.Headers.Set(t.Header, timing)

		return response
	}

	return dataconv.WrapGoValue(middleware), nil
}

// CompressionMiddleware handles response compression
type CompressionMiddleware struct {
	Level   int
	MinSize int
	Types   []string
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(level, minSize int, types []string) *CompressionMiddleware {
	return &CompressionMiddleware{
		Level:   level,
		MinSize: minSize,
		Types:   types,
	}
}

// Middleware returns the compression middleware function
func (c *CompressionMiddleware) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Process request
		response := next(req)

		// Check if compression is supported by client
		acceptEncoding := req.Request.Header.Get("Accept-Encoding")
		if !strings.Contains(acceptEncoding, "gzip") {
			return response
		}

		// Check content type
		contentType := response.Headers.Get("Content-Type")
		if !c.shouldCompress(contentType) {
			return response
		}

		// Check content size
		if len(response.Body) < c.MinSize {
			return response
		}

		// Compress response body
		compressed, err := c.compressBody(response.Body)
		if err != nil {
			return response // Return uncompressed on error
		}

		// Update response
		response.Body = string(compressed)
		response.Headers.Set("Content-Encoding", "gzip")
		response.Headers.Set("Vary", "Accept-Encoding")

		return response
	}

	return dataconv.WrapGoValue(middleware), nil
}

// shouldCompress checks if the content type should be compressed
func (c *CompressionMiddleware) shouldCompress(contentType string) bool {
	for _, t := range c.Types {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

// compressBody compresses the response body using gzip
func (c *CompressionMiddleware) compressBody(body string) ([]byte, error) {
	var compressed strings.Builder

	writer, err := gzip.NewWriterLevel(&compressed, c.Level)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write([]byte(body))
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return []byte(compressed.String()), nil
}

// SecurityHeadersConfig holds security headers configuration
type SecurityHeadersConfig struct {
	FrameOptions       string
	ContentTypeOptions string
	XSSProtection      string
	HSTS               string
	CSP                string
	ReferrerPolicy     string
}

// SecurityHeadersMiddleware adds security headers
type SecurityHeadersMiddleware struct {
	Config SecurityHeadersConfig
}

// NewSecurityHeadersMiddleware creates a new security headers middleware
func NewSecurityHeadersMiddleware(config SecurityHeadersConfig) *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{
		Config: config,
	}
}

// Middleware returns the security headers middleware function
func (s *SecurityHeadersMiddleware) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Process request
		response := next(req)

		// Add security headers
		if s.Config.FrameOptions != "" {
			response.Headers.Set("X-Frame-Options", s.Config.FrameOptions)
		}

		if s.Config.ContentTypeOptions != "" {
			response.Headers.Set("X-Content-Type-Options", s.Config.ContentTypeOptions)
		}

		if s.Config.XSSProtection != "" {
			response.Headers.Set("X-XSS-Protection", s.Config.XSSProtection)
		}

		if s.Config.HSTS != "" {
			response.Headers.Set("Strict-Transport-Security", s.Config.HSTS)
		}

		if s.Config.CSP != "" {
			response.Headers.Set("Content-Security-Policy", s.Config.CSP)
		}

		if s.Config.ReferrerPolicy != "" {
			response.Headers.Set("Referrer-Policy", s.Config.ReferrerPolicy)
		}

		return response
	}

	return dataconv.WrapGoValue(middleware), nil
}
