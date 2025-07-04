package web

import (
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

	// Return a Starlark builtin that can be used directly as middleware
	return starlark.NewBuiltin("cors_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// This function will be called by srv.use() with request and next_handler
		// For now, return a simple placeholder
		return starlark.None, nil
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

	// Return a Starlark builtin placeholder
	return starlark.NewBuiltin("logging_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
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

	// Return a Starlark builtin placeholder
	return starlark.NewBuiltin("timing_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
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

	// Return a Starlark builtin placeholder
	return starlark.NewBuiltin("compression_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
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

	// Return a Starlark builtin placeholder
	return starlark.NewBuiltin("security_headers_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	}), nil
}
