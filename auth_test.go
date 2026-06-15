package web

// Behavioural tests for the authenticators (auth.go). Each Authenticator's
// Middleware() is a MiddlewareFunc and authenticate() works on a plain *Request,
// so the full accept/reject logic is exercised without a socket or a real
// browser challenge.
//
// Sections:
//   - constructors: api_key_auth / bearer_auth / basic_auth + their arg errors
//   - api_key: header / query / missing / invalid
//   - bearer: prefix handling, custom header, validate_func accept/reject/error
//   - basic: decode, credential match, malformed header, challenge response
//   - Middleware(): stores auth_user on success, short-circuits on failure
//   - AuthenticatorWrapper: Attr / middleware() / unknown attr

import (
	"encoding/base64"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// constByValidator returns a Starlark validate_func that accepts exactly `good`.
func constByValidator(t *testing.T, good string) starlark.Callable {
	t.Helper()
	src := `
def validate(token):
    return {"user": "ok"} if token == "` + good + `" else None
`
	globals, err := starlark.ExecFile(&starlark.Thread{}, "v.star", src, nil)
	if err != nil {
		t.Fatalf("ExecFile validator: %v", err)
	}
	return globals["validate"].(starlark.Callable)
}

func newAuthFromBuiltin(t *testing.T, fn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error), args starlark.Tuple, kwargs []starlark.Tuple) *Authenticator {
	t.Helper()
	v := callBuiltin(t, "auth", fn, args, kwargs)
	aw, ok := v.(*AuthenticatorWrapper)
	if !ok {
		t.Fatalf("expected *AuthenticatorWrapper, got %T", v)
	}
	return aw.auth
}

// --- constructors ------------------------------------------------------------

func TestAuthConstructorArgErrors(t *testing.T) {
	m := NewModule()
	badList := starlark.NewList([]starlark.Value{starlark.MakeInt(1)})
	badDict := starlark.NewDict(1)
	_ = badDict.SetKey(starlark.String("u"), starlark.MakeInt(1)) // non-string value

	t.Run("api_key_bad_keys", func(t *testing.T) {
		err := callBuiltinErr(t, "api_key_auth", m.apiKeyAuth, starlark.Tuple{badList}, nil)
		if !strings.Contains(err.Error(), "invalid keys") {
			t.Errorf("error = %q, want invalid keys", err)
		}
	})
	t.Run("basic_bad_users", func(t *testing.T) {
		err := callBuiltinErr(t, "basic_auth", m.basicAuth, starlark.Tuple{badDict}, nil)
		if !strings.Contains(err.Error(), "invalid users") {
			t.Errorf("error = %q, want invalid users", err)
		}
	})
	t.Run("bearer_missing_validate_func", func(t *testing.T) {
		// validate_func is required; omitting it must error, never panic later.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("bearer_auth with no validate_func panicked: %v", r)
			}
		}()
		_ = callBuiltinErr(t, "bearer_auth", m.bearerAuth, nil, nil)
	})
}

// --- api_key -----------------------------------------------------------------

func TestAPIKeyAuthenticate(t *testing.T) {
	m := NewModule()
	keys := starlark.NewList([]starlark.Value{starlark.String("secret1"), starlark.String("secret2")})
	auth := newAuthFromBuiltin(t, m.apiKeyAuth, starlark.Tuple{keys}, nil)

	t.Run("valid_via_header", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAPIKey): "secret1"}, Query: map[string]string{}}
		res := auth.authenticate(req)
		if !res.Success {
			t.Errorf("expected success, got %+v", res)
		}
	})
	t.Run("valid_via_query", func(t *testing.T) {
		req := &Request{Headers: map[string]string{}, Query: map[string]string{"api_key": "secret2"}}
		res := auth.authenticate(req)
		if !res.Success {
			t.Errorf("expected success via query, got %+v", res)
		}
	})
	t.Run("missing_key_401", func(t *testing.T) {
		req := &Request{Headers: map[string]string{}, Query: map[string]string{}}
		res := auth.authenticate(req)
		if res.Success || res.ErrorCode != 401 {
			t.Errorf("expected 401 failure, got %+v", res)
		}
		if res.Message != "API key required" {
			t.Errorf("message = %q, want 'API key required'", res.Message)
		}
	})
	t.Run("invalid_key_401", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAPIKey): "wrong"}, Query: map[string]string{}}
		res := auth.authenticate(req)
		if res.Success || res.Message != "Invalid API key" {
			t.Errorf("expected 'Invalid API key' failure, got %+v", res)
		}
	})
}

// --- bearer ------------------------------------------------------------------

func TestBearerAuthenticate(t *testing.T) {
	m := NewModule()
	validator := constByValidator(t, "goodtoken")
	auth := newAuthFromBuiltin(t, m.bearerAuth,
		starlark.Tuple{validator}, nil)

	t.Run("valid_bearer_prefix", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Bearer goodtoken"}}
		res := auth.authenticate(req)
		if !res.Success {
			t.Errorf("expected success, got %+v", res)
		}
	})
	t.Run("rejected_token", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Bearer badtoken"}}
		res := auth.authenticate(req)
		if res.Success || res.ErrorCode != 401 || res.Message != "Invalid token" {
			t.Errorf("expected 'Invalid token' 401, got %+v", res)
		}
	})
	t.Run("missing_header", func(t *testing.T) {
		req := &Request{Headers: map[string]string{}}
		res := auth.authenticate(req)
		if res.Success || res.Message != "Authorization header required" {
			t.Errorf("expected 'Authorization header required', got %+v", res)
		}
	})
	t.Run("non_bearer_format", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Token xyz"}}
		res := auth.authenticate(req)
		if res.Success || res.Message != "Bearer token format required" {
			t.Errorf("expected 'Bearer token format required', got %+v", res)
		}
	})

	t.Run("custom_header_used_directly", func(t *testing.T) {
		customAuth := newAuthFromBuiltin(t, m.bearerAuth,
			starlark.Tuple{constByValidator(t, "rawtoken")},
			[]starlark.Tuple{{starlark.String("header"), starlark.String("X-Token")}})
		req := &Request{Headers: map[string]string{canonicalHeader("X-Token"): "rawtoken"}}
		res := customAuth.authenticate(req)
		if !res.Success {
			t.Errorf("expected custom-header success, got %+v", res)
		}
	})

	t.Run("validate_func_error_becomes_500", func(t *testing.T) {
		src := `
def validate(token):
    fail("validator exploded")
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "v.star", src, nil)
		errAuth := newAuthFromBuiltin(t, m.bearerAuth,
			starlark.Tuple{globals["validate"].(starlark.Callable)}, nil)
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Bearer anything"}}
		res := errAuth.authenticate(req)
		if res.Success || res.ErrorCode != 500 || res.Message != "Token validation error" {
			t.Errorf("expected 500 'Token validation error', got %+v", res)
		}
	})
}

// --- basic -------------------------------------------------------------------

func TestBasicAuthenticate(t *testing.T) {
	m := NewModule()
	users := starlark.NewDict(1)
	_ = users.SetKey(starlark.String("admin"), starlark.String("hunter2"))
	auth := newAuthFromBuiltin(t, m.basicAuth, starlark.Tuple{users}, nil)

	basicHeader := func(u, p string) map[string]string {
		return map[string]string{
			canonicalHeader(HeaderAuthorization): "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p)),
		}
	}

	t.Run("valid", func(t *testing.T) {
		res := auth.authenticate(&Request{Headers: basicHeader("admin", "hunter2")})
		if !res.Success {
			t.Errorf("expected success, got %+v", res)
		}
	})
	t.Run("wrong_password", func(t *testing.T) {
		res := auth.authenticate(&Request{Headers: basicHeader("admin", "nope")})
		if res.Success || res.Message != "Invalid credentials" {
			t.Errorf("expected 'Invalid credentials', got %+v", res)
		}
	})
	t.Run("missing_header_challenge", func(t *testing.T) {
		res := auth.authenticate(&Request{Headers: map[string]string{}})
		if res.Success || res.ErrorCode != 401 {
			t.Errorf("expected 401, got %+v", res)
		}
		if !strings.Contains(res.Message, "Basic realm=") {
			t.Errorf("message = %q, want it to carry the Basic realm", res.Message)
		}
	})
	t.Run("not_basic_scheme", func(t *testing.T) {
		res := auth.authenticate(&Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Bearer x"}})
		if res.Success || res.Message != "Basic authentication required" {
			t.Errorf("expected 'Basic authentication required', got %+v", res)
		}
	})
	t.Run("undecodable_base64", func(t *testing.T) {
		res := auth.authenticate(&Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Basic !!!notbase64"}})
		if res.Success || res.Message != "Invalid credentials format" {
			t.Errorf("expected 'Invalid credentials format', got %+v", res)
		}
	})
	t.Run("missing_colon", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte("nocolon"))
		res := auth.authenticate(&Request{Headers: map[string]string{canonicalHeader(HeaderAuthorization): "Basic " + enc}})
		if res.Success || res.Message != "Invalid credentials format" {
			t.Errorf("expected 'Invalid credentials format', got %+v", res)
		}
	})
}

// --- Middleware() ------------------------------------------------------------

func TestAuthenticatorMiddleware(t *testing.T) {
	m := NewModule()
	keys := starlark.NewList([]starlark.Value{starlark.String("k")})
	auth := newAuthFromBuiltin(t, m.apiKeyAuth, starlark.Tuple{keys}, nil)
	mw := auth.Middleware()

	t.Run("success_stores_auth_user_and_calls_next", func(t *testing.T) {
		req := &Request{Headers: map[string]string{canonicalHeader(HeaderAPIKey): "k"}, Query: map[string]string{}, Context: map[string]interface{}{}}
		called := false
		resp := mw(req, func(*Request) *Response {
			called = true
			return &Response{StatusCode: 200, Headers: map[string]string{}}
		})
		if !called {
			t.Errorf("next was not called on successful auth")
		}
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if _, ok := req.Context["auth_user"]; !ok {
			t.Errorf("auth_user not stored in context: %v", req.Context)
		}
	})

	t.Run("failure_short_circuits_with_401", func(t *testing.T) {
		req := &Request{Headers: map[string]string{}, Query: map[string]string{}, Context: map[string]interface{}{}}
		called := false
		resp := mw(req, func(*Request) *Response {
			called = true
			return &Response{StatusCode: 200}
		})
		if called {
			t.Errorf("next must NOT be called when auth fails")
		}
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("basic_failure_adds_www_authenticate", func(t *testing.T) {
		users := starlark.NewDict(1)
		_ = users.SetKey(starlark.String("u"), starlark.String("p"))
		basic := newAuthFromBuiltin(t, m.basicAuth, starlark.Tuple{users},
			[]starlark.Tuple{{starlark.String("realm"), starlark.String("Zone")}})
		resp := basic.Middleware()(&Request{Headers: map[string]string{}, Context: map[string]interface{}{}},
			func(*Request) *Response { return &Response{StatusCode: 200} })
		if got := resp.Headers[canonicalHeader(HeaderWWWAuthenticate)]; !strings.Contains(got, `Basic realm="Zone"`) {
			t.Errorf("WWW-Authenticate = %q, want it to carry Basic realm=\"Zone\"", got)
		}
	})

	t.Run("unknown_auth_type_returns_500", func(t *testing.T) {
		bad := &Authenticator{authType: "mystery", config: map[string]interface{}{}}
		res := bad.authenticate(&Request{Headers: map[string]string{}})
		if res.Success || res.ErrorCode != 500 {
			t.Errorf("unknown auth type should be a 500 failure, got %+v", res)
		}
	})
}

// --- AuthenticatorWrapper ----------------------------------------------------

func TestAuthenticatorWrapper(t *testing.T) {
	m := NewModule()
	keys := starlark.NewList([]starlark.Value{starlark.String("k")})
	v := callBuiltin(t, "api_key_auth", m.apiKeyAuth, starlark.Tuple{keys}, nil)
	aw := v.(*AuthenticatorWrapper)

	if aw.Type() != "web.Authenticator" {
		t.Errorf("Type() = %q, want web.Authenticator", aw.Type())
	}
	if !strings.Contains(aw.String(), "api_key") {
		t.Errorf("String() = %q, want it to mention the auth type", aw.String())
	}
	if names := aw.AttrNames(); len(names) != 1 || names[0] != "middleware" {
		t.Errorf("AttrNames() = %v, want [middleware]", names)
	}

	// middleware() yields a MiddlewareWrapper.
	mAttr, err := aw.Attr("middleware")
	if err != nil {
		t.Fatalf("Attr(middleware): %v", err)
	}
	b := mAttr.(*starlark.Builtin)
	res, err := b.CallInternal(&starlark.Thread{}, nil, nil)
	if err != nil {
		t.Fatalf("middleware(): %v", err)
	}
	if _, ok := res.(*MiddlewareWrapper); !ok {
		t.Errorf("middleware() returned %T, want *MiddlewareWrapper", res)
	}

	// Unknown attribute is a clean NoSuchAttrError, not a panic.
	if _, err := aw.Attr("nope"); err == nil {
		t.Errorf("Attr(nope): expected error, got nil")
	}
}
