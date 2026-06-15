# 🌐 `web` — Flask-inspired web server for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/web.svg)](https://pkg.go.dev/github.com/starpkg/web)
[![license](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/starpkg/web)](https://goreportcard.com/report/github.com/starpkg/web)
[![codecov](https://codecov.io/gh/starpkg/web/graph/badge.svg)](https://codecov.io/gh/starpkg/web)
![binary footprint](https://img.shields.io/badge/binary_footprint-%2B5.1_MB-blue)

Build server-side HTTP applications from Starlark with a Flask-inspired API:
routing, route groups, path-pattern middleware, request/response helpers,
pluggable authentication, and custom error handlers — on top of
[gin](https://github.com/gin-gonic/gin).

## Overview

Within the Star\* ecosystem `starpkg` is *"support for necessary **local**
operations plus simple abstractions over common **online** services, for ease of
use."* `web` is a **local capability**: it stands up an HTTP listener inside the
host process so a script can *serve* requests (the inverse of an HTTP *client* —
for outbound calls use starlet's `http` module).

- **Routing** — `create_server` returns a server with per-method registrars
  (`get`/`post`/`put`/…), `route`, and route `group`s; Flask-style `/{id}`
  path parameters.
- **Responses** — builders for plain, JSON, HTML, text, file, download, and
  redirect/error responses, returned from a `func(req)` handler.
- **Middleware** — built-in CORS, logging, security headers, timing, JSON,
  compression, rate limiting, caching, and request-size limits, plus custom
  `func(req, next)` middleware, attached globally or per path pattern.
- **Authentication** — pluggable `api_key_auth`, `bearer_auth`, and
  `basic_auth` authenticators that yield middleware.
- **Safe by default** — a default-deny **loopback bind** guardrail keeps a
  server started from an untrusted script off the network unless the host opts
  in with `allow_public_bind`.

For the complete per-builtin / per-object-method reference — signatures,
parameters, returns, errors, examples — and the configuration accessors, see
**[docs/API.md](docs/API.md)**.

## Installation

```bash
go get github.com/starpkg/web
```

## Quick start

Wire the module into a Starlet interpreter, then `load("web", …)` from a script:

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

## Starlark API at a glance

Module builtins (`load("web", …)`):

- `create_server(host?, port?, server_header?)` — create an HTTP server object.
- `response(body, status?, headers?)` — plain response.
- `json_response(data, status?, headers?)` — JSON response.
- `html_response(content, status?, headers?)` — HTML response.
- `text_response(text, status?)` — plain-text response.
- `file_response(filepath, content_type?, filename?)` — serve a file (optionally as a download).
- `send_file(filepath, content_type?)` — serve a file.
- `send_data(data, filename, content_type?)` — send in-memory data as a download.
- `redirect(location, status?)` — redirect response.
- `error_response(status, message?)` — error response.
- `api_key_auth(keys?, header?, query_param?)` — API-key authenticator.
- `bearer_auth(validate_func, header?)` — bearer-token authenticator.
- `basic_auth(users?, realm?)` — HTTP Basic authenticator.
- `cors_middleware(origins?, methods?, headers?, credentials?)` — CORS middleware.
- `logging_middleware(format?)` — request logging middleware.
- `security_headers_middleware(frame_options?, content_type_options?, xss_protection?, hsts?, csp?, referrer_policy?)` — security headers middleware.
- `timing_middleware(header?)` — response-time header middleware.
- `json_middleware()` — JSON request/response middleware.
- `compression_middleware(level?, min_size?, types?)` — gzip compression middleware.
- `rate_limit_middleware(requests?, window?, key_func?)` — per-key rate limiting middleware.
- `cache_middleware(max_age?, private?, patterns?, vary?)` — `Cache-Control` middleware.
- `request_size_middleware(max_content_length?, max_url_length?, max_headers?)` — request-size limiting middleware.

Server object: `get` / `post` / `put` / `delete` / `patch` / `options` / `head`,
`route`, `group`, `use`, `use_for`, `error_handler`, `start`, `stop`, `run`,
`is_running`.

Request object — properties `method`, `url`, `path`, `host`, `remote`,
`client_ip`, `proto`, `headers`, `query`, `context`; methods `body`, `json`,
`form`, `files`, `cookie`, `param`, `get_header`, `bearer_token`, `basic_auth`.
Uploaded-file entries expose `read`.

Response object — settable `status_code`, `headers`, `body`, `file_path`;
methods `set_header`, `get_header`, `set_cookie`, `delete_cookie`.
Authenticators expose `middleware`. Custom middleware receives a `next` callable.

See **[docs/API.md](docs/API.md)** for the full signatures, return values,
errors, and examples of every builtin and method above.

## Configuration

The module's options (`host`, `port`, `read_timeout`, `write_timeout`,
`max_body_size`, `debug_mode`, `server_header`, `allow_public_bind`) are
configured via environment variables (`WEB_*`) or per-option `get_<key>` /
`set_<key>` accessor builtins, and serve as defaults for the servers the module
creates. The opt-in `allow_public_bind` lever lets the host expose a server
beyond loopback; it defaults to off. See the
[Configuration section of docs/API.md](docs/API.md#configuration) for the full
option table, defaults, accessors, and the bind-guardrail details.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
