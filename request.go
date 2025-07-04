package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
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

// Starlark-accessible methods

// Method returns the HTTP method
func (r *Request) Method() starlark.String {
	return starlark.String(r.Request.Method)
}

// URL returns the request URL
func (r *Request) URL() starlark.String {
	return starlark.String(r.Request.URL.String())
}

// Path returns the URL path
func (r *Request) Path() starlark.String {
	return starlark.String(r.Request.URL.Path)
}

// Host returns the host header
func (r *Request) Host() starlark.String {
	return starlark.String(r.Request.Host)
}

// Remote returns the remote address
func (r *Request) Remote() starlark.String {
	return starlark.String(r.Request.RemoteAddr)
}

// ClientIP extracts the client IP address
func (r *Request) ClientIP() starlark.String {
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

// Proto returns the protocol
func (r *Request) Proto() starlark.String {
	return starlark.String(r.Request.Proto)
}

// Headers returns the request headers as a Starlark dict
func (r *Request) Headers() *starlark.Dict {
	headers := starlark.NewDict(len(r.Request.Header))
	for name, values := range r.Request.Header {
		if len(values) == 1 {
			headers.SetKey(starlark.String(name), starlark.String(values[0]))
		} else {
			valueList := make([]starlark.Value, len(values))
			for i, v := range values {
				valueList[i] = starlark.String(v)
			}
			headers.SetKey(starlark.String(name), starlark.NewList(valueList))
		}
	}
	return headers
}

// Query returns the query parameters as a Starlark dict
func (r *Request) Query() *starlark.Dict {
	query := starlark.NewDict(len(r.Request.URL.Query()))
	for name, values := range r.Request.URL.Query() {
		if len(values) == 1 {
			query.SetKey(starlark.String(name), starlark.String(values[0]))
		} else {
			valueList := make([]starlark.Value, len(values))
			for i, v := range values {
				valueList[i] = starlark.String(v)
			}
			query.SetKey(starlark.String(name), starlark.NewList(valueList))
		}
	}
	return query
}

// Context returns the request context for middleware data
func (r *Request) Context() *starlark.Dict {
	ctx := starlark.NewDict(len(r.context))
	for key, value := range r.context {
		starlarkValue, err := dataconv.Marshal(value)
		if err == nil {
			ctx.SetKey(starlark.String(key), starlarkValue)
		}
	}
	return ctx
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
		return dataconv.Marshal(r.parsedBody)
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

	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return starlark.None, nil // Return None for invalid JSON
	}

	r.parsedBody = jsonData
	return dataconv.Marshal(jsonData)
}

// Form parses form data
func (r *Request) Form(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.parsedForm != nil {
		// Return cached result
		form := starlark.NewDict(len(r.parsedForm))
		for name, values := range r.parsedForm {
			if len(values) == 1 {
				form.SetKey(starlark.String(name), starlark.String(values[0]))
			} else {
				valueList := make([]starlark.Value, len(values))
				for i, v := range values {
					valueList[i] = starlark.String(v)
				}
				form.SetKey(starlark.String(name), starlark.NewList(valueList))
			}
		}
		return form, nil
	}

	if err := r.Request.ParseForm(); err != nil {
		return starlark.None, err
	}

	r.parsedForm = r.Request.Form

	form := starlark.NewDict(len(r.parsedForm))
	for name, values := range r.parsedForm {
		if len(values) == 1 {
			form.SetKey(starlark.String(name), starlark.String(values[0]))
		} else {
			valueList := make([]starlark.Value, len(values))
			for i, v := range values {
				valueList[i] = starlark.String(v)
			}
			form.SetKey(starlark.String(name), starlark.NewList(valueList))
		}
	}

	return form, nil
}

// Files parses multipart form files
func (r *Request) Files(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if r.parsedFiles != nil {
		// Return cached result
		files := starlark.NewDict(len(r.parsedFiles))
		for name, file := range r.parsedFiles {
			fileValue, err := dataconv.Marshal(file)
			if err != nil {
				continue // Skip files that can't be marshaled
			}
			files.SetKey(starlark.String(name), fileValue)
		}
		return files, nil
	}

	if err := r.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB
		return starlark.NewDict(0), nil // Return empty dict if not multipart
	}

	r.parsedFiles = make(map[string]*FileUpload)

	if r.Request.MultipartForm != nil {
		for name, fileHeaders := range r.Request.MultipartForm.File {
			if len(fileHeaders) > 0 {
				fh := fileHeaders[0] // Take the first file
				file, err := fh.Open()
				if err != nil {
					continue
				}
				defer file.Close()

				content, err := io.ReadAll(file)
				if err != nil {
					continue
				}

				r.parsedFiles[name] = &FileUpload{
					Filename:    fh.Filename,
					ContentType: fh.Header.Get("Content-Type"),
					Size:        fh.Size,
					content:     content,
				}
			}
		}
	}

	files := starlark.NewDict(len(r.parsedFiles))
	for name, file := range r.parsedFiles {
		fileValue, err := dataconv.Marshal(file)
		if err != nil {
			continue // Skip files that can't be marshaled
		}
		files.SetKey(starlark.String(name), fileValue)
	}

	return files, nil
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

// SetContext sets a context value (used by middleware)
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
