// Package web provides a Starlark module for server-side web application development.
package web

import (
	"fmt"
	"time"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function
const ModuleName = "web"

// Configuration key constants
const (
	configKeyHost              = "host"
	configKeyPort              = "port"
	configKeyReadTimeout       = "read_timeout"
	configKeyWriteTimeout      = "write_timeout"
	configKeyMaxBodySize       = "max_body_size"
	configKeyEnableCORS        = "enable_cors"
	configKeyCORSOrigins       = "cors_origins"
	configKeyEnableCompression = "enable_compression"
	configKeyStaticCacheMaxAge = "static_cache_max_age"
)

var (
	none  = starlark.None
	empty = ""
)

// Module wraps the ConfigurableModule with specific functionality for web server management
type Module struct {
	cfgMod *base.ConfigurableModule
	ext    *base.ConfigurableModuleExt
}

// NewModule creates a new instance of Module with default configurations
func NewModule() *Module {
	return newModuleWithOptions(
		genConfigOption(configKeyHost, "Host address to bind to", "localhost"),
		genConfigOption(configKeyPort, "Port number to listen on", 8080),
		genConfigOption(configKeyReadTimeout, "Read timeout in seconds", 30),
		genConfigOption(configKeyWriteTimeout, "Write timeout in seconds", 30),
		genConfigOption(configKeyMaxBodySize, "Maximum request body size in bytes", int64(32*1024*1024)), // 32MB
		genConfigOption(configKeyEnableCORS, "Enable CORS middleware", false),
		genConfigOption(configKeyCORSOrigins, "CORS allowed origins", []string{"*"}),
		genConfigOption(configKeyEnableCompression, "Enable response compression", true),
		genConfigOption(configKeyStaticCacheMaxAge, "Static file cache max age in seconds", 3600),
	)
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values
func NewModuleWithConfig(host string, port int, readTimeout, writeTimeout int, maxBodySize int64, enableCORS bool, corsOrigins []string, enableCompression bool, staticCacheMaxAge int) *Module {
	return newModuleWithOptions(
		genConfigOption(configKeyHost, "Host address with preset value", host),
		genConfigOption(configKeyPort, "Port number with preset value", port),
		genConfigOption(configKeyReadTimeout, "Read timeout with preset value", readTimeout),
		genConfigOption(configKeyWriteTimeout, "Write timeout with preset value", writeTimeout),
		genConfigOption(configKeyMaxBodySize, "Maximum body size with preset value", maxBodySize),
		genConfigOption(configKeyEnableCORS, "Enable CORS with preset value", enableCORS),
		genConfigOption(configKeyCORSOrigins, "CORS origins with preset value", corsOrigins),
		genConfigOption(configKeyEnableCompression, "Enable compression with preset value", enableCompression),
		genConfigOption(configKeyStaticCacheMaxAge, "Static cache max age with preset value", staticCacheMaxAge),
	)
}

// Helper functions

// genConfigOption creates a configuration option with common settings
func genConfigOption[T any](name, description string, defaultValue T) *base.ConfigOption[T] {
	return base.NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar(fmt.Sprintf("WEB_%s", name))
}

// newModuleWithOptions creates a Module with the given configuration options
func newModuleWithOptions(
	hostOpt *base.ConfigOption[string],
	portOpt *base.ConfigOption[int],
	readTimeoutOpt *base.ConfigOption[int],
	writeTimeoutOpt *base.ConfigOption[int],
	maxBodySizeOpt *base.ConfigOption[int64],
	enableCORSOpt *base.ConfigOption[bool],
	corsOriginsOpt *base.ConfigOption[[]string],
	enableCompressionOpt *base.ConfigOption[bool],
	staticCacheMaxAgeOpt *base.ConfigOption[int],
) *Module {
	cm, _ := base.NewConfigurableModuleWithConfigOptions(
		hostOpt,
		portOpt,
		readTimeoutOpt,
		writeTimeoutOpt,
		maxBodySizeOpt,
		enableCORSOpt,
		corsOriginsOpt,
		enableCompressionOpt,
		staticCacheMaxAgeOpt,
	)
	return &Module{
		cfgMod: cm,
		ext:    cm.Extend(),
	}
}

// LoadModule returns the Starlark module loader with web-specific functions
func (m *Module) LoadModule() starlet.ModuleLoader {
	// Core web server functions
	additionalFuncs := starlark.StringDict{
		"create_server":          starlark.NewBuiltin(ModuleName+".create_server", m.createServer),
		"create_session_manager": starlark.NewBuiltin(ModuleName+".create_session_manager", m.createSessionManager),
		"response":               starlark.NewBuiltin(ModuleName+".response", m.response),
		"json_response":          starlark.NewBuiltin(ModuleName+".json_response", m.jsonResponse),
		"html_response":          starlark.NewBuiltin(ModuleName+".html_response", m.htmlResponse),
		"redirect":               starlark.NewBuiltin(ModuleName+".redirect", m.redirect),
		"error_response":         starlark.NewBuiltin(ModuleName+".error_response", m.errorResponse),
		"send_file":              starlark.NewBuiltin(ModuleName+".send_file", m.sendFile),
		"send_data":              starlark.NewBuiltin(ModuleName+".send_data", m.sendData),
		"basic_auth":             starlark.NewBuiltin(ModuleName+".basic_auth", m.basicAuth),
		"bearer_auth":            starlark.NewBuiltin(ModuleName+".bearer_auth", m.bearerAuth),
		"api_key_auth":           starlark.NewBuiltin(ModuleName+".api_key_auth", m.apiKeyAuth),

		// Built-in middleware functions
		"cors_middleware":             starlark.NewBuiltin(ModuleName+".cors_middleware", m.corsMiddleware),
		"logging_middleware":          starlark.NewBuiltin(ModuleName+".logging_middleware", m.loggingMiddleware),
		"timing_middleware":           starlark.NewBuiltin(ModuleName+".timing_middleware", m.timingMiddleware),
		"compression_middleware":      starlark.NewBuiltin(ModuleName+".compression_middleware", m.compressionMiddleware),
		"security_headers_middleware": starlark.NewBuiltin(ModuleName+".security_headers_middleware", m.securityHeadersMiddleware),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

// createServer creates a new HTTP server instance
func (m *Module) createServer(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Parse arguments
	var (
		host = starlark.String(m.ext.GetString(configKeyHost, "localhost"))
		port = starlark.MakeInt(m.ext.GetInt(configKeyPort, 8080))
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"host?", &host,
		"port?", &port,
	); err != nil {
		return none, err
	}

	// Convert port to integer
	portInt, ok := port.Int64()
	if !ok {
		return none, fmt.Errorf("port must be an integer")
	}

	// Create server configuration
	config := &ServerConfig{
		Host:              host.GoString(),
		Port:              int(portInt),
		ReadTimeout:       time.Duration(m.ext.GetInt(configKeyReadTimeout, 30)) * time.Second,
		WriteTimeout:      time.Duration(m.ext.GetInt(configKeyWriteTimeout, 30)) * time.Second,
		MaxBodySize:       int64(m.ext.GetInt(configKeyMaxBodySize, 32*1024*1024)),
		EnableCORS:        m.ext.GetBool(configKeyEnableCORS, false),
		CORSOrigins:       []string{"*"}, // Default value since GetStringSlice is not available
		EnableCompression: m.ext.GetBool(configKeyEnableCompression, true),
		StaticCacheMaxAge: m.ext.GetInt(configKeyStaticCacheMaxAge, 3600),
	}

	// Create and return server instance
	server := NewServer(config)
	result, err := dataconv.Marshal(server)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal server: %v", err)
	}
	return result, nil
}

// createSessionManager creates a session manager for handling user sessions
func (m *Module) createSessionManager(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		secret     starlark.String
		cookieName = starlark.String("session")
		maxAge     = starlark.MakeInt(86400) // 24 hours
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"secret", &secret,
		"cookie_name?", &cookieName,
		"max_age?", &maxAge,
	); err != nil {
		return none, err
	}

	maxAgeInt, ok := maxAge.Int64()
	if !ok {
		return none, fmt.Errorf("max_age must be an integer")
	}

	sessionManager := NewSessionManager(secret.GoString(), cookieName.GoString(), int(maxAgeInt))
	result, err := dataconv.Marshal(sessionManager)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to marshal session manager: %v", err)
	}
	return result, nil
}

// response creates a basic HTTP response
func (m *Module) response(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		body    starlark.String
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

	statusInt, ok := status.Int64()
	if !ok {
		return none, fmt.Errorf("status must be an integer")
	}

	resp := &Response{
		StatusCode: int(statusInt),
		Headers:    make(map[string][]string),
		Body:       body.GoString(),
	}

	// Add headers
	if headers.Len() > 0 {
		iter := headers.Iterate()
		defer iter.Done()
		var k starlark.Value
		for iter.Next(&k) {
			v, _, err := headers.Get(k)
			if err != nil {
				continue
			}

			keyStr := dataconv.StarString(k)
			valueStr := dataconv.StarString(v)

			if keyStr != "" && valueStr != "" {
				resp.Headers[keyStr] = []string{valueStr}
			}
		}
	}

	return resp.Struct(), nil
}

// jsonResponse creates a JSON HTTP response
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

	statusInt, ok := status.Int64()
	if !ok {
		return none, fmt.Errorf("status must be an integer")
	}

	// Convert Starlark value to JSON
	jsonData, err := dataconv.Unmarshal(data)
	if err != nil {
		return none, fmt.Errorf("failed to convert data to JSON: %v", err)
	}

	resp := &Response{
		StatusCode: int(statusInt),
		Headers:    make(map[string][]string),
		JSONData:   jsonData,
	}

	// Set content type
	resp.Headers["Content-Type"] = []string{"application/json"}

	// Add additional headers
	if headers.Len() > 0 {
		iter := headers.Iterate()
		defer iter.Done()
		var k starlark.Value
		for iter.Next(&k) {
			v, _, err := headers.Get(k)
			if err != nil {
				continue
			}

			keyStr := dataconv.StarString(k)
			valueStr := dataconv.StarString(v)

			if keyStr != "" && valueStr != "" {
				resp.Headers[keyStr] = []string{valueStr}
			}
		}
	}

	return resp.Struct(), nil
}

// htmlResponse creates an HTML HTTP response
func (m *Module) htmlResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		content starlark.String
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

	statusInt, ok := status.Int64()
	if !ok {
		return none, fmt.Errorf("status must be an integer")
	}

	resp := &Response{
		StatusCode: int(statusInt),
		Headers:    make(map[string][]string),
		Body:       content.GoString(),
	}

	// Set content type
	resp.Headers["Content-Type"] = []string{"text/html"}

	// Add additional headers
	if headers.Len() > 0 {
		iter := headers.Iterate()
		defer iter.Done()
		var k starlark.Value
		for iter.Next(&k) {
			v, _, err := headers.Get(k)
			if err != nil {
				continue
			}

			keyStr := dataconv.StarString(k)
			valueStr := dataconv.StarString(v)

			if keyStr != "" && valueStr != "" {
				resp.Headers[keyStr] = []string{valueStr}
			}
		}
	}

	return resp.Struct(), nil
}

// redirect creates a redirect response
func (m *Module) redirect(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		location starlark.String
		status   = starlark.MakeInt(302)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"location", &location,
		"status?", &status,
	); err != nil {
		return none, err
	}

	statusInt, ok := status.Int64()
	if !ok {
		return none, fmt.Errorf("status must be an integer")
	}

	resp := &Response{
		StatusCode: int(statusInt),
		Headers:    make(map[string][]string),
		Body:       "",
	}

	// Set location header
	resp.Headers["Location"] = []string{location.GoString()}

	return resp.Struct(), nil
}

// errorResponse creates an error response
func (m *Module) errorResponse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		status  starlark.Int
		message = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"status", &status,
		"message?", &message,
	); err != nil {
		return none, err
	}

	statusInt, ok := status.Int64()
	if !ok {
		return none, fmt.Errorf("status must be an integer")
	}

	resp := &Response{
		StatusCode: int(statusInt),
		Headers:    make(map[string][]string),
		Body:       message.GoString(),
	}

	// Set content type
	resp.Headers["Content-Type"] = []string{"text/plain"}

	return resp.Struct(), nil
}

// sendFile sends a file from the filesystem
func (m *Module) sendFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		filepath    starlark.String
		contentType = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"filepath", &filepath,
		"content_type?", &contentType,
	); err != nil {
		return none, err
	}

	resp := &Response{
		StatusCode: 200,
		Headers:    make(map[string][]string),
		FilePath:   filepath.GoString(),
	}

	// Set content type if provided
	if contentType.GoString() != "" {
		resp.Headers["Content-Type"] = []string{contentType.GoString()}
	}

	return resp.Struct(), nil
}

// sendData sends raw data as a file download
func (m *Module) sendData(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data        starlark.String
		filename    starlark.String
		contentType = starlark.String("application/octet-stream")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"data", &data,
		"filename", &filename,
		"content_type?", &contentType,
	); err != nil {
		return none, err
	}

	resp := &Response{
		StatusCode: 200,
		Headers:    make(map[string][]string),
		Body:       data.GoString(),
	}

	// Set content type and attachment headers
	resp.Headers["Content-Type"] = []string{contentType.GoString()}
	resp.Headers["Content-Disposition"] = []string{fmt.Sprintf("attachment; filename=\"%s\"", filename.GoString())}

	return resp.Struct(), nil
}

// basicAuth creates a basic HTTP authentication validator
func (m *Module) basicAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var users *starlark.Dict
	var realm starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"users", &users,
		"realm?", &realm,
	); err != nil {
		return starlark.None, err
	}

	// Convert users dict to Go map
	userMap := make(map[string]string)
	if users != nil {
		iter := users.Iterate()
		defer iter.Done()
		var k starlark.Value
		for iter.Next(&k) {
			v, _, err := users.Get(k)
			if err != nil {
				continue
			}
			userMap[dataconv.StarString(k)] = dataconv.StarString(v)
		}
	}

	// Create authenticator
	auth := &BasicAuth{
		users: userMap,
		realm: realm.GoString(),
	}

	return auth.Struct(), nil
}

// bearerAuth creates a bearer token authentication validator
func (m *Module) bearerAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var validateFunc *starlark.Function

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"validate_func", &validateFunc,
	); err != nil {
		return starlark.None, err
	}

	// Create authenticator
	auth := &BearerAuth{
		validateFunc: validateFunc,
	}

	return auth.Struct(), nil
}

// apiKeyAuth creates an API key authentication validator
func (m *Module) apiKeyAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var keys *starlark.List
	var header starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"keys", &keys,
		"header?", &header,
	); err != nil {
		return starlark.None, err
	}

	// Convert keys list to Go slice
	keySlice := make([]string, keys.Len())
	for i := 0; i < keys.Len(); i++ {
		keySlice[i] = dataconv.StarString(keys.Index(i))
	}

	// Create authenticator
	auth := &APIKeyAuth{
		keys:   keySlice,
		header: header.GoString(),
	}

	return auth.Struct(), nil
}

// Built-in middleware functions

// corsMiddleware creates CORS middleware
func (m *Module) corsMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		origins     = starlark.NewList([]starlark.Value{starlark.String("*")})
		methods     = starlark.NewList([]starlark.Value{starlark.String("GET"), starlark.String("POST"), starlark.String("PUT"), starlark.String("DELETE"), starlark.String("OPTIONS")})
		headers     = starlark.NewList([]starlark.Value{starlark.String("Content-Type"), starlark.String("Authorization")})
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
		return none, err
	}

	// Convert to Go slices
	originSlice := make([]string, origins.Len())
	for i := 0; i < origins.Len(); i++ {
		originSlice[i] = dataconv.StarString(origins.Index(i))
	}

	methodSlice := make([]string, methods.Len())
	for i := 0; i < methods.Len(); i++ {
		methodSlice[i] = dataconv.StarString(methods.Index(i))
	}

	headerSlice := make([]string, headers.Len())
	for i := 0; i < headers.Len(); i++ {
		headerSlice[i] = dataconv.StarString(headers.Index(i))
	}

	maxAgeInt, _ := maxAge.Int64()

	middleware := NewCORSMiddleware(originSlice, methodSlice, headerSlice, bool(credentials), int(maxAgeInt))
	return dataconv.WrapGoValue(middleware), nil
}

// loggingMiddleware creates logging middleware
func (m *Module) loggingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		format      = starlark.String("{method} {path} {status} {duration}")
		skipPaths   = starlark.NewList(nil)
		includeBody = starlark.Bool(false)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"format?", &format,
		"skip_paths?", &skipPaths,
		"include_body?", &includeBody,
	); err != nil {
		return none, err
	}

	// Convert skip_paths to Go slice
	skipPathSlice := make([]string, skipPaths.Len())
	for i := 0; i < skipPaths.Len(); i++ {
		skipPathSlice[i] = dataconv.StarString(skipPaths.Index(i))
	}

	middleware := NewLoggingMiddleware(format.GoString(), skipPathSlice, bool(includeBody))
	return dataconv.WrapGoValue(middleware), nil
}

// timingMiddleware creates timing middleware
func (m *Module) timingMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		header    = starlark.String("X-Response-Time")
		precision = starlark.MakeInt(3)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"header?", &header,
		"precision?", &precision,
	); err != nil {
		return none, err
	}

	precisionInt, _ := precision.Int64()

	middleware := NewTimingMiddleware(header.GoString(), int(precisionInt))
	return dataconv.WrapGoValue(middleware), nil
}

// compressionMiddleware creates compression middleware
func (m *Module) compressionMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		level   = starlark.MakeInt(6)
		minSize = starlark.MakeInt(1024)
		types   = starlark.NewList([]starlark.Value{
			starlark.String("text/html"),
			starlark.String("text/css"),
			starlark.String("application/javascript"),
			starlark.String("application/json"),
		})
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"level?", &level,
		"min_size?", &minSize,
		"types?", &types,
	); err != nil {
		return none, err
	}

	levelInt, _ := level.Int64()
	minSizeInt, _ := minSize.Int64()

	// Convert types to Go slice
	typeSlice := make([]string, types.Len())
	for i := 0; i < types.Len(); i++ {
		typeSlice[i] = dataconv.StarString(types.Index(i))
	}

	middleware := NewCompressionMiddleware(int(levelInt), int(minSizeInt), typeSlice)
	return dataconv.WrapGoValue(middleware), nil
}

// securityHeadersMiddleware creates security headers middleware
func (m *Module) securityHeadersMiddleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return none, err
	}

	config := SecurityHeadersConfig{
		FrameOptions:       frameOptions.GoString(),
		ContentTypeOptions: contentTypeOptions.GoString(),
		XSSProtection:      xssProtection.GoString(),
		HSTS:               hsts.GoString(),
		CSP:                csp.GoString(),
		ReferrerPolicy:     referrerPolicy.GoString(),
	}

	middleware := NewSecurityHeadersMiddleware(config)
	return dataconv.WrapGoValue(middleware), nil
}
