package web

// Tests for route registration (router.go) and the shared Starlark value-protocol
// surfaces of the wrappers (Type/String/Truth/Hash/AttrNames). These need no TTY
// or network -- registering a route on a gin engine does not open a socket.
//
// Sections:
//   - convertPathParams: Flask {id} -> gin :id rewriting
//   - RouteGroup: per-method registration + wrapper Attr/AttrNames
//   - wrapper protocol: Type/String/Truth/Hash/Freeze for each *Wrapper

import (
	"testing"

	"go.starlark.net/starlark"
)

// --- convertPathParams -------------------------------------------------------

func TestConvertPathParams(t *testing.T) {
	cases := map[string]string{
		"/users/{id}":                "/users/:id",
		"/users/{id}/posts/{postId}": "/users/:id/posts/:postId",
		"/static/file":               "/static/file",
		"/":                          "/",
		"/a/{b}/c":                   "/a/:b/c",
	}
	for in, want := range cases {
		if got := convertPathParams(in); got != want {
			t.Errorf("convertPathParams(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- RouteGroup --------------------------------------------------------------

func TestRouteGroupRegistration(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 0)
	group := srv.Group("/api")
	handler := starlark.NewBuiltin("h", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	// Each per-method registrar must succeed (no error, no panic).
	regs := []struct {
		name string
		fn   func(string, starlark.Callable) error
	}{
		{"Get", group.Get},
		{"Post", group.Post},
		{"Put", group.Put},
		{"Delete", group.Delete},
		{"Patch", group.Patch},
		{"Options", group.Options},
		{"Head", group.Head},
	}
	for _, r := range regs {
		if err := r.fn("/{id}", handler); err != nil {
			t.Errorf("RouteGroup.%s: %v", r.name, err)
		}
	}
}

func TestRouteGroupWrapperAttr(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 0)
	gw := NewRouteGroupWrapper(srv.Group("/v1"))

	t.Run("method_builtins", func(t *testing.T) {
		for _, name := range []string{"get", "post", "put", "delete", "patch", "options", "head"} {
			v, err := gw.Attr(name)
			if err != nil {
				t.Errorf("Attr(%q): %v", name, err)
				continue
			}
			if _, ok := v.(*starlark.Builtin); !ok {
				t.Errorf("Attr(%q) = %T, want *starlark.Builtin", name, v)
			}
		}
	})

	t.Run("unknown_attr_errors", func(t *testing.T) {
		if _, err := gw.Attr("nope"); err == nil {
			t.Errorf("Attr(nope): expected error")
		}
	})

	t.Run("attr_names", func(t *testing.T) {
		if len(gw.AttrNames()) != 7 {
			t.Errorf("AttrNames count = %d, want 7", len(gw.AttrNames()))
		}
	})

	t.Run("registered_method_builtin_invokes_handler", func(t *testing.T) {
		v, _ := gw.Attr("get")
		b := v.(*starlark.Builtin)
		handler := starlark.NewBuiltin("h", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, nil
		})
		if _, err := b.CallInternal(&starlark.Thread{},
			starlark.Tuple{starlark.String("/p"), handler}, nil); err != nil {
			t.Errorf("group.get(/p, handler): %v", err)
		}
	})
}

// --- wrapper protocol --------------------------------------------------------

func TestWrapperValueProtocol(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 8080)
	wrappers := []struct {
		name string
		v    starlark.Value
		typ  string
	}{
		{"server", NewServerWrapper(srv), "web.Server"},
		{"request", NewRequestWrapper(&Request{Method: "GET", Path: "/p"}), "web.Request"},
		{"response", NewResponseWrapper(&Response{StatusCode: 200}), "web.Response"},
		{"middleware", NewMiddlewareWrapper(loggingMiddleware("")), "web.Middleware"},
		{"route_group", NewRouteGroupWrapper(srv.Group("/g")), "web.RouteGroup"},
		{"authenticator", NewAuthenticatorWrapper(&Authenticator{authType: "basic"}), "web.Authenticator"},
	}

	for _, w := range wrappers {
		t.Run(w.name, func(t *testing.T) {
			if w.v.Type() != w.typ {
				t.Errorf("Type() = %q, want %q", w.v.Type(), w.typ)
			}
			if w.v.String() == "" {
				t.Errorf("String() empty for %s", w.name)
			}
			if w.v.Truth() != starlark.True {
				t.Errorf("Truth() = %v, want True", w.v.Truth())
			}
			// All wrappers are intentionally unhashable.
			if _, err := w.v.Hash(); err == nil {
				t.Errorf("Hash() should error (unhashable) for %s", w.name)
			}
			// Freeze must not panic.
			w.v.Freeze()
		})
	}
}

// MiddlewareWrapper exposes no attributes and has none to look up.
func TestMiddlewareWrapperAttrs(t *testing.T) {
	mw := NewMiddlewareWrapper(loggingMiddleware(""))
	if len(mw.AttrNames()) != 0 {
		t.Errorf("MiddlewareWrapper AttrNames = %v, want empty", mw.AttrNames())
	}
	if _, err := mw.Attr("anything"); err == nil {
		t.Errorf("Attr should error for MiddlewareWrapper")
	}
}

// ServerWrapper exposes the HTTP-method + lifecycle attribute set.
func TestServerWrapperAttrNames(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	names := sw.AttrNames()
	// 7 HTTP methods + 10 other attrs (route/start/stop/run/is_running/group/static/use/use_for/error_handler).
	if len(names) != 17 {
		t.Errorf("ServerWrapper AttrNames count = %d, want 17: %v", len(names), names)
	}
	if _, err := sw.Attr("static"); err != nil {
		t.Errorf("Attr(static): %v", err)
	}
	if _, err := sw.Attr("get"); err != nil {
		t.Errorf("Attr(get): %v", err)
	}
	if _, err := sw.Attr("is_running"); err != nil {
		t.Errorf("Attr(is_running): %v", err)
	}
	if _, err := sw.Attr("nonexistent"); err == nil {
		t.Errorf("Attr(nonexistent): expected error")
	}
}
