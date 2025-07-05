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
- **🛣️ Flexible Routing**: Support for path parameters and route groups
- **🔧 Middleware Support**: Extensible middleware system for cross-cutting concerns
- **📁 Static Files**: Built-in static file serving capabilities
- **🎯 Error Handling**: Comprehensive error handling and status code management
- **⚙️ Configurable**: Environment-based configuration with sensible defaults

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
    auth = req.bearer_token()    # Bearer token
    
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

```python
def protected_handler(req):
    # Bearer token authentication
    token = req.bearer_token()
    if not token:
        return error_response(401, "Authentication required")
    
    # Basic authentication
    auth = req.basic_auth()
    if auth:
        username, password = auth
        # Validate credentials...
    
    return json_response({"message": "Access granted"})

srv.get("/protected", protected_handler)
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

## 🧪 Testing

The module includes comprehensive tests covering:

- ✅ Basic server functionality
- ✅ HTTP method handling
- ✅ Response builders
- ✅ Route groups and parameters
- ✅ Error handling
- ✅ Request/response processing

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
- `req.bearer_token()` - Extract Bearer token
- `req.basic_auth()` - Get basic auth credentials

### Response Object

- `resp.status_code` - HTTP status code
- `resp.headers` - Response headers
- `resp.body` - Response body
- `resp.set_cookie(name, value, **options)` - Set cookie
- `resp.delete_cookie(name, **options)` - Delete cookie

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of the excellent [Gin](https://github.com/gin-gonic/gin) web framework
- Inspired by [Flask](https://flask.palletsprojects.com/) for API design
- Part of the [Starlark](https://github.com/bazelbuild/starlark) ecosystem
