package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlight/convert"
	"go.starlark.net/starlark"
)

// Middleware types and functions

// pathPattern represents a pattern for path-specific middleware
type pathPattern struct {
	pattern string
	regex   *regexp.Regexp
}

// newPathPattern creates a new path pattern for middleware matching
func newPathPattern(pattern string) (*pathPattern, error) {
	// Convert wildcard patterns to regex
	regexStr := "^"
	if strings.HasSuffix(pattern, "/*") {
		// Handle /path/* wildcard
		regexStr += strings.Replace(pattern, "/*", "(/.*)?", 1)
	} else if strings.HasSuffix(pattern, "*") {
		// Handle path* wildcard
		regexStr += strings.Replace(pattern, "*", ".*", 1)
	} else {
		// Handle exact path match
		regexStr += pattern
	}
	regexStr += "$"

	regex, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, err
	}

	return &pathPattern{
		pattern: pattern,
		regex:   regex,
	}, nil
}

// matches checks if a path matches the pattern
func (p *pathPattern) matches(path string) bool {
	return p.regex.MatchString(path)
}

// applyMiddleware applies a middleware to the handler chain
func applyMiddleware(handler HandlerFunc, middleware MiddlewareFunc) HandlerFunc {
	return func(req *Request) *Response {
		// Convert handler to NextFunc
		nextFunc := func(r *Request) *Response {
			return handler(r)
		}
		return middleware(req, nextFunc)
	}
}

// callStarlarkMiddleware calls a Starlark middleware function
func callStarlarkMiddleware(thread *starlark.Thread, middleware starlark.Callable, req *Request, next NextFunc) (*Response, error) {
	// Convert request to Starlark
	reqValue, err := convert.ToValue(req)
	if err != nil {
		return &Response{
			StatusCode: 500,
			Headers:    make(http.Header),
			Body:       fmt.Sprintf("Failed to convert request for middleware: %v", err),
		}, err
	}

	// Create a Starlark function for the next handler
	nextFunc := starlark.NewBuiltin("next", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if len(args) > 0 {
			// If args are provided, the middleware is modifying the request
			modifiedReq, err := dataconv.Unmarshal(args[0])
			if err != nil {
				return starlark.None, fmt.Errorf("invalid request object passed to next: %v", err)
			}

			if modReq, ok := modifiedReq.(*Request); ok {
				response := next(modReq)
				respValue, err := convert.ToValue(response)
				if err != nil {
					return starlark.None, fmt.Errorf("failed to convert response: %v", err)
				}
				return respValue, nil
			}
			return starlark.None, fmt.Errorf("invalid request object type")
		}

		// Default: call next with original request
		response := next(req)
		respValue, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}
		return respValue, nil
	})

	// Call the middleware with request and next function
	result, err := starlark.Call(thread, middleware, starlark.Tuple{reqValue, nextFunc}, nil)
	if err != nil {
		return nil, err
	}

	// Convert result to Response
	responseObj, err := dataconv.Unmarshal(result)
	if err != nil {
		return nil, err
	}

	if resp, ok := responseObj.(*Response); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("middleware returned invalid response type")
}

// wrapStarlarkMiddleware wraps a Starlark middleware function
func wrapStarlarkMiddleware(middleware starlark.Callable) MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		resp, err := callStarlarkMiddleware(&starlark.Thread{}, middleware, req, next)
		if err != nil {
			// If middleware execution fails, return an error response
			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       fmt.Sprintf("Middleware error: %v", err),
			}
		}
		return resp
	}
}

// Built-in middleware functions

// corsMiddleware creates a CORS middleware
func corsMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		origins     *starlark.List
		methods     *starlark.List
		headers     *starlark.List
		credentials = starlark.Bool(false)
		maxAge      = starlark.MakeInt(86400)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"origins?", &origins,
		"methods?", &methods,
		"headers?", &headers,
		"credentials?", &credentials,
		"max_age?", &maxAge,
	); err != nil {
		return starlark.None, err
	}

	// Convert origins to Go slice
	var originsSlice []string
	if origins != nil {
		originsSlice = make([]string, origins.Len())
		for i := 0; i < origins.Len(); i++ {
			if originStr, ok := origins.Index(i).(starlark.String); ok {
				originsSlice[i] = originStr.GoString()
			}
		}
	} else {
		originsSlice = []string{"*"}
	}

	// Convert methods to Go slice
	var methodsSlice []string
	if methods != nil {
		methodsSlice = make([]string, methods.Len())
		for i := 0; i < methods.Len(); i++ {
			if methodStr, ok := methods.Index(i).(starlark.String); ok {
				methodsSlice[i] = methodStr.GoString()
			}
		}
	} else {
		methodsSlice = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"}
	}

	// Convert headers to Go slice
	var headersSlice []string
	if headers != nil {
		headersSlice = make([]string, headers.Len())
		for i := 0; i < headers.Len(); i++ {
			if headerStr, ok := headers.Index(i).(starlark.String); ok {
				headersSlice[i] = headerStr.GoString()
			}
		}
	} else {
		headersSlice = []string{"Content-Type", "Authorization"}
	}

	maxAgeInt, _ := maxAge.Int64()

	// Create the actual middleware function
	corsMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Handle preflight requests
		if req.Request.Method == "OPTIONS" {
			headers := make(http.Header)

			// Set allowed origins
			if len(originsSlice) == 1 && originsSlice[0] == "*" {
				headers["Access-Control-Allow-Origin"] = []string{"*"}
			} else {
				origin := req.Request.Header.Get("Origin")
				for _, allowedOrigin := range originsSlice {
					if origin == allowedOrigin {
						headers["Access-Control-Allow-Origin"] = []string{origin}
						break
					}
				}
			}

			// Set other CORS headers
			headers["Access-Control-Allow-Methods"] = []string{strings.Join(methodsSlice, ", ")}
			headers["Access-Control-Allow-Headers"] = []string{strings.Join(headersSlice, ", ")}
			headers["Access-Control-Max-Age"] = []string{fmt.Sprintf("%d", maxAgeInt)}

			if bool(credentials) {
				headers["Access-Control-Allow-Credentials"] = []string{"true"}
			}

			return &Response{
				StatusCode: http.StatusOK,
				Headers:    headers,
				Body:       "",
			}
		}

		// Handle normal requests
		resp := next(req)

		// Set CORS headers on the response
		origin := req.Request.Header.Get("Origin")
		if resp.Headers == nil {
			resp.Headers = make(http.Header)
		}

		if len(originsSlice) == 1 && originsSlice[0] == "*" {
			resp.Headers["Access-Control-Allow-Origin"] = []string{"*"}
		} else if origin != "" {
			for _, allowedOrigin := range originsSlice {
				if origin == allowedOrigin {
					resp.Headers["Access-Control-Allow-Origin"] = []string{origin}
					break
				}
			}
		}

		if bool(credentials) {
			resp.Headers["Access-Control-Allow-Credentials"] = []string{"true"}
		}

		return resp
	}

	return starlark.NewBuiltin("cors_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// This function will be called by srv.use() with request and next_handler
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result to Response
			respObj, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Invalid response from next handler: %v", err),
				}
			}

			if resp, ok := respObj.(*Response); ok {
				return resp
			}

			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       "Next handler returned invalid response type",
			}
		}

		// Call the actual middleware function
		response := corsMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}

		return result, nil
	}), nil
}

// loggingMiddleware creates a logging middleware
func loggingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		format     = starlark.String("{method} {path} {status} {duration}")
		skipPaths  *starlark.List
		skipStatus *starlark.List
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"format?", &format,
		"skip_paths?", &skipPaths,
		"skip_status?", &skipStatus,
	); err != nil {
		return starlark.None, err
	}

	// Convert skip paths to Go slice
	var skipPathsSlice []string
	if skipPaths != nil {
		skipPathsSlice = make([]string, skipPaths.Len())
		for i := 0; i < skipPaths.Len(); i++ {
			if pathStr, ok := skipPaths.Index(i).(starlark.String); ok {
				skipPathsSlice[i] = pathStr.GoString()
			}
		}
	}

	// Convert skip status to Go slice
	var skipStatusSlice []int
	if skipStatus != nil {
		skipStatusSlice = make([]int, skipStatus.Len())
		for i := 0; i < skipStatus.Len(); i++ {
			if statusInt, ok := skipStatus.Index(i).(starlark.Int); ok {
				if val, ok := statusInt.Int64(); ok {
					skipStatusSlice[i] = int(val)
				}
			}
		}
	}

	formatStr := format.GoString()

	// Create the actual middleware function
	loggingMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Check if path should be skipped
		for _, skipPath := range skipPathsSlice {
			if strings.HasPrefix(req.Request.URL.Path, skipPath) {
				return next(req)
			}
		}

		// Start timer
		start := time.Now()

		// Process request
		resp := next(req)

		// Check if status should be skipped
		for _, skipStat := range skipStatusSlice {
			if resp.StatusCode == skipStat {
				return resp
			}
		}

		// Calculate duration
		duration := time.Since(start)

		// Format log message
		log := formatStr
		log = strings.Replace(log, "{method}", req.Request.Method, -1)
		log = strings.Replace(log, "{path}", req.Request.URL.Path, -1)
		log = strings.Replace(log, "{status}", fmt.Sprintf("%d", resp.StatusCode), -1)
		log = strings.Replace(log, "{duration}", fmt.Sprintf("%.3f", float64(duration.Microseconds())/1000.0), -1)
		log = strings.Replace(log, "{size}", fmt.Sprintf("%d", len(resp.Body)), -1)

		// Print log (in real implementation this should use a configurable logger)
		fmt.Println(log)

		return resp
	}

	// Return a Starlark builtin that implements the middleware
	return starlark.NewBuiltin("logging_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result to Response
			respObj, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Invalid response from next handler: %v", err),
				}
			}

			if resp, ok := respObj.(*Response); ok {
				return resp
			}

			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       "Next handler returned invalid response type",
			}
		}

		// Call the actual middleware function
		response := loggingMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}

		return result, nil
	}), nil
}

// timingMiddleware creates a timing middleware that adds response time header
func timingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		header    = starlark.String("X-Response-Time")
		precision = starlark.MakeInt(3)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"header?", &header,
		"precision?", &precision,
	); err != nil {
		return starlark.None, err
	}

	headerName := header.GoString()
	precisionInt, _ := precision.Int64()

	// Create the actual middleware function
	timingMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Start timer
		start := time.Now()

		// Process request
		resp := next(req)

		// Calculate duration
		duration := time.Since(start)

		// Add timing header
		if resp.Headers == nil {
			resp.Headers = make(http.Header)
		}

		// Format with specified precision
		format := fmt.Sprintf("%%.%df", precisionInt)
		timeValue := fmt.Sprintf(format, float64(duration.Microseconds())/1000.0)

		resp.Headers[headerName] = []string{timeValue + "ms"}

		return resp
	}

	// Return a Starlark builtin that implements the middleware
	return starlark.NewBuiltin("timing_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result to Response
			respObj, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Invalid response from next handler: %v", err),
				}
			}

			if resp, ok := respObj.(*Response); ok {
				return resp
			}

			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       "Next handler returned invalid response type",
			}
		}

		// Call the actual middleware function
		response := timingMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}

		return result, nil
	}), nil
}

// compressionMiddleware creates a compression middleware
func compressionMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		level   = starlark.MakeInt(6)
		minSize = starlark.MakeInt(1024)
		types   *starlark.List
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"level?", &level,
		"min_size?", &minSize,
		"types?", &types,
	); err != nil {
		return starlark.None, err
	}

	levelInt, _ := level.Int64()
	minSizeInt, _ := minSize.Int64()

	// Convert content types to Go slice
	var typesSlice []string
	if types != nil {
		typesSlice = make([]string, types.Len())
		for i := 0; i < types.Len(); i++ {
			if typeStr, ok := types.Index(i).(starlark.String); ok {
				typesSlice[i] = typeStr.GoString()
			}
		}
	} else {
		// Default content types to compress
		typesSlice = []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/javascript",
			"application/json",
			"application/xml",
		}
	}

	// Create the actual middleware function
	compressionMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Check if the client accepts gzip encoding
		acceptEncoding := req.Request.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		if !supportsGzip {
			// If client doesn't support gzip, skip compression
			return next(req)
		}

		// Process request
		resp := next(req)

		// Don't compress if response is too small
		if len(resp.Body) < int(minSizeInt) {
			return resp
		}

		// Check content type
		contentType := ""
		if resp.Headers != nil {
			if values, ok := resp.Headers["Content-Type"]; ok && len(values) > 0 {
				contentType = values[0]
			}
		}

		shouldCompress := false
		for _, t := range typesSlice {
			if strings.HasPrefix(contentType, t) {
				shouldCompress = true
				break
			}
		}

		if !shouldCompress {
			return resp
		}

		// Compress response body
		var b bytes.Buffer
		gz, err := gzip.NewWriterLevel(&b, int(levelInt))
		if err != nil {
			// Fallback to default compression if level is invalid
			gz = gzip.NewWriter(&b)
		}

		if _, err := gz.Write([]byte(resp.Body)); err != nil {
			// If compression fails, return uncompressed
			return resp
		}

		if err := gz.Close(); err != nil {
			// If compression fails, return uncompressed
			return resp
		}

		// Update response
		if resp.Headers == nil {
			resp.Headers = make(http.Header)
		}

		resp.Headers["Content-Encoding"] = []string{"gzip"}
		resp.Headers["Vary"] = []string{"Accept-Encoding"}
		resp.Body = b.String()

		return resp
	}

	// Return a Starlark builtin that implements the middleware
	return starlark.NewBuiltin("compression_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result to Response
			respObj, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Invalid response from next handler: %v", err),
				}
			}

			if resp, ok := respObj.(*Response); ok {
				return resp
			}

			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       "Next handler returned invalid response type",
			}
		}

		// Call the actual middleware function
		response := compressionMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}

		return result, nil
	}), nil
}

// securityHeadersMiddleware creates a security headers middleware
func securityHeadersMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		frameOptions       = starlark.String("DENY")
		contentTypeOptions = starlark.String("nosniff")
		xssProtection      = starlark.String("1; mode=block")
		hsts               = starlark.String("")
		csp                = starlark.String("")
		referrerPolicy     = starlark.String("strict-origin-when-cross-origin")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"frame_options?", &frameOptions,
		"content_type_options?", &contentTypeOptions,
		"xss_protection?", &xssProtection,
		"hsts?", &hsts,
		"csp?", &csp,
		"referrer_policy?", &referrerPolicy,
	); err != nil {
		return starlark.None, err
	}

	frameOpt := frameOptions.GoString()
	contentTypeOpt := contentTypeOptions.GoString()
	xssOpt := xssProtection.GoString()
	hstsOpt := hsts.GoString()
	cspOpt := csp.GoString()
	referrerOpt := referrerPolicy.GoString()

	// Create the actual middleware function
	securityHeadersMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Process request first
		resp := next(req)

		// Add security headers to response
		if resp.Headers == nil {
			resp.Headers = make(http.Header)
		}

		// Set security headers if configured
		if frameOpt != "" {
			resp.Headers["X-Frame-Options"] = []string{frameOpt}
		}

		if contentTypeOpt != "" {
			resp.Headers["X-Content-Type-Options"] = []string{contentTypeOpt}
		}

		if xssOpt != "" {
			resp.Headers["X-XSS-Protection"] = []string{xssOpt}
		}

		if hstsOpt != "" {
			resp.Headers["Strict-Transport-Security"] = []string{hstsOpt}
		}

		if cspOpt != "" {
			resp.Headers["Content-Security-Policy"] = []string{cspOpt}
		}

		if referrerOpt != "" {
			resp.Headers["Referrer-Policy"] = []string{referrerOpt}
		}

		return resp
	}

	// Return a Starlark builtin that implements the middleware
	return starlark.NewBuiltin("security_headers_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result to Response
			respObj, err := dataconv.Unmarshal(result)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Invalid response from next handler: %v", err),
				}
			}

			if resp, ok := respObj.(*Response); ok {
				return resp
			}

			return &Response{
				StatusCode: 500,
				Headers:    make(http.Header),
				Body:       "Next handler returned invalid response type",
			}
		}

		// Call the actual middleware function
		response := securityHeadersMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result, err := convert.ToValue(response)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert response: %v", err)
		}

		return result, nil
	}), nil
}
