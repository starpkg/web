package web

import (
	"io"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Request methods for Starlark integration

// Body returns the raw request body as a string
func (r *Request) Body() string {
	if r.ginCtx == nil {
		return ""
	}

	body, err := io.ReadAll(r.ginCtx.Request.Body)
	if err != nil {
		return ""
	}

	return string(body)
}

// Json parses the request body as JSON and returns a Starlark value
func (r *Request) Json() starlark.Value {
	if r.ginCtx == nil {
		return starlark.None
	}

	body := r.Body()
	if body == "" {
		return starlark.None
	}

	// Try to parse as JSON
	jsonValue, err := dataconv.UnmarshalStarlarkJSON([]byte(body))
	if err != nil {
		return starlark.None
	}

	return jsonValue
}

// Form returns form data as a map
func (r *Request) Form() map[string]string {
	if r.ginCtx == nil {
		return make(map[string]string)
	}

	if err := r.ginCtx.Request.ParseForm(); err != nil {
		return make(map[string]string)
	}

	formData := make(map[string]string)
	for key, values := range r.ginCtx.Request.Form {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}

	return formData
}

// Files returns uploaded files (placeholder for now)
func (r *Request) Files() map[string]*FileUpload {
	// TODO: Implement file upload handling
	return make(map[string]*FileUpload)
}

// Cookie returns the value of a specific cookie
func (r *Request) Cookie(name string) string {
	if r.ginCtx == nil {
		return ""
	}

	cookie, err := r.ginCtx.Cookie(name)
	if err != nil {
		return ""
	}

	return cookie
}

// Param returns a URL parameter by name (e.g., from /users/{id})
func (r *Request) Param(name string) string {
	if r.ginCtx == nil {
		return ""
	}

	return r.ginCtx.Param(name)
}

// GetHeader returns a header value by name
func (r *Request) GetHeader(name string, defaultValue ...string) string {
	if r.ginCtx == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}

	value := r.ginCtx.GetHeader(name)
	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return value
}

// BearerToken extracts the Bearer token from the Authorization header
func (r *Request) BearerToken() string {
	if r.ginCtx == nil {
		return ""
	}

	authHeader := r.ginCtx.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[7:]
	}

	return ""
}

// BasicAuth returns the username and password from Basic Authentication
func (r *Request) BasicAuth() (string, string) {
	if r.ginCtx == nil {
		return "", ""
	}

	username, password, hasAuth := r.ginCtx.Request.BasicAuth()
	if !hasAuth {
		return "", ""
	}

	return username, password
}

// FileUpload represents an uploaded file (placeholder structure)
type FileUpload struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	// TODO: Add actual file handling
}

// Read returns the file content as string (placeholder)
func (f *FileUpload) Read() string {
	// TODO: Implement file reading
	return ""
}

// ReadBytes returns the file content as bytes (placeholder)
func (f *FileUpload) ReadBytes() []byte {
	// TODO: Implement file reading
	return nil
}

// Save saves the file to disk (placeholder)
func (f *FileUpload) Save(path string) error {
	// TODO: Implement file saving
	return nil
}
