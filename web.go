// Package web provides a Starlark module for server-side web applications.
// It offers a Flask-inspired API for building HTTP servers with routing, route
// groups, path-pattern middleware, request/response handling, pluggable
// authentication, and custom error handlers, built on top of gin.
package web

import (
	"fmt"
	"strings"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function
const ModuleName = "web"

// Configuration key constants
const (
	configKeyHost         = "host"
	configKeyPort         = "port"
	configKeyReadTimeout  = "read_timeout"
	configKeyWriteTimeout = "write_timeout"
	configKeyMaxBodySize  = "max_body_size"
	configKeyDebugMode    = "debug_mode"
	configKeyServerHeader = "server_header"
	// configKeyAllowPublicBind gates binding to a non-loopback (publicly
	// reachable) address. It defaults to false so a server started from an
	// untrusted script stays on localhost unless the host explicitly opts in.
	configKeyAllowPublicBind = "allow_public_bind"
)

var (
	none = starlark.None
)

// Module wraps the ConfigurableModule with specific functionality for web server
type Module struct {
	cfgMod *base.ConfigurableModule
	ext    *base.ConfigurableModuleExt
}

// NewModule creates a new instance of Module with default configurations.
// This is the primary entry point for creating a web module that can be loaded
// into a Starlark environment. The module provides functions for creating HTTP servers,
// handling requests and responses, and managing web application lifecycle.
func NewModule() *Module {
	return newModuleWithOptions(
		genConfigOption(configKeyHost, "Default host to bind to", "localhost"),
		genConfigOption(configKeyPort, "Default port to listen on", 8080),
		genConfigOption(configKeyReadTimeout, "Read timeout in seconds", 30),
		genConfigOption(configKeyWriteTimeout, "Write timeout in seconds", 30),
		genConfigOption(configKeyMaxBodySize, "Maximum request body size in bytes", int64(32<<20)), // 32MB
		genConfigOption(configKeyDebugMode, "Enable debug mode", false),
		genConfigOption(configKeyServerHeader, "Custom server header", "Starlark-Web/1.0"),
		genConfigOption(configKeyAllowPublicBind, "Allow binding to a non-loopback (public) address", false),
	)
}

// Helper functions

// genConfigOption creates a configuration option with common settings
func genConfigOption[T any](name, description string, defaultValue T) *base.ConfigOption[T] {
	return base.NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar(strings.ToUpper(ModuleName + "_" + name))
}

// newModuleWithOptions creates a Module with the given configuration options
func newModuleWithOptions(
	hostOpt *base.ConfigOption[string],
	portOpt *base.ConfigOption[int],
	readTimeoutOpt *base.ConfigOption[int],
	writeTimeoutOpt *base.ConfigOption[int],
	maxBodySizeOpt *base.ConfigOption[int64],
	debugModeOpt *base.ConfigOption[bool],
	serverHeaderOpt *base.ConfigOption[string],
	allowPublicBindOpt *base.ConfigOption[bool],
) *Module {
	cm, _ := base.NewConfigurableModuleWithConfigOptions(
		hostOpt,
		portOpt,
		readTimeoutOpt,
		writeTimeoutOpt,
		maxBodySizeOpt,
		debugModeOpt,
		serverHeaderOpt,
		allowPublicBindOpt,
	)
	return &Module{
		cfgMod: cm,
		ext:    cm.Extend(),
	}
}

// LoadModule returns the Starlark module loader with web-specific functions.
// This method provides the complete set of web module functions that can be
// called from Starlark scripts, including server creation and response builders.
func (m *Module) LoadModule() starlet.ModuleLoader {
	// Module functions
	additionalFuncs := starlark.StringDict{
		"create_server":  starlark.NewBuiltin(ModuleName+".create_server", m.createServer),
		"response":       starlark.NewBuiltin(ModuleName+".response", m.response),
		"json_response":  starlark.NewBuiltin(ModuleName+".json_response", m.jsonResponse),
		"html_response":  starlark.NewBuiltin(ModuleName+".html_response", m.htmlResponse),
		"text_response":  starlark.NewBuiltin(ModuleName+".text_response", m.textResponse),
		"file_response":  starlark.NewBuiltin(ModuleName+".file_response", m.fileResponse),
		"redirect":       starlark.NewBuiltin(ModuleName+".redirect", m.redirect),
		"error_response": starlark.NewBuiltin(ModuleName+".error_response", m.errorResponse),
		"send_file":      starlark.NewBuiltin(ModuleName+".send_file", m.sendFile),
		"send_data":      starlark.NewBuiltin(ModuleName+".send_data", m.sendData),
		"static_dir":     starlark.NewBuiltin(ModuleName+".static_dir", m.staticDir),
		// Authentication functions
		"api_key_auth": starlark.NewBuiltin(ModuleName+".api_key_auth", m.apiKeyAuth),
		"bearer_auth":  starlark.NewBuiltin(ModuleName+".bearer_auth", m.bearerAuth),
		"basic_auth":   starlark.NewBuiltin(ModuleName+".basic_auth", m.basicAuth),
		// Middleware functions
		"cors_middleware":             starlark.NewBuiltin(ModuleName+".cors_middleware", m.corsMiddleware),
		"logging_middleware":          starlark.NewBuiltin(ModuleName+".logging_middleware", m.loggingMiddleware),
		"security_headers_middleware": starlark.NewBuiltin(ModuleName+".security_headers_middleware", m.securityHeadersMiddleware),
		"timing_middleware":           starlark.NewBuiltin(ModuleName+".timing_middleware", m.timingMiddleware),
		"json_middleware":             starlark.NewBuiltin(ModuleName+".json_middleware", m.jsonMiddleware),
		"compression_middleware":      starlark.NewBuiltin(ModuleName+".compression_middleware", m.compressionMiddleware),
		"rate_limit_middleware":       starlark.NewBuiltin(ModuleName+".rate_limit_middleware", m.rateLimitMiddleware),
		"cache_middleware":            starlark.NewBuiltin(ModuleName+".cache_middleware", m.cacheMiddleware),
		"request_size_middleware":     starlark.NewBuiltin(ModuleName+".request_size_middleware", m.requestSizeMiddleware),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

// createServer is a Starlark function that creates a new HTTP server.
// It accepts optional host and port parameters and returns a ServerWrapper
// that provides Starlark-compatible methods for route registration and server lifecycle.
func (m *Module) createServer(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		host         = starlark.String("")
		port         = starlark.MakeInt(0)
		serverHeader = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"host?", &host,
		"port?", &port,
		"server_header?", &serverHeader,
	); err != nil {
		return none, err
	}

	// Get configuration values
	defaultHost := m.ext.GetString(configKeyHost)
	defaultPort := m.ext.GetInt(configKeyPort)

	// Use provided values or defaults
	serverHost := string(host)
	if serverHost == "" {
		serverHost = defaultHost
	}

	serverPort := defaultPort
	if port.Sign() != 0 {
		portInt, ok := port.Int64()
		if !ok {
			return none, fmt.Errorf("invalid port value")
		}
		serverPort = int(portInt)
	}

	// Validate port
	if serverPort <= 0 || serverPort > 65535 {
		return none, fmt.Errorf("port must be between 1 and 65535")
	}

	// Create server instance using the new constructor
	server := newServer(m, serverHost, serverPort)

	// Override server header if provided
	if string(serverHeader) != "" {
		server.serverHeader = string(serverHeader)
	}

	// Convert to Starlark value using wrapper
	return NewServerWrapper(server), nil
}

// Response builders

// response creates a basic HTTP response.
// This function constructs an HTTP response with the specified body, status code,
// and headers. It returns a ResponseWrapper that can be returned from route handlers.
func (m *Module) response(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		body    = starlark.String("")
		status  = starlark.MakeInt(200)
		headers = starlark.NewDict(0)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"body", &body,
		"status?", &status,
		"headers?", &headers,
	); err != nil {
		return none, err
	}

	statusCode, _ := status.Int64()
	headerMap := make(map[string]string)

	for _, item := range headers.Items() {
		key, ok := item[0].(starlark.String)
		if !ok {
			continue
		}
		value, ok := item[1].(starlark.String)
		if !ok {
			continue
		}
		headerMap[string(key)] = string(value)
	}

	response := &Response{
		StatusCode: int(statusCode),
		Headers:    headerMap,
		Body:       string(body),
	}
	return NewResponseWrapper(response), nil
}

// jsonResponse creates a JSON HTTP response.
// This function automatically serializes Starlark data structures to JSON
// and sets the appropriate Content-Type header. It handles various Starlark types
// including dicts, lists, strings, and numbers.
func (m *Module) jsonResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data    starlark.Value
		status  = starlark.MakeInt(200)
		headers = starlark.NewDict(0)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"data", &data,
		"status?", &status,
		"headers?", &headers,
	); err != nil {
		return none, err
	}

	// Convert data to JSON, if it's not a string or bytes, encode it to JSON
	var bodyStr string
	switch data.(type) {
	case starlark.String:
		bodyStr = string(data.(starlark.String))
	case starlark.Bytes:
		bodyStr = string(data.(starlark.Bytes))
	default:
		var err error
		if bodyStr, err = dataconv.EncodeStarlarkJSON(data); err != nil {
			return none, fmt.Errorf("failed to marshal JSON: %v", err)
		}
	}

	statusCode, _ := status.Int64()
	headerMap, err := starlarkDictToStringMap(headers)
	if err != nil {
		return none, fmt.Errorf("invalid headers: %v", err)
	}

	// Set content type
	headerMap[canonicalHeader(HeaderContentType)] = MIMEApplicationJSON

	response := &Response{
		StatusCode: int(statusCode),
		Headers:    headerMap,
		Body:       bodyStr,
	}
	return NewResponseWrapper(response), nil
}

// htmlResponse creates an HTML HTTP response
func (m *Module) htmlResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		content = starlark.String("")
		status  = starlark.MakeInt(200)
		headers = starlark.NewDict(0)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"content", &content,
		"status?", &status,
		"headers?", &headers,
	); err != nil {
		return none, err
	}

	statusCode, _ := status.Int64()
	headerMap, err := starlarkDictToStringMap(headers)
	if err != nil {
		return none, fmt.Errorf("invalid headers: %v", err)
	}

	// Set content type
	headerMap[canonicalHeader(HeaderContentType)] = MIMETextHTML

	response := &Response{
		StatusCode: int(statusCode),
		Headers:    headerMap,
		Body:       string(content),
	}

	return NewResponseWrapper(response), nil
}

// textResponse creates a text response
func (m *Module) textResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text   = starlark.String("")
		status = starlark.MakeInt(200)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &text,
		"status?", &status,
	); err != nil {
		return none, err
	}

	statusCode, _ := status.Int64()

	response := &Response{
		StatusCode: int(statusCode),
		Headers: map[string]string{
			canonicalHeader(HeaderContentType): MIMETextPlain,
		},
		Body: string(text),
	}

	return NewResponseWrapper(response), nil
}

// fileResponse creates a file response
func (m *Module) fileResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		filepath    = starlark.String("")
		contentType = starlark.String("")
		filename    = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"filepath", &filepath,
		"content_type?", &contentType,
		"filename?", &filename,
	); err != nil {
		return none, err
	}

	response := &Response{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       "",
		FilePath:   string(filepath),
	}

	if contentType != "" {
		response.Headers[canonicalHeader(HeaderContentType)] = string(contentType)
	}

	if filename != "" {
		response.Headers[canonicalHeader(HeaderContentDisposition)] = fmt.Sprintf("attachment; filename=%s", string(filename))
	}

	return NewResponseWrapper(response), nil
}

// redirect creates a redirect response
func (m *Module) redirect(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		location = starlark.String("")
		status   = starlark.MakeInt(302)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"location", &location,
		"status?", &status,
	); err != nil {
		return none, err
	}

	statusCode, _ := status.Int64()

	response := &Response{
		StatusCode: int(statusCode),
		Headers: map[string]string{
			canonicalHeader(HeaderLocation): string(location),
		},
		Body: "",
	}

	return NewResponseWrapper(response), nil
}

// errorResponse creates an error response
func (m *Module) errorResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		status  = starlark.MakeInt(500)
		message = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"status", &status,
		"message?", &message,
	); err != nil {
		return none, err
	}

	statusCode, _ := status.Int64()

	response := &Response{
		StatusCode: int(statusCode),
		Headers:    map[string]string{},
		Body:       string(message),
	}

	return NewResponseWrapper(response), nil
}

// sendFile creates a file response
func (m *Module) sendFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		filepath    = starlark.String("")
		contentType = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"filepath", &filepath,
		"content_type?", &contentType,
	); err != nil {
		return none, err
	}

	response := &Response{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       "",
		FilePath:   string(filepath),
	}

	if contentType != "" {
		response.Headers[canonicalHeader(HeaderContentType)] = string(contentType)
	}

	return NewResponseWrapper(response), nil
}

// sendData creates a data response
func (m *Module) sendData(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data        = starlark.String("")
		filename    = starlark.String("")
		contentType = starlark.String(MIMEApplicationOctetStream)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"data", &data,
		"filename", &filename,
		"content_type?", &contentType,
	); err != nil {
		return none, err
	}

	response := &Response{
		StatusCode: 200,
		Headers: map[string]string{
			canonicalHeader(HeaderContentType):        string(contentType),
			canonicalHeader(HeaderContentDisposition): fmt.Sprintf("attachment; filename=%s", string(filename)),
		},
		Body: string(data),
	}

	return NewResponseWrapper(response), nil
}

// Authentication functions

// apiKeyAuth creates an API key authenticator
func (m *Module) apiKeyAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		keys       = starlark.NewList(nil)
		header     = starlark.String(HeaderAPIKey)
		queryParam = starlark.String("api_key")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"keys?", &keys,
		"header?", &header,
		"query_param?", &queryParam,
	); err != nil {
		return none, err
	}

	// Convert Starlark list to Go slice
	keySlice, err := starlarkListToStringSlice(keys)
	if err != nil {
		return none, fmt.Errorf("invalid keys: %v", err)
	}

	auth := &Authenticator{
		authType: "api_key",
		config: map[string]interface{}{
			"keys":        keySlice,
			"header":      canonicalHeader(string(header)),
			"query_param": string(queryParam),
		},
	}

	return NewAuthenticatorWrapper(auth), nil
}

// bearerAuth creates a bearer token authenticator
func (m *Module) bearerAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		validateFunc starlark.Callable
		header       = starlark.String(HeaderAuthorization)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"validate_func", &validateFunc,
		"header?", &header,
	); err != nil {
		return none, err
	}

	auth := &Authenticator{
		validateFunc: validateFunc,
		authType:     "bearer",
		config: map[string]interface{}{
			"header": canonicalHeader(string(header)),
		},
	}

	return NewAuthenticatorWrapper(auth), nil
}

// basicAuth creates a basic authenticator
func (m *Module) basicAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		users = starlark.NewDict(0)
		realm = starlark.String("Restricted")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"users?", &users,
		"realm?", &realm,
	); err != nil {
		return none, err
	}

	// Convert Starlark dict to Go map
	userMap, err := starlarkDictToStringMap(users)
	if err != nil {
		return none, fmt.Errorf("invalid users: %v", err)
	}

	auth := &Authenticator{
		authType: "basic",
		config: map[string]interface{}{
			"users": userMap,
			"realm": string(realm),
		},
	}

	return NewAuthenticatorWrapper(auth), nil
}

// Middleware functions

// corsMiddleware creates a CORS middleware
func (m *Module) corsMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		origins     = starlark.NewList(nil)
		methods     = starlark.NewList(nil)
		headers     = starlark.NewList(nil)
		credentials = starlark.Bool(false)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"origins?", &origins,
		"methods?", &methods,
		"headers?", &headers,
		"credentials?", &credentials,
	); err != nil {
		return none, err
	}

	// Convert lists to slices
	originsSlice, err := starlarkListToStringSlice(origins)
	if err != nil {
		return none, fmt.Errorf("invalid origins: %v", err)
	}

	methodsSlice, err := starlarkListToStringSlice(methods)
	if err != nil {
		return none, fmt.Errorf("invalid methods: %v", err)
	}

	headersSlice, err := starlarkListToStringSlice(headers)
	if err != nil {
		return none, fmt.Errorf("invalid headers: %v", err)
	}

	middleware := corsMiddleware(originsSlice, methodsSlice, headersSlice, bool(credentials))
	return NewMiddlewareWrapper(middleware), nil
}

// loggingMiddleware creates a logging middleware
func (m *Module) loggingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var format = starlark.String("")

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"format?", &format,
	); err != nil {
		return none, err
	}

	middleware := loggingMiddleware(string(format))
	return NewMiddlewareWrapper(middleware), nil
}

// securityHeadersMiddleware creates a security headers middleware
func (m *Module) securityHeadersMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		frameOptions       = starlark.String("DENY")
		contentTypeOptions = starlark.String("nosniff")
		xssProtection      = starlark.String("1; mode=block")
		hsts               = starlark.String("")
		csp                = starlark.String("")
		referrerPolicy     = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"frame_options?", &frameOptions,
		"content_type_options?", &contentTypeOptions,
		"xss_protection?", &xssProtection,
		"hsts?", &hsts,
		"csp?", &csp,
		"referrer_policy?", &referrerPolicy,
	); err != nil {
		return none, err
	}

	config := make(map[string]string)
	if string(frameOptions) != "" {
		config["X-Frame-Options"] = string(frameOptions)
	}
	if string(contentTypeOptions) != "" {
		config["X-Content-Type-Options"] = string(contentTypeOptions)
	}
	if string(xssProtection) != "" {
		config["X-XSS-Protection"] = string(xssProtection)
	}
	if string(hsts) != "" {
		config["Strict-Transport-Security"] = string(hsts)
	}
	if string(csp) != "" {
		config["Content-Security-Policy"] = string(csp)
	}
	if string(referrerPolicy) != "" {
		config["Referrer-Policy"] = string(referrerPolicy)
	}

	middleware := securityHeadersMiddleware(config)
	return NewMiddlewareWrapper(middleware), nil
}

// timingMiddleware creates a timing middleware
func (m *Module) timingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var header = starlark.String("X-Response-Time")

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"header?", &header,
	); err != nil {
		return none, err
	}

	middleware := timingMiddleware(string(header))
	return NewMiddlewareWrapper(middleware), nil
}

// jsonMiddleware creates a JSON middleware
func (m *Module) jsonMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return none, err
	}

	middleware := jsonMiddleware()
	return NewMiddlewareWrapper(middleware), nil
}

// compressionMiddleware creates a compression middleware
func (m *Module) compressionMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		level   = starlark.MakeInt(6)
		minSize = starlark.MakeInt(1024)
		types   = starlark.NewList(nil)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"level?", &level,
		"min_size?", &minSize,
		"types?", &types,
	); err != nil {
		return none, err
	}

	// Convert parameters
	levelInt, err := starlarkIntToInt(level)
	if err != nil {
		return none, fmt.Errorf("invalid level: %v", err)
	}

	minSizeInt, err := starlarkIntToInt(minSize)
	if err != nil {
		return none, fmt.Errorf("invalid min_size: %v", err)
	}

	typesSlice, err := starlarkListToStringSlice(types)
	if err != nil {
		return none, fmt.Errorf("invalid types: %v", err)
	}

	middleware := compressionMiddleware(levelInt, minSizeInt, typesSlice)
	return NewMiddlewareWrapper(middleware), nil
}

// rateLimitMiddleware creates a rate limiting middleware
func (m *Module) rateLimitMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		requests                   = starlark.MakeInt(100)
		window                     = starlark.MakeInt(60)
		keyFunc  starlark.Callable = nil
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"requests?", &requests,
		"window?", &window,
		"key_func?", &keyFunc,
	); err != nil {
		return none, err
	}

	// Convert parameters
	requestsInt, err := starlarkIntToInt(requests)
	if err != nil {
		return none, fmt.Errorf("invalid requests: %v", err)
	}

	windowInt, err := starlarkIntToInt(window)
	if err != nil {
		return none, fmt.Errorf("invalid window: %v", err)
	}

	// Create key function wrapper
	var goKeyFunc func(*Request) string
	if keyFunc != nil {
		goKeyFunc = func(req *Request) string {
			thread := &starlark.Thread{Name: "rate_limit_key"}
			reqWrapper := NewRequestWrapper(req)
			args := starlark.Tuple{reqWrapper}
			result, err := starlark.Call(thread, keyFunc, args, nil)
			if err != nil {
				return req.ClientIP // fallback to client IP
			}
			if str, ok := result.(starlark.String); ok {
				return string(str)
			}
			return req.ClientIP
		}
	}

	middleware := rateLimitMiddleware(requestsInt, windowInt, goKeyFunc, nil)
	return NewMiddlewareWrapper(middleware), nil
}

// cacheMiddleware creates a response caching middleware
func (m *Module) cacheMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		maxAge   = starlark.MakeInt(3600)
		private  = starlark.Bool(false)
		patterns = starlark.NewList(nil)
		vary     = starlark.NewList(nil)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"max_age?", &maxAge,
		"private?", &private,
		"patterns?", &patterns,
		"vary?", &vary,
	); err != nil {
		return none, err
	}

	// Convert parameters
	maxAgeInt, err := starlarkIntToInt(maxAge)
	if err != nil {
		return none, fmt.Errorf("invalid max_age: %v", err)
	}

	privateBool, err := starlarkBoolToBool(private)
	if err != nil {
		return none, fmt.Errorf("invalid private: %v", err)
	}

	patternsSlice, err := starlarkListToStringSlice(patterns)
	if err != nil {
		return none, fmt.Errorf("invalid patterns: %v", err)
	}

	varySlice, err := starlarkListToStringSlice(vary)
	if err != nil {
		return none, fmt.Errorf("invalid vary: %v", err)
	}

	middleware := cacheMiddleware(maxAgeInt, privateBool, patternsSlice, varySlice)
	return NewMiddlewareWrapper(middleware), nil
}

// requestSizeMiddleware creates a request size limiting middleware
func (m *Module) requestSizeMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		maxContentLength = starlark.MakeInt(10 * 1024 * 1024) // 10MB
		maxURLLength     = starlark.MakeInt(2048)
		maxHeaders       = starlark.MakeInt(100)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"max_content_length?", &maxContentLength,
		"max_url_length?", &maxURLLength,
		"max_headers?", &maxHeaders,
	); err != nil {
		return none, err
	}

	// Convert parameters
	maxContentLengthInt64, ok := maxContentLength.Int64()
	if !ok {
		return none, fmt.Errorf("invalid max_content_length")
	}

	maxURLLengthInt, err := starlarkIntToInt(maxURLLength)
	if err != nil {
		return none, fmt.Errorf("invalid max_url_length: %v", err)
	}

	maxHeadersInt, err := starlarkIntToInt(maxHeaders)
	if err != nil {
		return none, fmt.Errorf("invalid max_headers: %v", err)
	}

	middleware := requestSizeMiddleware(maxContentLengthInt64, maxURLLengthInt, maxHeadersInt)
	return NewMiddlewareWrapper(middleware), nil
}
