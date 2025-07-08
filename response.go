package web

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Ensure ResponseWrapper implements the required Starlark interfaces
var (
	_ starlark.Value       = (*ResponseWrapper)(nil)
	_ starlark.HasAttrs    = (*ResponseWrapper)(nil)
	_ starlark.HasSetField = (*ResponseWrapper)(nil)
)

// Response represents an HTTP response.
// This structure holds the complete response data including status code,
// headers, body content, and optional file path for file responses.
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	FilePath   string            `json:"file_path,omitempty"`
}

// ResponseWrapper wraps the Response struct to provide Starlark-compatible interface.
// This wrapper exposes response properties and methods to Starlark scripts,
// allowing manipulation of cookies and access to response metadata.
type ResponseWrapper struct {
	response *Response
}

// NewResponseWrapper creates a new ResponseWrapper.
// This function wraps a Response to make it accessible from Starlark
// with proper attribute access and method calls.
func NewResponseWrapper(response *Response) *ResponseWrapper {
	return &ResponseWrapper{response: response}
}

// String returns the string representation of the response
func (rw *ResponseWrapper) String() string {
	return fmt.Sprintf("<web.Response status=%d>", rw.response.StatusCode)
}

// Type returns the type name for Starlark
func (rw *ResponseWrapper) Type() string {
	return "web.Response"
}

// Freeze marks the response as frozen (immutable)
func (rw *ResponseWrapper) Freeze() {
	// Response is immutable after creation
}

// Truth returns the truth value of the response
func (rw *ResponseWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns the hash of the response (not supported)
func (rw *ResponseWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", rw.Type())
}

// Attr returns the value of the specified attribute
func (rw *ResponseWrapper) Attr(name string) (starlark.Value, error) {
	switch name {
	case "status_code":
		return starlark.MakeInt(rw.response.StatusCode), nil
	case "headers":
		// Convert headers map to Starlark dict
		dict := starlark.NewDict(len(rw.response.Headers))
		for k, v := range rw.response.Headers {
			dict.SetKey(starlark.String(k), starlark.String(v))
		}
		return dict, nil
	case "body":
		return starlark.String(rw.response.Body), nil
	case "file_path":
		return starlark.String(rw.response.FilePath), nil
	case "set_cookie":
		return starlark.NewBuiltin("set_cookie", rw.setCookieMethod), nil
	case "delete_cookie":
		return starlark.NewBuiltin("delete_cookie", rw.deleteCookieMethod), nil
	case "set_header":
		return starlark.NewBuiltin("set_header", rw.setHeaderMethod), nil
	case "get_header":
		return starlark.NewBuiltin("get_header", rw.getHeaderMethod), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", rw.Type(), name))
	}
}

// AttrNames returns the list of available attributes
func (rw *ResponseWrapper) AttrNames() []string {
	return []string{"status_code", "headers", "body", "file_path", "set_cookie", "delete_cookie", "set_header", "get_header"}
}

// SetField sets the value of the specified field
func (rw *ResponseWrapper) SetField(name string, value starlark.Value) error {
	switch name {
	case "status_code":
		if statusInt, ok := value.(starlark.Int); ok {
			if status, ok := statusInt.Int64(); ok {
				rw.response.StatusCode = int(status)
				return nil
			}
		}
		return fmt.Errorf("status_code must be an integer")
	case "headers":
		if headersDict, ok := value.(*starlark.Dict); ok {
			// Clear existing headers and set new ones
			rw.response.Headers = make(map[string]string)
			for _, item := range headersDict.Items() {
				key, keyOk := item[0].(starlark.String)
				val, valOk := item[1].(starlark.String)
				if keyOk && valOk {
					rw.response.Headers[string(key)] = string(val)
				}
			}
			return nil
		}
		return fmt.Errorf("headers must be a dict")
	case "body":
		if bodyStr, ok := value.(starlark.String); ok {
			rw.response.Body = string(bodyStr)
			return nil
		}
		return fmt.Errorf("body must be a string")
	case "file_path":
		if pathStr, ok := value.(starlark.String); ok {
			rw.response.FilePath = string(pathStr)
			return nil
		}
		return fmt.Errorf("file_path must be a string")
	default:
		return starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", rw.Type(), name))
	}
}

// setCookieMethod handles the set_cookie() method call
func (rw *ResponseWrapper) setCookieMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		name     string
		value    string
		maxAge   starlark.Value = starlark.None
		path                    = starlark.String("/")
		domain                  = starlark.String("")
		secure                  = starlark.Bool(false)
		httpOnly                = starlark.Bool(true)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"value", &value,
		"max_age?", &maxAge,
		"path?", &path,
		"domain?", &domain,
		"secure?", &secure,
		"http_only?", &httpOnly,
	); err != nil {
		return nil, err
	}

	cookie := fmt.Sprintf("%s=%s; Path=%s", name, value, string(path))

	if string(domain) != "" {
		cookie += fmt.Sprintf("; Domain=%s", string(domain))
	}

	if maxAge != starlark.None {
		if maxAgeInt, ok := maxAge.(starlark.Int); ok {
			if age, ok := maxAgeInt.Int64(); ok {
				cookie += fmt.Sprintf("; Max-Age=%d", age)
			}
		}
	}

	if bool(secure) {
		cookie += "; Secure"
	}

	if bool(httpOnly) {
		cookie += "; HttpOnly"
	}

	// Add to Set-Cookie header
	if existing, exists := rw.response.Headers["Set-Cookie"]; exists {
		rw.response.Headers["Set-Cookie"] = existing + ", " + cookie
	} else {
		rw.response.Headers["Set-Cookie"] = cookie
	}

	return starlark.None, nil
}

// deleteCookieMethod handles the delete_cookie() method call
func (rw *ResponseWrapper) deleteCookieMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		name   string
		path   = starlark.String("/")
		domain = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"path?", &path,
		"domain?", &domain,
	); err != nil {
		return nil, err
	}

	cookie := fmt.Sprintf("%s=; Path=%s; Max-Age=0", name, string(path))

	if string(domain) != "" {
		cookie += fmt.Sprintf("; Domain=%s", string(domain))
	}

	// Add to Set-Cookie header
	if existing, exists := rw.response.Headers["Set-Cookie"]; exists {
		rw.response.Headers["Set-Cookie"] = existing + ", " + cookie
	} else {
		rw.response.Headers["Set-Cookie"] = cookie
	}

	return starlark.None, nil
}

// setHeaderMethod handles the set_header() method call
func (rw *ResponseWrapper) setHeaderMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, value string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"value", &value,
	); err != nil {
		return nil, err
	}

	if rw.response.Headers == nil {
		rw.response.Headers = make(map[string]string)
	}
	rw.response.Headers[name] = value

	return starlark.None, nil
}

// getHeaderMethod handles the get_header() method call
func (rw *ResponseWrapper) getHeaderMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"default?", &defaultValue,
	); err != nil {
		return nil, err
	}

	if rw.response.Headers == nil {
		return defaultValue, nil
	}

	if value, exists := rw.response.Headers[name]; exists {
		return starlark.String(value), nil
	}

	return defaultValue, nil
}
