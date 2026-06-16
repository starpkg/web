package web

// Static file serving for the web module.
//
// `static_dir(root, …)` builds a read-only StaticDir handle; `srv.static(prefix,
// dir)` mounts it. Mounts are served as a NoRoute FALLBACK — explicit routes
// always win, and an unmatched GET/HEAD under a mounted prefix is served from
// disk. A miss (no file / directory without an index / unsafe path) falls
// through to the server's normal 404, so the filesystem is never enumerated.
//
// The serving itself goes through net/http's ServeContent, which gives Range
// requests, conditional GET (If-Modified-Since / If-None-Match -> 304), correct
// Content-Type by extension, and sendfile zero-copy for an *os.File — so the
// default behaviour is correct without the script author handling any of it.

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.starlark.net/starlark"
)

// defaultIndexFiles are the directory default pages tried, in order.
var defaultIndexFiles = []string{"index.html", "index.htm"}

// StaticDir is a read-only static-file root mounted on a server with
// srv.static(prefix, dir). It is created by static_dir(). It is a configuration
// handle, not a response — it carries no script-callable methods.
type StaticDir struct {
	root         string   // root as given (for display)
	rootAbs      string   // absolute, cleaned root used as the lexical traversal anchor
	realRoot     string   // symlink-resolved root; a served file's real path must stay under it
	index        []string // directory default pages, in order
	spa          bool     // serve the first index page for unmatched paths
	cacheControl string   // optional Cache-Control header for served files
}

var _ starlark.Value = (*StaticDir)(nil)

func (sd *StaticDir) String() string        { return fmt.Sprintf("<web.StaticDir root=%s>", sd.root) }
func (sd *StaticDir) Type() string          { return "web.StaticDir" }
func (sd *StaticDir) Freeze()               {}
func (sd *StaticDir) Truth() starlark.Bool  { return starlark.True }
func (sd *StaticDir) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", sd.Type()) }

// staticDir is the `static_dir(root, index?, spa?, cache_control?)` builtin.
func (m *Module) staticDir(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		root         string
		index        starlark.Value = starlark.None
		spa                         = false
		cacheControl                = ""
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"root", &root,
		"index?", &index,
		"spa?", &spa,
		"cache_control?", &cacheControl,
	); err != nil {
		return none, err
	}
	if strings.TrimSpace(root) == "" {
		return none, fmt.Errorf("%s: root must not be empty", b.Name())
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return none, fmt.Errorf("%s: invalid root %q: %w", b.Name(), root, err)
	}
	idx, err := parseIndexList(b.Name(), index)
	if err != nil {
		return none, err
	}
	// Resolve the root's symlinks once so the per-request escape check compares
	// real paths (this also normalizes platform symlinks like macOS /var ->
	// /private/var). Falls back to the lexical abs path if root is absent now.
	realRoot := rootAbs
	if rr, err := filepath.EvalSymlinks(rootAbs); err == nil {
		realRoot = rr
	}
	return &StaticDir{root: root, rootAbs: rootAbs, realRoot: realRoot, index: idx, spa: spa, cacheControl: cacheControl}, nil
}

// parseIndexList accepts None (default), a single string, or a list of strings.
func parseIndexList(fnName string, v starlark.Value) ([]string, error) {
	switch t := v.(type) {
	case starlark.NoneType:
		return defaultIndexFiles, nil
	case starlark.String:
		s := strings.TrimSpace(string(t))
		if s == "" {
			return nil, fmt.Errorf("%s: index entry must not be empty", fnName)
		}
		return []string{s}, nil
	case *starlark.List:
		out := make([]string, 0, t.Len())
		for i := 0; i < t.Len(); i++ {
			s, ok := starlark.AsString(t.Index(i))
			if !ok {
				return nil, fmt.Errorf("%s: index entry %d must be a string, got %s", fnName, i, t.Index(i).Type())
			}
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("%s: index entry %d must not be empty", fnName, i)
			}
			out = append(out, s)
		}
		return out, nil // empty list => no index pages (directories then 404)
	default:
		return nil, fmt.Errorf("%s: index must be a string or list of strings, got %s", fnName, v.Type())
	}
}

// staticMount binds a URL prefix to a StaticDir.
type staticMount struct {
	prefix string // normalized: "/" or "/p" (no trailing slash)
	sd     *StaticDir
}

// matchPrefix reports whether urlPath falls under this mount and, if so, the
// path relative to the mount prefix (always starting with "/").
func (mt *staticMount) matchPrefix(urlPath string) (string, bool) {
	if mt.prefix == "/" {
		return urlPath, true
	}
	if urlPath == mt.prefix {
		return "/", true
	}
	if strings.HasPrefix(urlPath, mt.prefix+"/") {
		return urlPath[len(mt.prefix):], true
	}
	return "", false
}

// RegisterStatic mounts a StaticDir at prefix. Most-specific (longest) prefix
// wins when several mounts overlap.
func (s *Server) RegisterStatic(prefix string, sd *StaticDir) error {
	if sd == nil {
		return fmt.Errorf("static: dir must not be nil")
	}
	mt := &staticMount{prefix: normalizeStaticPrefix(prefix), sd: sd}
	s.mu.Lock()
	// Copy-on-write: build a fresh slice rather than appending/sorting in place,
	// so the lock-free iteration in tryServeStatic (which copies only the slice
	// header under the read lock) always walks an immutable backing array.
	next := make([]*staticMount, 0, len(s.staticMounts)+1)
	next = append(next, s.staticMounts...)
	next = append(next, mt)
	sort.SliceStable(next, func(i, j int) bool {
		return len(next[i].prefix) > len(next[j].prefix)
	})
	s.staticMounts = next
	s.mu.Unlock()
	return nil
}

// normalizeStaticPrefix yields "/" for empty/root, otherwise a leading-slashed
// prefix with no trailing slash.
func normalizeStaticPrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

// tryServeStatic attempts to serve the request from a mounted StaticDir. It
// returns true only when it has written a response. GET/HEAD only; everything
// else (and any miss) returns false so the caller's 404 path runs. A matched
// prefix is authoritative: once a mount claims the path, other mounts are not
// tried (a miss there is a 404, not a fall-through to a broader mount).
func (s *Server) tryServeStatic(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	s.mu.RLock()
	mounts := s.staticMounts
	s.mu.RUnlock()
	if len(mounts) == 0 {
		return false
	}
	urlPath := c.Request.URL.Path
	for _, mt := range mounts {
		rel, ok := mt.matchPrefix(urlPath)
		if !ok {
			continue
		}
		if mt.sd.serve(c, rel) {
			return true
		}
		if mt.sd.spa {
			return mt.sd.serveSPAFallback(c)
		}
		return false
	}
	return false
}

// serve resolves rel under the root and serves the file (or a directory's index
// page). Returns false — without writing — for an unsafe path, a missing file,
// or a directory without an index page (no listing is ever produced).
func (sd *StaticDir) serve(c *gin.Context, rel string) bool {
	fsPath, ok := sd.resolve(rel)
	if !ok {
		return false
	}
	f, fi, ok := sd.openWithin(fsPath)
	if !ok {
		return false
	}
	if fi.IsDir() {
		f.Close()
		for _, idx := range sd.index {
			if sd.serveExactFile(c, filepath.Join(fsPath, idx)) {
				return true
			}
		}
		return false // directory without an index page: never list, fall through to 404
	}
	defer f.Close()
	sd.writeFile(c, f, fi.Name(), fi)
	return true
}

// serveExactFile serves fsPath only if it is an existing regular file within root.
func (sd *StaticDir) serveExactFile(c *gin.Context, fsPath string) bool {
	f, fi, ok := sd.openWithin(fsPath)
	if !ok {
		return false
	}
	defer f.Close()
	if fi.IsDir() {
		return false
	}
	sd.writeFile(c, f, fi.Name(), fi)
	return true
}

// openWithin opens fsPath only if its real (symlink-resolved) path stays under
// the real root — so a symlink inside the served tree cannot escape it. Returns
// ok=false (without opening) for a missing path, a broken/looping symlink, or
// an escape. The caller closes the returned file.
func (sd *StaticDir) openWithin(fsPath string) (*os.File, os.FileInfo, bool) {
	real, err := filepath.EvalSymlinks(fsPath)
	if err != nil {
		return nil, nil, false
	}
	if !withinRoot(sd.realRoot, real) {
		return nil, nil, false
	}
	f, err := os.Open(fsPath)
	if err != nil {
		return nil, nil, false
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, false
	}
	return f, fi, true
}

// serveSPAFallback serves the first index page for an unmatched path (used only
// when spa is enabled), so a client-routed single-page app still loads.
func (sd *StaticDir) serveSPAFallback(c *gin.Context) bool {
	if len(sd.index) == 0 {
		return false
	}
	return sd.serveExactFile(c, filepath.Join(sd.rootAbs, sd.index[0]))
}

// writeFile sets the optional cache header and streams the file via
// http.ServeContent (Range, conditional GET, content-type, sendfile).
func (sd *StaticDir) writeFile(c *gin.Context, f *os.File, name string, fi os.FileInfo) {
	h := c.Writer.Header()
	if sd.cacheControl != "" {
		h.Set("Cache-Control", sd.cacheControl)
	}
	// A weak validator from size+modtime lets http.ServeContent answer
	// If-None-Match with 304 (stronger revalidation than mod-time alone). Weak
	// (W/) because it is not a byte-for-byte content hash.
	if h.Get("ETag") == "" {
		h.Set("ETag", fmt.Sprintf(`W/"%x-%x"`, fi.Size(), fi.ModTime().UnixNano()))
	}
	http.ServeContent(c.Writer, c.Request, name, fi.ModTime(), f)
}

// resolve maps a URL-relative path to an absolute on-disk path anchored under
// the root, or returns ok=false for anything unsafe: a traversal escape, a
// dotfile/dotdir segment, or a known junk segment. The URL path is cleaned with
// path.Clean("/"+rel) (which collapses "." and resolves ".." against the root),
// then re-checked against the absolute root as defense in depth.
func (sd *StaticDir) resolve(rel string) (string, bool) {
	if rel == "" {
		rel = "/"
	}
	clean := path.Clean("/" + strings.TrimPrefix(rel, "/"))
	for _, seg := range strings.Split(clean, "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, ".") && seg != ".well-known" { // dotfiles / dotdirs
			// .well-known is the one allowed dot-segment: it is the standard
			// public location (RFC 8615) for ACME HTTP-01 challenges,
			// security.txt, etc. A dotfile *inside* it (e.g. .well-known/.x) is
			// still rejected by this same loop on the next segment.
			return "", false
		}
		if seg == "@eaDir" { // Synology metadata junk
			return "", false
		}
	}
	fsPath := filepath.Join(sd.rootAbs, filepath.FromSlash(clean))
	if !withinRoot(sd.rootAbs, fsPath) {
		return "", false
	}
	return fsPath, true
}

// withinRoot reports whether p resolves to rootAbs or a path beneath it.
func withinRoot(rootAbs, p string) bool {
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	return pAbs == rootAbs || strings.HasPrefix(pAbs, rootAbs+string(os.PathSeparator))
}
