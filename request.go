package web

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlight/convert"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Request wraps an HTTP request with additional functionality for Starlark
type Request struct {
	*http.Request
	parsedBody  interface{}
	parsedForm  url.Values
	parsedFiles map[string]*FileUpload
	context     map[string]interface{}
	params      map[string]string
}

// FileUpload represents an uploaded file
type FileUpload struct {
	Filename    string
	ContentType string
	Size        int64
	content     []byte
}

// NewRequest creates a new Request wrapper
func NewRequest(r *http.Request) *Request {
	return &Request{
		Request: r,
		context: make(map[string]interface{}),
		params:  make(map[string]string),
	}
}

// Struct returns a Starlark struct representation of the Request
func (r *Request) Struct() *starlarkstruct.Struct {
	// Create headers dict using helper
	headers := createMultiValueDict(r.Request.Header)

	// Create query dict using helper
	query := createMultiValueDict(r.Request.URL.Query())

	// Create context dict
	ctx := starlark.NewDict(len(r.context))
	for key, value := range r.context {
		starlarkValue, err := convert.ToValue(value)
		if err == nil {
			ctx.SetKey(starlark.String(key), starlarkValue)
		}
	}

	// Build struct with properties
	sd := starlark.StringDict{
		"method":       starlark.String(r.Request.Method),
		"url":          starlark.String(r.Request.URL.String()),
		"path":         starlark.String(r.Request.URL.Path),
		"host":         starlark.String(r.Request.Host),
		"remote":       starlark.String(r.Request.RemoteAddr),
		"client_ip":    r.getClientIP(),
		"proto":        starlark.String(r.Request.Proto),
		"headers":      headers,
		"query":        query,
		"context":      ctx,
		"body":         starlark.NewBuiltin("body", r.Body),
		"json":         starlark.NewBuiltin("json", r.JSON),
		"form":         starlark.NewBuiltin("form", r.Form),
		"files":        starlark.NewBuiltin("files", r.Files),
		"cookie":       starlark.NewBuiltin("cookie", r.Cookie),
		"param":        starlark.NewBuiltin("param", r.Param),
		"get_header":   starlark.NewBuiltin("get_header", r.GetHeader),
		"bearer_token": starlark.NewBuiltin("bearer_token", r.BearerToken),
		"basic_auth":   starlark.NewBuiltin("basic_auth", r.BasicAuth),
	}
	return starlarkstruct.FromStringDict(starlark.String("Request"), sd)
}

// getClientIP extracts the client IP address
func (r *Request) getClientIP() starlark.String {
	// Check X-Forwarded-For header first
	xff := r.Request.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return starlark.String(strings.TrimSpace(ips[0]))
		}
	}

	// Check X-Real-IP header
	xri := r.Request.Header.Get("X-Real-IP")
	if xri != "" {
		return starlark.String(xri)
	}

	// Fall back to RemoteAddr
	ip := r.Request.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return starlark.String(ip)
}

// Body returns the raw request body
func (r *Request) Body(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.Request.Body == nil {
		return starlark.String(""), nil
	}

	body, err := io.ReadAll(r.Request.Body)
	if err != nil {
		return starlark.None, err
	}

	// Reset body for subsequent reads
	r.Request.Body = io.NopCloser(bytes.NewReader(body))

	return starlark.String(string(body)), nil
}

// JSON parses the request body as JSON
func (r *Request) JSON(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.parsedBody != nil {
		// Return cached result
		return convert.ToValue(r.parsedBody)
	}

	if r.Request.Body == nil {
		return starlark.None, nil
	}

	body, err := io.ReadAll(r.Request.Body)
	if err != nil {
		return starlark.None, err
	}

	// Reset body for subsequent reads
	r.Request.Body = io.NopCloser(bytes.NewReader(body))

	if len(body) == 0 {
		return starlark.None, nil
	}

	// Use starlet's JSON unmarshaler to get proper Starlark types
	starlarkValue, err := dataconv.UnmarshalStarlarkJSON(body)
	if err != nil {
		return starlark.None, nil // Return None for invalid JSON
	}

	// Cache the parsed result for subsequent calls
	r.parsedBody = starlarkValue
	return starlarkValue, nil
}

// Form parses form data
func (r *Request) Form(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.parsedForm != nil {
		// Return cached result using helper
		return createMultiValueDict(r.parsedForm), nil
	}

	// Parse form
	if err := r.Request.ParseForm(); err != nil {
		return starlark.None, err
	}

	r.parsedForm = r.Request.Form

	// Convert to Starlark dict using helper
	return createMultiValueDict(r.parsedForm), nil
}

// Files parses multipart form files
func (r *Request) Files(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.parsedFiles != nil {
		// Return cached result
		filesDict := starlark.NewDict(len(r.parsedFiles))
		for name, file := range r.parsedFiles {
			filesDict.SetKey(starlark.String(name), file.Struct())
		}
		return filesDict, nil
	}

	if err := r.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		return starlark.None, nil // Return None if not multipart form
	}

	if r.Request.MultipartForm == nil || r.Request.MultipartForm.File == nil {
		return starlark.NewDict(0), nil
	}

	r.parsedFiles = make(map[string]*FileUpload)
	for name, fileHeaders := range r.Request.MultipartForm.File {
		if len(fileHeaders) > 0 {
			fileHeader := fileHeaders[0] // Take the first file

			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()

			content, err := io.ReadAll(file)
			if err != nil {
				continue
			}

			r.parsedFiles[name] = &FileUpload{
				Filename:    fileHeader.Filename,
				ContentType: fileHeader.Header.Get("Content-Type"),
				Size:        int64(len(content)),
				content:     content,
			}
		}
	}

	// Convert to Starlark dict
	filesDict := starlark.NewDict(len(r.parsedFiles))
	for name, file := range r.parsedFiles {
		filesDict.SetKey(starlark.String(name), file.Struct())
	}

	return filesDict, nil
}

// Cookie returns a cookie value
func (r *Request) Cookie(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return starlark.None, err
	}

	cookie, err := r.Request.Cookie(name.GoString())
	if err != nil {
		return starlark.None, nil
	}

	return starlark.String(cookie.Value), nil
}

// Param returns a path parameter
func (r *Request) Param(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return starlark.None, err
	}

	if value, ok := r.params[name.GoString()]; ok {
		return starlark.String(value), nil
	}

	return starlark.None, nil
}

// GetHeader returns a header value with optional default
func (r *Request) GetHeader(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		name         starlark.String
		defaultValue = starlark.None
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"default?", &defaultValue,
	); err != nil {
		return starlark.None, err
	}

	value := r.Request.Header.Get(name.GoString())
	if value == "" {
		return defaultValue, nil
	}

	return starlark.String(value), nil
}

// BearerToken extracts the Bearer token from Authorization header
func (r *Request) BearerToken(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	auth := r.Request.Header.Get("Authorization")
	if auth == "" {
		return starlark.None, nil
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return starlark.None, nil
	}

	return starlark.String(auth[7:]), nil
}

// BasicAuth extracts Basic authentication credentials
func (r *Request) BasicAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	username, password, ok := r.Request.BasicAuth()
	if !ok {
		return starlark.None, nil
	}

	return starlark.Tuple{starlark.String(username), starlark.String(password)}, nil
}

// SetParam sets a path parameter (used by router)
func (r *Request) SetParam(name, value string) {
	r.params[name] = value
}

// SetContext sets a value in the request context
func (r *Request) SetContext(key string, value interface{}) {
	r.context[key] = value
}

// GetContext gets a context value
func (r *Request) GetContext(key string) interface{} {
	return r.context[key]
}

// FileUpload methods

// Read returns the file content as a string
func (f *FileUpload) Read(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(string(f.content)), nil
}

// ReadBytes returns the file content as bytes
func (f *FileUpload) ReadBytes(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Bytes(f.content), nil
}

// Save saves the file to disk
func (f *FileUpload) Save(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filepath starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &filepath); err != nil {
		return starlark.None, err
	}

	// Validate path to prevent directory traversal
	cleanPath := path.Clean(filepath.GoString())
	if strings.Contains(cleanPath, "..") {
		return starlark.None, fmt.Errorf("invalid file path: %s", filepath.GoString())
	}

	file, err := os.Create(cleanPath)
	if err != nil {
		return starlark.None, err
	}
	defer file.Close()

	_, err = file.Write(f.content)
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

// Struct returns a Starlark struct representation of the FileUpload
func (f *FileUpload) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"filename":     starlark.String(f.Filename),
		"content_type": starlark.String(f.ContentType),
		"size":         starlark.MakeInt64(f.Size),
		"read":         starlark.NewBuiltin("read", f.Read),
		"read_bytes":   starlark.NewBuiltin("read_bytes", f.ReadBytes),
		"save":         starlark.NewBuiltin("save", f.Save),
	}
	return starlarkstruct.FromStringDict(starlark.String("FileUpload"), sd)
}
