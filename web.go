// Package web provides a Starlark module for server-side web applications.
// It offers a Flask-inspired API for building HTTP servers with routing, middleware,
// request/response handling, and session management capabilities.
package web

import (
	"fmt"

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
	configKeyEnableCORS   = "enable_cors"
	configKeyDebugMode    = "debug_mode"
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
		genConfigOption(configKeyEnableCORS, "Enable CORS by default", false),
		genConfigOption(configKeyDebugMode, "Enable debug mode", false),
	)
}

// Helper functions

// genConfigOption creates a configuration option with common settings
func genConfigOption[T any](name, description string, defaultValue T) *base.ConfigOption[T] {
	return base.NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar(fmt.Sprintf("%s_%s", ModuleName, name))
}

// newModuleWithOptions creates a Module with the given configuration options
func newModuleWithOptions(
	hostOpt *base.ConfigOption[string],
	portOpt *base.ConfigOption[int],
	readTimeoutOpt *base.ConfigOption[int],
	writeTimeoutOpt *base.ConfigOption[int],
	maxBodySizeOpt *base.ConfigOption[int64],
	enableCORSOpt *base.ConfigOption[bool],
	debugModeOpt *base.ConfigOption[bool],
) *Module {
	cm, _ := base.NewConfigurableModuleWithConfigOptions(
		hostOpt,
		portOpt,
		readTimeoutOpt,
		writeTimeoutOpt,
		maxBodySizeOpt,
		enableCORSOpt,
		debugModeOpt,
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
		"redirect":       starlark.NewBuiltin(ModuleName+".redirect", m.redirect),
		"error_response": starlark.NewBuiltin(ModuleName+".error_response", m.errorResponse),
		"send_file":      starlark.NewBuiltin(ModuleName+".send_file", m.sendFile),
		"send_data":      starlark.NewBuiltin(ModuleName+".send_data", m.sendData),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

// createServer is a Starlark function that creates a new HTTP server.
// It accepts optional host and port parameters and returns a ServerWrapper
// that provides Starlark-compatible methods for route registration and server lifecycle.
func (m *Module) createServer(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		host = starlark.String("")
		port = starlark.MakeInt(0)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"host?", &host,
		"port?", &port,
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

	// Convert to Starlark value using wrapper
	return &ServerWrapper{server: server}, nil
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

	// Set content type
	headerMap["Content-Type"] = "application/json"

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

	// Set content type
	headerMap["Content-Type"] = "text/html"

	response := &Response{
		StatusCode: int(statusCode),
		Headers:    headerMap,
		Body:       string(content),
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
			"Location": string(location),
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
		response.Headers["Content-Type"] = string(contentType)
	}

	return NewResponseWrapper(response), nil
}

// sendData creates a data response
func (m *Module) sendData(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data        = starlark.String("")
		filename    = starlark.String("")
		contentType = starlark.String("application/octet-stream")
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
			"Content-Type":        string(contentType),
			"Content-Disposition": fmt.Sprintf("attachment; filename=%s", string(filename)),
		},
		Body: string(data),
	}

	return NewResponseWrapper(response), nil
}
