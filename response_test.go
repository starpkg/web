package web

// Thematic tests for the module's value builders, invoked directly through the
// Starlark builtins they back (web.go) so the package has standalone coverage
// independent of a running HTTP server.
//
// Sections:
//   - response builders: response / json_response / text_response / html_response
//   - middleware constructors: a couple of m.* builtins return MiddlewareWrappers
//   - cookies: set_cookie/delete_cookie produce distinct Set-Cookie header lines

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

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
