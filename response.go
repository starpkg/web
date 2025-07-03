package web

import (
	"net/http"

	"go.starlark.net/starlark"
)

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       string
	JSONData   interface{}
	FilePath   string
}

// NewResponse creates a new Response
func NewResponse(statusCode int, body string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers:    make(http.Header),
		Body:       body,
	}
}

// Starlark-accessible methods

// SetCookie sets a cookie on the response
func (r *Response) SetCookie(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		name     starlark.String
		value    starlark.String
		maxAge   = starlark.None
		path     = starlark.String("/")
		domain   = starlark.String("")
		secure   = starlark.Bool(false)
		httpOnly = starlark.Bool(true)
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
		return starlark.None, err
	}

	cookie := &http.Cookie{
		Name:     name.GoString(),
		Value:    value.GoString(),
		Path:     path.GoString(),
		Domain:   domain.GoString(),
		Secure:   bool(secure),
		HttpOnly: bool(httpOnly),
	}

	// Set max age if provided
	if maxAge != starlark.None {
		if maxAgeInt, ok := maxAge.(starlark.Int); ok {
			if age, ok := maxAgeInt.Int64(); ok {
				cookie.MaxAge = int(age)
			}
		}
	}

	r.Headers.Add("Set-Cookie", cookie.String())
	return starlark.None, nil
}

// DeleteCookie deletes a cookie
func (r *Response) DeleteCookie(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		name   starlark.String
		path   = starlark.String("/")
		domain = starlark.String("")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"path?", &path,
		"domain?", &domain,
	); err != nil {
		return starlark.None, err
	}

	cookie := &http.Cookie{
		Name:   name.GoString(),
		Value:  "",
		Path:   path.GoString(),
		Domain: domain.GoString(),
		MaxAge: -1,
	}

	r.Headers.Add("Set-Cookie", cookie.String())
	return starlark.None, nil
}

// GetStatusCode returns the status code
func (r *Response) GetStatusCode() starlark.Int {
	return starlark.MakeInt(r.StatusCode)
}

// GetHeaders returns the headers as a Starlark dict
func (r *Response) GetHeaders() *starlark.Dict {
	headers := starlark.NewDict(len(r.Headers))
	for name, values := range r.Headers {
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

// GetBody returns the response body
func (r *Response) GetBody() starlark.String {
	return starlark.String(r.Body)
}
