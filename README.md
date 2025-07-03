# 🌐 Web Module

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/starpkg/web)](https://goreportcard.com/report/github.com/starpkg/web)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Fast and intuitive server-side web framework for Starlark**

Build modern web applications with Flask-like simplicity and Go performance. The `web` module provides a high-performance, Flask-inspired web server framework exclusively for server-side functionality, complementing the existing `http` client module.

## Features

🚀 **High Performance** - Built on Go's `net/http` for production-ready performance  
🛡️ **Security by Default** - Built-in protection against common vulnerabilities  
🔧 **Flask-inspired API** - Familiar patterns adapted for Starlark constraints  
⚡ **Zero Dependencies** - Uses only Go standard library for maximum reliability  
🔌 **Middleware System** - Flexible middleware pipeline with built-in middleware  
🍪 **Session Management** - Secure session handling with encryption  
🔐 **Authentication** - Built-in Basic, Bearer, and API key authentication  
📁 **Static Files** - Efficient static file serving with caching  
🎯 **Path Parameters** - Express-style route parameters and wildcards  

## Quick Start

```python
load("web", "create_server", "response", "json_response")

def main():
    # Create server
    srv = create_server(host="localhost", port=8080)
    
    # Simple route
    def home(req):
        return response("Welcome to Starlark Web!")
    
    # JSON API
    def api_info(req):
        return json_response({
            "name": "My API",
            "version": "1.0",
            "method": req.method,
            "path": req.path,
        })
    
    # Register routes
    srv.get("/", home)
    srv.get("/api/info", api_info)
    
    # Start server
    srv.run()

main()
```

## Core Functions

### Server Creation

#### `create_server(host="localhost", port=8080, **config) -> Server`

Creates a new HTTP server instance with the specified configuration.

```python
# Basic server
srv = create_server(port=8080)

# Custom configuration
srv = create_server(
    host="0.0.0.0",
    port=3000
)
```

### Response Builders

#### `response(body, status=200, headers={}) -> Response`

Creates a basic HTTP response.

```python
def handler(req):
    return response("Hello, World!")

def custom_response(req):
    return response(
        "Custom content",
        status=201,
        headers={"X-Custom": "value"}
    )
```

#### `json_response(data, status=200, headers={}) -> Response`

Creates a JSON HTTP response.

```python
def api_handler(req):
    return json_response({
        "message": "Success",
        "data": [1, 2, 3],
        "user": req.context.get("user")
    })
```

#### `html_response(content, status=200, headers={}) -> Response`

Creates an HTML HTTP response.

```python
def page_handler(req):
    html = """
    <html>
        <body><h1>Welcome!</h1></body>
    </html>
    """
    return html_response(html)
```

#### `redirect(location, status=302) -> Response`

Creates a redirect response.

```python
def old_page(req):
    return redirect("/new-page", status=301)
```

#### `error_response(status, message="") -> Response`

Creates an error response.

```python
def protected_handler(req):
    if not req.context.get("user"):
        return error_response(401, "Authentication required")
    return response("Protected content")
```

### File Operations

#### `send_file(filepath, content_type=None) -> Response`

Sends a file from the filesystem.

```python
def download_handler(req):
    filename = req.param("filename")
    return send_file("files/{}.format(filename)")
```

#### `send_data(data, filename, content_type="application/octet-stream") -> Response`

Sends raw data as a file download.

```python
def generate_csv(req):
    csv_data = "id,name\n1,Alice\n2,Bob"
    return send_data(csv_data, "users.csv", "text/csv")
```

## Routing

### Route Registration

```python
# Method-specific routes
srv.get("/users", list_users)
srv.post("/users", create_user)
srv.put("/users/{id}", update_user)
srv.delete("/users/{id}", delete_user)

# Multiple methods
srv.route(["GET", "HEAD"], "/info", info_handler)
srv.route("POST", "/webhook", webhook_handler)
```

### Path Parameters

```python
# Named parameters
srv.get("/users/{id}", get_user)
srv.get("/posts/{post_id}/comments/{comment_id}", get_comment)

# Wildcard parameters
srv.get("/files/{*filepath}", serve_files)

def get_user(req):
    user_id = req.param("id")
    return json_response({"user_id": user_id})
```

### Static Files

```python
# Serve static files
srv.static("/static", "./public")

# Single-page application
srv.spa("/app", "./dist", fallback="index.html")
```

## Request Object

The request object provides access to all HTTP request information:

### Properties

```python
def handler(req):
    print(req.method)        # HTTP method
    print(req.url)           # Full URL
    print(req.path)          # URL path
    print(req.host)          # Host header
    print(req.remote)        # Client address
    print(req.client_ip)     # Client IP (extracts from headers)
    print(req.proto)         # Protocol (HTTP/1.1)
    
    headers = req.headers()  # Request headers dict
    query = req.query()      # Query parameters dict
    context = req.context()  # Middleware context dict
```

### Methods

```python
def handler(req):
    # Body content
    body = req.body()
    json_data = req.json()
    form_data = req.form()
    files = req.files()
    
    # Headers and cookies
    auth = req.get_header("Authorization")
    session_id = req.cookie("session")
    
    # Path parameters
    user_id = req.param("id")
    
    # Authentication helpers
    token = req.bearer_token()
    username, password = req.basic_auth()
```

## Middleware System

Middleware functions form a chain where each can inspect/modify requests and responses:

### Basic Middleware

```python
def logging_middleware(request, next_handler):
    print("Request: {} {}".format(request.method, request.path))
    
    # Call next handler
    response = next_handler(request)
    
    print("Response: {}".format(response.status_code))
    return response

srv.use(logging_middleware)
```

### Built-in Middleware

#### CORS Middleware

```python
srv.use(cors_middleware(
    origins=["https://example.com", "https://app.example.com"],
    methods=["GET", "POST", "PUT", "DELETE"],
    headers=["Content-Type", "Authorization"],
    credentials=True,
    max_age=86400
))
```

#### Security Headers

```python
srv.use(security_headers_middleware(
    frame_options="DENY",
    content_type_options="nosniff",
    xss_protection="1; mode=block",
    hsts="max-age=31536000; includeSubDomains",
    csp="default-src 'self'"
))
```

#### Request Logging

```python
srv.use(logging_middleware(
    format="{method} {path} {status} {duration}ms",
    skip_paths=["/health", "/metrics"]
))
```

#### Response Timing

```python
srv.use(timing_middleware(
    header="X-Response-Time",
    precision=3
))
```

#### Compression

```python
srv.use(compression_middleware(
    level=6,
    min_size=1024,
    types=["text/html", "application/json", "text/css"]
))
```

## Authentication

### Basic Authentication

```python
auth = basic_auth(
    users={"admin": "secret123", "user": "password"},
    realm="Admin Area"
)

srv.use_for("/admin/*", auth.middleware())
```

### Bearer Token Authentication

```python
def validate_token(token):
    # Your token validation logic
    if token == "valid-token":
        return {"user": "alice", "role": "admin"}
    return None

auth = bearer_auth(validate_token)
srv.use_for("/api/*", auth.middleware())
```

### API Key Authentication

```python
auth = api_key_auth(
    keys=["key1", "key2", "key3"],
    header="X-API-Key"
)

srv.use_for("/api/*", auth.middleware())
```

## Session Management

```python
# Create session manager
session_manager = create_session_manager(
    secret="your-secret-key",
    cookie_name="session",
    max_age=86400  # 24 hours
)

def login_handler(req):
    session = session_manager.get_session(req)
    session.set("user_id", "123")
    session.set("username", "alice")
    return response("Logged in")

def profile_handler(req):
    session = session_manager.get_session(req)
    user_id = session.get("user_id")
    if user_id == None:
        return redirect("/login")
    return response("Welcome back!")
```

## Complete Examples

### RESTful API

```python
load("web", "create_server", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    # In-memory database
    users = {}
    next_id = [1]
    
    def list_users(req):
        return json_response(list(users.values()))
    
    def get_user(req):
        user_id = int(req.param("id"))
        if user_id not in users:
            return error_response(404, "User not found")
        return json_response(users[user_id])
    
    def create_user(req):
        data = req.json()
        if not data or not data.get("name"):
            return error_response(400, "Name required")
        
        user = {
            "id": next_id[0],
            "name": data["name"],
            "email": data.get("email", "")
        }
        users[next_id[0]] = user
        next_id[0] += 1
        
        return json_response(user, status=201)
    
    def update_user(req):
        user_id = int(req.param("id"))
        if user_id not in users:
            return error_response(404, "User not found")
        
        data = req.json()
        user = users[user_id]
        
        if data.get("name"):
            user["name"] = data["name"]
        if data.get("email"):
            user["email"] = data["email"]
        
        return json_response(user)
    
    def delete_user(req):
        user_id = int(req.param("id"))
        if user_id not in users:
            return error_response(404, "User not found")
        
        del users[user_id]
        return response("", status=204)
    
    # Register routes
    srv.get("/api/users", list_users)
    srv.get("/api/users/{id}", get_user)
    srv.post("/api/users", create_user)
    srv.put("/api/users/{id}", update_user)
    srv.delete("/api/users/{id}", delete_user)
    
    # Add middleware
    srv.use(cors_middleware())
    srv.use(logging_middleware())
    
    print("Starting API server on http://localhost:8080")
    srv.run()

main()
```

### Web Application with Sessions

```python
load("web", "create_server", "response", "html_response", "redirect", "create_session_manager")

def main():
    srv = create_server(port=8080)
    
    # Session management
    session_mgr = create_session_manager(secret="session-secret")
    
    def home(req):
        session = session_mgr.get_session(req)
        username = session.get("username")
        
        if username:
            html = "<h1>Welcome back, {}!</h1><a href='/logout'>Logout</a>".format(username)
        else:
            html = "<h1>Welcome!</h1><a href='/login'>Login</a>"
        
        return html_response(html)
    
    def login_form(req):
        html = """
        <form method="post" action="/login">
            <input type="text" name="username" placeholder="Username" required>
            <input type="password" name="password" placeholder="Password" required>
            <button type="submit">Login</button>
        </form>
        """
        return html_response(html)
    
    def login_post(req):
        form = req.form()
        username = form.get("username")
        password = form.get("password")
        
        # Simple validation
        if username == "admin" and password == "secret":
            session = session_mgr.get_session(req)
            session.set("username", username)
            return redirect("/")
        
        return html_response("Invalid credentials", status=401)
    
    def logout(req):
        session = session_mgr.get_session(req)
        session.clear()
        return redirect("/")
    
    # Routes
    srv.get("/", home)
    srv.get("/login", login_form)
    srv.post("/login", login_post)
    srv.get("/logout", logout)
    
    print("Starting web app on http://localhost:8080")
    srv.run()

main()
```

## Configuration

### Environment Variables

Configure the web module using environment variables:

```bash
export WEB_HOST="0.0.0.0"
export WEB_PORT="8080"
export WEB_READ_TIMEOUT="30"
export WEB_WRITE_TIMEOUT="30"
export WEB_MAX_BODY_SIZE="104857600"  # 100MB
export WEB_ENABLE_CORS="true"
export WEB_CORS_ORIGINS="https://example.com,https://app.example.com"
```

### Server Configuration

```python
# Custom server configuration
srv = create_server(
    host="0.0.0.0",
    port=8080,
    read_timeout=60,
    write_timeout=60,
    max_body_size=100*1024*1024,  # 100MB
    enable_cors=True,
    cors_origins=["https://example.com"]
)
```

## Migration from Flask

| Flask | Starlark Web |
|-------|--------------|
| `app = Flask(__name__)` | `srv = create_server()` |
| `@app.route("/")` | `srv.get("/", handler)` |
| `request` global | `request` parameter |
| `jsonify(data)` | `json_response(data)` |
| `redirect(url)` | `redirect(url)` |
| `abort(404)` | `error_response(404)` |
| `session["key"]` | `session.get("key")` / `session.set("key", value)` |

## Performance Tips

1. **Use middleware sparingly** - Each middleware adds overhead
2. **Cache static files** - Use proper cache headers  
3. **Limit session data** - Store only essential data in sessions
4. **Enable compression** - For text-based responses
5. **Use path-specific middleware** - Apply middleware only where needed

## Security Best Practices

1. **Always use HTTPS in production**
2. **Set secure session cookies**
3. **Validate all input data**
4. **Use strong session secrets**
5. **Enable security headers**
6. **Implement rate limiting**

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.