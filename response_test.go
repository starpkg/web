package web

// Thematic tests for the module's value builders, invoked directly through the
// Starlark builtins they back (web.go) so the package has standalone coverage
// independent of a running HTTP server.
//
// Sections:
//   - response builders: response / json_response / text_response / html_response
//   - response builders (file/redirect/error): file_response / send_file /
//     send_data / redirect / error_response, plus their argument-validation errors
//   - response-builder argument errors: bad-type/missing args -> clean errors
//   - middleware constructors: a couple of m.* builtins return MiddlewareWrappers
//   - middleware-constructor argument errors: invalid list/int/bool args -> errors
//   - ResponseWrapper: Attr / SetField / get_header / set_header behaviour & errors
//   - cookies: set_cookie/delete_cookie produce distinct Set-Cookie header lines

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// callBuiltinErr invokes a module builtin expecting an error, returning it.
func callBuiltinErr(t *testing.T, name string, fn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error), args starlark.Tuple, kwargs []starlark.Tuple) error {
	t.Helper()
	b := starlark.NewBuiltin(name, fn)
	_, err := fn(&starlark.Thread{}, b, args, kwargs)
	if err == nil {
		t.Fatalf("%s: expected an error, got nil", name)
	}
	return err
}

// callBuiltin is a small helper to invoke a module builtin like Starlark would.
func callBuiltin(t *testing.T, name string, fn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error), args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
	t.Helper()
	b := starlark.NewBuiltin(name, fn)
	v, err := fn(&starlark.Thread{}, b, args, kwargs)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", name, err)
	}
	return v
}

func asResponse(t *testing.T, v starlark.Value) *Response {
	t.Helper()
	rw, ok := v.(*ResponseWrapper)
	if !ok {
		t.Fatalf("expected *ResponseWrapper, got %T", v)
	}
	return rw.response
}

// --- response builders -------------------------------------------------------

func TestResponseBuilders(t *testing.T) {
	m := NewModule()

	t.Run("response", func(t *testing.T) {
		v := callBuiltin(t, "response", m.response, starlark.Tuple{starlark.String("hi")}, nil)
		resp := asResponse(t, v)
		if resp.Body != "hi" {
			t.Errorf("body = %q, want %q", resp.Body, "hi")
		}
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("json_response", func(t *testing.T) {
		data := starlark.NewDict(1)
		_ = data.SetKey(starlark.String("ok"), starlark.True)
		v := callBuiltin(t, "json_response", m.jsonResponse, starlark.Tuple{data}, nil)
		resp := asResponse(t, v)
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != MIMEApplicationJSON {
			t.Errorf("content-type = %q, want %q", got, MIMEApplicationJSON)
		}
		if !strings.Contains(resp.Body, "ok") || !strings.Contains(resp.Body, "true") {
			t.Errorf("json body = %q, want it to contain the encoded dict", resp.Body)
		}
	})

	t.Run("text_response", func(t *testing.T) {
		v := callBuiltin(t, "text_response", m.textResponse,
			starlark.Tuple{starlark.String("plain")},
			[]starlark.Tuple{{starlark.String("status"), starlark.MakeInt(201)}})
		resp := asResponse(t, v)
		if resp.Body != "plain" {
			t.Errorf("body = %q, want %q", resp.Body, "plain")
		}
		if resp.StatusCode != 201 {
			t.Errorf("status = %d, want 201", resp.StatusCode)
		}
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != MIMETextPlain {
			t.Errorf("content-type = %q, want %q", got, MIMETextPlain)
		}
	})

	t.Run("html_response", func(t *testing.T) {
		v := callBuiltin(t, "html_response", m.htmlResponse, starlark.Tuple{starlark.String("<h1>hi</h1>")}, nil)
		resp := asResponse(t, v)
		if resp.Body != "<h1>hi</h1>" {
			t.Errorf("body = %q, want %q", resp.Body, "<h1>hi</h1>")
		}
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != MIMETextHTML {
			t.Errorf("content-type = %q, want %q", got, MIMETextHTML)
		}
	})
}

// --- response builders (file/redirect/error) ---------------------------------

func TestFileAndDataResponseBuilders(t *testing.T) {
	m := NewModule()

	t.Run("file_response_minimal", func(t *testing.T) {
		v := callBuiltin(t, "file_response", m.fileResponse,
			starlark.Tuple{starlark.String("/tmp/data.bin")}, nil)
		resp := asResponse(t, v)
		if resp.FilePath != "/tmp/data.bin" {
			t.Errorf("file_path = %q, want /tmp/data.bin", resp.FilePath)
		}
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		// No content_type / filename means no Content-Type / Disposition headers.
		if _, ok := resp.Headers[canonicalHeader(HeaderContentType)]; ok {
			t.Errorf("unexpected Content-Type header: %v", resp.Headers)
		}
		if _, ok := resp.Headers[canonicalHeader(HeaderContentDisposition)]; ok {
			t.Errorf("unexpected Content-Disposition header: %v", resp.Headers)
		}
	})

	t.Run("file_response_full", func(t *testing.T) {
		v := callBuiltin(t, "file_response", m.fileResponse,
			starlark.Tuple{starlark.String("/tmp/report.pdf")},
			[]starlark.Tuple{
				{starlark.String("content_type"), starlark.String("application/pdf")},
				{starlark.String("filename"), starlark.String("report.pdf")},
			})
		resp := asResponse(t, v)
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != "application/pdf" {
			t.Errorf("content-type = %q, want application/pdf", got)
		}
		if got := resp.Headers[canonicalHeader(HeaderContentDisposition)]; got != "attachment; filename=report.pdf" {
			t.Errorf("content-disposition = %q, want attachment; filename=report.pdf", got)
		}
	})

	t.Run("send_file", func(t *testing.T) {
		v := callBuiltin(t, "send_file", m.sendFile,
			starlark.Tuple{starlark.String("/tmp/x.txt")},
			[]starlark.Tuple{{starlark.String("content_type"), starlark.String("text/plain")}})
		resp := asResponse(t, v)
		if resp.FilePath != "/tmp/x.txt" {
			t.Errorf("file_path = %q, want /tmp/x.txt", resp.FilePath)
		}
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != "text/plain" {
			t.Errorf("content-type = %q, want text/plain", got)
		}
		// send_file never sets an attachment filename.
		if _, ok := resp.Headers[canonicalHeader(HeaderContentDisposition)]; ok {
			t.Errorf("send_file must not set Content-Disposition: %v", resp.Headers)
		}
	})

	t.Run("send_data_default_content_type", func(t *testing.T) {
		v := callBuiltin(t, "send_data", m.sendData,
			starlark.Tuple{starlark.String("raw-bytes"), starlark.String("dump.bin")}, nil)
		resp := asResponse(t, v)
		if resp.Body != "raw-bytes" {
			t.Errorf("body = %q, want raw-bytes", resp.Body)
		}
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != MIMEApplicationOctetStream {
			t.Errorf("content-type = %q, want %q", got, MIMEApplicationOctetStream)
		}
		if got := resp.Headers[canonicalHeader(HeaderContentDisposition)]; got != "attachment; filename=dump.bin" {
			t.Errorf("content-disposition = %q, want attachment; filename=dump.bin", got)
		}
	})
}

func TestRedirectAndErrorResponseBuilders(t *testing.T) {
	m := NewModule()

	t.Run("redirect_default_status", func(t *testing.T) {
		v := callBuiltin(t, "redirect", m.redirect,
			starlark.Tuple{starlark.String("/login")}, nil)
		resp := asResponse(t, v)
		if resp.StatusCode != 302 {
			t.Errorf("status = %d, want 302", resp.StatusCode)
		}
		if got := resp.Headers[canonicalHeader(HeaderLocation)]; got != "/login" {
			t.Errorf("location = %q, want /login", got)
		}
	})

	t.Run("redirect_custom_status", func(t *testing.T) {
		v := callBuiltin(t, "redirect", m.redirect,
			starlark.Tuple{starlark.String("https://example.com")},
			[]starlark.Tuple{{starlark.String("status"), starlark.MakeInt(301)}})
		resp := asResponse(t, v)
		if resp.StatusCode != 301 {
			t.Errorf("status = %d, want 301", resp.StatusCode)
		}
	})

	t.Run("error_response", func(t *testing.T) {
		v := callBuiltin(t, "error_response", m.errorResponse,
			starlark.Tuple{starlark.MakeInt(404)},
			[]starlark.Tuple{{starlark.String("message"), starlark.String("not here")}})
		resp := asResponse(t, v)
		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
		if resp.Body != "not here" {
			t.Errorf("body = %q, want %q", resp.Body, "not here")
		}
	})

	t.Run("error_response_default_message", func(t *testing.T) {
		v := callBuiltin(t, "error_response", m.errorResponse,
			starlark.Tuple{starlark.MakeInt(500)}, nil)
		resp := asResponse(t, v)
		if resp.StatusCode != 500 {
			t.Errorf("status = %d, want 500", resp.StatusCode)
		}
		if resp.Body != "" {
			t.Errorf("body = %q, want empty", resp.Body)
		}
	})
}

// --- response-builder argument errors ----------------------------------------

// Required/typed arguments must produce a clean error (never a panic) when a
// script passes the wrong shape.
func TestResponseBuilderArgErrors(t *testing.T) {
	m := NewModule()

	cases := []struct {
		name string
		fn   func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)
		args starlark.Tuple
	}{
		{"response_missing_body", m.response, nil},
		{"response_body_not_string", m.response, starlark.Tuple{starlark.MakeInt(1)}},
		{"json_response_missing_data", m.jsonResponse, nil},
		{"text_response_missing_text", m.textResponse, nil},
		{"html_response_content_not_string", m.htmlResponse, starlark.Tuple{starlark.MakeInt(1)}},
		{"file_response_missing_filepath", m.fileResponse, nil},
		{"redirect_missing_location", m.redirect, nil},
		{"error_response_missing_status", m.errorResponse, nil},
		{"send_file_missing_filepath", m.sendFile, nil},
		{"send_data_missing_filename", m.sendData, starlark.Tuple{starlark.String("d")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked: %v", c.name, r)
				}
			}()
			_ = callBuiltinErr(t, c.name, c.fn, c.args, nil)
		})
	}
}

// json_response must surface a clean "failed to marshal JSON" error rather than
// panic when handed an unserialisable value (e.g. a function).
func TestJSONResponseUnserialisable(t *testing.T) {
	m := NewModule()
	fn := starlark.NewBuiltin("noop", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
	err := callBuiltinErr(t, "json_response", m.jsonResponse, starlark.Tuple{fn}, nil)
	if !strings.Contains(err.Error(), "marshal JSON") {
		t.Errorf("error = %q, want it to mention marshal JSON", err)
	}
}

// json_response passes through a string/bytes body verbatim (no re-encoding).
func TestJSONResponseVerbatimBody(t *testing.T) {
	m := NewModule()

	t.Run("string", func(t *testing.T) {
		v := callBuiltin(t, "json_response", m.jsonResponse,
			starlark.Tuple{starlark.String(`{"raw":1}`)}, nil)
		resp := asResponse(t, v)
		if resp.Body != `{"raw":1}` {
			t.Errorf("body = %q, want verbatim string", resp.Body)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		v := callBuiltin(t, "json_response", m.jsonResponse,
			starlark.Tuple{starlark.Bytes(`[1,2]`)}, nil)
		resp := asResponse(t, v)
		if resp.Body != `[1,2]` {
			t.Errorf("body = %q, want verbatim bytes", resp.Body)
		}
	})
}

// --- middleware constructors -------------------------------------------------

func TestMiddlewareConstructors(t *testing.T) {
	m := NewModule()

	t.Run("logging_middleware", func(t *testing.T) {
		v := callBuiltin(t, "logging_middleware", m.loggingMiddleware, nil, nil)
		if _, ok := v.(*MiddlewareWrapper); !ok {
			t.Fatalf("expected *MiddlewareWrapper, got %T", v)
		}
	})

	t.Run("cors_middleware", func(t *testing.T) {
		v := callBuiltin(t, "cors_middleware", m.corsMiddleware, nil, nil)
		if _, ok := v.(*MiddlewareWrapper); !ok {
			t.Fatalf("expected *MiddlewareWrapper, got %T", v)
		}
	})

	t.Run("json_middleware", func(t *testing.T) {
		v := callBuiltin(t, "json_middleware", m.jsonMiddleware, nil, nil)
		if _, ok := v.(*MiddlewareWrapper); !ok {
			t.Fatalf("expected *MiddlewareWrapper, got %T", v)
		}
	})
}

// --- middleware-constructor wiring -------------------------------------------

// The module-level middleware builtins must produce middleware that behaves the
// same as the low-level constructors. Driving the resulting MiddlewareFunc keeps
// the constructor wiring (defaults, conversions) covered without a socket.
func TestMiddlewareBuiltinWiring(t *testing.T) {
	m := NewModule()
	next := func(*Request) *Response { return &Response{StatusCode: 200, Headers: map[string]string{}} }

	t.Run("timing", func(t *testing.T) {
		v := callBuiltin(t, "timing_middleware", m.timingMiddleware, nil, nil)
		resp := v.(*MiddlewareWrapper).Execute(&Request{Method: http.MethodGet, Path: "/"}, next)
		if _, ok := resp.Headers[canonicalHeader(HeaderXResponseTime)]; !ok {
			t.Errorf("timing builtin did not add the response-time header: %v", resp.Headers)
		}
	})

	t.Run("cache", func(t *testing.T) {
		v := callBuiltin(t, "cache_middleware", m.cacheMiddleware,
			nil, []starlark.Tuple{{starlark.String("max_age"), starlark.MakeInt(50)}})
		resp := v.(*MiddlewareWrapper).Execute(&Request{Method: http.MethodGet, Path: "/"}, next)
		if got := resp.Headers[canonicalHeader(HeaderCacheControl)]; got != "public, max-age=50" {
			t.Errorf("cache builtin cache-control = %q, want public, max-age=50", got)
		}
	})

	t.Run("request_size", func(t *testing.T) {
		v := callBuiltin(t, "request_size_middleware", m.requestSizeMiddleware,
			nil, []starlark.Tuple{{starlark.String("max_content_length"), starlark.MakeInt(2)}})
		req := &Request{Method: http.MethodPost, Path: "/", Headers: map[string]string{}, bodyData: []byte("big")}
		resp := v.(*MiddlewareWrapper).Execute(req, next)
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("request_size builtin status = %d, want 413", resp.StatusCode)
		}
	})
}

// rate_limit_middleware with a Starlark key_func must use the script-returned key,
// and fall back to the client IP when the key_func errors or returns a non-string.
func TestRateLimitBuiltinKeyFunc(t *testing.T) {
	m := NewModule()
	next := func(*Request) *Response { return &Response{StatusCode: 200, Headers: map[string]string{}} }

	t.Run("custom_string_key", func(t *testing.T) {
		// key_func returns a constant key, so two different IPs share one bucket.
		src := `
def key(req):
    return "shared"
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "k.star", src, nil)
		v := callBuiltin(t, "rate_limit_middleware", m.rateLimitMiddleware, nil,
			[]starlark.Tuple{
				{starlark.String("requests"), starlark.MakeInt(1)},
				{starlark.String("key_func"), globals["key"].(starlark.Callable)},
			})
		mw := v.(*MiddlewareWrapper)
		if r := mw.Execute(&Request{Method: http.MethodGet, Path: "/", ClientIP: "1.1.1.1"}, next); r.StatusCode != 200 {
			t.Fatalf("first = %d, want 200", r.StatusCode)
		}
		// Different IP, same key -> over the shared limit.
		if r := mw.Execute(&Request{Method: http.MethodGet, Path: "/", ClientIP: "2.2.2.2"}, next); r.StatusCode != http.StatusTooManyRequests {
			t.Errorf("second (shared key) = %d, want 429", r.StatusCode)
		}
	})

	t.Run("key_func_error_falls_back_to_ip", func(t *testing.T) {
		src := `
def key(req):
    fail("no key")
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "k.star", src, nil)
		v := callBuiltin(t, "rate_limit_middleware", m.rateLimitMiddleware, nil,
			[]starlark.Tuple{
				{starlark.String("requests"), starlark.MakeInt(1)},
				{starlark.String("key_func"), globals["key"].(starlark.Callable)},
			})
		mw := v.(*MiddlewareWrapper)
		// Distinct IPs are bucketed independently because the key falls back to IP.
		if r := mw.Execute(&Request{Method: http.MethodGet, Path: "/", ClientIP: "3.3.3.3"}, next); r.StatusCode != 200 {
			t.Fatalf("ip-a first = %d, want 200", r.StatusCode)
		}
		if r := mw.Execute(&Request{Method: http.MethodGet, Path: "/", ClientIP: "4.4.4.4"}, next); r.StatusCode != 200 {
			t.Errorf("ip-b first = %d, want 200 (independent IP fallback bucket)", r.StatusCode)
		}
	})

	t.Run("oversized_requests_arg_errors", func(t *testing.T) {
		// A value that overflows int64 trips the "invalid requests" conversion
		// branch rather than the UnpackArgs type check.
		huge := starlark.MakeInt(1).Lsh(100) // 2**100, too large for Int64
		err := callBuiltinErr(t, "rate_limit_middleware", m.rateLimitMiddleware,
			starlark.Tuple{huge}, nil)
		if !strings.Contains(err.Error(), "invalid requests") {
			t.Errorf("error = %q, want invalid requests", err)
		}
	})
}

// --- middleware-constructor argument errors ----------------------------------

// Each middleware constructor converts its list/int/bool arguments; a wrong-typed
// element must yield a clean "invalid ..." error, not a panic.
func TestMiddlewareConstructorArgErrors(t *testing.T) {
	m := NewModule()
	badList := starlark.NewList([]starlark.Value{starlark.MakeInt(1)}) // non-string element

	cases := []struct {
		name string
		fn   func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)
		args starlark.Tuple
		want string
	}{
		{"cors_bad_origins", m.corsMiddleware, starlark.Tuple{badList}, "invalid origins"},
		{"compression_bad_types", m.compressionMiddleware,
			starlark.Tuple{starlark.MakeInt(6), starlark.MakeInt(1024), badList}, "invalid types"},
		{"cache_bad_patterns", m.cacheMiddleware,
			starlark.Tuple{starlark.MakeInt(60), starlark.False, badList}, "invalid patterns"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked: %v", c.name, r)
				}
			}()
			err := callBuiltinErr(t, c.name, c.fn, c.args, nil)
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to contain %q", err, c.want)
			}
		})
	}
}

// security_headers_middleware drops empty header values and keeps non-empty ones.
func TestSecurityHeadersConstructorConfig(t *testing.T) {
	m := NewModule()
	// Override defaults to empty, set only HSTS — only HSTS should appear.
	v := callBuiltin(t, "security_headers_middleware", m.securityHeadersMiddleware, nil,
		[]starlark.Tuple{
			{starlark.String("frame_options"), starlark.String("")},
			{starlark.String("content_type_options"), starlark.String("")},
			{starlark.String("xss_protection"), starlark.String("")},
			{starlark.String("hsts"), starlark.String("max-age=600")},
		})
	mw, ok := v.(*MiddlewareWrapper)
	if !ok {
		t.Fatalf("expected *MiddlewareWrapper, got %T", v)
	}
	resp := mw.Execute(&Request{Method: http.MethodGet, Path: "/"}, func(*Request) *Response {
		return &Response{StatusCode: 200, Headers: map[string]string{}}
	})
	if got := resp.Headers[canonicalHeader("Strict-Transport-Security")]; got != "max-age=600" {
		t.Errorf("HSTS header = %q, want max-age=600", got)
	}
	if _, ok := resp.Headers[canonicalHeader("X-Frame-Options")]; ok {
		t.Errorf("X-Frame-Options should be absent when set empty: %v", resp.Headers)
	}
}

// --- ResponseWrapper ---------------------------------------------------------

func TestResponseWrapperAttr(t *testing.T) {
	rw := NewResponseWrapper(&Response{
		StatusCode: 201,
		Headers:    map[string]string{"X-Test": "yes"},
		Body:       "hello",
		FilePath:   "/tmp/f",
	})

	t.Run("scalar_attrs", func(t *testing.T) {
		if v, _ := rw.Attr("status_code"); v.(starlark.Int) != starlark.MakeInt(201) {
			t.Errorf("status_code = %v, want 201", v)
		}
		if v, _ := rw.Attr("body"); string(v.(starlark.String)) != "hello" {
			t.Errorf("body = %v, want hello", v)
		}
		if v, _ := rw.Attr("file_path"); string(v.(starlark.String)) != "/tmp/f" {
			t.Errorf("file_path = %v, want /tmp/f", v)
		}
	})

	t.Run("headers_dict", func(t *testing.T) {
		v, err := rw.Attr("headers")
		if err != nil {
			t.Fatalf("Attr(headers): %v", err)
		}
		dict, ok := v.(*starlark.Dict)
		if !ok {
			t.Fatalf("headers attr is %T, want *starlark.Dict", v)
		}
		got, found, _ := dict.Get(starlark.String("X-Test"))
		if !found || string(got.(starlark.String)) != "yes" {
			t.Errorf("headers[X-Test] = %v (found=%v), want yes", got, found)
		}
	})

	t.Run("unknown_attr_errors", func(t *testing.T) {
		if _, err := rw.Attr("nope"); err == nil {
			t.Error("Attr(nope): expected NoSuchAttrError, got nil")
		}
	})
}

func TestResponseWrapperSetField(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		rw := NewResponseWrapper(&Response{Headers: map[string]string{"old": "1"}})
		if err := rw.SetField("status_code", starlark.MakeInt(418)); err != nil {
			t.Fatalf("SetField status_code: %v", err)
		}
		if rw.response.StatusCode != 418 {
			t.Errorf("status_code = %d, want 418", rw.response.StatusCode)
		}
		if err := rw.SetField("body", starlark.String("teapot")); err != nil {
			t.Fatalf("SetField body: %v", err)
		}
		if rw.response.Body != "teapot" {
			t.Errorf("body = %q, want teapot", rw.response.Body)
		}
		if err := rw.SetField("file_path", starlark.String("/p")); err != nil {
			t.Fatalf("SetField file_path: %v", err)
		}
		// Setting headers replaces the whole map.
		nd := starlark.NewDict(1)
		_ = nd.SetKey(starlark.String("new"), starlark.String("v"))
		if err := rw.SetField("headers", nd); err != nil {
			t.Fatalf("SetField headers: %v", err)
		}
		if rw.response.Headers["new"] != "v" || rw.response.Headers["old"] != "" {
			t.Errorf("headers not replaced: %v", rw.response.Headers)
		}
	})

	t.Run("type_errors", func(t *testing.T) {
		rw := NewResponseWrapper(&Response{})
		cases := []struct {
			field string
			value starlark.Value
		}{
			{"status_code", starlark.String("x")},
			{"headers", starlark.String("x")},
			{"body", starlark.MakeInt(1)},
			{"file_path", starlark.MakeInt(1)},
			{"unknown_field", starlark.None},
		}
		for _, c := range cases {
			if err := rw.SetField(c.field, c.value); err == nil {
				t.Errorf("SetField(%q, %v): expected error, got nil", c.field, c.value)
			}
		}
	})
}

func TestResponseWrapperHeaderMethods(t *testing.T) {
	rw := NewResponseWrapper(&Response{})
	setB := starlark.NewBuiltin("set_header", rw.setHeaderMethod)
	getB := starlark.NewBuiltin("get_header", rw.getHeaderMethod)

	// set_header lazily initialises the headers map.
	if _, err := rw.setHeaderMethod(&starlark.Thread{}, setB,
		starlark.Tuple{starlark.String("X-A"), starlark.String("1")}, nil); err != nil {
		t.Fatalf("set_header: %v", err)
	}
	if rw.response.Headers["X-A"] != "1" {
		t.Errorf("header X-A = %q, want 1", rw.response.Headers["X-A"])
	}

	// get_header returns the value when present.
	got, err := rw.getHeaderMethod(&starlark.Thread{}, getB, starlark.Tuple{starlark.String("X-A")}, nil)
	if err != nil {
		t.Fatalf("get_header present: %v", err)
	}
	if string(got.(starlark.String)) != "1" {
		t.Errorf("get_header(X-A) = %v, want 1", got)
	}

	// get_header returns the supplied default when absent.
	got, err = rw.getHeaderMethod(&starlark.Thread{}, getB,
		starlark.Tuple{starlark.String("X-Missing"), starlark.String("fallback")}, nil)
	if err != nil {
		t.Fatalf("get_header default: %v", err)
	}
	if string(got.(starlark.String)) != "fallback" {
		t.Errorf("get_header(missing) = %v, want fallback", got)
	}

	// get_header on a nil-headers response returns the default, not a panic.
	empty := NewResponseWrapper(&Response{})
	got, err = empty.getHeaderMethod(&starlark.Thread{}, getB,
		starlark.Tuple{starlark.String("X")}, nil)
	if err != nil || got != starlark.None {
		t.Errorf("get_header on nil headers = (%v, %v), want (None, nil)", got, err)
	}
}

// delete_cookie appends an expiring Set-Cookie line (Max-Age=0).
func TestResponseDeleteCookie(t *testing.T) {
	rw := NewResponseWrapper(&Response{})
	b := starlark.NewBuiltin("delete_cookie", rw.deleteCookieMethod)
	if _, err := rw.deleteCookieMethod(&starlark.Thread{}, b,
		starlark.Tuple{starlark.String("session")},
		[]starlark.Tuple{{starlark.String("domain"), starlark.String("example.com")}}); err != nil {
		t.Fatalf("delete_cookie: %v", err)
	}
	if len(rw.response.Cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(rw.response.Cookies))
	}
	c := rw.response.Cookies[0]
	for _, want := range []string{"session=", "Max-Age=0", "Domain=example.com"} {
		if !strings.Contains(c, want) {
			t.Errorf("delete cookie %q missing %q", c, want)
		}
	}
}

// set_cookie renders every documented attribute in one line.
func TestSetCookieAttributes(t *testing.T) {
	rw := NewResponseWrapper(&Response{})
	b := starlark.NewBuiltin("set_cookie", rw.setCookieMethod)
	if _, err := rw.setCookieMethod(&starlark.Thread{}, b,
		starlark.Tuple{starlark.String("sid"), starlark.String("abc")},
		[]starlark.Tuple{
			{starlark.String("max_age"), starlark.MakeInt(3600)},
			{starlark.String("path"), starlark.String("/app")},
			{starlark.String("domain"), starlark.String("x.test")},
			{starlark.String("secure"), starlark.True},
			{starlark.String("http_only"), starlark.True},
		}); err != nil {
		t.Fatalf("set_cookie: %v", err)
	}
	c := rw.response.Cookies[0]
	for _, want := range []string{"sid=abc", "Path=/app", "Domain=x.test", "Max-Age=3600", "Secure", "HttpOnly"} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q missing %q", c, want)
		}
	}
}

// --- cookies -----------------------------------------------------------------

// Two set_cookie calls must accumulate two distinct cookies (Set-Cookie is not
// comma-combinable), not get joined into one mangled header value.
func TestResponseSetCookieAccumulates(t *testing.T) {
	rw := NewResponseWrapper(&Response{StatusCode: 200, Headers: map[string]string{}})
	b := starlark.NewBuiltin("set_cookie", rw.setCookieMethod)

	if _, err := rw.setCookieMethod(&starlark.Thread{}, b,
		starlark.Tuple{starlark.String("a"), starlark.String("1")}, nil); err != nil {
		t.Fatalf("set_cookie(a): %v", err)
	}
	if _, err := rw.setCookieMethod(&starlark.Thread{}, b,
		starlark.Tuple{starlark.String("b"), starlark.String("2")}, nil); err != nil {
		t.Fatalf("set_cookie(b): %v", err)
	}

	if len(rw.response.Cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d: %v", len(rw.response.Cookies), rw.response.Cookies)
	}
	// Cookies must not be smuggled into the flat Headers map.
	if v, ok := rw.response.Headers["Set-Cookie"]; ok {
		t.Errorf("Set-Cookie must not be stored in Headers, found %q", v)
	}
	if !strings.HasPrefix(rw.response.Cookies[0], "a=1") {
		t.Errorf("first cookie = %q, want it to start with a=1", rw.response.Cookies[0])
	}
	if !strings.HasPrefix(rw.response.Cookies[1], "b=2") {
		t.Errorf("second cookie = %q, want it to start with b=2", rw.response.Cookies[1])
	}
}

// applyResponse must emit one Set-Cookie header line per cookie, so a client
// sees two distinct cookies rather than a single comma-joined value.
func TestApplyResponseEmitsDistinctSetCookieLines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	s := newServer(NewModule(), "localhost", 0)
	resp := &Response{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       "ok",
		Cookies:    []string{"a=1; Path=/", "b=2; Path=/"},
	}
	s.applyResponse(c, resp)

	got := rec.Result().Header["Set-Cookie"]
	if len(got) != 2 {
		t.Fatalf("expected 2 Set-Cookie header lines, got %d: %v", len(got), got)
	}
	if got[0] != "a=1; Path=/" || got[1] != "b=2; Path=/" {
		t.Errorf("Set-Cookie lines = %v, want [a=1; Path=/ b=2; Path=/]", got)
	}
}
