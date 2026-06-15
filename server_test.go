package web

// Tests for server lifecycle behavior (server.go).
//
// Sections:
//   - bind guardrail (PKG-08): default-deny binding to non-loopback addresses
//   - use() argument validation (PKG-08, B2): no host panic on srv.use() with no args
//   - Start() bind errors: a failed listen must surface, not be swallowed
//   - create_server argument validation: port range / invalid port / header override
//   - server builtin arg validation: route / error_handler / use_for / group errors
//   - lifecycle errors: stop()/double-start guards
//   - wrapHandler + applyResponse: request->handler->response bridge via httptest

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

// --- Start() bind errors -----------------------------------------------------

// freeLoopbackPort grabs an OS-assigned loopback port and releases it so the
// caller can re-bind it on a known fixed number.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// A failed bind (the port is already taken) must surface from Start() instead
// of being swallowed, and IsRunning() must report false rather than lie.
func TestServerStartReportsBindError(t *testing.T) {
	port := freeLoopbackPort(t)

	first := newServer(NewModule(), "127.0.0.1", port)
	if err := first.Start(); err != nil {
		t.Fatalf("first Start() on free port %d: unexpected error: %v", port, err)
	}
	defer func() { _ = first.Stop() }()
	if !first.IsRunning() {
		t.Fatalf("first server should be running after a successful Start()")
	}

	// A second server on the SAME fixed port can't bind.
	second := newServer(NewModule(), "127.0.0.1", port)
	err := second.Start()
	if err == nil {
		_ = second.Stop()
		t.Fatalf("second Start() on busy port %d: expected a bind error, got nil", port)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) && !strings.Contains(err.Error(), "address already in use") && !strings.Contains(err.Error(), "bind") {
		t.Logf("note: bind error did not mention the port/bind: %v", err)
	}
	if second.IsRunning() {
		t.Errorf("second server must not report running after a failed bind")
	}
}

// --- create_server argument validation ---------------------------------------

func TestCreateServerValidation(t *testing.T) {
	m := NewModule()
	b := starlark.NewBuiltin("create_server", m.createServer)

	t.Run("defaults_to_module_config", func(t *testing.T) {
		v, err := m.createServer(&starlark.Thread{}, b, nil, nil)
		if err != nil {
			t.Fatalf("create_server defaults: %v", err)
		}
		sw := v.(*ServerWrapper)
		if sw.server.host != "localhost" || sw.server.port != 8080 {
			t.Errorf("defaults host/port = %s/%d, want localhost/8080", sw.server.host, sw.server.port)
		}
	})

	t.Run("port_out_of_range", func(t *testing.T) {
		// A negative or >65535 port is rejected. (port=0 is treated as "omitted"
		// and falls back to the module default, so it is not in this set.)
		for _, p := range []int64{-1, 65536, 70000} {
			_, err := m.createServer(&starlark.Thread{}, b, nil,
				[]starlark.Tuple{{starlark.String("port"), starlark.MakeInt64(p)}})
			if err == nil || !strings.Contains(err.Error(), "between 1 and 65535") {
				t.Errorf("port=%d: err = %v, want a range error", p, err)
			}
		}
	})

	t.Run("port_zero_falls_back_to_default", func(t *testing.T) {
		// port=0 means "not provided" (Sign()==0), so the module default is used.
		v, err := m.createServer(&starlark.Thread{}, b, nil,
			[]starlark.Tuple{{starlark.String("port"), starlark.MakeInt(0)}})
		if err != nil {
			t.Fatalf("create_server port=0: %v", err)
		}
		if sw := v.(*ServerWrapper); sw.server.port != 8080 {
			t.Errorf("port = %d, want the default 8080", sw.server.port)
		}
	})

	t.Run("valid_port_and_header_override", func(t *testing.T) {
		v, err := m.createServer(&starlark.Thread{}, b, nil,
			[]starlark.Tuple{
				{starlark.String("host"), starlark.String("127.0.0.1")},
				{starlark.String("port"), starlark.MakeInt(9999)},
				{starlark.String("server_header"), starlark.String("Custom/2.0")},
			})
		if err != nil {
			t.Fatalf("create_server: %v", err)
		}
		sw := v.(*ServerWrapper)
		if sw.server.port != 9999 || sw.server.host != "127.0.0.1" {
			t.Errorf("host/port = %s/%d, want 127.0.0.1/9999", sw.server.host, sw.server.port)
		}
		if sw.server.serverHeader != "Custom/2.0" {
			t.Errorf("server_header = %q, want Custom/2.0", sw.server.serverHeader)
		}
	})
}

// --- server builtin arg validation -------------------------------------------

func TestServerRouteValidation(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 0)
	handler := starlark.NewBuiltin("h", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	t.Run("string_method", func(t *testing.T) {
		if err := srv.Route(starlark.String("GET"), "/x", handler); err != nil {
			t.Errorf("string method: %v", err)
		}
	})
	t.Run("list_of_methods", func(t *testing.T) {
		methods := starlark.NewList([]starlark.Value{starlark.String("GET"), starlark.String("POST")})
		if err := srv.Route(methods, "/y", handler); err != nil {
			t.Errorf("list methods: %v", err)
		}
	})
	t.Run("list_with_non_string_errors", func(t *testing.T) {
		methods := starlark.NewList([]starlark.Value{starlark.MakeInt(1)})
		if err := srv.Route(methods, "/z", handler); err == nil {
			t.Errorf("non-string method element should error")
		}
	})
	t.Run("wrong_methods_type_errors", func(t *testing.T) {
		if err := srv.Route(starlark.MakeInt(5), "/w", handler); err == nil {
			t.Errorf("int methods should error")
		}
	})
	t.Run("unsupported_method_string_errors", func(t *testing.T) {
		if err := srv.Route(starlark.String("BREW"), "/teapot", handler); err == nil {
			t.Errorf("unsupported method should error")
		}
	})
}

func TestServerErrorHandlerValidation(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	b := starlark.NewBuiltin("error_handler", sw.errorHandler)
	handler := starlark.NewBuiltin("h", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	t.Run("single_int", func(t *testing.T) {
		if _, err := sw.errorHandler(&starlark.Thread{}, b, starlark.Tuple{starlark.MakeInt(404), handler}, nil); err != nil {
			t.Errorf("single int: %v", err)
		}
		if sw.server.errorHandlers.GetHandler(404) == nil {
			t.Errorf("404 handler not registered")
		}
	})
	t.Run("list_of_ints", func(t *testing.T) {
		codes := starlark.NewList([]starlark.Value{starlark.MakeInt(500), starlark.MakeInt(502)})
		if _, err := sw.errorHandler(&starlark.Thread{}, b, starlark.Tuple{codes, handler}, nil); err != nil {
			t.Errorf("list ints: %v", err)
		}
		if sw.server.errorHandlers.GetHandler(502) == nil {
			t.Errorf("502 handler not registered")
		}
	})
	t.Run("wrong_status_codes_type_errors", func(t *testing.T) {
		if _, err := sw.errorHandler(&starlark.Thread{}, b, starlark.Tuple{starlark.String("404"), handler}, nil); err == nil {
			t.Errorf("string status_codes should error")
		}
	})
}

func TestServerUseForValidation(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	b := starlark.NewBuiltin("use_for", sw.useFor)

	t.Run("middleware_object", func(t *testing.T) {
		mw := NewMiddlewareWrapper(loggingMiddleware(""))
		if _, err := sw.useFor(&starlark.Thread{}, b, starlark.Tuple{starlark.String("/api/*"), mw}, nil); err != nil {
			t.Errorf("middleware object: %v", err)
		}
	})
	t.Run("callable_middleware", func(t *testing.T) {
		src := `
def mw(req, next):
    return next(req)
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "m.star", src, nil)
		if _, err := sw.useFor(&starlark.Thread{}, b,
			starlark.Tuple{starlark.String("/*"), globals["mw"].(starlark.Callable)}, nil); err != nil {
			t.Errorf("callable middleware: %v", err)
		}
	})
	t.Run("non_middleware_errors", func(t *testing.T) {
		if _, err := sw.useFor(&starlark.Thread{}, b,
			starlark.Tuple{starlark.String("/*"), starlark.MakeInt(1)}, nil); err == nil {
			t.Errorf("int middleware should error")
		}
	})
}

func TestServerGroupBuiltin(t *testing.T) {
	sw := NewServerWrapper(newServer(NewModule(), "localhost", 0))
	b := starlark.NewBuiltin("group", sw.group)
	v, err := sw.group(&starlark.Thread{}, b, starlark.Tuple{starlark.String("/api")}, nil)
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	if _, ok := v.(*RouteGroupWrapper); !ok {
		t.Errorf("group() returned %T, want *RouteGroupWrapper", v)
	}
}

// --- lifecycle errors --------------------------------------------------------

func TestServerLifecycleGuards(t *testing.T) {
	t.Run("stop_when_not_running_errors", func(t *testing.T) {
		srv := newServer(NewModule(), "localhost", 0)
		if err := srv.Stop(); err == nil {
			t.Errorf("Stop() on a never-started server should error")
		}
	})

	t.Run("double_start_errors", func(t *testing.T) {
		srv := newServer(NewModule(), "127.0.0.1", 0)
		if err := srv.Start(); err != nil {
			t.Fatalf("first Start(): %v", err)
		}
		defer func() { _ = srv.Stop() }()
		if err := srv.Start(); err == nil {
			t.Errorf("second Start() should report already running")
		}
		// Run() on an already-running server must also refuse.
		if err := srv.Run(); err == nil {
			t.Errorf("Run() on a running server should error")
		}
	})
}

// --- wrapHandler + applyResponse ---------------------------------------------

// newGinContextForTest builds a gin.Context bound to an httptest recorder for a
// given method/path, with no real socket involved.
func newGinContextForTest(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, rec
}

func TestWrapHandlerBridge(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 0)

	t.Run("handler_response_is_written", func(t *testing.T) {
		src := `
def handler(req):
    return response("hello", status=201)
`
		m := NewModule()
		predeclared := starlark.StringDict{
			"response": starlark.NewBuiltin("response", m.response),
		}
		globals, err := starlark.ExecFile(&starlark.Thread{}, "h.star", src, predeclared)
		if err != nil {
			t.Fatalf("ExecFile: %v", err)
		}
		ginHandler := srv.wrapHandler(globals["handler"].(starlark.Callable))

		c, rec := newGinContextForTest(http.MethodGet, "/hello")
		ginHandler(c)
		if rec.Code != 201 {
			t.Errorf("status = %d, want 201", rec.Code)
		}
		if rec.Body.String() != "hello" {
			t.Errorf("body = %q, want hello", rec.Body.String())
		}
		// Server header injected.
		if got := rec.Result().Header.Get("Server"); got != "Starlark-Web/1.0" {
			t.Errorf("Server header = %q, want Starlark-Web/1.0", got)
		}
	})

	t.Run("handler_error_becomes_500", func(t *testing.T) {
		src := `
def handler(req):
    fail("kaboom")
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "h.star", src, nil)
		ginHandler := srv.wrapHandler(globals["handler"].(starlark.Callable))
		c, rec := newGinContextForTest(http.MethodGet, "/err")
		ginHandler(c)
		if rec.Code != 500 {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Handler error") {
			t.Errorf("body = %q, want a Handler error message", rec.Body.String())
		}
	})

	t.Run("non_response_return_becomes_500", func(t *testing.T) {
		src := `
def handler(req):
    return 42
`
		globals, _ := starlark.ExecFile(&starlark.Thread{}, "h.star", src, nil)
		ginHandler := srv.wrapHandler(globals["handler"].(starlark.Callable))
		c, rec := newGinContextForTest(http.MethodGet, "/bad")
		ginHandler(c)
		if rec.Code != 500 {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Invalid response format") {
			t.Errorf("body = %q, want 'Invalid response format'", rec.Body.String())
		}
	})
}

// applyResponse must serve a file when FilePath is set, and route a >=400
// status through a registered error handler.
func TestApplyResponseErrorHandlerRouting(t *testing.T) {
	srv := newServer(NewModule(), "localhost", 0)
	src := `
def handler(req):
    return resp
`
	predeclared := starlark.StringDict{
		"resp": NewResponseWrapper(&Response{
			StatusCode: 404,
			Headers:    map[string]string{canonicalHeader(HeaderContentType): MIMETextPlain},
			Body:       "custom not found",
		}),
	}
	globals, err := starlark.ExecFile(&starlark.Thread{}, "h.star", src, predeclared)
	if err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	srv.errorHandlers.RegisterHandler([]int{404}, globals["handler"].(starlark.Callable))

	c, rec := newGinContextForTest(http.MethodGet, "/missing")
	srv.applyResponse(c, &Response{StatusCode: 404, Headers: map[string]string{}, Body: "default"})
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if rec.Body.String() != "custom not found" {
		t.Errorf("body = %q, want the custom error handler body", rec.Body.String())
	}
}
