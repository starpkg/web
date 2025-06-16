# Web Module Design

## Overview

The `web` module provides HTTP server and client capabilities for Starlark scripts using only Go's standard library. It follows the base package configuration pattern and maintains consistency with other Starlark modules.

## Core Components

### 1. Server Component (`server`)

Creates HTTP servers with route handling and static file serving.

```starlark
srv = server(port=8080, host="0.0.0.0")
srv.route("/api/users", handler, methods=["GET", "POST"])
srv.static("/static", "./public")
srv.run()
```

### 2. Client Component (`client`)

Makes HTTP requests with JSON and form support.

```starlark
resp = client.get("https://api.example.com/data")
resp = client.post(url, json={"key": "value"})
```

### 3. Response Helpers

Utility functions for common response types:

- `json_response(data, status=200)` - JSON responses
- `html(content, status=200)` - HTML responses
- `redirect(location, status=302)` - Redirects
- `response(body, status=200, headers={})` - Custom responses

## API Design

### Server Configuration

Using base package pattern:

```go
type Config struct {
    Port         *base.ConfigOption[int]      // Default: 8080
    Host         *base.ConfigOption[string]   // Default: "0.0.0.0"
    ReadTimeout  *base.ConfigOption[int]      // Default: 30 seconds
    WriteTimeout *base.ConfigOption[int]      // Default: 30 seconds
    MaxBodySize  *base.ConfigOption[int64]    // Default: 10MB
}
```

### Request Object

Passed to all handlers:

```starlark
request = {
    # Properties
    "method": "GET",
    "path": "/api/users/123",
    "url": "http://localhost:8080/api/users/123",
    "remote_addr": "127.0.0.1:52341",
    
    # Methods
    "param": lambda key: str,        # URL parameters (:id)
    "query": lambda key=None: ...,   # Query parameters
    "header": lambda key: str,       # Request headers
    "body": lambda: str,             # Raw body
    "json": lambda: dict,            # JSON body
    "form": lambda: dict,            # Form data
    "cookie": lambda name: str,      # Cookies
}
```

### Response Format

Handlers return dictionaries:

```starlark
{
    "status": 200,                   # HTTP status code
    "body": "response body",         # Response body
    "headers": {                     # Optional headers
        "Content-Type": "text/plain"
    }
}
```

## Route Patterns

- Exact match: `/users`
- Named parameters: `/users/:id`
- Wildcard: `/files/*filepath`

Priority: Exact > Named > Wildcard

## Implementation Structure

```
web/
├── web.go           # Module registration and configuration
├── server.go        # HTTP server implementation
├── client.go        # HTTP client implementation
├── request.go       # Request object for handlers
├── response.go      # Response helpers
├── router.go        # Route matching logic
└── static.go        # Static file serving
```

## Key Implementation Notes

### 1. Use Standard Library Only

- `net/http` for server and client
- `encoding/json` for JSON handling
- `net/url` for URL parsing
- No external routing libraries

### 2. Thread Safety

- Each request in separate goroutine
- Immutable Starlark values
- Proper mutex usage for shared state

### 3. Resource Management

- Context propagation for cancellation
- Proper body cleanup with `defer`
- Connection pooling in http.Client

### 4. Error Handling

- Panic recovery in handlers
- Starlark fail() for configuration errors
- HTTP error responses for runtime errors

## Example Implementation Pattern

```go
// server.go - Server creation
func makeServer(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    var cfg Config
    // Parse configuration using base package
    
    srv := &Server{
        config: cfg,
        mux:    http.NewServeMux(),
        routes: make(map[string]*route),
    }
    
    return srv, nil
}

// request.go - Request object
type Request struct {
    *http.Request
    params map[string]string
}

func (r *Request) AttrNames() []string {
    return []string{"method", "path", "url", "param", "query", "json", ...}
}
```

## Testing Requirements

1. **Unit Tests** (`web_test.go`)
   - Route matching
   - Request/response parsing
   - Configuration validation

2. **Integration Tests** (`example_test.go`)
   - Full server lifecycle
   - Client-server communication
   - All response helpers
   - Error scenarios

## Security Considerations

- Path traversal prevention in static serving
- Request size limits
- Timeout enforcement
- No directory listing

## Future Considerations

- WebSocket support (separate module)
- Streaming responses
- Middleware API
- HTTP/2 when available in stdlib
