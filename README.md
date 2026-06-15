# 🌐 `web` — Flask-inspired web server for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/web.svg)](https://pkg.go.dev/github.com/starpkg/web)
[![license](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/starpkg/web)](https://goreportcard.com/report/github.com/starpkg/web)

Build server-side HTTP applications from Starlark with a Flask-inspired API:
routing, route groups, path-pattern middleware, request/response helpers,
pluggable authentication, and custom error handlers — on top of
[gin](https://github.com/gin-gonic/gin).

Within the Star\* ecosystem `starpkg` is *"support for necessary **local**
operations plus simple abstractions over common **online** services, for ease
of use."* `web` is a **local capability**: it stands up an HTTP listener inside
the host process so a script can *serve* requests (the inverse of an HTTP
*client* — for outbound calls use starlet's `http` module). Because a listening
socket is a host resource, the module ships a default-deny **loopback bind**
guardrail (see [Hardening](#hardening)).

## Script-facing API at a glance

Module functions: `create_server`, `response`, `json_response`,
`html_response`, `text_response`, `file_response`, `send_file`, `send_data`,
`redirect`, `error_response`, `api_key_auth`, `bearer_auth`, `basic_auth`,
`cors_middleware`, `logging_middleware`, `security_headers_middleware`,
`timing_middleware`, `json_middleware`, `compression_middleware`,
`rate_limit_middleware`, `cache_middleware`, `request_size_middleware`.

Server methods: `get`, `post`, `put`, `delete`, `patch`, `options`, `head`,
`route`, `group`, `use`, `use_for`, `error_handler`, `start`, `stop`, `run`,
`is_running`.

Request methods: `body`, `json`, `form`, `files`, `cookie`, `param`,
`get_header`, `bearer_token`, `basic_auth`. Uploaded-file entries expose
`read`.

Response methods: `set_header`, `get_header`, `set_cookie`, `delete_cookie`.
Authenticators expose `middleware`. Custom middleware receives a `next`
callable. Each is detailed in the sections below.

## Installation

```bash
go get github.com/starpkg/web
```

## Quick start

```go
package main

import (
    "github.com/1set/starlet"
    "github.com/starpkg/web"
)

func main() {
    machine := starlet.NewWithNames(nil, []string{"go_idiomatic"}, nil)
    machine.AddLazyloadModules(starlet.ModuleLoaderMap{
        web.ModuleName: web.NewModule().LoadModule(),
    })
    _, err := machine.RunScript([]byte(`
load("web", "create_server", "html_response", "json_response")

srv = create_server(host="localhost", port=8080)
srv.get("/", lambda req: html_response("<h1>Hello, World!</h1>"))
srv.get("/users/{id}", lambda req: json_response({"id": req.param("id")}))
srv.run()
`), nil)
    if err != nil {
        panic(err)
    }
}
```

A script loads the module, calls `create_server` to get a server object,
registers route handlers (each a `func(req)` returning a response), and starts
it with `srv.start()` (non-blocking) or `srv.run()` (blocking).

## Module functions

These are registered into the module namespace by `load("web", ...)`.

### Server

#### `create_server(host?, port?, server_header?)`

Creates an HTTP server. `host`/`port` fall back to the module configuration
when omitted; `server_header` overrides the configured `Server` response header
for this server. The port must be in `1..65535`. Returns a server object whose
methods are documented under [Server object](#server-object).

```python
srv = create_server(host="localhost", port=8080)
```

### Response builders

Each builder returns a response object that a handler returns to emit the HTTP
response. Their settable fields and methods are documented under
[Response object](#response-object).

#### `response(body, status?=200, headers?={})`

Builds a plain response from a string `body`, an optional `status` code, and an
optional `{name: value}` `headers` dict. No `Content-Type` is forced; when none
is set, the server sends `application/json` by default.

#### `json_response(data, status?=200, headers?={})`

Encodes `data` to JSON and sets `Content-Type: application/json`. If `data` is
already a `string` or `bytes` it is sent verbatim; otherwise it is serialized
(dicts, lists, numbers, …) via starlet's JSON encoder.

#### `html_response(content, status?=200, headers?={})`

HTML response; sets `Content-Type: text/html`.

#### `text_response(text, status?=200)`

Plain-text response; sets `Content-Type: text/plain`.

#### `file_response(filepath, content_type?, filename?)`

Serves the file at `filepath` from disk. `content_type` sets `Content-Type`;
`filename` adds `Content-Disposition: attachment; filename=...` to prompt a
download.

#### `send_file(filepath, content_type?)`

Serves the file at `filepath` from disk, optionally with an explicit
`content_type`. Like `file_response` but without the attachment filename.

#### `send_data(data, filename, content_type?="application/octet-stream")`

Sends the in-memory string `data` as a download. `filename` is required and
sets `Content-Disposition: attachment; filename=...`.

#### `redirect(location, status?=302)`

Redirect response carrying a `Location` header.

#### `error_response(status, message?="")`

Error response with the given `status` code and `message` body. (If a matching
`error_handler` is registered on the server, that handler renders the body
instead — see [`error_handler`](#srverror_handlerstatus_codes-handler).)

### Authentication

Each constructor returns an authenticator object exposing one method,
[`middleware()`](#authenticators), which yields a middleware you attach with
`srv.use_for(...)`. On failure the middleware short-circuits with a 401 (Basic
auth also sends a `WWW-Authenticate` challenge); on success it stores the user
info in the request context under `auth_user`.

#### `api_key_auth(keys?=[], header?="X-API-Key", query_param?="api_key")`

API-key authenticator. Accepts a key supplied either in the named `header` or
in the `query_param` query string parameter, matched against the allowed
`keys` list.

#### `bearer_auth(validate_func, header?="Authorization")`

Bearer-token authenticator. The `Bearer ` prefix is stripped from the
`Authorization` header (a custom `header` is used as-is or also de-prefixed),
then `validate_func(token)` is called: return user info on success, or `None`
to reject.

#### `basic_auth(users?={}, realm?="Restricted")`

HTTP Basic authenticator from a `{username: password}` dict. The `realm`
appears in the `WWW-Authenticate` challenge.

### Middleware

Each constructor returns a middleware object you attach with `srv.use(mw)`
(all paths) or `srv.use_for(pattern, mw)`. See [Middleware](#middleware-1) for
the path-pattern and custom-middleware model.

#### `cors_middleware(origins?=[], methods?=[], headers?=[], credentials?=False)`

Cross-Origin Resource Sharing. Empty `origins`/`methods`/`headers` default to
`*` / the common method set / `Content-Type, Authorization`. Handles
preflight `OPTIONS` with a `204` and echoes the configured headers.

#### `logging_middleware(format?="")`

Logs each request to stdout. `format` defaults to
`{method} {path} {status} {duration}ms`; the `{method}`, `{path}`, `{status}`,
and `{duration}` placeholders are substituted.

#### `security_headers_middleware(frame_options?="DENY", content_type_options?="nosniff", xss_protection?="1; mode=block", hsts?="", csp?="", referrer_policy?="")`

Adds common security response headers (`X-Frame-Options`,
`X-Content-Type-Options`, `X-XSS-Protection`, and — when non-empty —
`Strict-Transport-Security`, `Content-Security-Policy`, `Referrer-Policy`).

#### `timing_middleware(header?="X-Response-Time")`

Records handler duration and writes it to the named response `header`.

#### `json_middleware()`

Parses a JSON request body (when `Content-Type: application/json`) into the
request context under `json_data`, and tags JSON-looking response bodies with
`Content-Type: application/json` when unset.

#### `compression_middleware(level?=6, min_size?=1024, types?=[])`

gzip response compression when the client sends `Accept-Encoding: gzip`,
the body is at least `min_size` bytes, and its content type is compressible.
`level` is clamped to `1..9`; empty `types` defaults to the common text/JSON set.

#### `rate_limit_middleware(requests?=100, window?=60, key_func?=None)`

Per-key rate limiting: at most `requests` per `window` seconds, keyed by
`key_func(req)` (defaults to the client IP). Over the limit returns `429` with
a `Retry-After`; otherwise sets `X-RateLimit-*` headers. State is held in an
in-process memory store.

#### `cache_middleware(max_age?=3600, private?=False, patterns?=[], vary?=[])`

Adds `Cache-Control` (and optional `Vary`) headers to `GET` responses. When
`patterns` is non-empty, only matching request paths are affected; `private`
selects `private` over `public`.

#### `request_size_middleware(max_content_length?=10485760, max_url_length?=2048, max_headers?=100)`

Rejects oversized requests: `413` for a body over `max_content_length`, `414`
for a URL over `max_url_length`, `406` for more than `max_headers` headers.

## Server object

The object returned by `create_server` registers routes and controls the
server lifecycle.

### Routing

| Method | Description |
|--------|-------------|
| `srv.get(path, handler)` | Register a `GET` route. |
| `srv.post(path, handler)` | Register a `POST` route. |
| `srv.put(path, handler)` | Register a `PUT` route. |
| `srv.delete(path, handler)` | Register a `DELETE` route. |
| `srv.patch(path, handler)` | Register a `PATCH` route. |
| `srv.options(path, handler)` | Register an `OPTIONS` route. |
| `srv.head(path, handler)` | Register a `HEAD` route. |

Path parameters use Flask-style braces, e.g. `/users/{id}`, read back in the
handler with `req.param("id")`.

#### `srv.route(methods, path, handler)`

Registers one `handler` for one or more HTTP methods. `methods` is a method
string (e.g. `"GET"`) or a list of method strings.

#### `srv.group(prefix)`

Returns a route group sharing the `prefix`. The group exposes the same
per-method registrars (`grp.get(path, handler)`, `grp.post(...)`, … for
`put`/`delete`/`patch`/`options`/`head`).

### Middleware and error handlers

#### `srv.use(middleware)`

Adds global middleware (matches every path). Shorthand for
`srv.use_for("/*", middleware)`. Takes exactly one positional argument.

#### `srv.use_for(path_pattern, middleware)`

Adds `middleware` for requests whose path matches `path_pattern`. The argument
may be a middleware object (from a constructor above or an
[authenticator's `middleware()`](#authenticators)) or a custom callable
`func(req, next)` — see [Middleware](#middleware-1).

#### `srv.error_handler(status_codes, handler)`

Registers a custom `handler(req)` (returning a response) for one status code or
a list of status codes. The server invokes it whenever a response with a
matching status (including built-in `404`/`405`) is produced.

### Lifecycle

| Method | Description |
|--------|-------------|
| `srv.start()` | Start listening in the background (non-blocking); surfaces an immediate bind failure. |
| `srv.stop()` | Gracefully shut the server down. |
| `srv.run()` | Start and block until the server stops. |
| `srv.is_running()` | Return whether the server is currently running (`bool`). |

## Request object

The `req` passed to handlers and middleware exposes read-only **properties**:
`req.method`, `req.url`, `req.path`, `req.host`, `req.remote`, `req.client_ip`,
`req.proto`, `req.headers` (dict), `req.query` (dict), and `req.context`
(dict — middleware can read/write per-request state here, e.g. `auth_user`).

It also exposes **methods**:

#### `req.body()`

Raw request body as a string.

#### `req.json()`

Parses the body as JSON and returns the value, or `None` when the body is empty
or not valid JSON.

#### `req.form()`

Parses URL-encoded / multipart form data into a dict (a key with multiple
values becomes a list).

#### `req.files()`

Returns uploaded files (multipart) as a dict keyed by field name. Each entry is
a dict with `filename`, `size`, `content_type`, and a `read()` method returning
the file content as a string.

#### `req.cookie(name)`

Returns the named cookie's value, or `None`.

#### `req.param(name)`

Returns the named path parameter (e.g. `id` from `/users/{id}`), or `None`.

#### `req.get_header(name, default?=None)`

Returns the named request header, or `default` when absent.

#### `req.bearer_token(header?="Authorization")`

Extracts a Bearer token from the header (stripping the `Bearer ` prefix on
`Authorization`), or `None`.

#### `req.basic_auth()`

Returns a `(username, password)` tuple from HTTP Basic credentials, or `None`.

## Response object

A response (from a builder, or constructed by middleware) has settable fields
`status_code` (int), `headers` (dict), `body` (string), and `file_path`
(string), plus methods:

#### `set_header(name, value)`

Sets a response header.

#### `get_header(name, default?=None)`

Returns a response header, or `default` when absent.

#### `set_cookie(name, value, max_age?=None, path?="/", domain?="", secure?=False, http_only?=True)`

Appends a `Set-Cookie` header. Each call emits its own header line (cookies are
never comma-combined).

#### `delete_cookie(name, path?="/", domain?="")`

Appends a `Set-Cookie` line that expires the named cookie (`Max-Age=0`).

## Custom middleware

Middleware carries a path pattern; `srv.use(mw)` is shorthand for
`srv.use_for("/*", mw)`. Patterns support exact paths, `/*` prefix globs,
mid-path `*`, and `{param}` segments.

A custom middleware is a callable `func(req, next)` that returns a response. It
calls `next(req)` to invoke the rest of the chain (the `next` argument is itself
a builtin that takes the request and returns the downstream response), and may
inspect or mutate the response before returning it:

```python
def add_header(req, next):
    resp = next(req)
    resp.set_header("X-Custom-Server", "Starlark-Web")
    return resp

srv.use(add_header)
```

Built-in middleware is composed the same way:

```python
load("web", "create_server", "logging_middleware", "cors_middleware",
     "security_headers_middleware", "rate_limit_middleware")

srv = create_server()
srv.use(logging_middleware())                       # all paths
srv.use(cors_middleware(origins=["https://example.com"], credentials=True))
srv.use_for("/api/*", rate_limit_middleware(requests=100, window=60))
srv.use_for("/admin/*", security_headers_middleware(hsts="max-age=31536000"))
```

## Authenticators

Each authentication constructor returns an object exposing `auth.middleware()`,
which produces a middleware to attach with `srv.use_for(...)`:

```python
load("web", "basic_auth", "bearer_auth", "api_key_auth")

# Basic auth
auth = basic_auth(users={"admin": "secret"}, realm="Admin")
srv.use_for("/admin/*", auth.middleware())

# Bearer token, validated by a Starlark function
def check(token):
    return {"user": "alice"} if token == "s3cret" else None
srv.use_for("/api/*", bearer_auth(validate_func=check).middleware())

# API key (header or ?api_key=...)
srv.use_for("/svc/*", api_key_auth(keys=["k1", "k2"], header="X-API-Key").middleware())
```

In handlers, read auth data through the request helpers, e.g.
`req.basic_auth()`, `req.bearer_token()`, `req.get_header("X-API-Key")`, or the
`auth_user` entry the authenticator wrote into `req.context`.

## Hardening

By default the server binds to `localhost`, and starting it on any non-loopback
address (`0.0.0.0`, `::`, an empty host, or a public/LAN IP) is **refused** with
an error. This keeps a server started from an untrusted script off the network.
To expose it deliberately, opt in with `allow_public_bind=true` (or the
`WEB_ALLOW_PUBLIC_BIND` environment variable):

```python
srv = create_server(host="0.0.0.0", port=8080)
srv.run()   # error: refusing to bind to non-loopback host "0.0.0.0"; set allow_public_bind=true ...
```

The guardrail defaults to off — historical scripts that bind `localhost` keep
working unchanged. Full host-level network capability gating (deny-by-policy)
lives in the sandbox runtime layer; this is the module-local guardrail.

## Configuration

The module reads defaults from `base`'s config system; every option has an
environment-variable form.

| Option | Type | Default | Environment Variable | Description |
|--------|------|---------|----------------------|-------------|
| `host` | `string` | `"localhost"` | `WEB_HOST` | Default host to bind to |
| `port` | `int` | `8080` | `WEB_PORT` | Default port to listen on |
| `read_timeout` | `int` | `30` | `WEB_READ_TIMEOUT` | Read timeout in seconds |
| `write_timeout` | `int` | `30` | `WEB_WRITE_TIMEOUT` | Write timeout in seconds |
| `max_body_size` | `int` | `33554432` | `WEB_MAX_BODY_SIZE` | Maximum request body size in bytes (32 MiB) |
| `debug_mode` | `bool` | `false` | `WEB_DEBUG_MODE` | Enable Gin debug logging |
| `server_header` | `string` | `"Starlark-Web/1.0"` | `WEB_SERVER_HEADER` | Custom `Server` header value |
| `allow_public_bind` | `bool` | `false` | `WEB_ALLOW_PUBLIC_BIND` | Allow binding to a non-loopback (public) address |

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
