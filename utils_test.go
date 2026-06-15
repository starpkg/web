package web

// Unit tests for the pure converters / formatters / helpers (utils.go) plus the
// ErrorHandlerRegistry (errors.go). These are deterministic, side-effect-free
// functions, ideal for table-driven coverage with no TTY or network.
//
// Sections:
//   - starlark->go converters: list/dict/int/bool, including the error branches
//   - canonicalHeader / parseContentType / isCompressibleContentType / max
//   - canned error-response builders: shape + status codes
//   - ErrorHandlerRegistry: register/get + default & script-handler HandleError

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// --- starlark->go converters -------------------------------------------------

func TestStarlarkListToStringSlice(t *testing.T) {
	t.Run("nil_list", func(t *testing.T) {
		got, err := starlarkListToStringSlice(nil)
		if err != nil || len(got) != 0 {
			t.Errorf("nil list = (%v, %v), want ([], nil)", got, err)
		}
	})
	t.Run("string_elements", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{starlark.String("a"), starlark.String("b")})
		got, err := starlarkListToStringSlice(l)
		if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("= (%v, %v), want ([a b], nil)", got, err)
		}
	})
	t.Run("non_string_element_errors", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{starlark.String("a"), starlark.MakeInt(2)})
		_, err := starlarkListToStringSlice(l)
		if err == nil || !strings.Contains(err.Error(), "index 1") {
			t.Errorf("err = %v, want one mentioning index 1", err)
		}
	})
}

func TestStarlarkDictToStringMap(t *testing.T) {
	t.Run("nil_dict", func(t *testing.T) {
		got, err := starlarkDictToStringMap(nil)
		if err != nil || len(got) != 0 {
			t.Errorf("nil dict = (%v, %v), want ({}, nil)", got, err)
		}
	})
	t.Run("string_pairs", func(t *testing.T) {
		d := starlark.NewDict(1)
		_ = d.SetKey(starlark.String("k"), starlark.String("v"))
		got, err := starlarkDictToStringMap(d)
		if err != nil || got["k"] != "v" {
			t.Errorf("= (%v, %v), want ({k:v}, nil)", got, err)
		}
	})
	t.Run("non_string_value_errors", func(t *testing.T) {
		d := starlark.NewDict(1)
		_ = d.SetKey(starlark.String("k"), starlark.MakeInt(1))
		_, err := starlarkDictToStringMap(d)
		if err == nil || !strings.Contains(err.Error(), "non-string") {
			t.Errorf("err = %v, want a non-string key/value error", err)
		}
	})
}

func TestStarlarkIntToInt(t *testing.T) {
	cases := []struct {
		name    string
		value   starlark.Value
		want    int
		wantErr bool
	}{
		{"plain_int", starlark.MakeInt(42), 42, false},
		{"zero", starlark.MakeInt(0), 0, false},
		{"negative", starlark.MakeInt(-5), -5, false},
		{"not_an_int", starlark.String("7"), 0, true},
		{"bool_is_not_int", starlark.True, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := starlarkIntToInt(c.value)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error for %v", c.value)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("= (%d, %v), want (%d, nil)", got, err, c.want)
			}
		})
	}
}

func TestStarlarkBoolToBool(t *testing.T) {
	if v, err := starlarkBoolToBool(starlark.True); err != nil || !v {
		t.Errorf("True = (%v, %v), want (true, nil)", v, err)
	}
	if v, err := starlarkBoolToBool(starlark.False); err != nil || v {
		t.Errorf("False = (%v, %v), want (false, nil)", v, err)
	}
	if _, err := starlarkBoolToBool(starlark.MakeInt(1)); err == nil {
		t.Errorf("int should not convert to bool")
	}
}

// --- canonicalHeader / parseContentType / isCompressibleContentType / max ----

func TestCanonicalHeader(t *testing.T) {
	cases := map[string]string{
		"content-type":  "Content-Type",
		"CONTENT-TYPE":  "Content-Type",
		"x-api-key":     "X-Api-Key",
		"Authorization": "Authorization",
	}
	for in, want := range cases {
		if got := canonicalHeader(in); got != want {
			t.Errorf("canonicalHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseContentType(t *testing.T) {
	cases := map[string]string{
		"text/plain":                  "text/plain",
		"text/html; charset=utf-8":    "text/html",
		"application/json;charset=x":  "application/json",
		"":                            "",
		"application/octet-stream; q": "application/octet-stream",
	}
	for in, want := range cases {
		if got := parseContentType(in); got != want {
			t.Errorf("parseContentType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCompressibleContentType(t *testing.T) {
	compressible := []string{"text/plain", "text/html; charset=utf-8", "application/json", "application/javascript", "image/svg+xml"}
	for _, ct := range compressible {
		if !isCompressibleContentType(ct) {
			t.Errorf("isCompressibleContentType(%q) = false, want true", ct)
		}
	}
	incompressible := []string{"image/png", "application/octet-stream", "video/mp4", ""}
	for _, ct := range incompressible {
		if isCompressibleContentType(ct) {
			t.Errorf("isCompressibleContentType(%q) = true, want false", ct)
		}
	}
}

func TestMaxHelper(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{1, 2, 2}, {5, 3, 5}, {4, 4, 4}, {-1, -5, -1},
	}
	for _, c := range cases {
		if got := max(c.a, c.b); got != c.want {
			t.Errorf("max(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCreateRateLimitKey(t *testing.T) {
	if got := createRateLimitKey("rate_limit", "1.2.3.4"); got != "rate_limit:1.2.3.4" {
		t.Errorf("createRateLimitKey = %q, want rate_limit:1.2.3.4", got)
	}
}

// --- canned error-response builders ------------------------------------------

func TestCannedErrorResponseBuilders(t *testing.T) {
	t.Run("json_error", func(t *testing.T) {
		r := createJSONErrorResponse(403, "nope")
		if r.StatusCode != 403 {
			t.Errorf("status = %d, want 403", r.StatusCode)
		}
		if r.Headers[canonicalHeader(HeaderContentType)] != MIMEApplicationJSON {
			t.Errorf("content-type not JSON: %v", r.Headers)
		}
		if !strings.Contains(r.Body, `"code":403`) || !strings.Contains(r.Body, "nope") {
			t.Errorf("body = %q, want it to carry message + code", r.Body)
		}
	})

	t.Run("basic_auth_challenge", func(t *testing.T) {
		r := createBasicAuthChallengeResponse(401, "denied", "MyRealm")
		if got := r.Headers[canonicalHeader(HeaderWWWAuthenticate)]; got != `Basic realm="MyRealm"` {
			t.Errorf("WWW-Authenticate = %q, want Basic realm=\"MyRealm\"", got)
		}
	})

	t.Run("status_codes", func(t *testing.T) {
		if createTooManyRequestsResponse(30).StatusCode != 429 {
			t.Error("too-many-requests status != 429")
		}
		if got := createTooManyRequestsResponse(30).Headers[canonicalHeader(HeaderRetryAfter)]; got != "30" {
			t.Errorf("Retry-After = %q, want 30", got)
		}
		if createRequestEntityTooLargeResponse(100).StatusCode != 413 {
			t.Error("entity-too-large status != 413")
		}
		if createURITooLongResponse(50).StatusCode != 414 {
			t.Error("uri-too-long status != 414")
		}
		if createNotAcceptableResponse("x").StatusCode != 406 {
			t.Error("not-acceptable status != 406")
		}
	})
}

// --- ErrorHandlerRegistry ----------------------------------------------------

func TestErrorHandlerRegistry(t *testing.T) {
	t.Run("default_response_when_unregistered", func(t *testing.T) {
		reg := NewErrorHandlerRegistry()
		if reg.GetHandler(404) != nil {
			t.Errorf("unregistered handler should be nil")
		}
		resp := reg.HandleError(404, &Request{})
		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
		if !strings.Contains(resp.Body, `"code":404`) || !strings.Contains(resp.Body, "Not Found") {
			t.Errorf("default body = %q, want it to carry the status text + code", resp.Body)
		}
	})

	t.Run("registered_handler_renders_body", func(t *testing.T) {
		reg := NewErrorHandlerRegistry()
		src := `
def handler(req):
    return resp
`
		// Build a script handler that returns a ResponseWrapper provided as a global.
		thread := &starlark.Thread{}
		predeclared := starlark.StringDict{
			"resp": NewResponseWrapper(&Response{StatusCode: 404, Body: "custom 404", Headers: map[string]string{}}),
		}
		globals, err := starlark.ExecFile(thread, "h.star", src, predeclared)
		if err != nil {
			t.Fatalf("ExecFile: %v", err)
		}
		reg.RegisterHandler([]int{404, 410}, globals["handler"].(starlark.Callable))

		if reg.GetHandler(410) == nil {
			t.Errorf("handler should be registered for 410 too")
		}
		resp := reg.HandleError(404, &Request{Headers: map[string]string{}, Query: map[string]string{}, Context: map[string]interface{}{}})
		if resp.Body != "custom 404" {
			t.Errorf("body = %q, want custom 404", resp.Body)
		}
	})

	t.Run("handler_error_becomes_500", func(t *testing.T) {
		reg := NewErrorHandlerRegistry()
		src := `
def handler(req):
    fail("handler exploded")
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "h.star", src, nil)
		reg.RegisterHandler([]int{500}, globals["handler"].(starlark.Callable))
		resp := reg.HandleError(500, &Request{Headers: map[string]string{}, Query: map[string]string{}, Context: map[string]interface{}{}})
		if resp.StatusCode != 500 || !strings.Contains(resp.Body, "Error handler failed") {
			t.Errorf("got status=%d body=%q, want a 500 'Error handler failed'", resp.StatusCode, resp.Body)
		}
	})

	t.Run("handler_non_response_becomes_500", func(t *testing.T) {
		reg := NewErrorHandlerRegistry()
		src := `
def handler(req):
    return 123
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "h.star", src, nil)
		reg.RegisterHandler([]int{418}, globals["handler"].(starlark.Callable))
		resp := reg.HandleError(418, &Request{Headers: map[string]string{}, Query: map[string]string{}, Context: map[string]interface{}{}})
		if resp.StatusCode != 500 || !strings.Contains(resp.Body, "must return a response") {
			t.Errorf("got status=%d body=%q, want a 500 'must return a response'", resp.StatusCode, resp.Body)
		}
	})
}
