package web

// Behavioural tests for the built-in middleware (middleware.go). Each middleware
// is a MiddlewareFunc -- a pure func(*Request, NextFunc) *Response -- so it can be
// driven directly with a synthetic Request and a fake `next`, needing no TTY,
// no socket, and no network.
//
// Sections:
//   - cors: preflight 204 + normal-request header injection + credentials
//   - timing: response-time header written
//   - json: JSON body parsed into context + JSON-looking body tagged
//   - compression: gzip applied only when eligible (accept-encoding/size/type)
//   - rate_limit: per-key counting, 429 over the limit, X-RateLimit-* headers
//   - cache: Cache-Control on GET, pattern gating, method gating
//   - request_size: 413 / 414 / 406 thresholds
//   - logging: passes the response through unchanged
//   - createStarlarkMiddleware: script middleware + error/non-response fallbacks
//   - MemoryRateLimitStorage: Increment / Get / Set windowing

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
)

// okNext is a fake downstream handler returning a fixed 200 response.
func okNext(body string) NextFunc {
	return func(*Request) *Response {
		return &Response{StatusCode: 200, Headers: map[string]string{}, Body: body}
	}
}

// --- cors --------------------------------------------------------------------

func TestCORSMiddleware(t *testing.T) {
	t.Run("preflight", func(t *testing.T) {
		mw := corsMiddleware([]string{"https://a.test"}, nil, nil, true)
		resp := mw(&Request{Method: http.MethodOptions, Path: "/"}, okNext("should-not-run"))
		if resp.StatusCode != 204 {
			t.Errorf("preflight status = %d, want 204", resp.StatusCode)
		}
		if resp.Body != "" {
			t.Errorf("preflight body = %q, want empty", resp.Body)
		}
		if got := resp.Headers[canonicalHeader(HeaderAccessControlAllowOrigin)]; got != "https://a.test" {
			t.Errorf("allow-origin = %q, want https://a.test", got)
		}
		if got := resp.Headers[canonicalHeader(HeaderAccessControlAllowCredentials)]; got != "true" {
			t.Errorf("allow-credentials = %q, want true", got)
		}
		// Default methods/headers must be filled in.
		if got := resp.Headers[canonicalHeader(HeaderAccessControlAllowMethods)]; !strings.Contains(got, "GET") {
			t.Errorf("allow-methods = %q, want it to include GET", got)
		}
	})

	t.Run("normal_request_default_origin", func(t *testing.T) {
		mw := corsMiddleware(nil, nil, nil, false)
		resp := mw(&Request{Method: http.MethodGet, Path: "/"}, okNext("hi"))
		if resp.Body != "hi" {
			t.Errorf("body = %q, want hi (next must run)", resp.Body)
		}
		if got := resp.Headers[canonicalHeader(HeaderAccessControlAllowOrigin)]; got != "*" {
			t.Errorf("allow-origin = %q, want * (empty origins default)", got)
		}
		// Without credentials the credentials header must not be set on normal responses.
		if _, ok := resp.Headers[canonicalHeader(HeaderAccessControlAllowCredentials)]; ok {
			t.Errorf("allow-credentials should be absent when credentials=false")
		}
	})

	t.Run("normal_request_nil_headers_map", func(t *testing.T) {
		mw := corsMiddleware([]string{"*"}, nil, nil, true)
		// next returns a response with a nil Headers map; middleware must allocate it.
		resp := mw(&Request{Method: http.MethodGet, Path: "/"}, func(*Request) *Response {
			return &Response{StatusCode: 200, Body: "x"}
		})
		if resp.Headers[canonicalHeader(HeaderAccessControlAllowCredentials)] != "true" {
			t.Errorf("credentials header not set on nil-headers response")
		}
	})
}

// --- timing ------------------------------------------------------------------

func TestTimingMiddleware(t *testing.T) {
	t.Run("default_header", func(t *testing.T) {
		mw := timingMiddleware("") // empty -> default X-Response-Time
		resp := mw(&Request{Method: http.MethodGet, Path: "/"}, okNext("ok"))
		if got := resp.Headers[canonicalHeader(HeaderXResponseTime)]; !strings.HasSuffix(got, "ms") {
			t.Errorf("timing header = %q, want a value ending in ms", got)
		}
	})
	t.Run("custom_header", func(t *testing.T) {
		mw := timingMiddleware("X-Took")
		resp := mw(&Request{Method: http.MethodGet, Path: "/"}, okNext("ok"))
		if _, ok := resp.Headers[canonicalHeader("X-Took")]; !ok {
			t.Errorf("custom timing header X-Took missing: %v", resp.Headers)
		}
	})
}

// --- json --------------------------------------------------------------------

func TestJSONMiddleware(t *testing.T) {
	t.Run("parses_json_body_into_context", func(t *testing.T) {
		mw := jsonMiddleware()
		req := &Request{
			Method:   http.MethodPost,
			Path:     "/",
			Headers:  map[string]string{canonicalHeader(HeaderContentType): MIMEApplicationJSON},
			Context:  map[string]interface{}{},
			bodyData: []byte(`{"a":1}`),
		}
		var seen interface{}
		resp := mw(req, func(r *Request) *Response {
			seen = r.Context["json_data"]
			return &Response{StatusCode: 200, Headers: map[string]string{}}
		})
		if seen == nil {
			t.Errorf("json_data not stored in request context")
		}
		_ = resp
	})

	t.Run("tags_json_looking_body", func(t *testing.T) {
		mw := jsonMiddleware()
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{}, Context: map[string]interface{}{}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Body: `{"ok":true}`}
		})
		if got := resp.Headers[canonicalHeader(HeaderContentType)]; got != MIMEApplicationJSON {
			t.Errorf("content-type = %q, want %q for JSON-looking body", got, MIMEApplicationJSON)
		}
	})

	t.Run("leaves_non_json_body_untouched", func(t *testing.T) {
		mw := jsonMiddleware()
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{}, Context: map[string]interface{}{}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Body: "plain text"}
		})
		if _, ok := resp.Headers[canonicalHeader(HeaderContentType)]; ok {
			t.Errorf("content-type should not be set for a non-JSON body: %v", resp.Headers)
		}
	})
}

// --- compression -------------------------------------------------------------

func TestCompressionMiddleware(t *testing.T) {
	big := strings.Repeat("A", 4096)

	t.Run("compresses_eligible_response", func(t *testing.T) {
		mw := compressionMiddleware(6, 1024, nil)
		req := &Request{
			Method:  http.MethodGet,
			Path:    "/",
			Headers: map[string]string{canonicalHeader("Accept-Encoding"): "gzip, deflate"},
		}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Headers: map[string]string{canonicalHeader(HeaderContentType): MIMETextPlain}, Body: big}
		})
		if got := resp.Headers[canonicalHeader(HeaderContentEncoding)]; got != "gzip" {
			t.Fatalf("content-encoding = %q, want gzip", got)
		}
		// The body must be valid gzip that decodes back to the original.
		gr, err := gzip.NewReader(strings.NewReader(resp.Body))
		if err != nil {
			t.Fatalf("gzip.NewReader: %v", err)
		}
		decoded, _ := io.ReadAll(gr)
		if string(decoded) != big {
			t.Errorf("decoded body length = %d, want %d", len(decoded), len(big))
		}
	})

	t.Run("skips_when_client_does_not_accept_gzip", func(t *testing.T) {
		mw := compressionMiddleware(6, 1024, nil)
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Headers: map[string]string{canonicalHeader(HeaderContentType): MIMETextPlain}, Body: big}
		})
		if _, ok := resp.Headers[canonicalHeader(HeaderContentEncoding)]; ok {
			t.Errorf("must not compress when gzip not accepted")
		}
	})

	t.Run("skips_small_body", func(t *testing.T) {
		mw := compressionMiddleware(6, 1024, nil)
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{canonicalHeader("Accept-Encoding"): "gzip"}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Headers: map[string]string{canonicalHeader(HeaderContentType): MIMETextPlain}, Body: "tiny"}
		})
		if resp.Body != "tiny" {
			t.Errorf("small body should be left uncompressed, got %q", resp.Body)
		}
	})

	t.Run("skips_incompressible_type", func(t *testing.T) {
		mw := compressionMiddleware(6, 1024, nil)
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{canonicalHeader("Accept-Encoding"): "gzip"}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Headers: map[string]string{canonicalHeader(HeaderContentType): "image/png"}, Body: big}
		})
		if _, ok := resp.Headers[canonicalHeader(HeaderContentEncoding)]; ok {
			t.Errorf("image/png should not be compressed")
		}
	})

	t.Run("out_of_range_level_falls_back", func(t *testing.T) {
		// A level outside 1..9 must not error/panic; it falls back to the default.
		mw := compressionMiddleware(99, 1024, []string{MIMETextPlain})
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{canonicalHeader("Accept-Encoding"): "gzip"}}
		resp := mw(req, func(*Request) *Response {
			return &Response{StatusCode: 200, Headers: map[string]string{canonicalHeader(HeaderContentType): MIMETextPlain}, Body: big}
		})
		if resp.Headers[canonicalHeader(HeaderContentEncoding)] != "gzip" {
			t.Errorf("expected gzip with fallback level")
		}
	})
}

// --- rate_limit --------------------------------------------------------------

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("allows_under_limit_and_sets_headers", func(t *testing.T) {
		mw := rateLimitMiddleware(2, 60, nil, NewMemoryRateLimitStorage())
		req := &Request{Method: http.MethodGet, Path: "/", ClientIP: "1.2.3.4"}
		resp := mw(req, okNext("ok"))
		if resp.StatusCode != 200 {
			t.Fatalf("first request status = %d, want 200", resp.StatusCode)
		}
		if resp.Headers[HeaderXRateLimitLimit] != "2" {
			t.Errorf("X-RateLimit-Limit = %q, want 2", resp.Headers[HeaderXRateLimitLimit])
		}
		if resp.Headers[HeaderXRateLimitRemaining] != "1" {
			t.Errorf("X-RateLimit-Remaining = %q, want 1", resp.Headers[HeaderXRateLimitRemaining])
		}
	})

	t.Run("rejects_over_limit_with_429", func(t *testing.T) {
		store := NewMemoryRateLimitStorage()
		mw := rateLimitMiddleware(1, 60, nil, store)
		req := &Request{Method: http.MethodGet, Path: "/", ClientIP: "9.9.9.9"}
		if r := mw(req, okNext("ok")); r.StatusCode != 200 {
			t.Fatalf("first request status = %d, want 200", r.StatusCode)
		}
		r := mw(req, okNext("ok"))
		if r.StatusCode != http.StatusTooManyRequests {
			t.Errorf("over-limit status = %d, want 429", r.StatusCode)
		}
		if got := r.Headers[canonicalHeader(HeaderRetryAfter)]; got == "" {
			t.Errorf("429 response missing Retry-After header")
		}
	})

	t.Run("separate_keys_counted_independently", func(t *testing.T) {
		store := NewMemoryRateLimitStorage()
		mw := rateLimitMiddleware(1, 60, nil, store)
		a := &Request{Method: http.MethodGet, Path: "/", ClientIP: "10.0.0.1"}
		b := &Request{Method: http.MethodGet, Path: "/", ClientIP: "10.0.0.2"}
		if r := mw(a, okNext("ok")); r.StatusCode != 200 {
			t.Fatalf("client a first = %d, want 200", r.StatusCode)
		}
		if r := mw(b, okNext("ok")); r.StatusCode != 200 {
			t.Errorf("client b first = %d, want 200 (distinct key)", r.StatusCode)
		}
	})
}

// --- cache -------------------------------------------------------------------

func TestCacheMiddleware(t *testing.T) {
	t.Run("sets_cache_control_on_get", func(t *testing.T) {
		mw := cacheMiddleware(120, false, nil, []string{"Accept-Encoding"})
		resp := mw(&Request{Method: http.MethodGet, Path: "/x"}, okNext("ok"))
		if got := resp.Headers[canonicalHeader(HeaderCacheControl)]; got != "public, max-age=120" {
			t.Errorf("cache-control = %q, want public, max-age=120", got)
		}
		if got := resp.Headers[canonicalHeader(HeaderVary)]; got != "Accept-Encoding" {
			t.Errorf("vary = %q, want Accept-Encoding", got)
		}
	})

	t.Run("private_directive", func(t *testing.T) {
		mw := cacheMiddleware(30, true, nil, nil)
		resp := mw(&Request{Method: http.MethodGet, Path: "/x"}, okNext("ok"))
		if got := resp.Headers[canonicalHeader(HeaderCacheControl)]; got != "private, max-age=30" {
			t.Errorf("cache-control = %q, want private, max-age=30", got)
		}
	})

	t.Run("skips_non_get", func(t *testing.T) {
		mw := cacheMiddleware(60, false, nil, nil)
		resp := mw(&Request{Method: http.MethodPost, Path: "/x"}, okNext("ok"))
		if _, ok := resp.Headers[canonicalHeader(HeaderCacheControl)]; ok {
			t.Errorf("POST should not get a Cache-Control header")
		}
	})

	t.Run("pattern_gating", func(t *testing.T) {
		mw := cacheMiddleware(60, false, []string{"/api/*"}, nil)
		match := mw(&Request{Method: http.MethodGet, Path: "/api/users"}, okNext("ok"))
		if _, ok := match.Headers[canonicalHeader(HeaderCacheControl)]; !ok {
			t.Errorf("matching path should get Cache-Control")
		}
		miss := mw(&Request{Method: http.MethodGet, Path: "/other"}, okNext("ok"))
		if _, ok := miss.Headers[canonicalHeader(HeaderCacheControl)]; ok {
			t.Errorf("non-matching path must not get Cache-Control")
		}
	})
}

// --- request_size ------------------------------------------------------------

func TestRequestSizeMiddleware(t *testing.T) {
	t.Run("rejects_oversized_body_413", func(t *testing.T) {
		mw := requestSizeMiddleware(4, 2048, 100)
		req := &Request{Method: http.MethodPost, Path: "/", Headers: map[string]string{}, bodyData: []byte("toolong")}
		resp := mw(req, okNext("ok"))
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", resp.StatusCode)
		}
	})

	t.Run("rejects_long_url_414", func(t *testing.T) {
		mw := requestSizeMiddleware(1<<20, 5, 100)
		req := &Request{Method: http.MethodGet, Path: "/this/is/long", Headers: map[string]string{}}
		resp := mw(req, okNext("ok"))
		if resp.StatusCode != http.StatusRequestURITooLong {
			t.Errorf("status = %d, want 414", resp.StatusCode)
		}
	})

	t.Run("rejects_too_many_headers_406", func(t *testing.T) {
		mw := requestSizeMiddleware(1<<20, 2048, 1)
		req := &Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{"A": "1", "B": "2"}}
		resp := mw(req, okNext("ok"))
		if resp.StatusCode != http.StatusNotAcceptable {
			t.Errorf("status = %d, want 406", resp.StatusCode)
		}
	})

	t.Run("passes_within_limits", func(t *testing.T) {
		mw := requestSizeMiddleware(1<<20, 2048, 100)
		req := &Request{Method: http.MethodGet, Path: "/ok", Headers: map[string]string{"A": "1"}, Query: map[string]string{"q": "1"}}
		resp := mw(req, okNext("through"))
		if resp.StatusCode != 200 || resp.Body != "through" {
			t.Errorf("within-limits request blocked: status=%d body=%q", resp.StatusCode, resp.Body)
		}
	})
}

// --- logging -----------------------------------------------------------------

// logging_middleware must pass the downstream response through unchanged.
func TestLoggingMiddlewarePassThrough(t *testing.T) {
	mw := loggingMiddleware("")
	resp := mw(&Request{Method: http.MethodGet, Path: "/log"}, okNext("body"))
	if resp.StatusCode != 200 || resp.Body != "body" {
		t.Errorf("logging middleware altered response: status=%d body=%q", resp.StatusCode, resp.Body)
	}
}

// --- createStarlarkMiddleware ------------------------------------------------

func TestCreateStarlarkMiddleware(t *testing.T) {
	run := func(t *testing.T, src string) *Response {
		t.Helper()
		thread := &starlark.Thread{}
		globals, err := starlark.ExecFile(thread, "mw.star", src, nil)
		if err != nil {
			t.Fatalf("ExecFile: %v", err)
		}
		fn, ok := globals["mw"].(starlark.Callable)
		if !ok {
			t.Fatalf("script must define a callable `mw`")
		}
		gomw := createStarlarkMiddleware(fn)
		return gomw(&Request{Method: http.MethodGet, Path: "/", Headers: map[string]string{}, Context: map[string]interface{}{}},
			okNext("downstream"))
	}

	t.Run("calls_next_and_mutates_response", func(t *testing.T) {
		resp := run(t, `
def mw(req, next):
    resp = next(req)
    resp.set_header("X-Mw", "seen")
    return resp
`)
		if resp.Body != "downstream" {
			t.Errorf("body = %q, want downstream (next must run)", resp.Body)
		}
		if resp.Headers["X-Mw"] != "seen" {
			t.Errorf("middleware header not applied: %v", resp.Headers)
		}
	})

	t.Run("script_error_becomes_500", func(t *testing.T) {
		resp := run(t, `
def mw(req, next):
    fail("boom")
`)
		if resp.StatusCode != 500 {
			t.Errorf("status = %d, want 500 on script error", resp.StatusCode)
		}
		if !strings.Contains(resp.Body, "Middleware error") {
			t.Errorf("body = %q, want a Middleware error message", resp.Body)
		}
	})

	t.Run("non_response_return_becomes_500", func(t *testing.T) {
		resp := run(t, `
def mw(req, next):
    return "not a response"
`)
		if resp.StatusCode != 500 {
			t.Errorf("status = %d, want 500 when middleware returns a non-response", resp.StatusCode)
		}
		if !strings.Contains(resp.Body, "must return a response") {
			t.Errorf("body = %q, want a 'must return a response' message", resp.Body)
		}
	})
}

// --- MemoryRateLimitStorage --------------------------------------------------

func TestMemoryRateLimitStorage(t *testing.T) {
	s := NewMemoryRateLimitStorage()

	t.Run("increment_counts_within_window", func(t *testing.T) {
		n, err := s.Increment("k", time.Minute)
		if err != nil || n != 1 {
			t.Fatalf("first increment = (%d, %v), want (1, nil)", n, err)
		}
		n, _ = s.Increment("k", time.Minute)
		if n != 2 {
			t.Errorf("second increment = %d, want 2", n)
		}
	})

	t.Run("get_returns_current_count", func(t *testing.T) {
		if v, _ := s.Get("k"); v != 2 {
			t.Errorf("Get(k) = %d, want 2", v)
		}
		if v, _ := s.Get("absent"); v != 0 {
			t.Errorf("Get(absent) = %d, want 0", v)
		}
	})

	t.Run("expired_entry_resets", func(t *testing.T) {
		if _, err := s.Increment("short", time.Nanosecond); err != nil {
			t.Fatalf("increment short: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
		// An expired window starts a fresh count of 1.
		if n, _ := s.Increment("short", time.Minute); n != 1 {
			t.Errorf("expired Increment = %d, want 1", n)
		}
		// And a Get on a (now re-set but originally) expired key reads 0 before re-set.
		if v, _ := s.Get("expired-only"); v != 0 {
			t.Errorf("Get on never-set key = %d, want 0", v)
		}
	})

	t.Run("set_then_get", func(t *testing.T) {
		if err := s.Set("preset", 5, time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if v, _ := s.Get("preset"); v != 5 {
			t.Errorf("Get(preset) = %d, want 5", v)
		}
	})
}
