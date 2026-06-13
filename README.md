# 🌐 `web` — Flask-inspired Web Framework for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/web.svg)](https://pkg.go.dev/github.com/starpkg/web)

Build server-side HTTP applications from Starlark with a Flask-inspired API:
routing, path-pattern middleware, request/response helpers, and pluggable
authentication, on top of [gin](https://github.com/gin-gonic/gin).

## Installation

```bash
go get github.com/starpkg/web
```

## Functions

Module-level functions registered by `load("web", ...)`:

| Function | Signature | Description |
|----------|-----------|-------------|
| `create_server` | `create_server(host?, port?, server_header?) -> Server` | Create an HTTP server. Falls back to configured host/port. |
| `response` | `response(body, status?=200, headers?={}) -> Response` | Build a plain response. |
| `json_response` | `json_response(data, status?=200, headers?={}) -> Response` | Encode `data` to JSON; sets `Content-Type: application/json`. |
| `html_response` | `html_response(content, status?=200, headers?={}) -> Response` | HTML response; sets `Content-Type: text/html`. |
| `text_response` | `text_response(text, status?=200) -> Response` | Plain-text response; sets `Content-Type: text/plain`. |
| `file_response` | `file_response(filepath, content_type?, filename?) -> Response` | Serve a file from disk; `filename` adds a `Content-Disposition` attachment. |
| `send_file` | `send_file(filepath, content_type?) -> Response` | Serve a file from disk. |
| `send_data` | `send_data(data, filename, content_type?="application/octet-stream") -> Response` | Send raw bytes as an attachment. |
| `redirect` | `redirect(location, status?=302) -> Response` | Redirect response with a `Location` header. |
| `error_response` | `error_response(status, message?="") -> Response` | Error response with the given status and body. |
| `api_key_auth` | `api_key_auth(keys?=[], header?="X-API-Key", query_param?="api_key") -> Authenticator` | API-key authenticator (checks header then query param). |
| `bearer_auth` | `bearer_auth(validate_func, header?="Authorization") -> Authenticator` | Bearer-token authenticator; `validate_func(token)` returns user info or `None`. |
| `basic_auth` | `basic_auth(users?={}, realm?="Restricted") -> Authenticator` | HTTP Basic authenticator from a `{username: password}` dict. |
| `cors_middleware` | `cors_middleware(origins?=[], methods?=[], headers?=[], credentials?=False) -> Middleware` | Cross-Origin Resource Sharing headers. |
| `logging_middleware` | `logging_middleware(format?="") -> Middleware` | Request logging. |
| `security_headers_middleware` | `security_headers_middleware(frame_options?="DENY", content_type_options?="nosniff", xss_protection?="1; mode=block", hsts?="", csp?="", referrer_policy?="") -> Middleware` | Common security response headers. |
| `timing_middleware` | `timing_middleware(header?="X-Response-Time") -> Middleware` | Adds a response-time header. |
| `json_middleware` | `json_middleware() -> Middleware` | Enforces/normalizes JSON request/response handling. |
| `compression_middleware` | `compression_middleware(level?=6, min_size?=1024, types?=[]) -> Middleware` | gzip response compression. |
| `rate_limit_middleware` | `rate_limit_middleware(requests?=100, window?=60, key_func?=None) -> Middleware` | Per-key rate limiting; `key_func(req)` defaults to client IP. |
| `cache_middleware` | `cache_middleware(max_age?=3600, private?=False, patterns?=[], vary?=[]) -> Middleware` | HTTP cache-control headers. |
| `request_size_middleware` | `request_size_middleware(max_content_length?=10485760, max_url_length?=2048, max_headers?=100) -> Middleware` | Bounds request body/URL/header sizes. |

### Server methods

A `Server` returned by `create_server` exposes:

| Method | Description |
|--------|-------------|
| `srv.get/post/put/delete/patch/options/head(path, handler)` | Register a route for one HTTP method. |
| `srv.route(methods, path, handler)` | Register a route for one or more methods (`methods` is a string or list of strings). |
| `srv.group(prefix) -> RouteGroup` | Create a route group sharing a path prefix. |
| `srv.use(middleware)` | Add global middleware (shorthand for `use_for("/*", middleware)`). |
| `srv.use_for(path_pattern, middleware)` | Add middleware for paths matching `path_pattern`. |
| `srv.error_handler(status_codes, handler)` | Register a custom handler for one or more status codes. |
| `srv.start()` / `srv.stop()` | Start/stop the server (non-blocking start). |
| `srv.run()` | Start and block until the server stops. |
| `srv.is_running() -> bool` | Whether the server is currently running. |

### Request attributes and methods

The `req` passed to a handler exposes properties:
`req.method`, `req.url`, `req.path`, `req.host`, `req.remote`, `req.client_ip`,
`req.proto`, `req.headers`, `req.query`, `req.context`; and methods:

| Method | Description |
|--------|-------------|
| `req.body() -> str` | Raw request body. |
| `req.json() -> value` | Parsed JSON body (or `None`). |
| `req.form() -> dict` | Parsed form data. |
| `req.files() -> dict` | Uploaded files (multipart). |
| `req.cookie(name) -> str` | Cookie value (or `None`). |
| `req.param(name) -> str` | Path parameter (or `None`). |
| `req.get_header(name, default?=None) -> str` | Request header value. |
| `req.bearer_token(header?="Authorization") -> str` | Extract a Bearer token (or `None`). |
| `req.basic_auth() -> (user, pass)` | Basic-auth credentials tuple (or `None`). |

### Response attributes and methods

A `Response` exposes settable fields `status_code`, `headers`, `body`,
`file_path`, and methods `set_header(name, value)`,
`get_header(name, default?=None)`, `set_cookie(name, value, ...)`,
`delete_cookie(name, path?, domain?)`. An `Authenticator` exposes
`auth.middleware()`.

## Usage

```python
load("web", "create_server", "html_response", "json_response")

def main():
    srv = create_server(host="localhost", port=8080)

    def home(req):
        return html_response("<h1>Hello, World!</h1>")

    def user_profile(req):
        return json_response({"user_id": req.param("id"), "method": req.method})

    srv.get("/", home)
    srv.get("/users/{id}", user_profile)
    srv.run()

main()
```

### Middleware

All middleware carries a path pattern. `srv.use(mw)` is shorthand for
`srv.use_for("/*", mw)`.

```python
load("web", "create_server", "logging_middleware", "cors_middleware",
     "security_headers_middleware", "rate_limit_middleware")

srv = create_server()
srv.use(logging_middleware())                       # all paths
srv.use(cors_middleware(origins=["https://example.com"], credentials=True))
srv.use_for("/api/*", rate_limit_middleware(requests=100, window=60))
srv.use_for("/admin/*", security_headers_middleware(hsts="max-age=31536000"))
```

Custom middleware is a `func(req, next)` that returns a response:

```python
def add_header(req, next):
    resp = next(req)
    resp.set_header("X-Custom-Server", "Starlark-Web")
    return resp

srv.use(add_header)
```

### Authentication

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
`req.basic_auth()`, `req.bearer_token()`, or `req.get_header("X-API-Key")`.

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

> Full host-level network capability gating (deny-by-policy) lives in the
> sandbox runtime layer; this is the module-local guardrail.

## Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | `string` | `"localhost"` | Default host to bind to |
| `port` | `int` | `8080` | Default port to listen on |
| `read_timeout` | `int` | `30` | Read timeout in seconds |
| `write_timeout` | `int` | `30` | Write timeout in seconds |
| `max_body_size` | `int` | `33554432` | Maximum request body size in bytes (32 MiB) |
| `debug_mode` | `bool` | `false` | Enable Gin debug logging |
| `server_header` | `string` | `"Starlark-Web/1.0"` | Custom `Server` header value |
| `allow_public_bind` | `bool` | `false` | Allow binding to a non-loopback (public) address |

Settable via `WEB_HOST` / `WEB_PORT` / `WEB_READ_TIMEOUT` / `WEB_WRITE_TIMEOUT` /
`WEB_MAX_BODY_SIZE` / `WEB_DEBUG_MODE` / `WEB_SERVER_HEADER` /
`WEB_ALLOW_PUBLIC_BIND`.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
