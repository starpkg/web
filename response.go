package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/1set/starlet/dataconv"
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
	// Create headers dict using helper
	headers := createMultiValueDict(r.Headers)

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

// FromStarlarkStruct converts a Starlark Response struct back to a Go Response object
func ResponseFromStarlarkStruct(val starlark.Value) (*Response, error) {
	// If it's a struct, extract the fields
	if struct_, ok := val.(*starlarkstruct.Struct); ok {
		resp := &Response{
			Headers: make(map[string][]string),
		}

		// Extract status_code
		if statusVal, err := struct_.Attr("status_code"); err == nil {
			if statusInt, ok := statusVal.(starlark.Int); ok {
				if status64, ok := statusInt.Int64(); ok {
					resp.StatusCode = int(status64)
				}
			}
		}

		// Extract headers
		if headersVal, err := struct_.Attr("headers"); err == nil {
			if headersDict, ok := headersVal.(*starlark.Dict); ok {
				iter := headersDict.Iterate()
				defer iter.Done()
				var k starlark.Value
				for iter.Next(&k) {
					v, _, err := headersDict.Get(k)
					if err != nil {
						continue
					}
					keyStr := dataconv.StarString(k)
					valueStr := dataconv.StarString(v)
					if keyStr != "" {
						resp.Headers[keyStr] = []string{valueStr}
					}
				}
			}
		}

		// Extract body
		if bodyVal, err := struct_.Attr("body"); err == nil {
			resp.Body = dataconv.StarString(bodyVal)
		}

		return resp, nil
	}

	// Try to unmarshal as a fallback
	goValue, err := dataconv.Unmarshal(val)
	if err != nil {
		return nil, err
	}

	if resp, ok := goValue.(*Response); ok {
		return resp, nil
	}

	return nil, fmt.Errorf("cannot convert %T to Response", goValue)
}
