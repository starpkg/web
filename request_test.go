package web

// Tests for the request bridge (request.go). A *Request can be built directly,
// or from a synthetic gin.Context backed by an httptest request -- so the body
// caching, JSON/form parsing, header/cookie/param accessors, and the
// nil-context guards are all exercised without a real socket.
//
// Sections:
//   - createRequestFromGin: header/query extraction + body caching for re-reads
//   - body() / json(): cached body, empty/invalid JSON -> None
//   - form(): URL-encoded parse, single vs multi-valued keys
//   - files() + createFileInfo: multipart upload metadata + read()
//   - cookie() / param() / get_header(): present/absent + nil-context guards
//   - bearer_token() / basic_auth(): prefix handling + tuple result
//   - RequestWrapper: Attr properties, AttrNames, unknown attr error

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

func ginCtxWithRequest(req *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c
}

func callReqMethod(t *testing.T, name string, fn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error), args starlark.Tuple) starlark.Value {
	t.Helper()
	b := starlark.NewBuiltin(name, fn)
	v, err := fn(&starlark.Thread{}, b, args, nil)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

// --- createRequestFromGin ----------------------------------------------------

func TestCreateRequestFromGin(t *testing.T) {
	httpReq := httptest.NewRequest(http.MethodPost, "/path?q=1&q=2&z=9", strings.NewReader("cached-body"))
	httpReq.Header.Set("X-Custom", "v")
	c := ginCtxWithRequest(httpReq)

	req := createRequestFromGin(c)
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if req.Path != "/path" {
		t.Errorf("path = %q, want /path", req.Path)
	}
	if req.Headers["X-Custom"] != "v" {
		t.Errorf("header X-Custom = %q, want v", req.Headers["X-Custom"])
	}
	// Only the first value of a repeated query key is kept.
	if req.Query["q"] != "1" || req.Query["z"] != "9" {
		t.Errorf("query = %v, want q=1 z=9", req.Query)
	}
	// Body is cached so it can be read multiple times.
	if string(req.bodyData) != "cached-body" {
		t.Errorf("bodyData = %q, want cached-body", string(req.bodyData))
	}
}

// --- body() / json() ---------------------------------------------------------

func TestRequestBodyAndJSON(t *testing.T) {
	t.Run("body_reads_cached_bytes", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{bodyData: []byte("raw text")})
		v := callReqMethod(t, "body", rw.bodyMethod, nil)
		if string(v.(starlark.String)) != "raw text" {
			t.Errorf("body() = %v, want raw text", v)
		}
	})

	t.Run("json_parses_object", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{bodyData: []byte(`{"name":"x","n":5}`)})
		v := callReqMethod(t, "json", rw.jsonMethod, nil)
		d, ok := v.(*starlark.Dict)
		if !ok {
			t.Fatalf("json() = %T, want *starlark.Dict", v)
		}
		got, _, _ := d.Get(starlark.String("name"))
		if string(got.(starlark.String)) != "x" {
			t.Errorf("json[name] = %v, want x", got)
		}
	})

	t.Run("json_empty_body_is_none", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{bodyData: nil})
		v := callReqMethod(t, "json", rw.jsonMethod, nil)
		if v != starlark.None {
			t.Errorf("json() on empty body = %v, want None", v)
		}
	})

	t.Run("json_invalid_is_none", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{bodyData: []byte("not json{")})
		v := callReqMethod(t, "json", rw.jsonMethod, nil)
		if v != starlark.None {
			t.Errorf("json() on invalid body = %v, want None", v)
		}
	})
}

// --- form() ------------------------------------------------------------------

func TestRequestForm(t *testing.T) {
	t.Run("nil_context_returns_empty_dict", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{})
		v := callReqMethod(t, "form", rw.formMethod, nil)
		if d, ok := v.(*starlark.Dict); !ok || d.Len() != 0 {
			t.Errorf("form() with nil ctx = %v, want empty dict", v)
		}
	})

	t.Run("urlencoded_single_and_multi", func(t *testing.T) {
		body := "a=1&b=2&b=3"
		httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		httpReq.Header.Set("Content-Type", MIMEApplicationForm)
		c := ginCtxWithRequest(httpReq)
		req := createRequestFromGin(c)
		rw := NewRequestWrapper(req)

		v := callReqMethod(t, "form", rw.formMethod, nil)
		d := v.(*starlark.Dict)
		a, _, _ := d.Get(starlark.String("a"))
		if string(a.(starlark.String)) != "1" {
			t.Errorf("form[a] = %v, want 1", a)
		}
		// Repeated key becomes a list.
		bVal, _, _ := d.Get(starlark.String("b"))
		list, ok := bVal.(*starlark.List)
		if !ok || list.Len() != 2 {
			t.Errorf("form[b] = %v, want a 2-element list", bVal)
		}
	})
}

// --- files() + createFileInfo ------------------------------------------------

func TestRequestFiles(t *testing.T) {
	t.Run("nil_context_returns_empty_dict", func(t *testing.T) {
		rw := NewRequestWrapper(&Request{})
		v := callReqMethod(t, "files", rw.filesMethod, nil)
		if d, ok := v.(*starlark.Dict); !ok || d.Len() != 0 {
			t.Errorf("files() with nil ctx = %v, want empty dict", v)
		}
	})

	t.Run("multipart_upload_metadata_and_read", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("upload", "hello.txt")
		_, _ = fw.Write([]byte("file contents"))
		_ = w.Close()

		httpReq := httptest.NewRequest(http.MethodPost, "/", &buf)
		httpReq.Header.Set("Content-Type", w.FormDataContentType())
		c := ginCtxWithRequest(httpReq)
		// files() reads the multipart body via c.MultipartForm(); build the
		// Request with the live context (not createRequestFromGin, which would
		// drain the body into the re-read cache).
		rw := NewRequestWrapper(&Request{ginCtx: c, Headers: map[string]string{}, Query: map[string]string{}})

		v := callReqMethod(t, "files", rw.filesMethod, nil)
		d, ok := v.(*starlark.Dict)
		if !ok {
			t.Fatalf("files() = %T, want dict", v)
		}
		entry, found, _ := d.Get(starlark.String("upload"))
		if !found {
			t.Fatalf("upload field not present: %v", d)
		}
		fileDict := entry.(*starlark.Dict)
		name, _, _ := fileDict.Get(starlark.String("filename"))
		if string(name.(starlark.String)) != "hello.txt" {
			t.Errorf("filename = %v, want hello.txt", name)
		}
		size, _, _ := fileDict.Get(starlark.String("size"))
		if size.(starlark.Int) != starlark.MakeInt(len("file contents")) {
			t.Errorf("size = %v, want %d", size, len("file contents"))
		}
		// read() returns the file body as a string.
		readVal, _, _ := fileDict.Get(starlark.String("read"))
		readFn := readVal.(*starlark.Builtin)
		content, err := readFn.CallInternal(&starlark.Thread{}, nil, nil)
		if err != nil {
			t.Fatalf("read(): %v", err)
		}
		if string(content.(starlark.String)) != "file contents" {
			t.Errorf("read() = %v, want 'file contents'", content)
		}
	})
}

// --- cookie() / param() / get_header() ---------------------------------------

func TestRequestAccessors(t *testing.T) {
	httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
	httpReq.Header.Set("X-Token", "abc")
	httpReq.AddCookie(&http.Cookie{Name: "sess", Value: "xyz"})
	c := ginCtxWithRequest(httpReq)
	req := createRequestFromGin(c)
	rw := NewRequestWrapper(req)

	t.Run("cookie_present", func(t *testing.T) {
		v := callReqMethod(t, "cookie", rw.cookieMethod, starlark.Tuple{starlark.String("sess")})
		if string(v.(starlark.String)) != "xyz" {
			t.Errorf("cookie(sess) = %v, want xyz", v)
		}
	})
	t.Run("cookie_absent_is_none", func(t *testing.T) {
		v := callReqMethod(t, "cookie", rw.cookieMethod, starlark.Tuple{starlark.String("nope")})
		if v != starlark.None {
			t.Errorf("cookie(nope) = %v, want None", v)
		}
	})
	t.Run("get_header_present_and_default", func(t *testing.T) {
		v := callReqMethod(t, "get_header", rw.getHeaderMethod, starlark.Tuple{starlark.String("X-Token")})
		if string(v.(starlark.String)) != "abc" {
			t.Errorf("get_header(X-Token) = %v, want abc", v)
		}
		dv := callReqMethod(t, "get_header", rw.getHeaderMethod,
			starlark.Tuple{starlark.String("X-Missing"), starlark.String("fallback")})
		if string(dv.(starlark.String)) != "fallback" {
			t.Errorf("get_header default = %v, want fallback", dv)
		}
	})
	t.Run("param_absent_is_none", func(t *testing.T) {
		// No route params on a bare context -> None (not a panic).
		v := callReqMethod(t, "param", rw.paramMethod, starlark.Tuple{starlark.String("id")})
		if v != starlark.None {
			t.Errorf("param(id) = %v, want None", v)
		}
	})
}

// nil-context accessors must degrade to None/default, never panic.
func TestRequestNilContextGuards(t *testing.T) {
	rw := NewRequestWrapper(&Request{})

	if v := callReqMethod(t, "cookie", rw.cookieMethod, starlark.Tuple{starlark.String("x")}); v != starlark.None {
		t.Errorf("cookie() nil ctx = %v, want None", v)
	}
	if v := callReqMethod(t, "param", rw.paramMethod, starlark.Tuple{starlark.String("x")}); v != starlark.None {
		t.Errorf("param() nil ctx = %v, want None", v)
	}
	if v := callReqMethod(t, "basic_auth", rw.basicAuthMethod, nil); v != starlark.None {
		t.Errorf("basic_auth() nil ctx = %v, want None", v)
	}
	if v := callReqMethod(t, "bearer_token", rw.bearerTokenMethod, nil); v != starlark.None {
		t.Errorf("bearer_token() nil ctx = %v, want None", v)
	}
	// get_header with a default on a nil ctx returns the default.
	if v := callReqMethod(t, "get_header", rw.getHeaderMethod,
		starlark.Tuple{starlark.String("x"), starlark.String("d")}); string(v.(starlark.String)) != "d" {
		t.Errorf("get_header() nil ctx = %v, want d", v)
	}
}

// --- bearer_token() / basic_auth() -------------------------------------------

func TestRequestBearerToken(t *testing.T) {
	t.Run("authorization_bearer_prefix", func(t *testing.T) {
		httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
		httpReq.Header.Set("Authorization", "Bearer tok123")
		rw := NewRequestWrapper(createRequestFromGin(ginCtxWithRequest(httpReq)))
		v := callReqMethod(t, "bearer_token", rw.bearerTokenMethod, nil)
		if string(v.(starlark.String)) != "tok123" {
			t.Errorf("bearer_token() = %v, want tok123", v)
		}
	})
	t.Run("authorization_without_prefix_is_none", func(t *testing.T) {
		httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
		httpReq.Header.Set("Authorization", "Token nope")
		rw := NewRequestWrapper(createRequestFromGin(ginCtxWithRequest(httpReq)))
		v := callReqMethod(t, "bearer_token", rw.bearerTokenMethod, nil)
		if v != starlark.None {
			t.Errorf("bearer_token() non-Bearer = %v, want None", v)
		}
	})
	t.Run("custom_header_used_directly", func(t *testing.T) {
		httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
		httpReq.Header.Set("X-Token", "rawtok")
		rw := NewRequestWrapper(createRequestFromGin(ginCtxWithRequest(httpReq)))
		v := callReqMethod(t, "bearer_token", rw.bearerTokenMethod, starlark.Tuple{starlark.String("X-Token")})
		if string(v.(starlark.String)) != "rawtok" {
			t.Errorf("bearer_token(X-Token) = %v, want rawtok", v)
		}
	})
}

func TestRequestBasicAuth(t *testing.T) {
	httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
	httpReq.SetBasicAuth("alice", "pw")
	rw := NewRequestWrapper(createRequestFromGin(ginCtxWithRequest(httpReq)))
	v := callReqMethod(t, "basic_auth", rw.basicAuthMethod, nil)
	tup, ok := v.(starlark.Tuple)
	if !ok || len(tup) != 2 {
		t.Fatalf("basic_auth() = %v, want a 2-tuple", v)
	}
	if string(tup[0].(starlark.String)) != "alice" || string(tup[1].(starlark.String)) != "pw" {
		t.Errorf("basic_auth() = %v, want (alice, pw)", tup)
	}
}

// --- RequestWrapper ----------------------------------------------------------

func TestRequestWrapperAttr(t *testing.T) {
	rw := NewRequestWrapper(&Request{
		Method:   "GET",
		Path:     "/p",
		Host:     "h",
		ClientIP: "1.2.3.4",
		Headers:  map[string]string{"A": "1"},
		Query:    map[string]string{"q": "v"},
		Context:  map[string]interface{}{},
	})

	t.Run("string_properties", func(t *testing.T) {
		if v, _ := rw.Attr("method"); string(v.(starlark.String)) != "GET" {
			t.Errorf("method = %v", v)
		}
		if v, _ := rw.Attr("path"); string(v.(starlark.String)) != "/p" {
			t.Errorf("path = %v", v)
		}
		if v, _ := rw.Attr("client_ip"); string(v.(starlark.String)) != "1.2.3.4" {
			t.Errorf("client_ip = %v", v)
		}
	})

	t.Run("headers_marshalled_to_dict", func(t *testing.T) {
		v, err := rw.Attr("headers")
		if err != nil {
			t.Fatalf("Attr(headers): %v", err)
		}
		if _, ok := v.(*starlark.Dict); !ok {
			t.Errorf("headers attr is %T, want a dict", v)
		}
	})

	t.Run("method_builtins_present", func(t *testing.T) {
		for _, name := range []string{"body", "json", "form", "files", "cookie", "param", "get_header", "bearer_token", "basic_auth"} {
			if _, err := rw.Attr(name); err != nil {
				t.Errorf("Attr(%q): %v", name, err)
			}
		}
	})

	t.Run("unknown_attr_errors", func(t *testing.T) {
		if _, err := rw.Attr("ghost"); err == nil {
			t.Errorf("Attr(ghost): expected error, got nil")
		}
	})

	t.Run("attr_names_complete", func(t *testing.T) {
		if len(rw.AttrNames()) != 19 {
			t.Errorf("AttrNames() count = %d, want 19", len(rw.AttrNames()))
		}
	})
}
