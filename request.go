package web

import (
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/1set/starlet/dataconv"
	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// Ensure RequestWrapper implements the required Starlark interfaces
var (
	_ starlark.Value    = (*RequestWrapper)(nil)
	_ starlark.HasAttrs = (*RequestWrapper)(nil)
)

// Request represents an HTTP request.
// This structure holds the complete request data including method, URL, headers,
// query parameters, and provides access to the underlying gin context for
// advanced request processing.
type Request struct {
	Method   string                 `json:"method"`
	URL      string                 `json:"url"`
	Path     string                 `json:"path"`
	Host     string                 `json:"host"`
	Remote   string                 `json:"remote"`
	ClientIP string                 `json:"client_ip"`
	Proto    string                 `json:"proto"`
	Headers  map[string]string      `json:"headers"`
	Query    map[string]string      `json:"query"`
	Context  map[string]interface{} `json:"context"`
	ginCtx   *gin.Context           // Internal gin context
	bodyData []byte                 // Cached body data for multiple reads
}

// createRequestFromGin creates a Request from a gin.Context.
// This function extracts all relevant request information from the gin context
// and creates a Request struct that can be used in Starlark handlers.
func createRequestFromGin(c *gin.Context) *Request {
	// Extract headers
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Extract query parameters
	query := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}

	// Create context map for middleware data
	context := make(map[string]interface{})

	// Cache body data for multiple reads
	bodyData, _ := c.GetRawData()

	return &Request{
		Method:   c.Request.Method,
		URL:      c.Request.URL.String(),
		Path:     c.Request.URL.Path,
		Host:     c.Request.Host,
		Remote:   c.Request.RemoteAddr,
		ClientIP: c.ClientIP(),
		Proto:    c.Request.Proto,
		Headers:  headers,
		Query:    query,
		Context:  context,
		ginCtx:   c,
		bodyData: bodyData,
	}
}

// RequestWrapper wraps the Request struct to provide Starlark-compatible interface.
// This wrapper exposes request properties and methods to Starlark scripts,
// allowing access to request data, headers, parameters, and body content.
type RequestWrapper struct {
	request *Request
}

// NewRequestWrapper creates a new RequestWrapper.
// This function wraps a Request to make it accessible from Starlark
// with proper attribute access and method calls.
func NewRequestWrapper(request *Request) *RequestWrapper {
	return &RequestWrapper{request: request}
}

// String returns a string representation of the request.
func (rw *RequestWrapper) String() string {
	return fmt.Sprintf("<web.Request method=%s path=%s>", rw.request.Method, rw.request.Path)
}

// Type returns the Starlark type name.
func (rw *RequestWrapper) Type() string {
	return "web.Request"
}

// Freeze makes the request immutable (required by Starlark).
func (rw *RequestWrapper) Freeze() {
	// Request is immutable after creation
}

// Truth returns the truth value of the request (always true).
func (rw *RequestWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash of the request (not supported).
func (rw *RequestWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", rw.Type())
}

// Attr returns the value of a request attribute.
// This method provides access to request properties and methods from Starlark.
func (rw *RequestWrapper) Attr(name string) (starlark.Value, error) {
	switch name {
	case "method":
		return starlark.String(rw.request.Method), nil
	case "url":
		return starlark.String(rw.request.URL), nil
	case "path":
		return starlark.String(rw.request.Path), nil
	case "host":
		return starlark.String(rw.request.Host), nil
	case "remote":
		return starlark.String(rw.request.Remote), nil
	case "client_ip":
		return starlark.String(rw.request.ClientIP), nil
	case "proto":
		return starlark.String(rw.request.Proto), nil
	case "headers":
		return dataconv.Marshal(rw.request.Headers)
	case "query":
		return dataconv.Marshal(rw.request.Query)
	case "context":
		return dataconv.Marshal(rw.request.Context)
	case "body":
		return starlark.NewBuiltin("body", rw.bodyMethod), nil
	case "json":
		return starlark.NewBuiltin("json", rw.jsonMethod), nil
	case "form":
		return starlark.NewBuiltin("form", rw.formMethod), nil
	case "files":
		return starlark.NewBuiltin("files", rw.filesMethod), nil
	case "cookie":
		return starlark.NewBuiltin("cookie", rw.cookieMethod), nil
	case "param":
		return starlark.NewBuiltin("param", rw.paramMethod), nil
	case "get_header":
		return starlark.NewBuiltin("get_header", rw.getHeaderMethod), nil
	case "bearer_token":
		return starlark.NewBuiltin("bearer_token", rw.bearerTokenMethod), nil
	case "basic_auth":
		return starlark.NewBuiltin("basic_auth", rw.basicAuthMethod), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", rw.Type(), name))
	}
}

// AttrNames returns the list of available attributes.
func (rw *RequestWrapper) AttrNames() []string {
	return []string{
		"method", "url", "path", "host", "remote", "client_ip", "proto",
		"headers", "query", "context", "body", "json", "form", "files",
		"cookie", "param", "get_header", "bearer_token", "basic_auth",
	}
}

// bodyMethod returns the raw request body as a string.
// This method provides access to the complete request body content.
func (rw *RequestWrapper) bodyMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	return starlark.String(string(rw.request.bodyData)), nil
}

// jsonMethod parses the request body as JSON and returns the parsed data.
// This method automatically handles JSON parsing and returns appropriate Starlark values.
func (rw *RequestWrapper) jsonMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	if len(rw.request.bodyData) == 0 {
		return starlark.None, nil
	}

	// Try to parse as JSON using the existing dataconv package
	jsonValue, err := dataconv.DecodeStarlarkJSON(rw.request.bodyData)
	if err != nil {
		return starlark.None, nil
	}

	return jsonValue, nil
}

// formMethod parses form data from the request body.
// This method handles both URL-encoded and multipart form data.
func (rw *RequestWrapper) formMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.NewDict(0), nil
	}

	// If we have cached body data, we need to recreate the body for form parsing
	if len(rw.request.bodyData) > 0 {
		// Create a new reader from the cached body data
		bodyReader := strings.NewReader(string(rw.request.bodyData))
		rw.request.ginCtx.Request.Body = &readerCloser{bodyReader}
	}

	// Parse form data
	if err := rw.request.ginCtx.Request.ParseForm(); err != nil {
		return starlark.NewDict(0), nil
	}

	form := starlark.NewDict(len(rw.request.ginCtx.Request.Form))
	for key, values := range rw.request.ginCtx.Request.Form {
		if len(values) == 1 {
			form.SetKey(starlark.String(key), starlark.String(values[0]))
		} else {
			// Multiple values - create a list
			list := make([]starlark.Value, len(values))
			for i, v := range values {
				list[i] = starlark.String(v)
			}
			form.SetKey(starlark.String(key), starlark.NewList(list))
		}
	}

	return form, nil
}

// filesMethod returns uploaded files from multipart form data.
// This method provides access to file uploads in the request.
func (rw *RequestWrapper) filesMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.NewDict(0), nil
	}

	// Parse multipart form
	form, err := rw.request.ginCtx.MultipartForm()
	if err != nil {
		// If multipart form parsing fails, return empty dict
		return starlark.NewDict(0), nil
	}

	// Create Starlark dict to hold file information
	filesDict := starlark.NewDict(len(form.File))

	// Process each file field
	for fieldName, fileHeaders := range form.File {
		if len(fileHeaders) == 1 {
			// Single file
			fileInfo := createFileInfo(fileHeaders[0])
			filesDict.SetKey(starlark.String(fieldName), fileInfo)
		} else {
			// Multiple files - create a list
			fileList := make([]starlark.Value, len(fileHeaders))
			for i, fileHeader := range fileHeaders {
				fileList[i] = createFileInfo(fileHeader)
			}
			filesDict.SetKey(starlark.String(fieldName), starlark.NewList(fileList))
		}
	}

	return filesDict, nil
}

// createFileInfo creates a Starlark dict containing file information
func createFileInfo(fileHeader *multipart.FileHeader) starlark.Value {
	fileDict := starlark.NewDict(4)

	// Basic file information
	fileDict.SetKey(starlark.String("filename"), starlark.String(fileHeader.Filename))
	fileDict.SetKey(starlark.String("size"), starlark.MakeInt64(fileHeader.Size))

	// Content type from header
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileDict.SetKey(starlark.String("content_type"), starlark.String(contentType))

	// Create a method to read file content
	readMethod := starlark.NewBuiltin("read", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
			return nil, err
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return starlark.None, nil
		}
		defer file.Close()

		// Read all content
		content, err := io.ReadAll(file)
		if err != nil {
			return starlark.None, nil
		}

		return starlark.String(string(content)), nil
	})

	fileDict.SetKey(starlark.String("read"), readMethod)

	return fileDict
}

// cookieMethod returns the value of a specific cookie.
// This method provides access to HTTP cookies sent with the request.
func (rw *RequestWrapper) cookieMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.None, nil
	}

	cookie, err := rw.request.ginCtx.Cookie(name)
	if err != nil {
		return starlark.None, nil
	}

	return starlark.String(cookie), nil
}

// paramMethod returns the value of a path parameter.
// This method extracts parameters from the URL path (e.g., /users/{id}).
func (rw *RequestWrapper) paramMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.None, nil
	}

	param := rw.request.ginCtx.Param(name)
	if param == "" {
		return starlark.None, nil
	}

	return starlark.String(param), nil
}

// getHeaderMethod returns the value of a specific header with optional default.
// This method provides access to HTTP headers sent with the request.
func (rw *RequestWrapper) getHeaderMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name, "default?", &defaultValue); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return defaultValue, nil
	}

	headerValue := rw.request.ginCtx.GetHeader(name)
	if headerValue == "" {
		return defaultValue, nil
	}

	return starlark.String(headerValue), nil
}

// bearerTokenMethod extracts the Bearer token from the specified header.
// This method provides convenient access to Bearer authentication tokens.
// Supports custom header names and automatically handles Bearer prefix for Authorization header.
func (rw *RequestWrapper) bearerTokenMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var header starlark.String = "Authorization"
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "header?", &header); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.None, nil
	}

	headerName := string(header)
	authHeader := rw.request.ginCtx.GetHeader(headerName)
	if authHeader == "" {
		return starlark.None, nil
	}

	// For Authorization header, expect "Bearer " prefix with actual token
	// For custom headers, use value directly unless it has Bearer prefix
	const bearerPrefix = "Bearer "
	if headerName == "Authorization" {
		// Standard Authorization header - must have Bearer prefix with token
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			return starlark.None, nil
		}
		token := strings.TrimPrefix(authHeader, bearerPrefix)
		return starlark.String(token), nil
	} else {
		// Custom header - check if it has Bearer prefix, if so remove it, otherwise use as-is
		if strings.HasPrefix(authHeader, bearerPrefix) {
			token := strings.TrimPrefix(authHeader, bearerPrefix)
			return starlark.String(token), nil
		}
		// Use header value directly for custom headers
		return starlark.String(authHeader), nil
	}
}

// basicAuthMethod extracts username and password from Basic authentication.
// This method returns a tuple of (username, password) or None if not present.
func (rw *RequestWrapper) basicAuthMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	if rw.request.ginCtx == nil {
		return starlark.None, nil
	}

	username, password, ok := rw.request.ginCtx.Request.BasicAuth()
	if !ok {
		return starlark.None, nil
	}

	return starlark.Tuple{starlark.String(username), starlark.String(password)}, nil
}
