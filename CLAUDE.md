# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`starpkg/web` is an **L4 domain module** of the Star\* ecosystem: it lets a
Starlark script stand up an HTTP **server** with a Flask-inspired API — routes,
route groups, path-pattern middleware, request/response helpers, pluggable
authentication, and custom error handlers — built on
[gin](https://github.com/gin-gonic/gin).

In the `starpkg` framing ("support for necessary **local** operations plus
simple abstractions over common **online** services"), `web` is a **local
capability**: it opens a listening socket inside the host process so the script
can *serve* requests. It is the inverse of an HTTP *client* (outbound calls are
starlet's `http` module). Because a listening socket is a host resource, the
module ships a default-deny **loopback bind** guardrail (see Hardening).

Layer position: depends downward on `starpkg/base` (the module/config system),
`1set/starlet` (the Machine + `dataconv` for Go⇄Starlark JSON/value marshalling),
`1set/starlight` (`convert.FromValue` for handler-result fallback conversion),
and transitively `go.starlark.net`. Plus the third-party `gin` HTTP framework.
Nothing in the ecosystem depends on it.

## Dev commands

Pure Go library with a Makefile. From this repo:

```bash
make test                                  # -race -cover, the working bar
make ci                                    # -race -cover profile + bench compile (what CI runs)
go test ./... -run TestServerBindGuard     # a single test
gofmt -l . && go vet ./...                 # must be clean before commit
go run github.com/1set/meta/doccov@master .  # doc-coverage gate (exit 0 required)
```

**Verify on the go floor in Docker** — this repo's floor is **go 1.19** (see
Release discipline), and the pinned `go.starlark.net` baseline uses
`maphash.String` (needs ≥1.19), so behavior on the floor must be checked in a
container, not just on the (newer) local toolchain:

```bash
docker run --rm -v "$PWD":/src -v "$HOME/go/pkg/mod":/go/pkg/mod -w /src golang:1.19 go test -race -count=1 ./...
```

Integration scripts under `../test/web/*.star` live in the **private
`starpkg/test` repo** and auto-skip when that directory is absent (e.g. in CI);
`TestStarlarkScripts` drives them via `base.RunStarlarkTests`.

## Architecture (the part that spans files)

The module is a **bridge from gin's HTTP handling to Starlark callables**: a
gin handler converts each request into a `*Request`, runs the script-registered
handler (and any matching middleware) to produce a `*Response`, and writes it
back. Every script-visible thing is a Go struct exposed through a thin
`*Wrapper` implementing `starlark.Value`/`HasAttrs`.

- **`web.go`** — module entry. `Module` holds a `base.ConfigurableModule`;
  `NewModule()` constructs it with eight config options (host, port, timeouts,
  max body size, debug mode, server header, `allow_public_bind`). `LoadModule()`
  registers the module-level builtins: `create_server`, the response builders
  (`response`/`json_response`/`html_response`/`text_response`/`file_response`/
  `send_file`/`send_data`/`redirect`/`error_response`), the auth constructors
  (`api_key_auth`/`bearer_auth`/`basic_auth`), and the middleware constructors.
- **`server.go`** — `Server` + `ServerWrapper`. `newServer` builds the gin
  engine; `wrapHandler` is the core bridge (request → middleware chain →
  Starlark handler → response). `ServerWrapper` exposes `get/post/put/delete/
  patch/options/head`, `route`, `group`, `use`/`use_for`, `error_handler`,
  `start`/`stop`/`run`/`is_running`. Also holds the bind guardrail
  (`isLoopbackBindHost`, `checkBindAllowed`) and the `Middleware` struct
  (pattern + handler) with `MatchesPath`.
- **`request.go`** — `Request` (built by `createRequestFromGin`, body cached for
  re-reads) + `RequestWrapper` exposing properties (`method`/`path`/`headers`/
  `query`/`context`/…) and methods (`body`/`json`/`form`/`files`/`cookie`/
  `param`/`get_header`/`bearer_token`/`basic_auth`). Uploaded files become a
  dict with a `read` builtin.
- **`response.go`** — `Response` (status/headers/body/file_path + a separate
  `Cookies []string`) + `ResponseWrapper` (settable fields via `SetField`;
  methods `set_header`/`get_header`/`set_cookie`/`delete_cookie`).
- **`auth.go`** — `Authenticator` (api_key/bearer/basic) + `AuthenticatorWrapper`
  exposing `middleware`. `Middleware()` returns a `MiddlewareFunc` that rejects
  with 401 (Basic adds a `WWW-Authenticate` challenge) or stores `auth_user` in
  the request context and calls `next`.
- **`middleware.go`** — `MiddlewareFunc`/`NextFunc` types, `MiddlewareWrapper`,
  `createStarlarkMiddleware` (wraps a script `func(req, next)`; `next` is a
  builtin), and every built-in middleware (cors/logging/security_headers/
  timing/json/compression/rate_limit/cache/request_size) plus the in-memory
  `MemoryRateLimitStorage`.
- **`router.go`** — `HTTPMethod` constants, `RouteRegistrar`, `RouteGroup` +
  `RouteGroupWrapper`, and `convertPathParams` (Flask `{id}` → gin `:id`).
- **`path_matcher.go`** — `PathMatcher` + package-level `MatchesPattern`/
  `MatchesAny`/`ExtractParams`/`NormalizePath`/`IsValidPattern`. Glob (`/*`,
  mid-path `*`), `{param}`, and exact matching for middleware/cache patterns.
- **`errors.go`** — `ErrorResponse`, `ErrorHandlerRegistry` (status code →
  Starlark handler), invoked from `applyResponse`/`NoRoute`/`NoMethod`.
- **`utils.go`** — MIME/Header constants, Starlark⇄Go converters
  (`starlarkListToStringSlice`, `starlarkDictToStringMap`, …), `canonicalHeader`,
  and the canned JSON error-response builders.

## Invariants / hardening (preserve when editing)

The iron rule is **opt-in / default-off so old scripts run identically**.

1. **Default-deny public bind.** `create_server` defaults `host` to `localhost`;
   `Start()`/`Run()` call `checkBindAllowed`, which refuses any non-loopback
   host (`0.0.0.0`, `::`, empty, public/LAN IP) unless `allow_public_bind=true`
   (config / `WEB_ALLOW_PUBLIC_BIND`). This keeps a server started from an
   untrusted script off the network by default. `isLoopbackBindHost` is
   `net.ParseIP`-based, not substring-based — keep it parse-based. The full
   network capability gate (deny-by-policy) lives in the sandbox layer; this is
   the module-local guardrail.
2. **`Start()` surfaces bind failures.** `Start()` runs `ListenAndServe` in a
   goroutine, then waits briefly and re-checks `startErr`, so a failed bind
   (e.g. port in use) is returned to the caller instead of leaving
   `is_running()` lying. Don't swallow the goroutine's error.
3. **No host panics from script input.** A panicking script handler is caught by
   gin's `Recovery`; handler/middleware errors become a 500 response, never a
   host crash.
4. **Set-Cookie is not comma-combinable.** Cookies live in `Response.Cookies`
   (separate from `Headers`); `applyResponse` emits one `Add("Set-Cookie", v)`
   per entry. Don't fold cookies into the headers map.
5. **Body is cached for re-reads.** `createRequestFromGin` reads the body once
   into `bodyData` so `body()`/`json()`/`form()`/middleware can all read it;
   `form()` re-wraps the cached bytes. Don't re-read `c.Request.Body` directly.
6. **Backward compatibility.** `NewModule()` keeps every historical default
   (localhost bind allowed, public bind denied, 32 MiB body cap). Any new safety
   lever must default to the historical behavior.

## Test organization

Group by functional goal — **do not add one `*_test.go` per fix.** The thematic
files are: `config_test.go` (default/env/programmatic configuration + timeouts),
`server_test.go` (bind guard, `Start()` error surfacing, `use()` arity),
`response_test.go` (response builders, middleware constructors, and Set-Cookie
line accumulation), `path_matcher_test.go` (the matcher), and `example_test.go`
(the `../test/web` integration harness `TestStarlarkScripts`, the runnable
`Example_basicWebServer`, and the module smoke test). Add a new test as a
**section in the matching file**, not a new file. Tests are table/example-driven;
no third-party test framework.

## Documentation

Three layers must stay in sync (enforced by the doc standard,
`plan/starpkg文档标准（DOC-STD）`):

- **`README.md`** — every script-facing builtin and object method documented as
  a backtick whole-word (the doc-coverage gate, `doccov`, fails CI otherwise),
  with correct names/signatures/behaviour verified against the code. The
  "Script-facing API at a glance" list anchors every symbol; the per-section
  detail carries the semantics; the *Hardening* section documents the
  `allow_public_bind` host lever.
- **GoDoc** — package comment in `web.go` + a name-first doc comment on every
  exported symbol (gated by `revive`'s `exported` rule in CI).
- **`CLAUDE.md`** (this file) — the architecture/invariant map for future work.

## Release discipline

- **Floor = go 1.19** (`go.mod`), matching the pinned `go.starlark.net` baseline
  (`ffb3f39…`) and the `starlet v0.2.1` / `starlight v0.2.0` / `base v0.1.0`
  deps. A repo's floor only rises in its own isolated **pin PR**, which is the
  *last* PR of the series — never tag before it merges.
- **CI matrix** = `[1.19.x, 1.25.x]` via the centralized reusable workflow in
  `1set/meta` (`.github/workflows/build.yml` pins the `go-ci.yml` SHA and sets
  `go-floor: "1.19"` + `doc-coverage: true`). A standalone OpenSSF Scorecard
  workflow runs separately.
- **Bumping the version, the go floor, or tagging are user-confirmed actions** —
  never tag autonomously; default to patch bumps; published tags are immutable.
