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
- **📊 Request/Response**: Complete HTTP request and response handling with full attribute assignment support
- **🛣️ Flexible Routing**: Support for path parameters and route groups
- **🔧 Middleware Support**: Comprehensive middleware system with compression, rate limiting, caching, and security headers
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
    debug=True
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
    headers = req.headers        # Request headers
    query = req.query            # Query parameters
    
    # Get specific data
    user_id = req.param("id")    # Path parameter
    data = req.json()            # JSON body
    form = req.form()            # Form data
    auth = req.bearer_token()    # Bearer token from Authorization header
    custom_token = req.bearer_token(header="X-API-Token")  # Custom header
    
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

# Redirect
return redirect("/login")

# Error response
return error_response(404, "Not found")

# File response
return send_file("path/to/file.pdf")

# Modify response dynamically
def handler(req):
    resp = json_response({"message": "Hello"})
    resp.status_code = 201
    resp.body = '{"message": "Created"}'
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
def resource_handler(req):
    if req.method == "GET":
        return json_response({"action": "read"})
    elif req.method == "POST":
        return json_response({"action": "create"})
    elif req.method == "PUT":
        return json_response({"action": "update"})
    
srv.route(["GET", "POST", "PUT"], "/resource", resource_handler)
```

## 📊 Request Processing

### JSON Handling

```python
def create_user(req):
    data = req.json()
    if data == None:
        return error_response(400, "Invalid JSON")
    
    name = data.get("name")
    email = data.get("email")
    
    if not name or not email:
        return error_response(400, "Name and email required")
    
    # Process user creation...
    return json_response({"id": 123, "name": name, "email": email})

srv.post("/users", create_user)
```

### Form Data

```python
def upload_handler(req):
    form = req.form()
    title = form.get("title", "Untitled")
    
    files = req.files()
    uploaded_file = files.get("document")
    
    if uploaded_file:
        # Process file...
        return json_response({
            "title": title,
            "filename": uploaded_file.filename,
            "size": uploaded_file.size
        })
    
    return error_response(400, "No file uploaded")

srv.post("/upload", upload_handler)
```

### Authentication

The web module provides built-in authentication helpers that work as middleware to protect routes:

#### API Key Authentication

```python
load("web", "create_server", "api_key_auth", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Create API key authenticator
    api_auth = api_key_auth(
        keys=["secret-key-1", "secret-key-2"],
        header="X-API-Key",                    # Custom header (default: "X-API-Key")
        query_param="api_key"                  # Also check query parameter
    )
    
    def protected_handler(req):
        # User info is automatically added to request context
        user_info = req.context.get("auth_user", {})
        return json_response({
            "message": "Access granted",
            "user_info": user_info
        })
    
    # Apply authentication middleware
    srv.use(api_auth.middleware())
    srv.get("/api/protected", protected_handler)
    
    srv.run()
```

#### Bearer Token Authentication

```python
load("web", "create_server", "bearer_auth", "json_response")

def validate_token(token):
    # Your token validation logic
    if token == "valid-bearer-token":
        return {"user_id": 123, "username": "alice"}
    return None  # Invalid token

def main():
    srv = create_server(port=8080)
    
    # Create bearer token authenticator
    bearer_auth_obj = bearer_auth(
        validate_func=validate_token,
        header="Authorization"                 # Custom header (default: "Authorization")
    )
    
    def protected_handler(req):
        user_info = req.context.get("auth_user", {})
        return json_response({
            "message": "Access granted",
            "user": user_info
        })
    
    srv.use(bearer_auth_obj.middleware())
    srv.get("/api/user", protected_handler)
    
    srv.run()
```

#### Basic Authentication

```python
load("web", "create_server", "basic_auth", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Create basic authenticator
    basic_auth_obj = basic_auth(
        users={"admin": "secret123", "user": "password"},
        realm="Admin Area"
    )
    
    def admin_handler(req):
        user_info = req.context.get("auth_user", {})
        return json_response({
            "message": "Admin access granted",
            "username": user_info.get("username")
        })
    
    srv.use(basic_auth_obj.middleware())
    srv.get("/admin", admin_handler)
    
    srv.run()
```

#### Manual Authentication

For more control, you can manually extract authentication data:

```python
def protected_handler(req):
    # Bearer token authentication - multiple approaches
    
    # Standard Authorization header (expects "Bearer <token>")
    auth_token = req.bearer_token()
    
    # Custom header with Bearer prefix (expects "Bearer <token>")
    custom_bearer = req.bearer_token(header="X-Auth-Token")
    
    # Custom header with direct token (uses value as-is)
    api_key = req.bearer_token(header="X-API-Key")
    
    if not auth_token and not api_key:
        return error_response(401, "Authentication required")
    
    # Basic authentication
    auth = req.basic_auth()
    if auth:
        username, password = auth
        # Validate credentials...
    
    # Custom header authentication
    api_key = req.get_header("X-API-Key")
    if not api_key:
        return error_response(401, "API key required")
    
    return json_response({"message": "Access granted"})

srv.get("/protected", protected_handler)
```

#### Path-Specific Authentication

```python
# Apply authentication only to specific routes
srv.use_for("/api/*", api_auth.middleware())
srv.use_for("/admin/*", basic_auth_obj.middleware())

# Public routes (no authentication)
srv.get("/", public_handler)
srv.get("/health", health_check)

# Protected API routes
srv.get("/api/data", protected_api_handler)

# Protected admin routes  
srv.get("/admin/users", admin_users_handler)
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
```

#### Rate Limiting Middleware

```python
load("web", "create_server", "rate_limit_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default rate limiting (100 requests per 60 seconds)
    rate_limit_mw = rate_limit_middleware()
    srv.use(rate_limit_mw)
    
    # Custom rate limiting
    api_rate_limit = rate_limit_middleware(
        requests=50,     # 50 requests
        window=60,       # per 60 seconds
        key_func=lambda req: req.get_header("X-API-Key") or req.client_ip
    )
    
    # Apply to specific routes
    srv.use_for("/api/*", api_rate_limit)
    
    srv.get("/api/endpoint", lambda req: json_response({"status": "ok"}))
    srv.run()
```

#### Cache Middleware

```python
load("web", "create_server", "cache_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Public cache for static resources
    static_cache = cache_middleware(
        max_age=3600,               # 1 hour
        private=False,              # Public cache
        patterns=["/static/*", "/assets/*"]
    )
    srv.use(static_cache)
    
    # Private cache for user data
    user_cache = cache_middleware(
        max_age=1800,               # 30 minutes
        private=True,               # Private cache
        patterns=["/user/*"],
        vary=["Authorization"]      # Vary by auth header
    )
    srv.use(user_cache)
    
    srv.get("/static/logo.png", lambda req: send_file("logo.png"))
    srv.get("/user/profile", lambda req: json_response({"user": "data"}))
    srv.run()
```

#### Request Size Middleware

```python
load("web", "create_server", "request_size_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Default limits
    size_limit_mw = request_size_middleware()
    srv.use(size_limit_mw)
    
    # Custom limits
    strict_limits = request_size_middleware(
        max_content_length=1024*1024,  # 1MB
        max_url_length=2048,           # 2KB URL
        max_headers=50                 # 50 headers max
    )
    srv.use(strict_limits)
    
    srv.post("/upload", lambda req: json_response({"size": len(req.body or "")}))
    srv.run()
```

#### Security Headers Middleware

```python
load("web", "create_server", "security_headers_middleware", "response")

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
    
    srv.get("/", lambda req: response("Hello World"))
    srv.run()
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
        format="{method} {path} {status} {duration}ms {client_ip}"
    )
    srv.use(custom_logging)
    
    srv.get("/api/test", lambda req: json_response({"message": "logged"}))
    srv.run()
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
    custom_timing = timing_middleware(header="X-Processing-Time")
    srv.use(custom_timing)
    
    srv.get("/api/slow", lambda req: json_response({"data": "processed"}))
    srv.run()
```

#### JSON Middleware

```python
load("web", "create_server", "json_middleware", "json_response")

def main():
    srv = create_server(port=8080)
    
    # Automatically set JSON content type for json_response
    json_mw = json_middleware()
    srv.use(json_mw)
    
    srv.get("/api/data", lambda req: json_response({"auto": "json"}))
    srv.run()
```

### Custom Middleware

```python
# Custom middleware function
def custom_middleware(req, next):
    # Before request processing
    print("Before: {} {}".format(req.method, req.path))
    
    # Add custom header to request
    req.headers["X-Custom-Header"] = "added"
    
    # Call next middleware/handler
    response = next(req)
    
    # After request processing
    print("After: {} status={}".format(req.path, response.status_code))
    
    # Modify response
    response.set_header("X-Custom-Response", "processed")
    
    return response

# Apply custom middleware
srv.use(custom_middleware)
```

### Middleware Order

Middleware is executed in the order it's registered:

```python
def main():
    srv = create_server(port=8080)
    
    # 1. First - Security headers
    srv.use(security_headers_middleware())
    
    # 2. Second - CORS
    srv.use(cors_middleware())
    
    # 3. Third - Compression
    srv.use(compression_middleware())
    
    # 4. Fourth - Rate limiting
    srv.use(rate_limit_middleware())
    
    # 5. Fifth - Logging
    srv.use(logging_middleware())
    
    # 6. Last - Your route handlers
    srv.get("/", home_handler)
    
    srv.run()
```

### Path-Specific Middleware

```python
# Apply middleware to specific paths
srv.use_for("/api/*", rate_limit_middleware(requests=100))
srv.use_for("/admin/*", basic_auth_middleware())
srv.use_for("/static/*", cache_middleware(max_age=86400))

# Global middleware
srv.use(logging_middleware())
```

## 🔧 Configuration

### Environment Variables

```bash
# Server configuration
export WEB_HOST="0.0.0.0"
export WEB_PORT="8080"
export WEB_READ_TIMEOUT="60"
export WEB_WRITE_TIMEOUT="60"
export WEB_MAX_BODY_SIZE="104857600"  # 100MB

# Security configuration
export WEB_ENABLE_CORS="true"
export WEB_CORS_ORIGINS="https://example.com,https://app.example.com"
export WEB_ENABLE_COMPRESSION="true"
export WEB_DEBUG_MODE="false"
```

### Programmatic Configuration

```python
srv = create_server(
    host="localhost",
    port=8080,
    read_timeout=30,
    write_timeout=30,
    max_body_size=50*1024*1024,  # 50MB
    enable_cors=True,
    debug_mode=False
)
```

## 📁 Static Files

```python
def main():
    srv = create_server(port=8080)
    
    # Serve static files
    srv.static("/static", "./public")
    srv.static("/assets", "./dist/assets")
    
    # Single Page Application support
    srv.spa("/app", "./dist", fallback="index.html")
    
    srv.run()
```

## 🎯 Error Handling

### Custom Error Handlers

```python
def not_found_handler(req):
    return json_response({
        "error": "Not Found",
        "path": req.path,
        "method": req.method
    }, status=404)

def server_error_handler(req):
    return json_response({
        "error": "Internal Server Error",
        "message": "Something went wrong"
    }, status=500)

# Register error handlers
srv.error_handler(404, not_found_handler)
srv.error_handler(500, server_error_handler)
```

### Built-in Error Responses

```python
# Standard error responses
return error_response(400, "Bad request")
return error_response(401, "Unauthorized")
return error_response(403, "Forbidden")
return error_response(404, "Not found")
return error_response(500, "Internal server error")
```

## 🔒 Security Features

### CORS Support

```python
srv = create_server(
    enable_cors=True,
    cors_origins=["https://example.com", "https://app.example.com"]
)
```

### Request Validation

```python
def validate_json(req):
    data = req.json()
    if data == None:
        return error_response(400, "Invalid JSON")
    
    required_fields = ["name", "email"]
    for field in required_fields:
        if not data.get(field):
            return error_response(400, "{} is required".format(field))
    
    return None  # Validation passed

def create_user(req):
    error = validate_json(req)
    if error:
        return error
    
    # Process valid request...
    return json_response({"status": "created"})
```

## 📈 Performance Tips

1. **Use Route Groups**: Organize related routes for better performance
2. **Enable Compression**: Reduces response size for text-based content
3. **Limit Request Size**: Set appropriate `max_body_size` limits
4. **Static File Caching**: Use proper cache headers for static assets
5. **JSON Parsing**: Only parse JSON when needed to avoid overhead

## 🔄 Server Lifecycle

```python
def main():
    srv = create_server(port=8080)
    
    # Register routes
    srv.get("/", home_handler)
    
    # Non-blocking start
    srv.start()
    print("Server started on port 8080")
    
    # Do other work...
    
    # Stop server
    srv.stop()
    
    # Or blocking run (most common)
    # srv.run()

main()
```

## 🔧 Recent Improvements

### Response Object Enhancements

The response object now supports full attribute assignment in Starlark:

```python
def handler(req):
    resp = json_response({"initial": "data"})
    
    # Modify response attributes directly
    resp.status_code = 201
    resp.body = '{"updated": "data"}'
    
    # Set custom headers
    resp.set_header("X-Custom-Header", "value")
    resp.set_header("X-Processing-Time", "123ms")
    
    return resp
```

### Starlark Interface Compliance

All wrapper types now properly implement Starlark interfaces:

- **ServerWrapper**: `starlark.Value`, `starlark.HasAttrs`
- **RequestWrapper**: `starlark.Value`, `starlark.HasAttrs`
- **ResponseWrapper**: `starlark.Value`, `starlark.HasAttrs`, `starlark.HasSetField`
- **RouteGroupWrapper**: `starlark.Value`, `starlark.HasAttrs`
- **MiddlewareWrapper**: `starlark.Value`, `starlark.HasAttrs`
- **AuthenticatorWrapper**: `starlark.Value`, `starlark.HasAttrs`

This ensures proper attribute access and assignment behavior in Starlark scripts.

### Bug Fixes

- ✅ Fixed response body and status code assignment issues
- ✅ Resolved HTTP module parameter compatibility
- ✅ Improved cookie handling in tests
- ✅ Enhanced error handling and debugging output
- ✅ Fixed CORS middleware integration

## 🧪 Testing

The module includes comprehensive tests covering:

- ✅ Basic server functionality
- ✅ HTTP method handling
- ✅ Response builders and response object modification
- ✅ Route groups and parameters
- ✅ Error handling
- ✅ Request/response processing
- ✅ Middleware functionality
- ✅ Authentication systems
- ✅ CORS handling

Run tests:

```bash
go test -v github.com/starpkg/web
```

## 📋 Examples

### RESTful API

```python
load("web", "create_server", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    # In-memory storage
    users = {}
    next_id = [1]
    
    def list_users(req):
        return json_response(list(users.values()))
    
    def get_user(req):
        user_id = req.param("id")
        user = users.get(user_id)
        if not user:
            return error_response(404, "User not found")
        return json_response(user)
    
    def create_user(req):
        data = req.json()
        if not data:
            return error_response(400, "Invalid JSON")
        
        user = {
            "id": str(next_id[0]),
            "name": data.get("name"),
            "email": data.get("email")
        }
        users[user["id"]] = user
        next_id[0] += 1
        
        return json_response(user, status=201)
    
    def update_user(req):
        user_id = req.param("id")
        if user_id not in users:
            return error_response(404, "User not found")
        
        data = req.json()
        if not data:
            return error_response(400, "Invalid JSON")
        
        user = users[user_id]
        user.update(data)
        return json_response(user)
    
    def delete_user(req):
        user_id = req.param("id")
        if user_id not in users:
            return error_response(404, "User not found")
        
        del users[user_id]
        return response("", status=204)
    
    # Register routes
    srv.get("/users", list_users)
    srv.get("/users/{id}", get_user)
    srv.post("/users", create_user)
    srv.put("/users/{id}", update_user)
    srv.delete("/users/{id}", delete_user)
    
    print("API server running on http://localhost:8080")
    srv.run()

main()
```

### File Upload Server

```python
load("web", "create_server", "html_response", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    def upload_form(req):
        html = """
        <html>
            <body>
                <h1>File Upload</h1>
                <form method="post" action="/upload" enctype="multipart/form-data">
                    <input type="file" name="file" required>
                    <button type="submit">Upload</button>
                </form>
            </body>
        </html>
        """
        return html_response(html)
    
    def handle_upload(req):
        files = req.files()
        if not files or "file" not in files:
            return error_response(400, "No file uploaded")
        
        file = files["file"]
        
        # Validate file
        if file.size > 10 * 1024 * 1024:  # 10MB limit
            return error_response(400, "File too large")
        
        # Save file (simplified)
        return json_response({
            "filename": file.filename,
            "size": file.size,
            "content_type": file.content_type
        })
    
    srv.get("/", upload_form)
    srv.post("/upload", handle_upload)
    
    # Serve uploaded files
    srv.static("/uploads", "./uploads")
    
    srv.run()

main()
```

## 📄 API Reference

### Core Functions

- `create_server(host="localhost", port=8080, **config)` - Create HTTP server
- `response(body, status=200, headers={})` - Create basic response
- `json_response(data, status=200, headers={})` - Create JSON response
- `html_response(content, status=200, headers={})` - Create HTML response
- `redirect(location, status=302)` - Create redirect response
- `error_response(status, message="")` - Create error response
- `send_file(filepath, content_type=None)` - Send file response
- `send_data(data, filename, content_type)` - Send data as file

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
- `srv.spa(url_path, directory, fallback)` - Serve SPA
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
- `req.headers` - Request headers
- `req.query` - Query parameters
- `req.body()` - Raw body content
- `req.json()` - Parse JSON body
- `req.form()` - Parse form data
- `req.files()` - Get uploaded files
- `req.param(name)` - Get path parameter
- `req.cookie(name)` - Get cookie value
- `req.get_header(name, default)` - Get header with default
- `req.bearer_token(header="Authorization")` - Extract Bearer token from specified header
- `req.basic_auth()` - Get basic auth credentials

### Response Object

- `resp.status_code` - HTTP status code (assignable)
- `resp.headers` - Response headers
- `resp.body` - Response body (assignable)
- `resp.set_cookie(name, value, **options)` - Set cookie
- `resp.delete_cookie(name, **options)` - Delete cookie
- `resp.set_header(name, value)` - Set response header
- `resp.get_header(name, default)` - Get response header with default

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of the excellent [Gin](https://github.com/gin-gonic/gin) web framework
- Inspired by [Flask](https://flask.palletsprojects.com/) for API design
- Part of the [Starlark](https://github.com/bazelbuild/starlark) ecosystem
