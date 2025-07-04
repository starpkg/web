package web

import (
	"net/http"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
	JSONData   interface{}
	FilePath   string
}

// NewResponse creates a new Response
func NewResponse(statusCode int, body string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers:    make(map[string][]string),
		Body:       body,
	}
}

// Struct returns a Starlark struct representation of the Response
func (r *Response) Struct() *starlarkstruct.Struct {
	// Create headers dict
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

	sd := starlark.StringDict{
		"status_code":   starlark.MakeInt(r.StatusCode),
		"headers":       headers,
		"body":          starlark.String(r.Body),
		"set_cookie":    starlark.NewBuiltin("set_cookie", r.SetCookie),
		"delete_cookie": starlark.NewBuiltin("delete_cookie", r.DeleteCookie),
	}
	return starlarkstruct.FromStringDict(starlark.String("Response"), sd)
}

// Starlark-accessible methods

// SetCookie sets a cookie on the response
func (r *Response) SetCookie(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, value starlark.String
	var maxAge starlark.Int
	var path, domain starlark.String
	var secure, httpOnly starlark.Bool

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

	if maxAge != (starlark.Int{}) {
		if maxAgeInt, ok := maxAge.Int64(); ok {
			cookie.MaxAge = int(maxAgeInt)
		}
	}

	cookieStr := cookie.String()
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers["Set-Cookie"] = append(r.Headers["Set-Cookie"], cookieStr)

	return starlark.None, nil
}

// DeleteCookie deletes a cookie
func (r *Response) DeleteCookie(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	var path, domain starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name,
		"path?", &path,
		"domain?", &domain,
	); err != nil {
		return starlark.None, err
	}

	cookie := &http.Cookie{
		Name:    name.GoString(),
		Value:   "",
		Path:    path.GoString(),
		Domain:  domain.GoString(),
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	}

	cookieStr := cookie.String()
	if r.Headers == nil {
		r.Headers = make(map[string][]string)
	}
	r.Headers["Set-Cookie"] = append(r.Headers["Set-Cookie"], cookieStr)

	return starlark.None, nil
}

// GetStatusCode returns the status code
func (r *Response) GetStatusCode() starlark.Int {
	return starlark.MakeInt(r.StatusCode)
}
