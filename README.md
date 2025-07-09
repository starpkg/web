# 🌐 Web Module

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/web.svg)](https://pkg.go.dev/github.com/starpkg/web)
[![Go Report Card](https://goreportcard.com/badge/github.com/starpkg/web)](https://goreportcard.com/report/github.com/starpkg/web)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Fast and intuitive server-side web framework for Starlark**

Build modern web applications with Flask-like simplicity and Go performance. The `web` module provides a high-performance, Flask-inspired web server framework for Starlark scripts, focusing exclusively on server-side functionality.

## ✨ Features

- **🚀 High Performance**: Built on Gin framework with Go's performance
- **🐍 Flask-Inspired**: Familiar API patterns adapted for Starlark
- **🔒 Security First**: Built-in protection against common vulnerabilities
- **📊 Request/Response**: Complete HTTP request and response handling
- **🛣️ Flexible Routing**: Support for path parameters, route groups, and multiple HTTP methods
- **🔧 Middleware Support**: Comprehensive middleware system with CORS, logging, compression, rate limiting, and more
- **🔐 Authentication**: Built-in API key, bearer token, and basic authentication support
- **📁 Static Files**: Built-in static file serving capabilities
- **🎯 Error Handling**: Comprehensive error handling and status code management
- **⚙️ Configurable**: Environment-based configuration with sensible defaults
- **🔧 Starlark Integration**: Full Starlark interface compliance for all wrapper types

## 🏗️ Architecture

The web module is organized into several focused components:

```
web/
├── web.go          # Module initialization and main entry point
├── server.go       # Server lifecycle, routing, request handling
├── request.go      # Request handling and data extraction
├── response.go     # Response handling and data formatting
├── router.go       # Routing and middleware functionality
├── middleware.go   # Built-in middleware implementations
├── auth.go         # Authentication systems
├── utils.go        # Shared utilities and helper functions
└── README.md       # This documentation
```

## 📦 Installation

```bash
go get github.com/starpkg/web
```

## 🚀 Quick Start

### Basic Web Server

```python
load("web", "create_server", "response")

def main():
    srv = create_server(port=8080)
    
    def hello(req):
        return response("Hello, World!")
    
    srv.get("/", hello)
    srv.run()

main()
```

### JSON API

```python
load("web", "create_server", "json_response")

def main():
    srv = create_server(port=8080)
    
    def api_info(req):
        return json_response({
            "name": "My API",
            "version": "1.0",
            "method": req.method,
            "path": req.path
        })
    
    srv.get("/api/info", api_info)
    srv.run()

main()
```

## 📚 Core Concepts

### Server Creation

```python
# Basic server
srv = create_server()

# Configured server
srv = create_server(
    host="0.0.0.0",
    port=8080,
    server_header="My-Server/1.0"
)
```

### Route Registration

```python
# Method-specific routes
srv.get("/users", list_users)
srv.post("/users", create_user)
srv.put("/users/{id}", update_user)
srv.delete("/users/{id}", delete_user)

# Multiple methods for same route
srv.route(["GET", "HEAD"], "/api/info", api_info)
```

### Request Handling

```python
def handler(req):
    # Access request properties
    method = req.method           # HTTP method
    path = req.path              # URL path
    headers = req.headers        # Request headers dict
    query = req.query            # Query parameters dict
    
    # Get specific data
    user_id = req.param("id")    # Path parameter
    data = req.json()            # JSON body
    form = req.form()            # Form data
    bearer = req.bearer_token()  # Bearer token from Authorization header
    basic = req.basic_auth()     # Basic auth tuple (username, password)
    
    # Get headers with defaults
    user_agent = req.get_header("User-Agent", "Unknown")
    
    return response("OK")
```

### Response Building

```python
# Text response
return response("Hello, World!")

# JSON response
return json_response({"status": "ok"})

# HTML response
return html_response("<h1>Welcome</h1>")

# Text response with explicit content type
return text_response("Plain text content")

# File response
return file_response("path/to/file.pdf")

# File response with custom filename
return file_response("path/to/file.pdf", filename="download.pdf")

# Data response (for downloads)
return send_data("file content", "filename.txt", "text/plain")

# Redirect
return redirect("/login")

# Error response
return error_response(404, "Not found")

# Modify response headers
def handler(req):
    resp = json_response({"message": "Hello"})
    resp.set_header("X-Custom", "value")
    return resp
```

## 🛣️ Advanced Routing

### Path Parameters

```python
def get_user(req):
    user_id = req.param("id")
    return json_response({"user_id": user_id})

srv.get("/users/{id}", get_user)
```

### Route Groups

```python
# Create API group
api = srv.group("/api/v1")

def list_users(req):
    return json_response([{"id": 1, "name": "Alice"}])

def get_user(req):
    user_id = req.param("id")
    return json_response({"id": user_id})

# Register routes in group
api.get("/users", list_users)
api.get("/users/{id}", get_user)
```

### Multiple HTTP Methods

```python
# Handle multiple methods with same handler
def api_info(req):
    return json_response({
        "method": req.method,
        "timestamp": now().format("2006-01-02T15:04:05Z")
    })

srv.route(["GET", "POST"], "/api/info", api_info)
```

## 🔧 Middleware System

The web module provides a comprehensive middleware system for cross-cutting concerns. Middleware functions are executed in order before reaching route handlers.

### Built-in Middleware

#### CORS Middleware

```python
load("web", "create_server", "cors_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Basic CORS with defaults
    cors_mw = cors_middleware()
    srv.use(cors_mw)
    
    # Custom CORS configuration
    custom_cors = cors_middleware(
        origins=["https://example.com", "https://app.example.com"],
        methods=["GET", "POST", "PUT", "DELETE"],
        headers=["Content-Type", "Authorization", "X-Custom-Header"],
        credentials=True
    )
    srv.use(custom_cors)
    
    srv.get("/api/data", lambda req: json_response({"data": "cors enabled"}))
    srv.run()

main()
```

#### Compression Middleware

```python
load("web", "create_server", "compression_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default compression (gzip, level 6, min size 1KB)
    compression_mw = compression_middleware()
    srv.use(compression_mw)
    
    # Custom compression settings
    custom_compression = compression_middleware(
        level=9,                    # Maximum compression
        min_size=512,               # Compress files > 512 bytes
        types=["text/html", "application/json", "text/css"]  # Specific types
    )
    srv.use(custom_compression)
    
    def large_response(req):
        # Large response will be automatically compressed
        return json_response({
            "data": ["item_{}".format(i) for i in range(1000)]
        })
    
    srv.get("/api/large", large_response)
    srv.run()

main()
```

#### Rate Limiting Middleware

```python
load("web", "create_server", "rate_limit_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Basic rate limiting (100 requests per 60 seconds)
    rate_limiter = rate_limit_middleware(
        requests=100,
        window=60
    )
    srv.use(rate_limiter)
    
    # Custom rate limiting with key function
    def rate_key(req):
        # Rate limit by API key if present, otherwise by IP
        api_key = req.get_header("X-API-Key")
        if api_key != None:
            return "api_key:" + api_key
        return "ip:" + req.client_ip
    
    custom_rate_limiter = rate_limit_middleware(
        requests=1000,
        window=3600,  # 1 hour
        key_func=rate_key
    )
    srv.use(custom_rate_limiter)
    
    srv.get("/api/data", lambda req: json_response({"data": "rate limited"}))
    srv.run()

main()
```

#### Security Headers Middleware

```python
load("web", "create_server", "security_headers_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default security headers
    security_mw = security_headers_middleware()
    srv.use(security_mw)
    
    # Custom security headers
    custom_security = security_headers_middleware(
        frame_options="SAMEORIGIN",
        content_type_options="nosniff",
        xss_protection="1; mode=block",
        hsts="max-age=31536000; includeSubDomains",
        csp="default-src 'self'",
        referrer_policy="strict-origin-when-cross-origin"
    )
    srv.use(custom_security)
    
    srv.get("/", lambda req: html_response("<h1>Secure Headers</h1>"))
    srv.run()

main()
```

#### Logging Middleware

```python
load("web", "create_server", "logging_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default logging format
    logging_mw = logging_middleware()
    srv.use(logging_mw)
    
    # Custom logging format
    custom_logging = logging_middleware(
        format="{method} {path} {status} {duration}ms - {user_agent}"
    )
    srv.use(custom_logging)
    
    srv.get("/api/test", lambda req: json_response({"test": "logged"}))
    srv.run()

main()
```

#### Timing Middleware

```python
load("web", "create_server", "timing_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Add response time header
    timing_mw = timing_middleware()
    srv.use(timing_mw)
    
    # Custom timing header
    custom_timing = timing_middleware(header="X-Process-Time")
    srv.use(custom_timing)
    
    srv.get("/api/timed", lambda req: json_response({"timed": True}))
    srv.run()

main()
```

#### Request Size Middleware

```python
load("web", "create_server", "request_size_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default limits
    size_mw = request_size_middleware()
    srv.use(size_mw)
    
    # Custom limits
    custom_size = request_size_middleware(
        max_content_length=5 * 1024 * 1024,  # 5MB
        max_url_length=1024,                 # 1KB
        max_headers=50                       # 50 headers
    )
    srv.use(custom_size)
    
    def upload_handler(req):
        data = req.json()
        return json_response({"received": len(str(data))})
    
    srv.post("/api/upload", upload_handler)
    srv.run()

main()
```

#### Cache Middleware

```python
load("web", "create_server", "cache_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Basic caching
    cache_mw = cache_middleware()
    srv.use(cache_mw)
    
    # Custom caching with patterns
    custom_cache = cache_middleware(
        max_age=7200,        # 2 hours
        private=False,       # Public cache
        patterns=["/api/static/*", "/images/*"],
        vary=["Accept-Encoding", "User-Agent"]
    )
    srv.use(custom_cache)
    
    srv.get("/api/static/data", lambda req: json_response({"cached": True}))
    srv.run()

main()
```

### Custom Middleware

```python
def custom_middleware(req, next_handler):
    # Pre-processing
    print("Processing request to: {}".format(req.path))
    
    # Call next handler
    resp = next_handler(req)
    
    # Post-processing
    resp.set_header("X-Processed-By", "Custom-Middleware")
    
    return resp

# Use custom middleware
srv.use(custom_middleware)
```

### Path-Specific Middleware

```python
# Apply middleware only to specific paths
srv.use_for("/api/*", auth_middleware)
srv.use_for("/admin/*", admin_middleware)
```

## 🔐 Authentication

The web module provides built-in authentication systems for securing your APIs.

### Basic Authentication

```python
load("web", "create_server", "basic_auth", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Create basic auth
    auth = basic_auth(
        users={"admin": "password123", "user": "secret"},
        realm="Protected Area"
    )
    
    # Apply to specific routes
    srv.use_for("/protected/*", auth.middleware())
    
    def protected_data(req):
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        return json_response({"user": username, "data": "secret"})
    
    srv.get("/protected/data", protected_data)
    srv.run()

main()
```

### Bearer Token Authentication

```python
load("web", "create_server", "bearer_auth", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    # Token validation function
    def validate_token(token):
        # In real app, validate against database or JWT
        valid_tokens = {"abc123": {"user": "admin"}, "def456": {"user": "user"}}
        return valid_tokens.get(token)
    
    # Create bearer auth
    auth = bearer_auth(validate_func=validate_token)
    
    # Apply to API routes
    srv.use_for("/api/*", auth.middleware())
    
    def api_data(req):
        token = req.bearer_token()
        user_info = validate_token(token) if token != None else None
        return json_response({"user": user_info, "data": "protected"})
    
    srv.get("/api/data", api_data)
    srv.run()

main()
```

### API Key Authentication

```python
load("web", "create_server", "api_key_auth", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Create API key auth
    auth = api_key_auth(
        keys=["key123", "key456", "key789"],
        header="X-API-Key",
        query_param="api_key"
    )
    
    # Apply to API routes
    srv.use_for("/api/*", auth.middleware())
    
    def api_data(req):
        api_key = req.get_header("X-API-Key")
        return json_response({"api_key": api_key, "data": "authenticated"})
    
    srv.get("/api/data", api_data)
    srv.run()

main()
```

## 📁 Static Files

Serve static files like CSS, JavaScript, and images:

```python
load("web", "create_server", "html_response")

def main():
    srv = create_server(port=8080)
    
    # Serve static files from ./static directory at /static/* URL
    srv.static("/static", "./static")
    
    # Serve files from ./assets directory at /assets/* URL
    srv.static("/assets", "./assets")
    
    def home(req):
        return html_response("""
        <html>
            <head>
                <link rel="stylesheet" href="/static/style.css">
            </head>
            <body>
                <h1>Welcome</h1>
                <script src="/static/app.js"></script>
            </body>
        </html>
        """)
    
    srv.get("/", home)
    srv.run()

main()
```

## 🎯 Error Handling

### Custom Error Handlers

```python
load("web", "create_server", "json_response", "html_response")

def main():
    srv = create_server(port=8080)
    
    # Custom 404 handler
    def not_found(req):
        return html_response("""
        <html>
            <body>
                <h1>404 - Page Not Found</h1>
                <p>The page {} was not found.</p>
            </body>
        </html>
        """.format(req.path), status=404)
    
    # Custom 500 handler
    def server_error(req):
        return json_response({
            "error": "Internal Server Error",
            "message": "Something went wrong"
        }, status=500)
    
    # Register error handlers
    srv.error_handler(404, not_found)
    srv.error_handler(500, server_error)
    
    # Route that triggers 500 error
    def broken(req):
        fail("Intentional error")
    
    srv.get("/broken", broken)
    srv.run()

main()
```

## 🔧 Configuration

Configure the web server using environment variables or parameters:

```python
load("web", "create_server", "response")

def main():
    # Configuration via parameters
    srv = create_server(
        host="0.0.0.0",
        port=8080,
        server_header="MyApp/1.0"
    )
    
    # Configuration is also available via environment variables:
    # web_host=0.0.0.0
    # web_port=8080
    # web_read_timeout=30
    # web_write_timeout=30
    # web_max_body_size=33554432
    # web_debug_mode=false
    # web_server_header=Starlark-Web/1.0
    
    srv.get("/", lambda req: response("Configured server"))
    srv.run()

main()
```

## 📖 Complete Example

Here's a complete example showing multiple features:

```python
load("web", "create_server", "json_response", "html_response", "error_response",
     "basic_auth", "cors_middleware", "logging_middleware", "compression_middleware")
load("time", "now")

def main():
    srv = create_server(port=8080)
    
    # Add middleware
    srv.use(logging_middleware())
    srv.use(cors_middleware())
    srv.use(compression_middleware())
    
    # Create authentication
    auth = basic_auth(users={"admin": "secret"}, realm="Admin")
    
    # In-memory data store
    todos = []
    next_id = [1]
    
    # Public routes
    def home(req):
        return html_response("""
        <html>
            <body>
                <h1>Todo API</h1>
                <p>Endpoints:</p>
                <ul>
                    <li>GET /api/todos - List todos</li>
                    <li>POST /api/todos - Create todo</li>
                    <li>GET /admin/stats - Admin stats</li>
                </ul>
            </body>
        </html>
        """)
    
    def list_todos(req):
        return json_response(todos)
    
    def create_todo(req):
        data = req.json()
        if data == None or data.get("title") == None:
            return error_response(400, "Title is required")
        
        todo = {
            "id": next_id[0],
            "title": data["title"],
            "completed": False,
            "created": now().format("2006-01-02T15:04:05Z")
        }
        todos.append(todo)
        next_id[0] = next_id[0] + 1
        
        return json_response(todo, status=201)
    
    # Protected admin route
    def admin_stats(req):
        return json_response({
            "total_todos": len(todos),
            "completed": len([t for t in todos if t.get("completed", False)]),
            "pending": len([t for t in todos if not t.get("completed", False)])
        })
    
    # Register routes
    srv.get("/", home)
    srv.get("/api/todos", list_todos)
    srv.post("/api/todos", create_todo)
    
    # Protected routes
    srv.use_for("/admin/*", auth.middleware())
    srv.get("/admin/stats", admin_stats)
    
    print("Todo API running on http://localhost:8080")
    print("Admin credentials: admin/secret")
    srv.run()

main()
```

## 📋 API Reference

### Core Functions

- `create_server(host="localhost", port=8080, server_header="")` - Create HTTP server
- `response(body, status=200, headers={})` - Create basic response
- `json_response(data, status=200, headers={})` - Create JSON response
- `html_response(content, status=200, headers={})` - Create HTML response
- `text_response(text, status=200)` - Create text response
- `file_response(filepath, content_type="", filename="")` - Create file response
- `redirect(location, status=302)` - Create redirect response
- `error_response(status, message="")` - Create error response
- `send_file(filepath, content_type="")` - Send file response
- `send_data(data, filename, content_type="application/octet-stream")` - Send data response

### Authentication Functions

- `basic_auth(users={}, realm="Restricted")` - Basic HTTP authentication
- `bearer_auth(validate_func, header="Authorization")` - Bearer token authentication
- `api_key_auth(keys=[], header="X-API-Key", query_param="api_key")` - API key authentication

### Middleware Functions

- `cors_middleware(origins=[], methods=[], headers=[], credentials=False)` - CORS support
- `compression_middleware(level=6, min_size=1024, types=[])` - Response compression
- `rate_limit_middleware(requests=100, window=60, key_func=None)` - Rate limiting
- `cache_middleware(max_age=3600, private=False, patterns=[], vary=[])` - Response caching
- `request_size_middleware(max_content_length=10MB, max_url_length=2048, max_headers=100)` - Request size limits
- `security_headers_middleware(**headers)` - Security headers
- `logging_middleware(format="")` - Request logging
- `timing_middleware(header="X-Response-Time")` - Response timing
- `json_middleware()` - JSON content type handling

### Server Methods

- `srv.get(path, handler)` - Register GET route
- `srv.post(path, handler)` - Register POST route
- `srv.put(path, handler)` - Register PUT route
- `srv.delete(path, handler)` - Register DELETE route
- `srv.patch(path, handler)` - Register PATCH route
- `srv.options(path, handler)` - Register OPTIONS route
- `srv.head(path, handler)` - Register HEAD route
- `srv.route(methods, path, handler)` - Register multi-method route
- `srv.group(prefix)` - Create route group
- `srv.static(url_path, directory)` - Serve static files
- `srv.use(middleware)` - Add global middleware
- `srv.use_for(path_pattern, middleware)` - Add path-specific middleware
- `srv.error_handler(status_codes, handler)` - Register error handler
- `srv.start()` - Start server (non-blocking)
- `srv.run()` - Start server (blocking)
- `srv.stop()` - Stop server
- `srv.is_running()` - Check if server is running

### Request Object

- `req.method` - HTTP method
- `req.url` - Full URL
- `req.path` - URL path
- `req.host` - Host header
- `req.remote` - Remote address
- `req.client_ip` - Client IP address
- `req.proto` - HTTP protocol
- `req.headers` - Headers dict
- `req.query` - Query parameters dict
- `req.body()` - Raw body content
- `req.json()` - Parse JSON body
- `req.form()` - Parse form data
- `req.files()` - Uploaded files
- `req.param(name)` - Get path parameter
- `req.get_header(name, default=None)` - Get header with default
- `req.bearer_token(header="Authorization")` - Get bearer token
- `req.basic_auth()` - Get basic auth tuple
- `req.cookie(name)` - Get cookie value

### Response Object

- `resp.status_code` - HTTP status code
- `resp.headers` - Headers dict
- `resp.body` - Response body
- `resp.set_header(name, value)` - Set header
- `resp.get_header(name, default=None)` - Get header with default

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Built on top of the [Gin Web Framework](https://github.com/gin-gonic/gin)
- Inspired by [Flask](https://flask.palletsprojects.com/) for Python
- Part of the [Starlark Package Collection](https://github.com/starpkg)
