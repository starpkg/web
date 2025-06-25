# 🌐 Web Module Final Design & Development Plan

**Module Name**: `web`  
**Emoji**: 🌐  
**Description**: Fast and intuitive server-side web framework for Starlark  
**Tagline**: Build modern web applications with Flask-like simplicity and Go performance

## Executive Summary

The `web` module provides a high-performance, Flask-inspired web server framework for Starlark scripts. It focuses exclusively on server-side functionality, complementing the existing `http` client module. The design emphasizes simplicity, security, and performance while maintaining consistency with existing Starlark modules.

## Core Design Principles

1. **Server-side Only**: No client HTTP functionality (use `http` module for client needs)
2. **Flask-inspired API**: Familiar patterns adapted for Starlark constraints
3. **Standard Library Only**: Built entirely on Go's `net/http` for reliability
4. **Base Package Integration**: Follows established configuration patterns
5. **Security by Default**: Built-in protection against common vulnerabilities
6. **High Performance**: Leverages Go's strengths for production use

## Starlark Constraints & Adaptations

### Key Limitations Addressed

- ❌ **No Decorators**: Use method calls like `server.route()` or `server.get()` instead of `@app.route()`
- ❌ **No f-strings**: Use `.format()` method for string formatting
- ❌ **No try/except**: Use `fail()` for error handling
- ❌ **No classes**: Functional approach with module-level state
- ❌ **No `is`/`is not`**: Use `== None` and `!= None`
- ❌ **No while loops**: Use for loops with range or recursion
- ❌ **Limited global scope**: Variables in outer scope can be read but not reassigned without special handling

## API Design

### Core Module Functions

```python
# Server creation
server(host="localhost", port=8080, **config) -> Server

# Response builders
response(body, status=200, headers={}) -> Response
json_response(data, status=200, headers={}) -> Response  
html_response(content, status=200, headers={}) -> Response
redirect(location, status=302) -> Response
error_response(status, message="") -> Response

# Static file helpers
send_file(filepath, content_type=None) -> Response

# Authentication helpers
basic_auth(users={}, realm="Restricted") -> Authenticator
bearer_auth(validate_func) -> Authenticator
api_key_auth(keys=[], header="X-API-Key") -> Authenticator

# Session helpers
get_session() -> Session  # Get current request session
```

### Server Object API

```python
# Route registration (method-first for consistency with http module)
server.route(method, path, handler)                    # method can be string or list of strings
server.get(path, handler)
server.post(path, handler)
server.put(path, handler)
server.delete(path, handler)
server.patch(path, handler)
server.options(path, handler)
server.head(path, handler)

# Route groups
server.group(prefix) -> RouteGroup

# Static file serving
server.static(url_path, directory, index="index.html")
server.spa(url_path, directory, fallback="index.html")

# Middleware
server.use(middleware_func)
server.use_for(path_pattern, middleware_func)
server.before_request(func)
server.after_request(func)
server.error_handler(status_code, handler)

# Lifecycle
server.run()              # Blocking
server.start_background() # Non-blocking
server.stop()
server.is_running() -> bool
```

### Request Object (Global during request handling)

```python
# Properties (matching http module's ExportedServerRequest structure)
request.method          # HTTP method
request.url             # Full URL
request.path            # URL path
request.host            # Host header
request.remote          # Client address
request.proto           # Protocol (HTTP/1.1)
request.headers         # Dict of headers
request.query           # Dict of query parameters

# Methods
request.body()          # Raw body string
request.json()          # Parse JSON body (returns None if invalid)
request.form()          # Parse form data
request.files()         # Dict of uploaded files
request.cookie(name)    # Get cookie value
request.param(name)     # Get path parameter
request.get_header(name, default=None)
request.bearer_token()  # Extract Bearer token
request.basic_auth()    # Get (username, password) tuple
```

### Response Object

```python
# Properties
response.status_code    # HTTP status
response.headers        # Response headers
response.body          # Response body

# Methods
response.set_cookie(name, value, max_age=None, path="/", 
                   domain=None, secure=False, http_only=True)
response.delete_cookie(name, path="/", domain=None)
```

### Session Object (Global in request context)

```python
# Properties
session.id              # Session ID
session.is_new          # New session flag

# Methods
session.get(key, default=None)
session.set(key, value)
session.delete(key)
session.clear()
session.save()
session.flash(message, category="info")
session.get_flashes(category=None) -> list
```

### FileUpload Object

```python
# Properties
file.filename           # Original filename
file.content_type      # MIME type
file.size              # File size in bytes

# Methods
file.read()            # Read file content as string
file.read_bytes()      # Read file content as bytes
file.save(path)        # Save to disk
```

## Route Registration Methods

The web module provides flexible ways to register routes:

### 1. Method-Specific Functions
```python
srv.get("/users", list_users)
srv.post("/users", create_user)
srv.put("/users/{id}", update_user)
srv.delete("/users/{id}", delete_user)
```

### 2. Generic Route Method - Single HTTP Method
```python
srv.route("GET", "/users", list_users)
srv.route("POST", "/users", create_user)
```

### 3. Generic Route Method - Multiple HTTP Methods
```python
# Same handler for multiple methods
srv.route(["GET", "HEAD"], "/api/info", api_info)
srv.route(["GET", "POST", "PUT"], "/webhook", webhook_handler)

# Handler that checks request method internally
def resource_handler(req):
    if req.method == "GET":
        return get_resource_logic(req)
    elif req.method == "PUT":
        return update_resource_logic(req)
    elif req.method == "DELETE":
        return delete_resource_logic(req)
    else:
        return error_response(405, "Method not allowed")

srv.route(["GET", "PUT", "DELETE"], "/resource/{id}", resource_handler)
```

## Route Patterns

Priority order (highest to lowest):

1. **Exact match**: `/users`
2. **Named parameters**: `/users/{id}`
3. **Wildcard**: `/files/*filepath`

## Complete Usage Examples

### 1. Basic Web Server

```python
load("web", "server", "response", "json_response")

def main():
    srv = server(host="0.0.0.0", port=8080)
    
    # Simple text response
    def home(req):
        return response("Welcome to Starlark Web!")
    
    # JSON API endpoint
    def api_info(req):
        return json_response({
            "name": "Starlark Web API",
            "version": "1.0",
            "method": req.method,
            "path": req.path,
            "user_agent": req.get_header("User-Agent", "Unknown")
        })
    
    # HTML response
    def about(req):
        html = """
        <html>
            <head><title>About</title></head>
            <body>
                <h1>About Starlark Web</h1>
                <p>A fast web framework for Starlark</p>
            </body>
        </html>
        """
        return response(html, headers={"Content-Type": "text/html"})
    
    srv.get("/", home)
    srv.get("/about", about)
    
    # Multiple ways to register the same endpoint:
    # Method 1: Individual method registration
    # srv.get("/api/info", api_info)  
    # srv.head("/api/info", api_info)
    
    # Method 2: Using server.route() for multiple methods
    srv.route(["GET", "HEAD"], "/api/info", api_info)
    
    print("Server starting on http://{}:{}".format("0.0.0.0", 8080))
    srv.run()

main()
```

### 2. RESTful API with CRUD Operations

```python
load("web", "server", "json_response", "error_response")
load("time")

def main():
    srv = server(port=8080)
    
    # In-memory database
    users = {}
    next_id = 1
    
    # List all users
    def list_users(req):
        user_list = [user for user in users.values()]
        return json_response(user_list)
    
    # Get single user
    def get_user(req):
        user_id = req.param("id")
        if user_id == None:
            return error_response(400, "User ID required")
        
        user = users.get(int(user_id))
        if user == None:
            return error_response(404, "User not found")
        
        return json_response(user)
    
    # Create new user
    def create_user(req):
        data = req.json()
        if data == None:
            return error_response(400, "Invalid JSON")
        
        name = data.get("name")
        email = data.get("email")
        
        if name == None or email == None:
            return error_response(400, "Name and email required")
        
        user = {
            "id": next_id,
            "name": name,
            "email": email,
            "created_at": time.now().format(time.RFC3339)
        }
        users[next_id] = user
        next_id = next_id + 1
        
        return json_response(user, status=201)
    
    # Update user
    def update_user(req):
        user_id = req.param("id")
        if user_id == None:
            return error_response(400, "User ID required")
        
        user_id = int(user_id)
        user = users.get(user_id)
        if user == None:
            return error_response(404, "User not found")
        
        data = req.json()
        if data == None:
            return error_response(400, "Invalid JSON")
        
        # Update fields if provided
        if data.get("name") != None:
            user["name"] = data["name"]
        if data.get("email") != None:
            user["email"] = data["email"]
        
        user["updated_at"] = time.now().format(time.RFC3339)
        return json_response(user)
    
    # Delete user
    def delete_user(req):
        user_id = req.param("id")
        if user_id == None:
            return error_response(400, "User ID required")
        
        user_id = int(user_id)
        if users.get(user_id) == None:
            return error_response(404, "User not found")
        
        users.pop(user_id)
        return response("", status=204)
    
    # Register routes
    srv.get("/api/users", list_users)
    srv.get("/api/users/{id}", get_user)
    srv.post("/api/users", create_user)
    srv.put("/api/users/{id}", update_user)
    srv.delete("/api/users/{id}", delete_user)
    
    print("API server running on http://localhost:8080")
    print("Try: curl http://localhost:8080/api/users")
    srv.run()

main()
```

### 3. Authentication Examples

#### Basic Authentication

```python
load("web", "server", "basic_auth", "response", "json_response")

def main():
    srv = server(port=8080)
    
    # Create authenticator
    auth = basic_auth(users={
        "admin": "secret123",
        "user": "password"
    }, realm="Admin Area")
    
    # Public endpoint
    def public_page(req):
        return response("This is public")
    
    # Protected endpoint - manual check
    def admin_page(req):
        creds = req.basic_auth()
        if creds == None:
            return response(
                "Authentication required",
                status=401,
                headers={"WWW-Authenticate": 'Basic realm="Admin Area"'}
            )
        
        username, password = creds
        if not auth.validate(username, password):
            return error_response(401, "Invalid credentials")
        
        return response("Welcome to admin area, {}!".format(username))
    
    # Protected with middleware
    def secret_data(req):
        # Username is attached to request by middleware
        username = req.context.get("username", "Unknown")
        return json_response({
            "message": "Secret data",
            "user": username,
            "data": [1, 2, 3, 4, 5]
        })
    
    srv.get("/", public_page)
    srv.get("/admin", admin_page)
    
    # Apply auth middleware to specific routes
    srv.use_for("/api/*", auth.middleware())
    srv.get("/api/secret", secret_data)
    
    srv.run()

main()
```

#### Bearer Token Authentication

```python
load("web", "server", "bearer_auth", "json_response", "error_response")

def main():
    srv = server(port=8080)
    
    # Valid tokens (in production, check against database)
    valid_tokens = {
        "token-123": {"user": "alice", "role": "admin"},
        "token-456": {"user": "bob", "role": "user"}
    }
    
    # Token validator function
    def validate_token(token):
        return valid_tokens.get(token)
    
    # Create authenticator
    auth = bearer_auth(validate_token)
    
    # Login endpoint
    def login(req):
        data = req.json()
        if data == None:
            return error_response(400, "Invalid JSON")
        
        username = data.get("username")
        password = data.get("password")
        
        # Simple validation (use proper auth in production)
        if username == "alice" and password == "alice123":
            return json_response({"token": "token-123"})
        elif username == "bob" and password == "bob456":
            return json_response({"token": "token-456"})
        
        return error_response(401, "Invalid credentials")
    
    # Protected endpoint
    def user_profile(req):
        # Token info attached by middleware
        token_info = req.context.get("token_info")
        if token_info == None:
            return error_response(401, "Unauthorized")
        
        return json_response({
            "user": token_info["user"],
            "role": token_info["role"]
        })
    
    # Admin only endpoint
    def admin_data(req):
        token_info = req.context.get("token_info")
        if token_info == None or token_info.get("role") != "admin":
            return error_response(403, "Forbidden")
        
        return json_response({
            "message": "Admin only data",
            "secrets": ["secret1", "secret2"]
        })
    
    srv.post("/login", login)
    
    # Apply auth middleware to API routes
    srv.use_for("/api/*", auth.middleware())
    srv.get("/api/profile", user_profile)
    srv.get("/api/admin", admin_data)
    
    srv.run()

main()
```

### 4. Session Management

```python
load("web", "server", "response", "redirect", "json_response", "get_session")
load("time")

def main():
    srv = server(
        port=8080,
        session_secret="my-secret-key-change-in-production"
    )
    
    # Home page
    def home(req):
        session = get_session()
        username = session.get("username")
        
        if username != None:
            visits = session.get("visits", 0) + 1
            session.set("visits", visits)
            
            html = """
            <html>
                <body>
                    <h1>Welcome back, {}!</h1>
                    <p>This is visit number {}</p>
                    <p><a href="/logout">Logout</a></p>
                </body>
            </html>
            """.format(username, visits)
            return response(html, headers={"Content-Type": "text/html"})
        
        html = """
        <html>
            <body>
                <h1>Please Login</h1>
                <form method="post" action="/login">
                    <input name="username" placeholder="Username" required>
                    <input type="password" name="password" placeholder="Password" required>
                    <button type="submit">Login</button>
                </form>
            </body>
        </html>
        """
        return response(html, headers={"Content-Type": "text/html"})
    
    # Login handler - GET version
    def login_form(req):
        return redirect("/")
    
    # Login handler - POST version
    def login_post(req):
        form = req.form()
        username = form.get("username")
        password = form.get("password")
        
        # Simple auth (use proper validation in production)
        if username != None and password == "password":
            session = get_session()
            session.set("username", username)
            session.set("login_time", time.now().format(time.RFC3339))
            session.flash("Login successful!", "success")
            return redirect("/")
        
        session = get_session()
        session.flash("Invalid credentials", "error")
        return redirect("/")
    
    # API endpoint
    def api_status(req):
        session = get_session()
        username = session.get("username")
        
        if username == None:
            return error_response(401, "Not authenticated")
        
        return json_response({
            "username": username,
            "session_id": session.id,
            "login_time": session.get("login_time"),
            "visits": session.get("visits", 0)
        })
    
    # CORS preflight handler
    def api_status_options(req):
        return response("", headers={
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type"
        })
    
    # Logout handler
    def logout(req):
        session = get_session()
        session.clear()
        session.flash("You have been logged out", "info")
        return redirect("/")
    
    srv.get("/", home)
    srv.get("/login", login_form)
    srv.post("/login", login_post)
    srv.get("/logout", logout)
    
    # Alternative ways to register the API status endpoint:
    # Option 1: Individual methods
    # srv.get("/api/status", api_status)
    # srv.options("/api/status", api_status_options)
    
    # Option 2: Using server.route() with multiple methods and separate handlers
    srv.route("GET", "/api/status", api_status)
    srv.route("OPTIONS", "/api/status", api_status_options)
    
    srv.run()

main()
```

### 5. File Upload and Static Files

```python
load("web", "server", "response", "json_response", "send_file")
load("time")

def main():
    srv = server(port=8080)
    
    # Upload form
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
        return response(html, headers={"Content-Type": "text/html"})
    
    # Handle upload
    def handle_upload(req):
        files = req.files()
        if len(files) == 0:
            return error_response(400, "No file uploaded")
        
        # Get the first file
        file = files.get("file")
        if file == None:
            return error_response(400, "No file field")
        
        # Validate file
        if file.size > 10 * 1024 * 1024:  # 10MB limit
            return error_response(400, "File too large (max 10MB)")
        
        # Check content type
        allowed_types = ["image/jpeg", "image/png", "image/gif"]
        if file.content_type not in allowed_types:
            return error_response(400, "Only JPEG, PNG, and GIF allowed")
        
        # Save file
        filename = "upload_{}_{}".format(time.now().unix, file.filename)
        filepath = "uploads/{}".format(filename)
        file.save(filepath)
        
        return json_response({
            "filename": filename,
            "original_name": file.filename,
            "size": file.size,
            "content_type": file.content_type,
            "url": "/uploads/{}".format(filename)
        })
    
    # Download file
    def download(req):
        filename = req.param("filename")
        if filename == None:
            return error_response(400, "Filename required")
        
        # Security: prevent path traversal
        if ".." in filename or "/" in filename:
            return error_response(400, "Invalid filename")
        
        filepath = "uploads/{}".format(filename)
        return send_file(filepath)
    
    # Setup routes
    srv.get("/", upload_form)
    srv.post("/upload", handle_upload)
    srv.get("/download/{filename}", download)
    
    # Serve static files
    srv.static("/static", "./static")
    srv.static("/uploads", "./uploads")
    
    # SPA support
    srv.spa("/app", "./dist", fallback="index.html")
    
    srv.run()

main()
```

### 6. Middleware and Error Handling

```python
load("web", "server", "response", "json_response", "get_session")
load("time")

def main():
    srv = server(port=8080, debug=True)
    
    # Request timing middleware
    def timing_middleware(req, next_handler):
        start = time.now()
        
        # Call next handler
        resp = next_handler(req)
        
        # Calculate duration
        duration = time.since(start)
        resp.headers["X-Response-Time"] = "{:.3f}ms".format(duration.milliseconds)
        
        return resp
    
    # Logging middleware
    def logging_middleware(req, next_handler):
        print("{} {} {}".format(
            time.now().format(time.Kitchen),
            req.method,
            req.path
        ))
        
        resp = next_handler(req)
        
        print("  -> {} in {}".format(
            resp.status_code,
            resp.headers.get("X-Response-Time", "unknown")
        ))
        
        return resp
    
    # Auth check middleware
    def auth_middleware(req, next_handler):
        session = get_session()
        if session.get("user_id") == None:
            return error_response(401, "Authentication required")
        
        return next_handler(req)
    
    # CORS middleware
    def cors_middleware(req, next_handler):
        # Handle preflight
        if req.method == "OPTIONS":
            return response("", headers={
                "Access-Control-Allow-Origin": "*",
                "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
                "Access-Control-Allow-Headers": "Content-Type, Authorization",
                "Access-Control-Max-Age": "86400"
            })
        
        # Add CORS headers to response
        resp = next_handler(req)
        resp.headers["Access-Control-Allow-Origin"] = "*"
        return resp
    
    # Error handlers
    def not_found_handler(req):
        return json_response({
            "error": "Not Found",
            "path": req.path,
            "status": 404
        }, status=404)
    
    def server_error_handler(req):
        return json_response({
            "error": "Internal Server Error",
            "message": "Something went wrong",
            "status": 500
        }, status=500)
    
    # Routes
    def public_api(req):
        return json_response({"message": "Public API endpoint"})
    
    def protected_api(req):
        session = get_session()
        return json_response({
            "message": "Protected endpoint",
            "user_id": session.get("user_id")
        })
    
    def broken_endpoint(req):
        # This will trigger 500 error
        fail("Intentional error for testing")
    
    # Apply global middleware
    srv.use(timing_middleware)
    srv.use(logging_middleware)
    srv.use(cors_middleware)
    
    # Apply auth middleware to specific paths
    srv.use_for("/api/protected/*", auth_middleware)
    
    # Register routes
    srv.get("/api/public", public_api)
    srv.get("/api/protected/data", protected_api)
    srv.get("/api/broken", broken_endpoint)
    
    # Error handlers
    srv.error_handler(404, not_found_handler)
    srv.error_handler(500, server_error_handler)
    
    srv.run()

main()
```

### 7. Route Groups and Advanced Routing

```python
load("web", "server", "json_response", "basic_auth")

def main():
    srv = server(port=8080)
    
    # Create auth for admin routes
    admin_auth = basic_auth(users={"admin": "secret"})
    
    # Public routes
    def home(req):
        return response("Welcome to our API")
    
    def health(req):
        return json_response({"status": "healthy"})
    
    srv.get("/", home)
    srv.get("/health", health)
    
    # API v1 routes
    v1 = srv.group("/api/v1")
    
    def v1_users(req):
        return json_response([
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ])
    
    def v1_user(req):
        user_id = req.param("id")
        return json_response({"id": int(user_id), "name": "User {}".format(user_id)})
    
    v1.get("/users", v1_users)
    v1.get("/users/{id}", v1_user)
    
    # API v2 routes with middleware
    v2 = srv.group("/api/v2")
    v2.use(lambda req, next: (
        response("API v2 requires authentication", status=401)
        if req.get_header("X-API-Key") != "secret-key"
        else next(req)
    ))
    
    def v2_users(req):
        return json_response([
            {"id": 1, "name": "Alice", "email": "alice@example.com"},
            {"id": 2, "name": "Bob", "email": "bob@example.com"}
        ])
    
    v2.get("/users", v2_users)
    
    # Admin routes with auth
    admin = srv.group("/admin")
    admin.use(admin_auth.middleware())
    
    def admin_dashboard(req):
        return response("Admin Dashboard")
    
    def admin_users(req):
        return json_response({
            "users": 156,
            "active": 143,
            "suspended": 13
        })
    
    admin.get("/", admin_dashboard)
    admin.get("/stats/users", admin_users)
    
    # Wildcard routes - separate handlers per method
    def catch_all_get(req):
        filepath = req.param("path")
        return json_response({
            "message": "GET files",
            "path": filepath,
            "method": "GET"
        })
    
    def catch_all_post(req):
        filepath = req.param("path")
        return json_response({
            "message": "POST files",
            "path": filepath,
            "method": "POST"
        })
    
    def catch_all_put(req):
        filepath = req.param("path")
        return json_response({
            "message": "PUT files",
            "path": filepath,
            "method": "PUT"
        })
    
    def catch_all_delete(req):
        filepath = req.param("path")
        return json_response({
            "message": "DELETE files",
            "path": filepath,
            "method": "DELETE"
        })
    
    # Register each method separately
    srv.get("/files/*path", catch_all_get)
    srv.post("/files/*path", catch_all_post)
    srv.put("/files/*path", catch_all_put)
    srv.delete("/files/*path", catch_all_delete)
    
    # Resource endpoints - showing different approaches
    
    # Approach 1: Separate handlers with individual method registration
    def get_resource(req):
        resource_id = req.param("id")
        return json_response({"id": resource_id, "action": "retrieved"})
    
    def update_resource(req):
        resource_id = req.param("id")
        return json_response({"id": resource_id, "action": "updated"})
    
    def delete_resource(req):
        resource_id = req.param("id")
        return json_response({"id": resource_id, "action": "deleted"})
    
    srv.get("/resource/{id}", get_resource)
    srv.put("/resource/{id}", update_resource)
    srv.delete("/resource/{id}", delete_resource)
    
    # Approach 2: Single handler with server.route() for multiple methods
    def multi_method_resource(req):
        resource_id = req.param("id")
        
        if req.method == "GET":
            return json_response({"id": resource_id, "action": "retrieved"})
        elif req.method == "PUT":
            return json_response({"id": resource_id, "action": "updated"})
        elif req.method == "DELETE":
            return json_response({"id": resource_id, "action": "deleted"})
        else:
            return error_response(405, "Method not allowed")
    
    srv.route(["GET", "PUT", "DELETE"], "/multi-resource/{id}", multi_method_resource)
    
    srv.run()

main()
```

### 8. Streaming Responses and Large Files

```python
load("web", "server", "response")
load("time", "json")

def main():
    srv = server(port=8080)
    
    # Stream large CSV data
    def stream_csv(req):
        def generate_csv():
            # Header
            yield "id,name,email,created\n"
            
            # Generate 10000 rows
            for i in range(10000):
                yield "{},User{},user{}@example.com,{}\n".format(
                    i + 1,
                    i + 1,
                    i + 1,
                    time.now().format(time.RFC3339)
                )
        
        return response(
            generate_csv(),
            headers={
                "Content-Type": "text/csv",
                "Content-Disposition": "attachment; filename=users.csv"
            }
        )
    
    # Stream JSON array
    def stream_json_array(req):
        def generate_json():
            yield "["
            
            for i in range(1000):
                if i > 0:
                    yield ","
                yield json.encode({
                    "id": i + 1,
                    "value": "Item {}".format(i + 1),
                    "timestamp": time.now().unix
                })
            
            yield "]"
        
        return response(
            generate_json(),
            headers={"Content-Type": "application/json"}
        )
    
    # Server-sent events
    def sse_endpoint(req):
        def generate_events():
            for i in range(10):
                event = "data: {}\n\n".format(json.encode({
                    "message": "Event {}".format(i + 1),
                    "time": time.now().format(time.RFC3339)
                }))
                yield event
                time.sleep(1)  # Simulate delay
        
        return response(
            generate_events(),
            headers={
                "Content-Type": "text/event-stream",
                "Cache-Control": "no-cache",
                "Connection": "keep-alive"
            }
        )
    
    srv.get("/download/csv", stream_csv)
    srv.get("/api/stream", stream_json_array)
    srv.get("/events", sse_endpoint)
    
    srv.run()

main()
```

### 9. Complete Blog Application

```python
load("web", "server", "response", "json_response", "redirect", 
     "get_session", "basic_auth", "send_file")
load("time")

def main():
    srv = server(
        port=8080,
        session_secret="blog-secret-key"
    )
    
    # Simple in-memory database
    posts = []
    comments = {}
    next_post_id = 1
    next_comment_id = 1
    
    # Admin auth
    admin_auth = basic_auth(users={"admin": "admin123"})
    
    # Helper to render HTML template
    def render_html(title, content):
        return """
        <html>
            <head>
                <title>{}</title>
                <link rel="stylesheet" href="/static/style.css">
            </head>
            <body>
                <header>
                    <h1>My Blog</h1>
                    <nav>
                        <a href="/">Home</a>
                        <a href="/admin">Admin</a>
                    </nav>
                </header>
                <main>
                    {}
                </main>
            </body>
        </html>
        """.format(title, content)
    
    # Home page - list posts
    def home(req):
        content = "<h2>Recent Posts</h2>"
        
        if len(posts) == 0:
            content = content + "<p>No posts yet.</p>"
        else:
            content = content + "<ul>"
            for post in posts:
                content = content + '<li><a href="/post/{}">{}</a> - {}</li>'.format(
                    post["id"], 
                    post["title"],
                    post["created"]
                )
            content = content + "</ul>"
        
        html = render_html("Home", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # View single post
    def view_post(req):
        post_id = int(req.param("id"))
        post = None
        
        for p in posts:
            if p["id"] == post_id:
                post = p
                break
        
        if post == None:
            return error_response(404, "Post not found")
        
        # Build post HTML
        content = "<article>"
        content = content + "<h2>{}</h2>".format(post["title"])
        content = content + "<p class='meta'>Posted on {}</p>".format(post["created"])
        content = content + "<div class='content'>{}</div>".format(post["content"])
        content = content + "</article>"
        
        # Add comments section
        content = content + "<h3>Comments</h3>"
        post_comments = comments.get(post_id, [])
        
        if len(post_comments) == 0:
            content = content + "<p>No comments yet.</p>"
        else:
            for comment in post_comments:
                content = content + "<div class='comment'>"
                content = content + "<strong>{}</strong>: {}".format(
                    comment["author"], 
                    comment["text"]
                )
                content = content + "</div>"
        
        # Comment form
        content = content + """
        <form method="post" action="/post/{}/comment">
            <input name="author" placeholder="Your name" required>
            <textarea name="text" placeholder="Your comment" required></textarea>
            <button type="submit">Post Comment</button>
        </form>
        """.format(post_id)
        
        html = render_html(post["title"], content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # Post comment
    def post_comment(req):
        post_id = int(req.param("id"))
        
        # Check post exists
        post_exists = False
        for p in posts:
            if p["id"] == post_id:
                post_exists = True
                break
        
        if not post_exists:
            return error_response(404, "Post not found")
        
        form = req.form()
        author = form.get("author", "").strip()
        text = form.get("text", "").strip()
        
        if author == "" or text == "":
            return error_response(400, "Author and text required")
        
        # Add comment
        if comments.get(post_id) == None:
            comments[post_id] = []
        
        comment = {
            "id": next_comment_id,
            "author": author,
            "text": text,
            "created": time.now().format(time.DateTime)
        }
        comments[post_id].append(comment)
        next_comment_id = next_comment_id + 1
        
        return redirect("/post/{}".format(post_id))
    
    # Admin dashboard
    def admin_dashboard(req):
        content = "<h2>Admin Dashboard</h2>"
        content = content + "<p><a href='/admin/new'>Create New Post</a></p>"
        content = content + "<h3>All Posts</h3>"
        
        if len(posts) == 0:
            content = content + "<p>No posts yet.</p>"
        else:
            content = content + "<ul>"
            for post in posts:
                content = content + '<li>{} - <a href="/admin/edit/{}">Edit</a></li>'.format(
                    post["title"], 
                    post["id"]
                )
            content = content + "</ul>"
        
        html = render_html("Admin Dashboard", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # New post form
    def new_post_form(req):
        content = """
        <h2>Create New Post</h2>
        <form method="post" action="/admin/new">
            <input name="title" placeholder="Post title" required>
            <textarea name="content" placeholder="Post content" rows="10" required></textarea>
            <button type="submit">Create Post</button>
        </form>
        """
        
        html = render_html("New Post", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # Create post
    def create_post(req):
        form = req.form()
        title = form.get("title", "").strip()
        content = form.get("content", "").strip()
        
        if title == "" or content == "":
            return error_response(400, "Title and content required")
        
        post = {
            "id": next_post_id,
            "title": title,
            "content": content,
            "created": time.now().format(time.DateTime)
        }
        posts.append(post)
        next_post_id = next_post_id + 1
        
        session = get_session()
        session.flash("Post created successfully!", "success")
        
        return redirect("/admin")
    
    # API endpoints
    def api_posts(req):
        return json_response(posts)
    
    def api_post_comments(req):
        post_id = int(req.param("id"))
        return json_response(comments.get(post_id, []))
    
    # Register routes
    srv.get("/", home)
    srv.get("/post/{id}", view_post)
    srv.post("/post/{id}/comment", post_comment)
    
    # Admin routes (protected)
    srv.use_for("/admin/*", admin_auth.middleware())
    srv.get("/admin", admin_dashboard)
    srv.get("/admin/new", new_post_form)
    srv.post("/admin/new", create_post)
    
    # API routes
    srv.get("/api/posts", api_posts)
    srv.get("/api/posts/{id}/comments", api_post_comments)
    
    # Static files
    srv.static("/static", "./static")
    
    print("Blog running on http://localhost:8080")
    srv.run()

main()
```

## Configuration System

Using base package configuration pattern:

```go
type Config struct {
    // Server settings
    Host                *base.ConfigOption[string]    // Default: "localhost"
    Port                *base.ConfigOption[int]       // Default: 8080
    ReadTimeout         *base.ConfigOption[int]       // Default: 30 seconds
    WriteTimeout        *base.ConfigOption[int]       // Default: 30 seconds
    MaxBodySize         *base.ConfigOption[int64]     // Default: 32MB
    
    // Session settings
    SessionSecret       *base.ConfigOption[string]    // Required for sessions
    SessionCookieName   *base.ConfigOption[string]    // Default: "session"
    SessionMaxAge       *base.ConfigOption[int]       // Default: 86400 (24 hours)
    
    // Security settings
    EnableCORS          *base.ConfigOption[bool]      // Default: false
    CORSOrigins         *base.ConfigOption[[]string]  // Default: ["*"]
    EnableCompression   *base.ConfigOption[bool]      // Default: true
    
    // Static file settings
    StaticCacheMaxAge   *base.ConfigOption[int]       // Default: 3600
}
```

### Environment Variable Configuration

```bash
# Server configuration
export WEB_HOST="0.0.0.0"
export WEB_PORT="8080"
export WEB_READ_TIMEOUT="60"
export WEB_WRITE_TIMEOUT="60"
export WEB_MAX_BODY_SIZE="104857600"  # 100MB

# Session configuration
export WEB_SESSION_SECRET="your-secret-key-here"
export WEB_SESSION_COOKIE_NAME="app_session"
export WEB_SESSION_MAX_AGE="86400"

# Security configuration
export WEB_ENABLE_CORS="true"
export WEB_CORS_ORIGINS="https://example.com,https://app.example.com"
export WEB_ENABLE_COMPRESSION="true"

# Static files
export WEB_STATIC_CACHE_MAX_AGE="3600"
```

## Implementation Architecture

### File Structure

```
web/
├── web.go              # Module registration and configuration
├── server.go           # HTTP server implementation
├── router.go           # Route matching and dispatch
├── request.go          # Request wrapper
├── response.go         # Response builders
├── session.go          # Session management
├── middleware.go       # Middleware system
├── auth.go             # Authentication helpers
├── static.go           # Static file serving
├── utils.go            # Helper functions
├── web_test.go         # Unit tests
├── example_test.go     # Integration tests
└── README.md           # User documentation
```

### Core Components

#### 1. Server Structure

```go
type Server struct {
    config      *Config
    httpServer  *http.Server
    router      *Router
    middleware  []MiddlewareFunc
    sessions    *SessionManager
    running     atomic.Bool
    mu          sync.RWMutex
}
```

#### 2. Router Implementation

```go
type Router struct {
    routes      map[string]*RouteTree  // method -> route tree
    static      map[string]StaticConfig
    paramRegex  *regexp.Regexp
}

type RouteTree struct {
    exact       map[string]HandlerFunc
    param       map[string]*RouteTree
    wildcard    *HandlerFunc
}
```

#### 3. Middleware Pipeline

```go
type MiddlewareFunc func(*Request, NextFunc) *Response
type NextFunc func(*Request) *Response
type HandlerFunc func(*Request) *Response
```

## Development Plan

### Phase 1: Core Infrastructure (Week 1)

**Priority**: Critical  
**Effort**: 20-25 hours

#### Deliverables

- Basic HTTP server with lifecycle management
- Route registration and matching
- Request/Response wrappers
- Base package integration

#### Success Criteria

```python
srv = server(port=8080)
srv.get("/", lambda req: response("Hello, World!"))
srv.get("/api/data", lambda req: json_response({"status": "ok"}))
srv.run()
```

### Phase 2: Advanced Routing & Static Files (Week 2)

**Priority**: High  
**Effort**: 15-20 hours

#### Deliverables

- Path parameter extraction
- Static file serving with caching
- SPA support with fallback routing
- File upload handling

#### Success Criteria

```python
srv.get("/users/{id}", get_user_handler)
srv.static("/static", "./public")
srv.spa("/app", "./dist")
```

### Phase 3: Middleware & Authentication (Week 3)

**Priority**: High  
**Effort**: 20-25 hours

#### Deliverables

- Middleware pipeline system
- Built-in middleware (CORS, logging, compression)
- Basic and Bearer authentication
- Custom authentication support

#### Success Criteria

```python
srv.use(cors_middleware(origins=["*"]))
srv.use(logging_middleware())
srv.use(basic_auth({"admin": "password"}).middleware("/admin/*"))
```

### Phase 4: Sessions & Security (Week 4)

**Priority**: High  
**Effort**: 18-22 hours

#### Deliverables

- Secure session management
- Cookie handling
- CSRF protection
- Security headers middleware

#### Success Criteria

```python
def protected(request):
    if session.get("user_id") == None:
        return redirect("/login")
    return response("Protected content")

srv.get("/dashboard", protected)
```

### Phase 5: Polish & Performance (Week 5)

**Priority**: Medium  
**Effort**: 15-18 hours

#### Deliverables

- Performance optimizations
- Comprehensive documentation
- Example applications
- Benchmark suite

## Security Considerations

1. **Path Traversal Prevention**: Validate all static file paths
2. **Session Security**: Cryptographically secure IDs with HMAC signing
3. **CSRF Protection**: Token-based protection for state-changing operations
4. **Security Headers**: Default security headers (X-Frame-Options, etc.)
5. **Input Validation**: Size limits and content-type validation

## Testing Strategy

1. **Unit Tests** (`web_test.go`)
   - Route matching algorithms
   - Request/response parsing
   - Session management
   - Middleware execution

2. **Integration Tests** (`example_test.go`)
   - Full request/response cycle
   - Authentication flows
   - Static file serving
   - Error scenarios

3. **Performance Tests**
   - Concurrent request handling
   - Memory usage under load
   - Static file serving efficiency

## Error Handling

All errors in Starlark should use `fail()` since there's no try/except:

```python
def handler(req):
    data = req.json()
    if data == None:
        fail("Invalid JSON in request body")
    
    # This will be caught and returned as 500 error
    if data.get("required_field") == None:
        fail("required_field is missing")
    
    return json_response({"success": True})
```

## Performance Optimization Tips

1. **Use middleware sparingly**: Each middleware adds overhead
2. **Cache static files**: Use proper cache headers
3. **Limit session data**: Store only essential data in sessions
4. **Use streaming for large responses**: Don't load everything in memory
5. **Enable compression**: For text-based responses

## Migration Guide from Flask

| Flask | Starlark Web |
|-------|--------------|
| `@app.route("/")` | `srv.get("/", handler)` |
| `f"Hello {name}"` | `"Hello {}".format(name)` |
| `try/except` | Use `fail()` or check for `None` |
| `if x is None` | `if x == None` |
| `request` global | `request` parameter in handler |
| `session` global | `get_session()` function |

## Success Metrics

1. **Performance**: Handle 10,000+ requests/second on modest hardware
2. **Memory**: <100MB memory usage under normal load
3. **Latency**: <10ms response time for simple routes
4. **Reliability**: 99.9% uptime in production deployments
5. **Adoption**: Easy migration from Flask/Express applications
