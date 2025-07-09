# Web Module 🌐

[![Go Reference](https://pkg.go.dev/badge/github.com/1set/starpkg/web.svg)](https://pkg.go.dev/github.com/1set/starpkg/web)
[![Go Report Card](https://goreportcard.com/badge/github.com/1set/starpkg/web)](https://goreportcard.com/report/github.com/1set/starpkg/web)
[![License](https://img.shields.io/github/license/1set/starpkg)](https://github.com/1set/starpkg/blob/main/LICENSE)

Flask-inspired web framework for building server-side applications in Starlark.

## Overview

The `web` module provides a comprehensive web framework for Starlark that enables building modern web applications with:

- **Flask-inspired API**: Familiar routing and request handling patterns
- **Flexible Authentication**: Multiple authentication methods (Basic, Bearer, API Key)
- **Rich Middleware System**: 10+ built-in middleware functions for common tasks
- **Response Builders**: Convenient functions for HTML, JSON, and other response types
- **Route Management**: Support for route groups, path parameters, and HTTP methods
- **Static File Serving**: Built-in static file serving capabilities
- **File Upload Support**: Handle multipart form data and file uploads

## Installation

```bash
go get github.com/1set/starpkg/web
```

## Configuration

The web module supports the following configuration options:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | `string` | `"localhost"` | Server host address |
| `port` | `int` | `8080` | Server port |
| `read_timeout` | `int` | `30` | Read timeout in seconds |
| `write_timeout` | `int` | `30` | Write timeout in seconds |
| `max_header_size` | `int` | `1048576` | Maximum header size in bytes |
| `server_header` | `string` | `"Starlark-Web"` | Server header value |

## Basic Usage

### Creating a Server

```python
load("web", "create_server", "html_response")

def main():
    srv = create_server(port=8080, host="localhost")
    
    def hello(req):
        return html_response("<h1>Hello, World!</h1>")
    
    srv.get("/", hello)
    srv.run()

main()
```

### Route Registration

```python
# HTTP method-specific routes
srv.get("/users", list_users)
srv.post("/users", create_user)
srv.put("/users/{id}", update_user)
srv.delete("/users/{id}", delete_user)

# Multiple methods for same route
srv.route(["GET", "POST"], "/contact", handle_contact)

# Route with path parameters
def user_profile(req):
    user_id = req.param("id")
    return json_response({"user_id": user_id})

srv.get("/users/{id}", user_profile)
```

### Request Handling

```python
def handle_request(req):
    # HTTP method and path
    method = req.method
    path = req.path
    
    # Path parameters
    user_id = req.param("id")
    
    # Query parameters
    page = req.query("page", "1")
    
    # Headers
    auth_header = req.header("Authorization")
    
    # Form data
    form_data = req.form()
    name = form_data.get("name") if form_data else None
    
    # JSON data
    json_data = req.json()
    
    # File uploads
    file_data = req.file("upload")
    
    return json_response({"status": "ok"})
```

## Unified Middleware System

The web module provides a unified middleware system where **all middleware has path patterns**. This provides consistent behavior and powerful routing capabilities.

### Key Concepts

- **Global middleware** uses the pattern `"/*"` to match all paths
- **Path-specific middleware** uses patterns like `"/api/*"` or `"/admin/*"`
- **Exact path middleware** uses patterns like `"/api/upload"`
- **Complex patterns** support wildcards like `"/files/*/download"`
- **Parameter patterns** support Flask-style parameters like `"/users/{id}"`

The `use()` method is simply shorthand for `use_for("/*", middleware)` - both approaches are equivalent.

### Core Middleware Functions

#### 1. Logging Middleware

Logs all HTTP requests with customizable format.

```python
load("web", "logging_middleware")

# Basic logging
srv.use(logging_middleware())

# Custom format
srv.use(logging_middleware(
    format="{remote_addr} - {method} {path} - {status} ({duration}ms) - {user_agent}"
))

# Skip certain paths
srv.use(logging_middleware(
    skip_paths=["/health", "/metrics"]
))
```

**Parameters:**
- `format` (string): Log format template with placeholders:
  - `{remote_addr}`: Client IP address
  - `{method}`: HTTP method
  - `{path}`: Request path
  - `{status}`: Response status code
  - `{duration}`: Request duration in milliseconds
  - `{user_agent}`: User agent string
  - `{referer}`: Referer header
- `skip_paths` (list): List of paths to skip logging
- `output` (string): Log output destination ("stdout", "stderr", or file path)

#### 2. CORS Middleware

Handles Cross-Origin Resource Sharing (CORS) headers.

```python
load("web", "cors_middleware")

# Basic CORS (allows all origins)
srv.use(cors_middleware())

# Custom CORS configuration
srv.use(cors_middleware(
    origins=["https://example.com", "https://app.example.com"],
    methods=["GET", "POST", "PUT", "DELETE"],
    headers=["Content-Type", "Authorization", "X-API-Key"],
    exposed_headers=["X-Total-Count"],
    credentials=True,
    max_age=3600
))
```

**Parameters:**
- `origins` (list): Allowed origin domains (default: ["*"])
- `methods` (list): Allowed HTTP methods (default: ["GET", "POST", "PUT", "DELETE", "OPTIONS"])
- `headers` (list): Allowed request headers (default: ["*"])
- `exposed_headers` (list): Headers exposed to the client (default: [])
- `credentials` (bool): Allow credentials (default: False)
- `max_age` (int): Preflight cache duration in seconds (default: 300)

#### 3. Compression Middleware

Compresses response bodies using gzip.

```python
load("web", "compression_middleware")

# Basic compression
srv.use(compression_middleware())

# Custom compression settings
srv.use(compression_middleware(
    level=6,
    min_size=1024,
    types=["text/html", "application/json", "text/css", "application/javascript"]
))
```

**Parameters:**
- `level` (int): Compression level 1-9 (default: 6)
- `min_size` (int): Minimum response size to compress in bytes (default: 1024)
- `types` (list): MIME types to compress (default: text/* and application/json)

#### 4. Security Headers Middleware

Adds common security headers to responses.

```python
load("web", "security_headers_middleware")

# Basic security headers
srv.use(security_headers_middleware())

# Custom security configuration
srv.use(security_headers_middleware(
    content_type_options="nosniff",
    frame_options="DENY",
    xss_protection="1; mode=block",
    hsts_max_age=31536000,
    hsts_include_subdomains=True,
    content_security_policy="default-src 'self'",
    referrer_policy="strict-origin-when-cross-origin"
))
```

**Parameters:**
- `content_type_options` (string): X-Content-Type-Options header (default: "nosniff")
- `frame_options` (string): X-Frame-Options header (default: "DENY")
- `xss_protection` (string): X-XSS-Protection header (default: "1; mode=block")
- `hsts_max_age` (int): HSTS max-age in seconds (default: 31536000)
- `hsts_include_subdomains` (bool): Include subdomains in HSTS (default: False)
- `content_security_policy` (string): Content-Security-Policy header (default: None)
- `referrer_policy` (string): Referrer-Policy header (default: "strict-origin-when-cross-origin")

#### 5. Rate Limiting Middleware

Limits request rate per client IP address.

```python
load("web", "rate_limiting_middleware")

# Basic rate limiting (60 requests per minute)
srv.use(rate_limiting_middleware())

# Custom rate limiting
srv.use(rate_limiting_middleware(
    requests_per_minute=100,
    burst_size=10,
    window_size=60,
    key_func=lambda req: req.header("X-API-Key") or req.remote_addr
))
```

**Parameters:**
- `requests_per_minute` (int): Maximum requests per minute (default: 60)
- `burst_size` (int): Maximum burst requests (default: 10)
- `window_size` (int): Time window in seconds (default: 60)
- `key_func` (function): Function to generate rate limit key (default: uses IP address)
- `skip_successful` (bool): Skip rate limiting for successful requests (default: False)
- `headers` (bool): Include rate limit headers in response (default: True)

#### 6. Timing Middleware

Adds response timing information.

```python
load("web", "timing_middleware")

# Basic timing
srv.use(timing_middleware())

# Custom timing configuration
srv.use(timing_middleware(
    header="X-Response-Time",
    unit="ms",
    precision=2
))
```

**Parameters:**
- `header` (string): Response header name (default: "X-Response-Time")
- `unit` (string): Time unit ("ms", "s", "us") (default: "ms")
- `precision` (int): Decimal precision for timing (default: 2)

#### 7. Request Size Middleware

Limits request body and header sizes.

```python
load("web", "request_size_middleware")

# Basic size limits
srv.use(request_size_middleware())

# Custom size limits
srv.use(request_size_middleware(
    max_body_size=10485760,  # 10MB
    max_header_size=8192,    # 8KB
    max_url_length=2048,     # 2KB
    max_headers_count=50
))
```

**Parameters:**
- `max_body_size` (int): Maximum request body size in bytes (default: 1048576)
- `max_header_size` (int): Maximum header size in bytes (default: 4096)
- `max_url_length` (int): Maximum URL length (default: 2048)
- `max_headers_count` (int): Maximum number of headers (default: 50)

#### 8. Recovery Middleware

Recovers from panics and returns error responses.

```python
load("web", "recovery_middleware")

# Basic recovery
srv.use(recovery_middleware())

# Custom recovery configuration
srv.use(recovery_middleware(
    log_panics=True,
    debug_mode=False,
    recovery_handler=lambda req, err: error_response(500, "Internal Server Error")
))
```

**Parameters:**
- `log_panics` (bool): Log panic details (default: True)
- `debug_mode` (bool): Include stack trace in response (default: False)
- `recovery_handler` (function): Custom recovery handler function

#### 9. Caching Middleware

Adds HTTP caching headers to responses.

```python
load("web", "caching_middleware")

# Basic caching
srv.use(caching_middleware())

# Custom caching configuration
srv.use(caching_middleware(
    max_age=3600,
    private=False,
    no_cache=False,
    no_store=False,
    must_revalidate=False,
    etag_func=lambda req: generate_etag(req.path)
))
```

**Parameters:**
- `max_age` (int): Cache max-age in seconds (default: 3600)
- `private` (bool): Set cache as private (default: False)
- `no_cache` (bool): Set no-cache directive (default: False)
- `no_store` (bool): Set no-store directive (default: False)
- `must_revalidate` (bool): Set must-revalidate directive (default: False)
- `etag_func` (function): Function to generate ETag values

#### 10. Unified Middleware Application

Apply middleware using path patterns. All middleware functions in the unified system require a path pattern.

```python
# Global middleware (applies to all routes using "/*" pattern)
srv.use(logging_middleware())  # Equivalent to srv.use_for("/*", logging_middleware())
srv.use(cors_middleware())

# Path-specific middleware
srv.use_for("/api/*", rate_limit_middleware())
srv.use_for("/admin/*", security_headers_middleware())

# Exact path middleware
srv.use_for("/api/upload", request_size_middleware())

# Complex wildcard patterns
srv.use_for("/files/*/download", security_headers_middleware())

# Parameter patterns
srv.use_for("/users/{id}", timing_middleware())
srv.use_for("/users/{id}/posts/{post_id}", compression_middleware())
```

### Custom Middleware

Create custom middleware functions:

```python
def custom_middleware():
    def middleware(req):
        # Pre-processing
        req.set_header("X-Custom-Header", "MyValue")
        
        # Return None to continue to next middleware/handler
        # Return a response to short-circuit the chain
        return None
    
    return middleware

srv.use(custom_middleware())
```

## Authentication System

The web module supports multiple authentication methods that can be used independently or combined.

### Basic Authentication

HTTP Basic Authentication with username/password.

```python
load("web", "basic_auth")

# Create basic auth with users
auth = basic_auth(
    users={"admin": "secret", "user": "password"},
    realm="Protected Area"
)

# Apply to specific routes
srv.use_for("/admin/*", auth.middleware())

# Access auth info in handlers
def protected_handler(req):
    basic_info = req.basic_auth()
    if basic_info:
        username, password = basic_info
        return html_response("Welcome, {}!".format(username))
    return error_response(401, "Unauthorized")
```

### Bearer Token Authentication

Token-based authentication using Authorization header.

```python
load("web", "bearer_auth")

# Create bearer auth with tokens
auth = bearer_auth(
    tokens={"token123": "admin", "token456": "user"},
    realm="API Access"
)

# Apply to API routes
srv.use_for("/api/*", auth.middleware())

# Access token info in handlers
def api_handler(req):
    token_info = req.bearer_token()
    if token_info:
        return json_response({"user": token_info})
    return error_response(401, "Invalid token")
```

### API Key Authentication

API key authentication using custom headers.

```python
load("web", "api_key_auth")

# Create API key auth
auth = api_key_auth(
    api_keys={"key123": "admin", "key456": "user"},
    header="X-API-Key"
)

# Apply to API endpoints
srv.use_for("/api/*", auth.middleware())

# Access API key info in handlers
def api_handler(req):
    api_key_info = req.api_key()
    if api_key_info:
        return json_response({"user": api_key_info})
    return error_response(401, "Invalid API key")
```

## Response Builders

Convenient functions for creating different types of responses.

### HTML Response

```python
load("web", "html_response")

def home(req):
    return html_response("""
    <!DOCTYPE html>
    <html>
    <head><title>My App</title></head>
    <body><h1>Welcome!</h1></body>
    </html>
    """)

# With custom status code
def not_found(req):
    return html_response("<h1>Not Found</h1>", status=404)

# With custom headers
def custom_html(req):
    return html_response(
        "<h1>Custom</h1>",
        headers={"X-Custom": "value"}
    )
```

### JSON Response

```python
load("web", "json_response")

def api_users(req):
    return json_response({
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ]
    })

# With custom status code
def api_create_user(req):
    return json_response(
        {"id": 123, "name": "New User"},
        status=201
    )
```

### Error Response

```python
load("web", "error_response")

def handle_error(req):
    return error_response(400, "Bad Request")

# With custom error details
def detailed_error(req):
    return error_response(
        422,
        "Validation Error",
        details={"field": "email", "message": "Invalid email format"}
    )
```

### Redirect Response

```python
load("web", "redirect")

def old_page(req):
    return redirect("/new-page")

# With custom status code
def permanent_redirect(req):
    return redirect("/new-location", status=301)
```

### File Response

```python
load("web", "file_response")

def download_file(req):
    return file_response(
        "/path/to/file.pdf",
        filename="document.pdf",
        content_type="application/pdf"
    )
```

## Static File Serving

Serve static files from directories:

```python
# Serve static files from a directory
srv.static("/static", "./public")

# Serve single file
srv.static_file("/favicon.ico", "./assets/favicon.ico")

# With custom middleware for static files
srv.use_for("/static/*", [
    caching_middleware(max_age=86400),  # 24 hours
    compression_middleware()
])
```

## Complete Example

Here's a complete example demonstrating the unified middleware system:

```python
load("web", "create_server", "html_response", "json_response", "error_response", 
     "basic_auth", "cors_middleware", "logging_middleware", "compression_middleware", 
     "security_headers_middleware")

def main():
    srv = create_server(port=8080, server_header="MyApp/1.0")
    
    # Authentication
    auth = basic_auth(users={"admin": "secret"}, realm="Admin")
    
    # Routes
    def home(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head><title>My App</title></head>
            <body>
            <h1>Welcome!</h1>
            <p><a href="/api/status">API Status</a></p>
            <p><a href="/admin">Admin Area</a></p>
            </body>
        </html>
        """)
    
    def admin_area(req):
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info else "unknown"
        return html_response("<h1>Admin: {}</h1>".format(username))
    
    def api_status(req):
        return json_response({"status": "ok", "version": "1.0"})
    
    # Register routes
    srv.get("/", home)
    srv.get("/api/status", api_status)
    srv.get("/admin", admin_area)
    
    # ========================================
    # UNIFIED MIDDLEWARE SYSTEM
    # ========================================
    # All middleware now has path patterns
    
    # Global middleware (applies to all routes using "/*" pattern)
    srv.use(logging_middleware())  # Same as srv.use_for("/*", logging_middleware())
    srv.use(cors_middleware())
    
    # API-specific middleware (applies only to /api/* routes)
    srv.use_for("/api/*", compression_middleware())
    srv.use_for("/api/*", security_headers_middleware())
    
    # Admin-specific middleware (applies only to /admin/* routes)
    srv.use_for("/admin/*", auth.middleware())  # Authentication required
    srv.use_for("/admin/*", security_headers_middleware())
    
    # Static files
    srv.static("/static", "./public")
    
    print("Server running on http://localhost:8080")
    print("Features demonstrated:")
    print("- Global middleware using /* pattern")
    print("- Path-specific middleware for API and admin routes")
    print("- Authentication protection for admin area")
    print("- Multiple middleware layers combining correctly")
    srv.run()

main()
```

## API Reference

### Server Methods

- `create_server(**config)`: Create a new server instance
- `srv.get(path, handler)`: Register GET route
- `srv.post(path, handler)`: Register POST route
- `srv.put(path, handler)`: Register PUT route
- `srv.delete(path, handler)`: Register DELETE route
- `srv.route(methods, path, handler)`: Register route with multiple methods
- `srv.use(middleware)`: Apply global middleware
- `srv.use_for(path, middleware)`: Apply middleware to specific paths
- `srv.static(path, directory)`: Serve static files
- `srv.static_file(path, file)`: Serve single static file
- `srv.run()`: Start the server

### Request Methods

- `req.method`: HTTP method
- `req.path`: Request path
- `req.remote_addr`: Client IP address
- `req.param(name)`: Get path parameter
- `req.query(name, default=None)`: Get query parameter
- `req.header(name)`: Get header value
- `req.form()`: Get form data as dict
- `req.json()`: Parse JSON body
- `req.file(name)`: Get uploaded file
- `req.basic_auth()`: Get basic auth credentials
- `req.bearer_token()`: Get bearer token
- `req.api_key()`: Get API key

### Response Functions

- `html_response(content, status=200, headers=None)`: Create HTML response
- `json_response(data, status=200, headers=None)`: Create JSON response
- `error_response(status, message, details=None)`: Create error response
- `redirect(url, status=302)`: Create redirect response
- `file_response(path, filename=None, content_type=None)`: Create file response

### Authentication Functions

- `basic_auth(users, realm="Restricted")`: Create basic authentication
- `bearer_auth(tokens, realm="API")`: Create bearer token authentication
- `api_key_auth(api_keys, header="X-API-Key")`: Create API key authentication

### Middleware Functions

- `logging_middleware(**options)`: Request logging
- `cors_middleware(**options)`: CORS headers
- `compression_middleware(**options)`: Response compression
- `security_headers_middleware(**options)`: Security headers
- `rate_limiting_middleware(**options)`: Rate limiting
- `timing_middleware(**options)`: Response timing
- `request_size_middleware(**options)`: Request size limits
- `recovery_middleware(**options)`: Panic recovery
- `caching_middleware(**options)`: HTTP caching headers

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
