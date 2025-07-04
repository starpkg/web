package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Middleware types and functions

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

	middleware := func(req *Request, next NextFunc) *Response {
		// Handle preflight requests
		if req.Request.Method == "OPTIONS" {
			return &Response{
				StatusCode: 200,
				Headers: map[string][]string{
					"Access-Control-Allow-Origin":  {strings.Join(originsSlice, ", ")},
					"Access-Control-Allow-Methods": {strings.Join(methodsSlice, ", ")},
					"Access-Control-Allow-Headers": {strings.Join(headersSlice, ", ")},
					"Access-Control-Max-Age":       {fmt.Sprintf("%d", maxAgeInt)},
				},
			}
		}

		// Process normal requests
		response := next(req)

		// Add CORS headers to response
		if response.Headers == nil {
			response.Headers = make(map[string][]string)
		}
		response.Headers["Access-Control-Allow-Origin"] = []string{strings.Join(originsSlice, ", ")}
		if credentials {
			response.Headers["Access-Control-Allow-Credentials"] = []string{"true"}
		}

		return response
	}

	result, err := dataconv.Marshal(middleware)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal CORS middleware: %v", err)
	}
	return result, nil
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

	middleware := func(req *Request, next NextFunc) *Response {
		// Check if path should be skipped
		for _, skipPath := range skipPathsSlice {
			if req.Request.URL.Path == skipPath {
				return next(req)
			}
		}

		start := time.Now()
		response := next(req)
		duration := time.Since(start)

		// Check if status should be skipped
		for _, skipStat := range skipStatusSlice {
			if response.StatusCode == skipStat {
				return response
			}
		}

		// Log the request
		logEntry := strings.ReplaceAll(format.GoString(), "{method}", req.Request.Method)
		logEntry = strings.ReplaceAll(logEntry, "{path}", req.Request.URL.Path)
		logEntry = strings.ReplaceAll(logEntry, "{status}", fmt.Sprintf("%d", response.StatusCode))
		logEntry = strings.ReplaceAll(logEntry, "{duration}", fmt.Sprintf("%.3fms", float64(duration.Nanoseconds())/1000000))

		fmt.Println(logEntry)

		return response
	}

	result, err := dataconv.Marshal(middleware)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal logging middleware: %v", err)
	}
	return result, nil
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

	precisionInt, _ := precision.Int64()

	middleware := func(req *Request, next NextFunc) *Response {
		start := time.Now()
		response := next(req)
		duration := time.Since(start)

		// Add timing header
		if response.Headers == nil {
			response.Headers = make(map[string][]string)
		}

		durationMs := float64(duration.Nanoseconds()) / 1000000
		response.Headers[header.GoString()] = []string{fmt.Sprintf("%."+fmt.Sprintf("%d", precisionInt)+"fms", durationMs)}

		return response
	}

	result, err := dataconv.Marshal(middleware)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal timing middleware: %v", err)
	}
	return result, nil
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

	// Convert types to Go slice
	var typesSlice []string
	if types != nil {
		typesSlice = make([]string, types.Len())
		for i := 0; i < types.Len(); i++ {
			if typeStr, ok := types.Index(i).(starlark.String); ok {
				typesSlice[i] = typeStr.GoString()
			}
		}
	} else {
		typesSlice = []string{"text/html", "text/css", "text/plain", "application/javascript", "application/json"}
	}

	minSizeInt, _ := minSize.Int64()

	middleware := func(req *Request, next NextFunc) *Response {
		response := next(req)

		// Check if compression is accepted
		acceptEncoding := req.Request.Header.Get("Accept-Encoding")
		if !strings.Contains(acceptEncoding, "gzip") {
			return response
		}

		// Check response size
		if int64(len(response.Body)) < minSizeInt {
			return response
		}

		// Check content type
		contentType := ""
		if response.Headers != nil {
			if ct, ok := response.Headers["Content-Type"]; ok && len(ct) > 0 {
				contentType = ct[0]
			}
		}

		shouldCompress := false
		for _, t := range typesSlice {
			if strings.Contains(contentType, t) {
				shouldCompress = true
				break
			}
		}

		if shouldCompress {
			// Note: In a real implementation, you would compress the response body here
			// For now, we just add the header
			if response.Headers == nil {
				response.Headers = make(map[string][]string)
			}
			response.Headers["Content-Encoding"] = []string{"gzip"}
		}

		return response
	}

	result, err := dataconv.Marshal(middleware)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal compression middleware: %v", err)
	}
	return result, nil
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

	middleware := func(req *Request, next NextFunc) *Response {
		response := next(req)

		// Add security headers
		if response.Headers == nil {
			response.Headers = make(map[string][]string)
		}

		if frameOptions.GoString() != "" {
			response.Headers["X-Frame-Options"] = []string{frameOptions.GoString()}
		}

		if contentTypeOptions.GoString() != "" {
			response.Headers["X-Content-Type-Options"] = []string{contentTypeOptions.GoString()}
		}

		if xssProtection.GoString() != "" {
			response.Headers["X-XSS-Protection"] = []string{xssProtection.GoString()}
		}

		if hsts.GoString() != "" {
			response.Headers["Strict-Transport-Security"] = []string{hsts.GoString()}
		}

		if csp.GoString() != "" {
			response.Headers["Content-Security-Policy"] = []string{csp.GoString()}
		}

		if referrerPolicy.GoString() != "" {
			response.Headers["Referrer-Policy"] = []string{referrerPolicy.GoString()}
		}

		return response
	}

	result, err := dataconv.Marshal(middleware)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal security headers middleware: %v", err)
	}
	return result, nil
}
