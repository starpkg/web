package web

// Tests for server lifecycle behavior (server.go).
//
// Sections:
//   - bind guardrail (PKG-08): default-deny binding to non-loopback addresses
//   - use() argument validation (PKG-08, B2): no host panic on srv.use() with no args

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// --- bind guardrail (PKG-08) -------------------------------------------------

func TestIsLoopbackBindHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.5", true},
		{"::1", true},
		{"", false},        // empty == all interfaces
		{"0.0.0.0", false}, // IPv4 wildcard
		{"::", false},      // IPv6 wildcard
		{"192.0.2.10", false},
		{"10.0.0.1", false}, // private but still off-host reachable
		{"example.com", false},
	}
	for _, c := range cases {
		if got := isLoopbackBindHost(c.host); got != c.want {
			t.Errorf("isLoopbackBindHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// By default (allow_public_bind=false) the server must refuse to listen on a
// non-loopback address, via both Start() and Run(), before any socket is opened.
func TestServerBindGuardRefusesNonLoopback(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "", "::", "192.0.2.10"} {
		server := newServer(NewModule(), host, 0)

		err := server.Start()
		if err == nil {
			_ = server.Stop()
			t.Fatalf("Start() with host %q: expected guard error, got nil", host)
		}
		if !strings.Contains(err.Error(), "allow_public_bind") {
			t.Errorf("Start() with host %q: error %q should mention allow_public_bind", host, err)
		}
		if server.IsRunning() {
			t.Errorf("Start() with host %q: server must not be running after a refused bind", host)
		}

		// Run() blocks on success, but the guard returns synchronously first.
		if err := server.Run(); err == nil {
			t.Fatalf("Run() with host %q: expected guard error, got nil", host)
		}
	}
}

// Loopback hosts are allowed by default and the server actually starts.
func TestServerBindGuardAllowsLoopback(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1"} {
		server := newServer(NewModule(), host, 0)
		if err := server.Start(); err != nil {
			t.Fatalf("Start() with loopback host %q: unexpected error: %v", host, err)
		}
		if !server.IsRunning() {
			t.Errorf("Start() with loopback host %q: expected running server", host)
		}
		if err := server.Stop(); err != nil {
			t.Errorf("Stop() with loopback host %q: unexpected error: %v", host, err)
		}
	}
}

// With allow_public_bind=true the guard permits a non-loopback host.
func TestServerBindGuardOptIn(t *testing.T) {
	module := newModuleWithOptions(
		genConfigOption(configKeyHost, "host", "localhost"),
		genConfigOption(configKeyPort, "port", 0),
		genConfigOption(configKeyReadTimeout, "read", 30),
		genConfigOption(configKeyWriteTimeout, "write", 30),
		genConfigOption(configKeyMaxBodySize, "body", int64(32<<20)),
		genConfigOption(configKeyDebugMode, "debug", false),
		genConfigOption(configKeyServerHeader, "header", "Test/1.0"),
		genConfigOption(configKeyAllowPublicBind, "allow", true),
	)
	server := newServer(module, "0.0.0.0", 0)
	if err := server.checkBindAllowed(); err != nil {
		t.Errorf("checkBindAllowed() with allow_public_bind=true: unexpected error: %v", err)
	}
}

// --- use() argument validation (B2) ------------------------------------------

// srv.use() with no arguments previously indexed args[0] unconditionally,
// panicking the host. It must now return a clean error and never panic.
func TestServerUseRequiresMiddlewareArg(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	b := starlark.NewBuiltin("use", sw.use)

	cases := []struct {
		name   string
		args   starlark.Tuple
		kwargs []starlark.Tuple
	}{
		{"no args", nil, nil},
		{"too many args", starlark.Tuple{starlark.String("a"), starlark.String("b")}, nil},
		{"keyword arg", nil, []starlark.Tuple{{starlark.String("middleware"), starlark.None}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Recover guards against a regression to the panicking form.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("use(%s) panicked: %v", c.name, r)
				}
			}()
			ret, err := sw.use(&starlark.Thread{}, b, c.args, c.kwargs)
			if err == nil {
				t.Fatalf("use(%s): expected error, got nil (ret=%v)", c.name, ret)
			}
		})
	}
}

// A single valid middleware argument is accepted (delegates to use_for "/*").
func TestServerUseAcceptsSingleMiddleware(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	b := starlark.NewBuiltin("use", sw.use)

	mw := NewMiddlewareWrapper(loggingMiddleware(""))
	if _, err := sw.use(&starlark.Thread{}, b, starlark.Tuple{mw}, nil); err != nil {
		t.Fatalf("use(middleware): unexpected error: %v", err)
	}
	if len(sw.server.middleware) != 1 {
		t.Errorf("expected 1 registered middleware, got %d", len(sw.server.middleware))
	}
}
