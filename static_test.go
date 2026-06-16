package web

// Tests for static-file serving (static_dir + srv.static), grouped by goal:
//   - basic serving (index, by-path, content-type, HEAD)
//   - path-traversal / dotfile safety (resolve + end-to-end)
//   - directory handling (index page vs no-listing 404)
//   - conditional GET (304) and Range (206) via http.ServeContent
//   - SPA fallback (opt-in) and prefix-mount + explicit-route precedence
//   - static_dir() / static() argument validation

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.starlark.net/starlark"
)

// setupSite builds a temp site tree and returns its root.
func setupSite(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<h1>home</h1>")
	write("about.html", "about page")
	write("assets/app.js", "console.log('hi')")
	write("assets/data.txt", "0123456789")
	write("sub/index.html", "sub index")
	write(".secret", "topsecret")
	write("sub/.hidden", "hidden")
	write(".well-known/security.txt", "Contact: mailto:sec@example.com")
	write(".well-known/.hidden", "nope")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func newStaticServer(t *testing.T) *Server {
	t.Helper()
	return newServer(NewModule(), "localhost", 8080)
}

func staticDirFor(root string, spa bool) *StaticDir {
	abs, _ := filepath.Abs(filepath.Clean(root))
	real := abs
	if rr, err := filepath.EvalSymlinks(abs); err == nil {
		real = rr
	}
	return &StaticDir{root: root, rootAbs: abs, realRoot: real, index: defaultIndexFiles, spa: spa}
}

func do(s *Server, method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.engine.ServeHTTP(rec, req)
	return rec
}

func TestStaticServeBasic(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	if err := s.RegisterStatic("/", staticDirFor(root, false)); err != nil {
		t.Fatal(err)
	}

	// "/" -> index.html
	if r := do(s, "GET", "/", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "home") {
		t.Errorf("GET / = %d %q, want 200 home", r.Code, r.Body.String())
	}
	// by path
	if r := do(s, "GET", "/about.html", nil); r.Code != 200 || r.Body.String() != "about page" {
		t.Errorf("GET /about.html = %d %q", r.Code, r.Body.String())
	}
	// content-type by extension
	if r := do(s, "GET", "/assets/app.js", nil); r.Code != 200 || !strings.Contains(r.Header().Get("Content-Type"), "javascript") {
		t.Errorf("GET /assets/app.js ctype = %q (code %d)", r.Header().Get("Content-Type"), r.Code)
	}
	// HEAD: status + headers, empty body
	if r := do(s, "HEAD", "/about.html", nil); r.Code != 200 || r.Body.Len() != 0 {
		t.Errorf("HEAD /about.html = %d bodyLen=%d, want 200 / 0", r.Code, r.Body.Len())
	}
	// missing file -> 404
	if r := do(s, "GET", "/nope.html", nil); r.Code != 404 {
		t.Errorf("GET /nope.html = %d, want 404", r.Code)
	}
	// non-GET/HEAD falls through (no static for POST)
	if r := do(s, "POST", "/about.html", nil); r.Code == 200 {
		t.Errorf("POST /about.html served statically (code %d); should fall through", r.Code)
	}
}

func TestStaticPathSafety(t *testing.T) {
	root := setupSite(t)
	sd := staticDirFor(root, false)

	// resolve() must never escape the root, and must reject dotfiles.
	t.Run("resolveNeverEscapes", func(t *testing.T) {
		for _, rel := range []string{
			"/../../../etc/passwd", "/..", "/../", "/a/../../b",
			"/./about.html", "/sub/../about.html", "//etc//passwd",
			"/foo/%2e%2e/bar", // already-decoded by net/http normally; literal here stays a normal segment
		} {
			if p, ok := sd.resolve(rel); ok && !withinRoot(sd.rootAbs, p) {
				t.Errorf("resolve(%q) = %q escaped root %q", rel, p, sd.rootAbs)
			}
		}
	})
	t.Run("dotfilesRejected", func(t *testing.T) {
		for _, rel := range []string{"/.secret", "/sub/.hidden", "/.git/config", "/.env"} {
			if _, ok := sd.resolve(rel); ok {
				t.Errorf("resolve(%q) accepted a dotfile path", rel)
			}
		}
	})
	t.Run("junkSegmentRejected", func(t *testing.T) {
		if _, ok := sd.resolve("/@eaDir/thumb"); ok {
			t.Error("resolve accepted @eaDir junk segment")
		}
	})

	// End-to-end: traversal and dotfiles never leak content.
	s := newStaticServer(t)
	_ = s.RegisterStatic("/", sd)
	for _, target := range []string{"/.secret", "/sub/.hidden", "/../../etc/passwd"} {
		r := do(s, "GET", target, nil)
		if r.Code == 200 {
			t.Errorf("GET %s leaked (code 200, body %q)", target, r.Body.String())
		}
		if strings.Contains(r.Body.String(), "topsecret") || strings.Contains(r.Body.String(), "hidden") {
			t.Errorf("GET %s leaked secret content: %q", target, r.Body.String())
		}
	}

	// A symlink INSIDE the served tree pointing OUTSIDE must not be followed.
	t.Run("symlinkEscapeBlocked", func(t *testing.T) {
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "loot.txt"), []byte("EXFIL"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "escape")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks unsupported here: %v", err)
		}
		r := do(s, "GET", "/escape/loot.txt", nil)
		if r.Code == 200 || strings.Contains(r.Body.String(), "EXFIL") {
			t.Errorf("symlink escape served outside content: code=%d body=%q", r.Code, r.Body.String())
		}
	})
}

func TestStaticDirectoryHandling(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	_ = s.RegisterStatic("/", staticDirFor(root, false))

	// directory WITH an index page -> serves it
	if r := do(s, "GET", "/sub/", nil); r.Code != 200 || r.Body.String() != "sub index" {
		t.Errorf("GET /sub/ = %d %q, want 200 'sub index'", r.Code, r.Body.String())
	}
	if r := do(s, "GET", "/sub", nil); r.Code != 200 || r.Body.String() != "sub index" {
		t.Errorf("GET /sub = %d %q, want index", r.Code, r.Body.String())
	}
	// directory WITHOUT an index page -> 404, never a listing
	r := do(s, "GET", "/empty/", nil)
	if r.Code != 404 {
		t.Errorf("GET /empty/ = %d, want 404 (no listing)", r.Code)
	}
	if strings.Contains(strings.ToLower(r.Body.String()), "index of") || strings.Contains(r.Body.String(), "app.js") {
		t.Errorf("GET /empty/ produced a directory listing: %q", r.Body.String())
	}
}

func TestStaticConditionalAndRange(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	_ = s.RegisterStatic("/", staticDirFor(root, false))

	// Conditional GET: If-Modified-Since in the future -> 304.
	future := time.Now().UTC().Add(time.Hour).Format(http.TimeFormat)
	if r := do(s, "GET", "/assets/data.txt", map[string]string{"If-Modified-Since": future}); r.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d, want 304", r.Code)
	}
	// Range request -> 206 + partial body.
	r := do(s, "GET", "/assets/data.txt", map[string]string{"Range": "bytes=0-3"})
	if r.Code != http.StatusPartialContent {
		t.Errorf("range GET = %d, want 206", r.Code)
	}
	if r.Body.String() != "0123" {
		t.Errorf("range body = %q, want %q", r.Body.String(), "0123")
	}
	if r.Header().Get("Content-Range") == "" {
		t.Error("range response missing Content-Range header")
	}
}

func TestStaticWellKnownAndETag(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	_ = s.RegisterStatic("/", staticDirFor(root, false))

	// .well-known is the one allowed dot-path (RFC 8615: ACME, security.txt, …).
	if r := do(s, "GET", "/.well-known/security.txt", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "Contact") {
		t.Errorf("GET /.well-known/security.txt = %d %q, want 200", r.Code, r.Body.String())
	}
	// A dotfile *inside* .well-known is still blocked.
	if r := do(s, "GET", "/.well-known/.hidden", nil); r.Code == 200 {
		t.Errorf("GET /.well-known/.hidden served (code %d); dotfile must stay blocked", r.Code)
	}
	// Other dot-dirs remain blocked.
	if r := do(s, "GET", "/.git/config", nil); r.Code == 200 {
		t.Error("GET /.git/config served; must be blocked")
	}

	// Weak ETag is emitted and If-None-Match revalidates to 304.
	r := do(s, "GET", "/about.html", nil)
	etag := r.Header().Get("ETag")
	if !strings.HasPrefix(etag, "W/") {
		t.Fatalf("missing weak ETag, got %q", etag)
	}
	if r2 := do(s, "GET", "/about.html", map[string]string{"If-None-Match": etag}); r2.Code != http.StatusNotModified {
		t.Errorf("If-None-Match revalidation = %d, want 304", r2.Code)
	}
}

// TestStaticConcurrentSafe guards the copy-on-write mount registry: serving
// while RegisterStatic runs concurrently must be race-free (go test -race).
func TestStaticConcurrentSafe(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	_ = s.RegisterStatic("/", staticDirFor(root, false))
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); do(s, "GET", "/about.html", nil) }()
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) { defer wg.Done(); _ = s.RegisterStatic(fmt.Sprintf("/m%d", n), staticDirFor(root, false)) }(i)
	}
	wg.Wait()
}

func TestStaticSPAFallback(t *testing.T) {
	root := setupSite(t)

	// spa=False: unmatched -> 404.
	s1 := newStaticServer(t)
	_ = s1.RegisterStatic("/", staticDirFor(root, false))
	if r := do(s1, "GET", "/client/route", nil); r.Code != 404 {
		t.Errorf("spa=false GET /client/route = %d, want 404", r.Code)
	}

	// spa=True: unmatched non-file path -> root index (200).
	s2 := newStaticServer(t)
	_ = s2.RegisterStatic("/", staticDirFor(root, true))
	if r := do(s2, "GET", "/client/route", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "home") {
		t.Errorf("spa=true GET /client/route = %d %q, want 200 home", r.Code, r.Body.String())
	}
	// even with SPA, a dotfile is never served
	if r := do(s2, "GET", "/.secret", nil); strings.Contains(r.Body.String(), "topsecret") {
		t.Errorf("spa=true leaked dotfile: %q", r.Body.String())
	}
}

func TestStaticPrefixMountAndOverride(t *testing.T) {
	root := setupSite(t)
	s := newStaticServer(t)
	_ = s.RegisterStatic("/assets", staticDirFor(root, false))

	// Under the prefix: /assets/app.js -> root/app.js? No — mount strips prefix,
	// so /assets/assets/app.js maps to root/assets/app.js. Use a top-level file:
	if r := do(s, "GET", "/assets/about.html", nil); r.Code != 200 || r.Body.String() != "about page" {
		t.Errorf("GET /assets/about.html = %d %q, want about", r.Code, r.Body.String())
	}
	// Outside the prefix -> 404 (mount does not claim it).
	if r := do(s, "GET", "/about.html", nil); r.Code != 404 {
		t.Errorf("GET /about.html (outside /assets) = %d, want 404", r.Code)
	}
	// Prefix boundary is respected: /assetsX is not under /assets.
	if r := do(s, "GET", "/assetsX/about.html", nil); r.Code != 404 {
		t.Errorf("GET /assetsX/... = %d, want 404 (boundary)", r.Code)
	}

	// Explicit route always wins over the static fallback.
	s2 := newStaticServer(t)
	_ = s2.RegisterStatic("/", staticDirFor(root, false))
	if err := s2.Get("/about.html", dummyHandler(t)); err != nil {
		t.Fatal(err)
	}
	if r := do(s2, "GET", "/about.html", nil); r.Body.String() == "about page" {
		t.Errorf("explicit route did not win: got static file body")
	}
}

// dummyHandler returns a Starlark handler that responds with a sentinel.
func dummyHandler(t *testing.T) starlark.Callable {
	t.Helper()
	src := `
def h(req):
    return text_response("ROUTE")
`
	thread := &starlark.Thread{Name: "t"}
	predeclared := starlark.StringDict{
		"text_response": starlark.NewBuiltin("text_response", NewModule().textResponse),
	}
	globals, err := starlark.ExecFile(thread, "h.star", src, predeclared)
	if err != nil {
		t.Fatal(err)
	}
	return globals["h"].(starlark.Callable)
}

func TestStaticDirBuiltinValidation(t *testing.T) {
	m := NewModule()
	bi := starlark.NewBuiltin("web.static_dir", m.staticDir)
	th := &starlark.Thread{Name: "t"}
	call := func(args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
		return starlark.Call(th, bi, args, kw)
	}

	// empty root -> error
	if _, err := call(starlark.Tuple{starlark.String("  ")}, nil); err == nil {
		t.Error("empty root should error")
	}
	// valid root -> StaticDir with default index
	v, err := call(starlark.Tuple{starlark.String(".")}, nil)
	if err != nil {
		t.Fatalf("valid root errored: %v", err)
	}
	sd, ok := v.(*StaticDir)
	if !ok || len(sd.index) == 0 {
		t.Fatalf("static_dir did not return a StaticDir with default index: %v", v)
	}
	// index as a list
	v2, err := call(starlark.Tuple{starlark.String(".")},
		[]starlark.Tuple{{starlark.String("index"), starlark.NewList([]starlark.Value{starlark.String("home.html")})}})
	if err != nil {
		t.Fatalf("index list errored: %v", err)
	}
	if got := v2.(*StaticDir).index; len(got) != 1 || got[0] != "home.html" {
		t.Errorf("index list = %v, want [home.html]", got)
	}
	// bad index entry type -> error
	if _, err := call(starlark.Tuple{starlark.String(".")},
		[]starlark.Tuple{{starlark.String("index"), starlark.NewList([]starlark.Value{starlark.MakeInt(1)})}}); err == nil {
		t.Error("non-string index entry should error")
	}

	// srv.static with a non-StaticDir -> error
	s := newServer(m, "localhost", 8080)
	sw := NewServerWrapper(s)
	stat, _ := sw.Attr("static")
	if _, err := starlark.Call(th, stat.(*starlark.Builtin),
		starlark.Tuple{starlark.String("/"), starlark.String("not-a-dir")}, nil); err == nil {
		t.Error("static() with a non-StaticDir should error")
	}
}
