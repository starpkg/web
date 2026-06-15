# `web` — Starlark API Reference

The complete reference for every script-facing builtin, object method, and
configuration accessor exposed by the `web` module. For an overview,
installation, and a quickstart, see the [README](../README.md).

A script `load("web", …)`s the module-level builtins — `create_server`, the
response builders, the authentication constructors, and the middleware
constructors — plus a set of configuration accessors (`get_<key>` / `set_<key>`)
generated from the module's options. `create_server` returns a **server object**
that registers routes and controls the lifecycle; route handlers receive a
**request object** and return a **response object**.

## Contents

- [Server creation](#server-creation)
- [Response builders](#response-builders)
- [Authentication constructors](#authentication-constructors)
- [Middleware constructors](#middleware-constructors)
- [Server object](#server-object)
- [Request object](#request-object)
- [Response object](#response-object)
- [Custom middleware](#custom-middleware)
- [Hardening — the bind guardrail](#hardening--the-bind-guardrail)
- [Configuration](#configuration)

## Server creation

### `create_server(host?, port?, server_header?)`

Creates an HTTP server.

**Parameters:**

- `host` (string, optional): Host to bind to. Falls back to the `host` config
  option when omitted (default: `localhost`).
- `port` (int, optional): Port to listen on. Falls back to the `port` config
  option when omitted. Must be in `1..65535`.
- `server_header` (string, optional): Overrides the configured `Server`
  response header for this server.

**Returns:** A server object (see [Server object](#server-object)).

**Errors:** Fails if `port` is not a valid integer, or if it is outside
`1..65535`.

**Example:**

```python
srv = create_server(host="localhost", port=8080)
```

## Response builders

Each builder returns a response object that a handler returns to emit the HTTP
response. The settable fields and methods of that object are documented under
[Response object](#response-object).

### `response(body, status?, headers?)`

Builds a plain response from a string body, an optional status code, and an
optional `{name: value}` headers dict.

**Parameters:**

- `body` (string): The response body.
- `status` (int, optional): HTTP status code (default: `200`).
- `headers` (dict, optional): `{name: value}` response headers (default: `{}`).

**Returns:** A response object. No `Content-Type` is forced; when none is set,
the server sends `application/json` by default.

**Example:**

```python
response("OK", status=201, headers={"X-Request-Id": "abc"})
```

### `json_response(data, status?, headers?)`

Encodes `data` to JSON and sets `Content-Type: application/json`.

**Parameters:**

- `data`: The payload. A `string` or `bytes` value is sent verbatim; any other
  value (dict, list, number, …) is serialized via starlet's JSON encoder.
- `status` (int, optional): HTTP status code (default: `200`).
- `headers` (dict, optional): `{name: value}` response headers (default: `{}`).

**Returns:** A response object.

**Errors:** Fails if `data` cannot be marshalled to JSON, or if `headers` is not
a string→string dict.

**Example:**

```python
json_response({"id": 1, "name": "Alice"})
```

### `html_response(content, status?, headers?)`

HTML response; sets `Content-Type: text/html`.

**Parameters:**

- `content` (string): The HTML body.
- `status` (int, optional): HTTP status code (default: `200`).
- `headers` (dict, optional): `{name: value}` response headers (default: `{}`).

**Returns:** A response object.

**Example:**

```python
html_response("<h1>Hello, World!</h1>")
```

### `text_response(text, status?)`

Plain-text response; sets `Content-Type: text/plain`.

**Parameters:**

- `text` (string): The text body.
- `status` (int, optional): HTTP status code (default: `200`).

**Returns:** A response object.

**Example:**

```python
text_response("pong")
```

### `file_response(filepath, content_type?, filename?)`

Serves the file at `filepath` from disk.

**Parameters:**

- `filepath` (string): Path to the file to serve.
- `content_type` (string, optional): Sets the `Content-Type` header.
- `filename` (string, optional): Adds `Content-Disposition: attachment;
  filename=...` to prompt a download.

**Returns:** A response object.

**Example:**

```python
file_response("/srv/report.pdf", content_type="application/pdf",
              filename="report.pdf")
```

### `send_file(filepath, content_type?)`

Serves the file at `filepath` from disk — like `file_response` but without the
attachment filename.

**Parameters:**

- `filepath` (string): Path to the file to serve.
- `content_type` (string, optional): Sets the `Content-Type` header.

**Returns:** A response object.

**Example:**

```python
send_file("/srv/index.html", content_type="text/html")
```

### `send_data(data, filename, content_type?)`

Sends the in-memory string `data` as a download.

**Parameters:**

- `data` (string): The in-memory content to send.
- `filename` (string): Required; sets `Content-Disposition: attachment;
  filename=...`.
- `content_type` (string, optional): The `Content-Type` header (default:
  `application/octet-stream`).

**Returns:** A response object.

**Example:**

```python
send_data("col1,col2\n1,2\n", filename="data.csv", content_type="text/csv")
```

### `redirect(location, status?)`

Redirect response carrying a `Location` header.

**Parameters:**

- `location` (string): The target URL for the `Location` header.
- `status` (int, optional): HTTP status code (default: `302`).

**Returns:** A response object.

**Example:**

```python
redirect("/login", status=303)
```

### `error_response(status, message?)`

Error response with the given status code and message body. If a matching
`error_handler` is registered on the server, that handler renders the body
instead — see [`error_handler`](#error_handlerstatus_codes-handler).

**Parameters:**

- `status` (int): HTTP status code.
- `message` (string, optional): The response body (default: `""`).

**Returns:** A response object.

**Example:**

```python
error_response(404, message="Not Found")
```

## Authentication constructors

Each constructor returns an **authenticator object** exposing a single method,
[`middleware()`](#authenticator-object), which yields a middleware you attach
with `srv.use_for(...)`. On failure the middleware short-circuits with a `401`
(Basic auth also sends a `WWW-Authenticate` challenge); on success it stores the
user info in the request context under `auth_user`.

### `api_key_auth(keys?, header?, query_param?)`

API-key authenticator. Accepts a key supplied either in the named `header` or in
the `query_param` query-string parameter, matched against the allowed `keys`
list.

**Parameters:**

- `keys` (list, optional): Allowed API keys (default: `[]`).
- `header` (string, optional): Header to read the key from (default:
  `X-API-Key`).
- `query_param` (string, optional): Query-string parameter to read the key from
  (default: `api_key`).

**Returns:** An authenticator object.

**Errors:** Fails if `keys` is not a list of strings.

**Example:**

```python
api_key_auth(keys=["k1", "k2"], header="X-API-Key")
```

### `bearer_auth(validate_func, header?)`

Bearer-token authenticator. The `Bearer` scheme prefix (and its trailing space)
is stripped from the `Authorization` header (a custom `header` is used as-is or
also de-prefixed), then `validate_func(token)` is called.

**Parameters:**

- `validate_func` (callable): Called as `validate_func(token)`; return user info
  on success, or `None` to reject.
- `header` (string, optional): Header to read the token from (default:
  `Authorization`).

**Returns:** An authenticator object.

**Example:**

```python
def check(token):
    return {"user": "alice"} if token == "s3cret" else None

bearer_auth(validate_func=check)
```

### `basic_auth(users?, realm?)`

HTTP Basic authenticator from a `{username: password}` dict.

**Parameters:**

- `users` (dict, optional): `{username: password}` credentials (default: `{}`).
- `realm` (string, optional): Realm shown in the `WWW-Authenticate` challenge
  (default: `Restricted`).

**Returns:** An authenticator object.

**Errors:** Fails if `users` is not a string→string dict.

**Example:**

```python
basic_auth(users={"admin": "secret"}, realm="Admin")
```

### Authenticator object

An authenticator (from any of the three constructors above) exposes one method.

#### `middleware()`

Produces a middleware to attach with `srv.use_for(...)`.

- **Parameters:** None
- **Returns:** A middleware object.

```python
auth = basic_auth(users={"admin": "secret"})
srv.use_for("/admin/*", auth.middleware())
```

In handlers, read auth data through the request helpers — `req.basic_auth()`,
`req.bearer_token()`, `req.get_header("X-API-Key")` — or the `auth_user` entry
the authenticator wrote into `req.context`.

## Middleware constructors

Each constructor returns a middleware object you attach with `srv.use(mw)` (all
paths) or `srv.use_for(pattern, mw)`. See [Custom middleware](#custom-middleware)
for the path-pattern and custom-middleware model.

### `cors_middleware(origins?, methods?, headers?, credentials?)`

Cross-Origin Resource Sharing. Handles preflight `OPTIONS` with a `204` and
echoes the configured headers.

**Parameters:**

- `origins` (list, optional): Allowed origins. Empty defaults to `*` (default:
  `[]`).
- `methods` (list, optional): Allowed methods. Empty defaults to the common
  method set (default: `[]`).
- `headers` (list, optional): Allowed request headers. Empty defaults to
  `Content-Type, Authorization` (default: `[]`).
- `credentials` (bool, optional): Whether to allow credentials (default:
  `False`).

**Returns:** A middleware object.

**Example:**

```python
cors_middleware(origins=["https://example.com"], credentials=True)
```

### `logging_middleware(format?)`

Logs each request to stdout.

**Parameters:**

- `format` (string, optional): Log line format (default:
  `{method} {path} {status} {duration}ms`). The `{method}`, `{path}`,
  `{status}`, and `{duration}` placeholders are substituted.

**Returns:** A middleware object.

**Example:**

```python
logging_middleware()
```

### `security_headers_middleware(frame_options?, content_type_options?, xss_protection?, hsts?, csp?, referrer_policy?)`

Adds common security response headers. Each option emits its header only when
non-empty.

**Parameters:**

- `frame_options` (string, optional): `X-Frame-Options` value (default: `DENY`).
- `content_type_options` (string, optional): `X-Content-Type-Options` value
  (default: `nosniff`).
- `xss_protection` (string, optional): `X-XSS-Protection` value (default:
  `1; mode=block`).
- `hsts` (string, optional): `Strict-Transport-Security` value (default: `""`,
  header omitted).
- `csp` (string, optional): `Content-Security-Policy` value (default: `""`,
  header omitted).
- `referrer_policy` (string, optional): `Referrer-Policy` value (default: `""`,
  header omitted).

**Returns:** A middleware object.

**Example:**

```python
security_headers_middleware(hsts="max-age=31536000")
```

### `timing_middleware(header?)`

Records handler duration and writes it to the named response header.

**Parameters:**

- `header` (string, optional): Response header to write the duration to
  (default: `X-Response-Time`).

**Returns:** A middleware object.

**Example:**

```python
timing_middleware()
```

### `json_middleware()`

Parses a JSON request body (when `Content-Type: application/json`) into the
request context under `json_data`, and tags JSON-looking response bodies with
`Content-Type: application/json` when unset.

**Parameters:** None

**Returns:** A middleware object.

**Example:**

```python
json_middleware()
```

### `compression_middleware(level?, min_size?, types?)`

gzip response compression when the client sends `Accept-Encoding: gzip`, the body
is at least `min_size` bytes, and its content type is compressible.

**Parameters:**

- `level` (int, optional): gzip level. A value outside `1..9` falls back to `6`
  (default: `6`).
- `min_size` (int, optional): Minimum body size in bytes to compress (default:
  `1024`).
- `types` (list, optional): Compressible content types. Empty defaults to the
  common text/JSON set (default: `[]`).

**Returns:** A middleware object.

**Example:**

```python
compression_middleware(level=9, min_size=512)
```

### `rate_limit_middleware(requests?, window?, key_func?)`

Per-key rate limiting: at most `requests` per `window` seconds. Over the limit
returns `429` with a `Retry-After`; otherwise sets `X-RateLimit-*` headers. State
is held in an in-process memory store.

**Parameters:**

- `requests` (int, optional): Max requests per window (default: `100`).
- `window` (int, optional): Window length in seconds (default: `60`).
- `key_func` (callable, optional): Called as `key_func(req)` to derive the rate
  key; defaults to the client IP (and falls back to the client IP if the
  callable errors or does not return a string).

**Returns:** A middleware object.

**Example:**

```python
rate_limit_middleware(requests=100, window=60)
```

### `cache_middleware(max_age?, private?, patterns?, vary?)`

Adds `Cache-Control` (and optional `Vary`) headers to `GET` responses.

**Parameters:**

- `max_age` (int, optional): `max-age` in seconds (default: `3600`).
- `private` (bool, optional): Selects `private` over `public` (default:
  `False`).
- `patterns` (list, optional): When non-empty, only matching request paths are
  affected (default: `[]`).
- `vary` (list, optional): Header names for the `Vary` header (default: `[]`).

**Returns:** A middleware object.

**Example:**

```python
cache_middleware(max_age=600, patterns=["/static/*"], vary=["Accept-Encoding"])
```

### `request_size_middleware(max_content_length?, max_url_length?, max_headers?)`

Rejects oversized requests.

**Parameters:**

- `max_content_length` (int, optional): Body size cap in bytes; over it returns
  `413` (default: `10485760`, i.e. 10 MiB).
- `max_url_length` (int, optional): URL length cap; over it returns `414`
  (default: `2048`).
- `max_headers` (int, optional): Header count cap; over it returns `406`
  (default: `100`).

**Returns:** A middleware object.

**Example:**

```python
request_size_middleware(max_content_length=1048576)
```

## Server object

The object returned by `create_server` registers routes and controls the server
lifecycle.

### Routing

#### `get(path, handler)`

Registers a `GET` route.

- `path` (string): Route path; Flask-style braces (`/users/{id}`) define path
  parameters, read back with `req.param("id")`.
- `handler` (callable): `func(req)` returning a response.
- **Returns:** None

#### `post(path, handler)`

Registers a `POST` route. Same parameters as [`get`](#getpath-handler).

#### `put(path, handler)`

Registers a `PUT` route. Same parameters as [`get`](#getpath-handler).

#### `delete(path, handler)`

Registers a `DELETE` route. Same parameters as [`get`](#getpath-handler).

#### `patch(path, handler)`

Registers a `PATCH` route. Same parameters as [`get`](#getpath-handler).

#### `options(path, handler)`

Registers an `OPTIONS` route. Same parameters as [`get`](#getpath-handler).

#### `head(path, handler)`

Registers a `HEAD` route. Same parameters as [`get`](#getpath-handler).

#### `route(methods, path, handler)`

Registers one `handler` for one or more HTTP methods.

- `methods` (string or list): A method string (e.g. `"GET"`) or a list of method
  strings.
- `path` (string): Route path.
- `handler` (callable): `func(req)` returning a response.
- **Returns:** None

```python
srv.route(["GET", "POST"], "/submit", handle_submit)
```

#### `group(prefix)`

Returns a route group sharing the `prefix`.

- `prefix` (string): Path prefix shared by the group's routes.
- **Returns:** A route group exposing the same per-method registrars
  (`grp.get(path, handler)`, `grp.post(...)`, and `put`/`delete`/`patch`/
  `options`/`head`).

```python
api = srv.group("/api/v1")
api.get("/users", list_users)
```

### Middleware and error handlers

#### `use(middleware)`

Adds global middleware (matches every path). Shorthand for
`srv.use_for("/*", middleware)`. Takes exactly one positional argument.

- `middleware`: A middleware object or a custom `func(req, next)` callable.
- **Returns:** None

#### `use_for(path_pattern, middleware)`

Adds `middleware` for requests whose path matches `path_pattern`.

- `path_pattern` (string): Pattern — exact path, `/*` prefix glob, mid-path `*`,
  or `{param}` segments.
- `middleware`: A middleware object (from a constructor above or an
  [authenticator's `middleware()`](#middleware)) or a custom `func(req, next)`
  callable — see [Custom middleware](#custom-middleware).
- **Returns:** None

#### `error_handler(status_codes, handler)`

Registers a custom handler for one status code or a list of status codes. The
server invokes it whenever a response with a matching status (including built-in
`404`/`405`) is produced.

- `status_codes` (int or list): One status code, or a list of codes.
- `handler` (callable): `handler(req)` returning a response.
- **Returns:** None

```python
srv.error_handler([404, 405], lambda req: json_response({"error": "not found"}, status=404))
```

### Lifecycle

#### `start()`

Starts listening in the background (non-blocking); surfaces an immediate bind
failure (e.g. port in use, or a denied non-loopback bind).

- **Parameters:** None
- **Returns:** None

#### `stop()`

Gracefully shuts the server down.

- **Parameters:** None
- **Returns:** None

#### `run()`

Starts and blocks until the server stops.

- **Parameters:** None
- **Returns:** None

#### `is_running()`

Returns whether the server is currently running.

- **Parameters:** None
- **Returns:** Boolean.

## Request object

The `req` passed to handlers and middleware exposes read-only **properties** and
**methods**.

**Properties:** `method`, `url`, `path`, `host`, `remote`, `client_ip`, `proto`,
`headers` (dict), `query` (dict), and `context` (dict — middleware can read/write
per-request state here, e.g. `auth_user`).

### `body()`

Returns the raw request body as a string.

- **Parameters:** None
- **Returns:** String.

### `json()`

Parses the body as JSON and returns the value.

- **Parameters:** None
- **Returns:** The parsed value, or `None` when the body is empty or not valid
  JSON.

### `form()`

Parses URL-encoded / multipart form data into a dict.

- **Parameters:** None
- **Returns:** A dict; a key with multiple values becomes a list.

### `files()`

Returns uploaded files (multipart) keyed by field name.

- **Parameters:** None
- **Returns:** A dict keyed by field name. Each entry is a dict with `filename`,
  `size`, `content_type`, and a `read()` method returning the file content as a
  string.

```python
for field, f in req.files().items():
    data = f["read"]()
    print(field, f["filename"], f["size"], f["content_type"])
```

### `cookie(name)`

Returns the named cookie's value.

- `name` (string): Cookie name.
- **Returns:** The cookie value, or `None`.

### `param(name)`

Returns the named path parameter (e.g. `id` from `/users/{id}`).

- `name` (string): Path-parameter name.
- **Returns:** The parameter value, or `None`.

### `get_header(name, default?)`

Returns the named request header.

- `name` (string): Header name.
- `default` (optional): Returned when the header is absent (default: `None`).
- **Returns:** The header value, or `default`.

### `bearer_token(header?)`

Extracts a Bearer token, stripping the `Bearer` scheme prefix and its trailing
space on `Authorization`.

- `header` (string, optional): Header to read from (default: `Authorization`).
- **Returns:** The token string, or `None`.

### `basic_auth()`

Returns HTTP Basic credentials.

- **Parameters:** None
- **Returns:** A `(username, password)` tuple, or `None`.

## Response object

A response (from a builder, or constructed by middleware) has settable fields and
methods.

**Settable fields:** `status_code` (int), `headers` (dict), `body` (string), and
`file_path` (string).

### `set_header(name, value)`

Sets a response header.

- `name` (string): Header name.
- `value` (string): Header value.
- **Returns:** None

### `get_header(name, default?)`

Returns a response header.

- `name` (string): Header name.
- `default` (optional): Returned when the header is absent (default: `None`).
- **Returns:** The header value, or `default`.

### `set_cookie(name, value, max_age?, path?, domain?, secure?, http_only?)`

Appends a `Set-Cookie` header. Each call emits its own header line (cookies are
never comma-combined).

- `name` (string): Cookie name.
- `value` (string): Cookie value.
- `max_age` (int, optional): `Max-Age` in seconds (default: `None`, omitted).
- `path` (string, optional): Cookie path (default: `/`).
- `domain` (string, optional): Cookie domain (default: `""`).
- `secure` (bool, optional): Sets the `Secure` flag (default: `False`).
- `http_only` (bool, optional): Sets the `HttpOnly` flag (default: `True`).
- **Returns:** None

### `delete_cookie(name, path?, domain?)`

Appends a `Set-Cookie` line that expires the named cookie (`Max-Age=0`).

- `name` (string): Cookie name.
- `path` (string, optional): Cookie path (default: `/`).
- `domain` (string, optional): Cookie domain (default: `""`).
- **Returns:** None

## Custom middleware

Middleware carries a path pattern; `srv.use(mw)` is shorthand for
`srv.use_for("/*", mw)`. Patterns support exact paths, `/*` prefix globs,
mid-path `*`, and `{param}` segments.

A custom middleware is a callable `func(req, next)` that returns a response. It
calls `next(req)` to invoke the rest of the chain (`next` is itself a builtin
that takes the request and returns the downstream response), and may inspect or
mutate the response before returning it:

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

## Hardening — the bind guardrail

By default the server binds to `localhost`, and starting it on any non-loopback
address (`0.0.0.0`, `::`, an empty host, or a public/LAN IP) is **refused** with
an error. This keeps a server started from an untrusted script off the network.
To expose it deliberately, opt in with the `allow_public_bind` config option
(or the `WEB_ALLOW_PUBLIC_BIND` environment variable):

```python
srv = create_server(host="0.0.0.0", port=8080)
srv.run()   # error: refusing to bind to non-loopback host "0.0.0.0"; set allow_public_bind=true ...
```

The guardrail defaults to off — historical scripts that bind `localhost` keep
working unchanged. Full host-level network capability gating (deny-by-policy)
lives in the sandbox runtime layer; this is the module-local guardrail.

## Configuration

Each module configuration option is exposed to scripts as a pair of generated
accessor builtins (loaded from the `web` module alongside the functions above):

- **`get_<key>()`** — returns the current value of the option.
- **`set_<key>(value)`** — sets the option (returns `None`).

An option's value resolves in priority order: an explicit `set_<key>` value, the
environment variable, then the default. The `host` and `port` options serve as
defaults used by `create_server` when the corresponding argument is not provided;
the timeouts, body cap, debug mode, server header, and bind guardrail apply to
every server the module creates.

None of the `web` options are secret, so every option exposes **both**
`get_<key>` and `set_<key>`. (A secret option would expose only its `set_<key>`
accessor — never a getter — but this module has none.)

| Option | Getter | Setter | Type | Env var | Default | Description |
|--------|--------|--------|------|---------|---------|-------------|
| `host` | `get_host` | `set_host` | string | `WEB_HOST` | `localhost` | Default host to bind to |
| `port` | `get_port` | `set_port` | int | `WEB_PORT` | `8080` | Default port to listen on |
| `read_timeout` | `get_read_timeout` | `set_read_timeout` | int | `WEB_READ_TIMEOUT` | `30` | Read timeout in seconds |
| `write_timeout` | `get_write_timeout` | `set_write_timeout` | int | `WEB_WRITE_TIMEOUT` | `30` | Write timeout in seconds |
| `max_body_size` | `get_max_body_size` | `set_max_body_size` | int | `WEB_MAX_BODY_SIZE` | `33554432` | Maximum request body size in bytes (32 MiB) |
| `debug_mode` | `get_debug_mode` | `set_debug_mode` | bool | `WEB_DEBUG_MODE` | `false` | Enable Gin debug logging |
| `server_header` | `get_server_header` | `set_server_header` | string | `WEB_SERVER_HEADER` | `Starlark-Web/1.0` | Custom `Server` header value |
| `allow_public_bind` | `get_allow_public_bind` | `set_allow_public_bind` | bool | `WEB_ALLOW_PUBLIC_BIND` | `false` | Allow binding to a non-loopback (public) address |

**Example:**

```python
load(
    "web",
    "create_server",
    # getters
    "get_host", "get_port", "get_read_timeout", "get_write_timeout",
    "get_max_body_size", "get_debug_mode", "get_server_header",
    "get_allow_public_bind",
    # setters
    "set_host", "set_port", "set_read_timeout", "set_write_timeout",
    "set_max_body_size", "set_debug_mode", "set_server_header",
    "set_allow_public_bind",
)

set_port(9090)
print(get_port())  # 9090

srv = create_server()  # binds to host=localhost, port=9090
srv.run()
```
