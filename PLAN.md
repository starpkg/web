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
- ❌ **No yield**: Streaming responses not supported

## Function Documentation

### Core Module Functions

#### Server Creation

```python
create_server(host="localhost", port=8080, **config) -> Server
```

**Purpose**: Creates a new HTTP server instance with the specified configuration.  
**Parameters**:

- `host` (string): Host address to bind to (default: "localhost")
- `port` (int): Port number to listen on (default: 8080)
- `**config`: Additional configuration options (see Configuration section)  
**Returns**: Server object that can be used to register routes and start the server

#### Session Management

```python
create_session_manager(secret, cookie_name="session", max_age=86400, **options) -> SessionManager
```

**Purpose**: Creates a session manager for handling user sessions across requests.  
**Parameters**:

- `secret` (string): Secret key for session encryption (required)
- `cookie_name` (string): Name of the session cookie (default: "session")
- `max_age` (int): Session lifetime in seconds (default: 86400/24 hours)
- `**options`: Additional session configuration  
**Returns**: SessionManager object for handling sessions

#### Response Builders

```python
response(body, status=200, headers={}) -> Response
```

**Purpose**: Creates a basic HTTP response with the given body and status.  
**Parameters**:

- `body` (string): Response body content
- `status` (int): HTTP status code (default: 200)
- `headers` (dict): Additional response headers  
**Returns**: Response object

```python
json_response(data, status=200, headers={}) -> Response
```

**Purpose**: Creates a JSON HTTP response from the given data.  
**Parameters**:

- `data` (any): Data to serialize as JSON
- `status` (int): HTTP status code (default: 200)
- `headers` (dict): Additional response headers  
**Returns**: Response object with Content-Type: application/json

```python
html_response(content, status=200, headers={}) -> Response
```

**Purpose**: Creates an HTML HTTP response with proper content type.  
**Parameters**:

- `content` (string): HTML content
- `status` (int): HTTP status code (default: 200)
- `headers` (dict): Additional response headers  
**Returns**: Response object with Content-Type: text/html

```python
redirect(location, status=302) -> Response
```

**Purpose**: Creates a redirect response to the specified location.  
**Parameters**:

- `location` (string): URL to redirect to
- `status` (int): Redirect status code (default: 302)  
**Returns**: Response object with Location header

```python
error_response(status, message="") -> Response
```

**Purpose**: Creates an error response with the given status and message.  
**Parameters**:

- `status` (int): HTTP error status code
- `message` (string): Error message (optional)  
**Returns**: Response object with error status

#### File Helpers

```python
send_file(filepath, content_type=None) -> Response
```

**Purpose**: Sends a file from the filesystem as a response.  
**Parameters**:

- `filepath` (string): Path to the file to send
- `content_type` (string): MIME type (auto-detected if None)  
**Returns**: Response object with file content

```python
send_data(data, filename, content_type="application/octet-stream") -> Response
```

**Purpose**: Sends raw data as a file download response.  
**Parameters**:

- `data` (string or bytes): Data to send
- `filename` (string): Suggested filename for download
- `content_type` (string): MIME type (default: "application/octet-stream")  
**Returns**: Response object with attachment headers

#### Authentication Helpers

```python
basic_auth(users={}, realm="Restricted") -> Authenticator
```

**Purpose**: Creates a basic HTTP authentication handler.  
**Parameters**:

- `users` (dict): Username -> password mapping
- `realm` (string): Authentication realm name  
**Returns**: Authenticator object with validation methods

```python
bearer_auth(validate_func) -> Authenticator
```

**Purpose**: Creates a bearer token authentication handler.  
**Parameters**:

- `validate_func` (function): Function to validate tokens, returns user info or None  
**Returns**: Authenticator object with validation methods

```python
api_key_auth(keys=[], header="X-API-Key") -> Authenticator
```

**Purpose**: Creates an API key authentication handler.  
**Parameters**:

- `keys` (list): List of valid API keys
- `header` (string): Header name for API key (default: "X-API-Key")  
**Returns**: Authenticator object with validation methods

## Handler Function Interfaces

### Request Handler Interface

```python
def handler(request) -> Response:
    """
    Standard request handler interface.
    
    Args:
        request: Request object containing all request information
    
    Returns:
        Response object with status, headers, and body
    """
```

### Middleware Interface

```python
def middleware(request, next_handler) -> Response:
    """
    Middleware function interface.
    
    See "Middleware System Design" section for detailed documentation and examples.
    """
```

### Error Handler Interface

```python
def error_handler(request) -> Response:
    """
    Error handler interface for specific status codes.
    
    Args:
        request: Request object that caused the error
    
    Returns:
        Response object for the error
    """
```

### Authentication Validator Interface

```python
def token_validator(token) -> dict or None:
    """
    Token validation function interface for bearer_auth.
    
    Args:
        token: Bearer token string
    
    Returns:
        User info dict if valid, None if invalid
    """
```

## Object APIs

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

# Middleware (redesigned for flexibility)
server.use(middleware_func)                             # Global middleware
server.use_for(path_pattern, middleware_func)           # Path-specific middleware

# Error handling
server.error_handler(status_codes, handler)             # status_codes can be int or list of ints

# Lifecycle
server.run()              # Blocking
server.start_background() # Non-blocking
server.stop()
server.is_running() -> bool
```

### Request Object (Available in handlers)

```python
# Properties (matching http module's ExportedServerRequest structure)
request.method          # HTTP method
request.url             # Full URL
request.path            # URL path
request.host            # Host header
request.remote          # Client address
request.client_ip       # Client IP address (extracted from headers/connection)
request.proto           # Protocol (HTTP/1.1)
request.headers         # Dict of headers
request.query           # Dict of query parameters
request.context         # Dict for storing middleware data

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

### SessionManager Object API

```python
# Main method
session_manager.get_session(request) -> Session         # Get session for request

# Session configuration
session_manager.configure(cookie_name, max_age, secure, http_only)
```

### Session Object (Returned by session_manager.get_session)

```python
# Properties
session.id              # Session ID
session.is_new          # New session flag

# Methods
session.get(key, default=None)
session.set(key, value)
session.delete(key)
session.clear()
session.save()          # Explicitly save session (automatic in most cases)
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

## Middleware System

The `web` module uses a flexible middleware system that allows you to intercept and modify requests and responses. Middleware functions form a chain where each middleware can:

1. **Inspect/modify the request** before passing it to the next handler
2. **Call the next handler** in the chain (or skip it)
3. **Inspect/modify the response** before returning it

### Middleware Function Interface

```python
def middleware_function(request, next_handler):
    """
    Middleware function interface.
    
    Args:
        request: The incoming HTTP request object
        next_handler: Function to call the next middleware or final route handler
    
    Returns:
        Response object (can be modified response from next_handler or completely new response)
    """
    # Pre-processing: modify request, check conditions, etc.
    
    # Call next handler in chain (route handler or next middleware)
    response = next_handler(request)
    
    # Post-processing: modify response, log, etc.
    
    return response
```

### Middleware Registration

```python
# Global middleware (applies to all routes)
server.use(middleware_function)

# Path-specific middleware (applies only to matching paths)
server.use_for("/api/*", auth_middleware)
server.use_for("/admin/*", admin_auth_middleware)
```

### Execution Order

Middleware executes in **registration order** for the request phase, and **reverse order** for the response phase:

```python
srv.use(middleware_1)  # Registered first
srv.use(middleware_2)  # Registered second
srv.use(middleware_3)  # Registered third

# Execution flow:
# Request: middleware_1 -> middleware_2 -> middleware_3 -> route_handler
# Response: route_handler -> middleware_3 -> middleware_2 -> middleware_1
```

### Middleware Patterns

#### 1. Request Inspection/Modification

```python
def logging_middleware(request, next_handler):
    """Log all incoming requests."""
    print("Request: {} {}".format(request.method, request.path))
    
    # Continue to next handler
    response = next_handler(request)
    
    print("Response: {}".format(response.status_code))
    return response

srv.use(logging_middleware)
```

#### 2. Response Modification

```python
def cors_middleware(request, next_handler):
    """Add CORS headers to all responses."""
    
    # Handle preflight requests
    if request.method == "OPTIONS":
        return response("", headers={
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type, Authorization"
        })
    
    # Process normal requests
    resp = next_handler(request)
    
    # Add CORS headers to response
    resp.headers["Access-Control-Allow-Origin"] = "*"
    resp.headers["Access-Control-Allow-Credentials"] = "true"
    
    return resp

srv.use(cors_middleware)
```

#### 3. Request Timing

```python
load("time")

def timing_middleware(request, next_handler):
    """Measure request processing time."""
    start_time = time.now()
    
    # Process request
    response = next_handler(request)
    
    # Calculate duration
    duration = time.since(start_time)
    response.headers["X-Response-Time"] = "{:.3f}ms".format(duration.milliseconds)
    
    return response

srv.use(timing_middleware)
```

#### 4. Authentication Middleware

```python
def auth_middleware(request, next_handler):
    """Require authentication for protected routes."""
    
    # Check for authorization header
    auth_header = request.get_header("Authorization")
    if auth_header == None:
        return error_response(401, "Authorization header required")
    
    # Validate token (simplified)
    if not auth_header.startswith("Bearer "):
        return error_response(401, "Invalid authorization format")
    
    token = auth_header[7:]  # Remove "Bearer " prefix
    user_info = validate_token(token)  # Your validation function
    
    if user_info == None:
        return error_response(401, "Invalid token")
    
    # Add user info to request context for route handlers
    request.context["user"] = user_info
    
    # Continue to next handler
    return next_handler(request)

# Apply only to API routes
srv.use_for("/api/*", auth_middleware)
```

#### 5. Early Return (Skip Route Handler)

```python
def maintenance_middleware(request, next_handler):
    """Return maintenance message during downtime."""
    
    # Check if in maintenance mode
    if is_maintenance_mode():
        return response(
            "Service temporarily unavailable",
            status=503,
            headers={"Retry-After": "3600"}
        )
    
    # Normal processing
    return next_handler(request)

srv.use(maintenance_middleware)
```

#### 6. Request Body Processing

```python
def json_parser_middleware(request, next_handler):
    """Pre-parse JSON for all POST/PUT requests."""
    
    if request.method in ["POST", "PUT"] and request.get_header("Content-Type") == "application/json":
        try:
            # Parse JSON and store in context
            json_data = request.json()
            if json_data != None:
                request.context["json"] = json_data
            else:
                return error_response(400, "Invalid JSON")
        except:
            return error_response(400, "Malformed JSON")
    
    return next_handler(request)

srv.use(json_parser_middleware)
```

#### 7. Rate Limiting

```python
def rate_limit_middleware(request, next_handler):
    """Simple rate limiting by IP address."""
    
    client_ip = request.client_ip
    current_time = time.now().unix
    
    # Get request count for this IP (simplified - use proper storage in production)
    request_count = get_request_count(client_ip, current_time)
    
    if request_count > 100:  # 100 requests per minute
        return error_response(429, "Rate limit exceeded")
    
    # Increment counter
    increment_request_count(client_ip, current_time)
    
    return next_handler(request)

srv.use_for("/api/*", rate_limit_middleware)
```

### Path-Specific Middleware

You can apply middleware only to specific path patterns:

```python
# Authentication only for admin routes
srv.use_for("/admin/*", admin_auth_middleware)

# Rate limiting only for API
srv.use_for("/api/*", rate_limit_middleware)

# Special handling for webhooks
srv.use_for("/webhooks/*", webhook_validation_middleware)

# Multiple middlewares for the same path
srv.use_for("/api/v1/*", api_v1_middleware)
srv.use_for("/api/v1/*", api_v1_auth_middleware)
```

### Middleware Execution Example

```python
def main():
    srv = create_server(port=8080)
    
    # Global middleware (applies to all routes)
    srv.use(logging_middleware)     # 1st: logs request
    srv.use(timing_middleware)      # 2nd: measures time
    srv.use(cors_middleware)        # 3rd: adds CORS headers
    
    # Path-specific middleware
    srv.use_for("/api/*", auth_middleware)  # Only for /api/* routes
    
    def api_handler(request):
        # request.context["user"] is available here (set by auth_middleware)
        user = request.context.get("user")
        return json_response({"message": "Hello {}".format(user["name"])})
    
    srv.get("/api/data", api_handler)
    srv.run()

# Execution flow for GET /api/data:
# 1. logging_middleware (logs request)
# 2. timing_middleware (starts timer)
# 3. cors_middleware (handles CORS)
# 4. auth_middleware (validates auth, sets user context)
# 5. api_handler (actual route handler)
# 6. auth_middleware (response phase - usually just passes through)
# 7. cors_middleware (adds CORS headers to response)
# 8. timing_middleware (adds timing header)
# 9. logging_middleware (logs response)
```

### Advanced Middleware Patterns

#### Conditional Middleware

```python
def conditional_middleware(request, next_handler):
    """Apply different logic based on request properties."""
    
    if request.path.startswith("/api/"):
        # API-specific processing
        if request.get_header("Content-Type") != "application/json":
            return error_response(400, "API requires JSON")
    
    response = next_handler(request)
    
    if request.path.startswith("/api/"):
        # Ensure API responses are JSON
        if "Content-Type" not in response.headers:
            response.headers["Content-Type"] = "application/json"
    
    return response
```

#### Error Handling Middleware

```python
def error_handling_middleware(request, next_handler):
    """Catch and handle errors from downstream handlers."""
    
    try:
        return next_handler(request)
    except Exception as e:
        # Log error
        print("Error processing request: {}".format(str(e)))
        
        # Return user-friendly error
        return error_response(500, "Internal server error")
```

#### Session Middleware

```python
def session_middleware(session_manager):
    """Create middleware that provides session access."""
    
    def middleware(request, next_handler):
        # Get session for this request
        session = session_manager.get_session(request)
        
        # Make session available in request context
        request.context["session"] = session
        
        # Process request
        response = next_handler(request)
        
        # Save session changes
        session.save()
        
        return response
    
    return middleware

# Usage
session_mgr = create_session_manager(secret="my-secret")
srv.use(session_middleware(session_mgr))
```

### Middleware Best Practices

1. **Order Matters**: Register middleware in logical order (auth before business logic)
2. **Always Call Next**: Unless intentionally stopping the chain
3. **Use Context**: Store data in `request.context` for downstream handlers
4. **Handle Errors**: Consider what happens if next_handler fails
5. **Keep It Focused**: Each middleware should have a single responsibility
6. **Performance**: Avoid heavy operations in frequently-used middleware

## Built-in Preset Middleware

The `web` module provides a comprehensive set of ready-to-use middleware for common web development needs:

### Security Middleware

#### 1. CORS Middleware

```python
load("web", "cors_middleware")

# Simple CORS (allow all origins)
srv.use(cors_middleware())

# Configured CORS
srv.use(cors_middleware(
    origins=["https://example.com", "https://app.example.com"],
    methods=["GET", "POST", "PUT", "DELETE"],
    headers=["Content-Type", "Authorization"],
    credentials=True,
    max_age=86400
))
```

#### 2. Security Headers Middleware

```python
load("web", "security_headers_middleware")

srv.use(security_headers_middleware(
    frame_options="DENY",                    # X-Frame-Options
    content_type_options="nosniff",          # X-Content-Type-Options
    xss_protection="1; mode=block",          # X-XSS-Protection
    hsts="max-age=31536000; includeSubDomains",  # Strict-Transport-Security
    csp="default-src 'self'",                # Content-Security-Policy
    referrer_policy="strict-origin-when-cross-origin"
))
```

#### 3. Rate Limiting Middleware

```python
load("web", "rate_limit_middleware")

# Simple rate limiting (100 requests per minute per IP)
srv.use(rate_limit_middleware(
    requests=100,
    window=60,
    key_func=lambda req: req.client_ip
))

# Advanced rate limiting with custom storage
srv.use(rate_limit_middleware(
    requests=1000,
    window=3600,
    key_func=lambda req: req.get_header("X-API-Key", req.client_ip),
    storage=redis_storage,  # Custom storage backend
    skip_func=lambda req: req.path.startswith("/health")
))
```

#### 4. Request Size Middleware

```python
load("web", "request_size_middleware")

srv.use(request_size_middleware(
    max_content_length=50 * 1024 * 1024,  # 50MB limit
    max_url_length=2048,                   # URL length limit
    max_headers=100                        # Header count limit
))
```

### Performance Middleware

#### 5. Compression Middleware

```python
load("web", "compression_middleware")

srv.use(compression_middleware(
    level=6,                          # Compression level (1-9)
    min_size=1024,                    # Only compress responses > 1KB
    types=["text/html", "text/css", "application/javascript", "application/json"]
))
```

#### 6. Caching Middleware

```python
load("web", "cache_middleware")

# Static file caching
srv.use(cache_middleware(
    max_age=3600,                     # 1 hour cache
    private=False,                    # Public cache
    patterns=["/static/*", "/assets/*"]
))

# API response caching
srv.use(cache_middleware(
    max_age=300,                      # 5 minutes
    vary=["Authorization"],           # Vary by auth header
    patterns=["/api/public/*"]
))
```

#### 7. Request Timeout Middleware

```python
load("web", "timeout_middleware")

srv.use(timeout_middleware(
    timeout=30,                       # 30 second timeout
    message="Request timeout"
))
```

### Observability Middleware

#### 8. Logging Middleware

```python
load("web", "logging_middleware")

# Basic request logging
srv.use(logging_middleware())

# Detailed logging with custom format
srv.use(logging_middleware(
    format='"{method} {path} {status} {duration}ms {size}bytes"',
    include_headers=["User-Agent", "X-Forwarded-For"],
    skip_paths=["/health", "/metrics"]
))
```

#### 9. Metrics Middleware

```python
load("web", "metrics_middleware")

srv.use(metrics_middleware(
    track_requests=True,
    track_duration=True,
    track_status_codes=True,
    track_routes=True,
    prometheus_format=True
))

# Access metrics endpoint
srv.get("/metrics", metrics_endpoint)
```

#### 10. Request ID Middleware

```python
load("web", "request_id_middleware")

srv.use(request_id_middleware(
    header="X-Request-ID",
    generator=lambda: generate_uuid4()  # Custom ID generator
))
```

#### 11. Request Timing Middleware

```python
load("web", "timing_middleware")

srv.use(timing_middleware(
    header="X-Response-Time",         # Header name for timing
    precision=3                       # Decimal places
))
```

### Development Middleware

#### 12. Error Handling Middleware

```python
load("web", "error_middleware")

srv.use(error_middleware(
    debug=True,                       # Show stack traces in development
    log_errors=True,                  # Log all errors
    custom_handlers={
        404: lambda req: json_response({"error": "Not found"}, status=404),
        500: lambda req: json_response({"error": "Server error"}, status=500)
    }
))
```

#### 13. Debug Middleware

```python
load("web", "debug_middleware")

# Only in development
if runtime.getenv("ENV") == "development":
    srv.use(debug_middleware(
        show_request_info=True,       # Log request details
        show_response_info=True,      # Log response details
        measure_memory=True,          # Track memory usage
        profile_routes=True           # Performance profiling
    ))
```

### Content Processing Middleware

#### 14. JSON Parser Middleware

```python
load("web", "json_middleware")

srv.use(json_middleware(
    strict=True,                      # Strict JSON parsing
    max_size=10 * 1024 * 1024,       # 10MB JSON limit
    error_handler=lambda req, err: error_response(400, "Invalid JSON")
))
```

#### 15. Form Parser Middleware

```python
load("web", "form_middleware")

srv.use(form_middleware(
    max_size=50 * 1024 * 1024,       # 50MB form limit
    max_files=10,                     # Max uploaded files
    allowed_types=["image/jpeg", "image/png", "application/pdf"]
))
```

### Authentication & Authorization Middleware

#### 16. JWT Middleware

```python
load("web", "jwt_middleware")

srv.use_for("/api/*", jwt_middleware(
    secret="your-jwt-secret",
    algorithm="HS256",
    required_claims=["sub", "exp"],
    audience="your-api",
    issuer="your-service"
))
```

#### 17. API Key Middleware

```python
load("web", "api_key_middleware")

srv.use_for("/api/*", api_key_middleware(
    header="X-API-Key",
    query_param="api_key",
    validator=lambda key: validate_api_key(key),
    required_scopes=["read", "write"]
))
```

#### 18. Session Middleware

```python
load("web", "session_middleware")

session_mgr = create_session_manager(secret="session-secret")
srv.use(session_middleware(
    session_manager=session_mgr,
    cookie_name="session",
    max_age=86400,
    secure=True,
    http_only=True
))
```

### Utility Middleware

#### 19. Redirect Middleware

```python
load("web", "redirect_middleware")

# Redirect HTTP to HTTPS
srv.use(redirect_middleware(
    redirects={
        "http://example.com": "https://example.com",
        "/old-path": "/new-path"
    },
    status=301
))
```

#### 20. Maintenance Middleware

```python
load("web", "maintenance_middleware")

srv.use(maintenance_middleware(
    enabled=lambda: check_maintenance_mode(),
    message="Service temporarily unavailable",
    retry_after=3600,
    bypass_paths=["/health", "/admin/maintenance"]
))
```

### Usage Patterns

#### Recommended Middleware Stack

```python
def main():
    srv = create_server(port=8080)
    
    # 1. Request processing & security (early)
    srv.use(request_id_middleware())
    srv.use(security_headers_middleware())
    srv.use(rate_limit_middleware(requests=1000, window=3600))
    srv.use(request_size_middleware(max_content_length=10*1024*1024))
    
    # 2. Observability
    srv.use(logging_middleware())
    srv.use(timing_middleware())
    srv.use(metrics_middleware())
    
    # 3. Content processing
    srv.use(compression_middleware())
    srv.use(json_middleware())
    srv.use(form_middleware())
    
    # 4. CORS (after content processing)
    srv.use(cors_middleware(origins=["https://app.example.com"]))
    
    # 5. Authentication (path-specific)
    srv.use_for("/api/*", jwt_middleware(secret="jwt-secret"))
    srv.use_for("/admin/*", session_middleware(session_mgr))
    
    # 6. Error handling (last)
    srv.use(error_middleware(debug=False))
    
    # Routes
    srv.get("/api/data", api_handler)
    srv.run()
```

#### Development vs Production

```python
def setup_middleware(srv, env="production"):
    # Always enabled
    srv.use(request_id_middleware())
    srv.use(logging_middleware())
    srv.use(security_headers_middleware())
    
    if env == "development":
        srv.use(debug_middleware())
        srv.use(error_middleware(debug=True))
        srv.use(cors_middleware())  # Permissive CORS
    else:
        srv.use(rate_limit_middleware(requests=100, window=60))
        srv.use(compression_middleware())
        srv.use(error_middleware(debug=False))
        srv.use(cors_middleware(origins=["https://yourdomain.com"]))
```

### Custom Middleware Factory Pattern

```python
def create_auth_middleware(auth_service):
    """Factory function for creating authentication middleware."""
    
    def auth_middleware(request, next_handler):
        token = request.bearer_token()
        if not token:
            return error_response(401, "Authentication required")
        
        user = auth_service.validate_token(token)
        if not user:
            return error_response(401, "Invalid token")
        
        request.context["user"] = user
        return next_handler(request)
    
    return auth_middleware

# Usage
auth_service = AuthService(database=db)
srv.use_for("/api/*", create_auth_middleware(auth_service))
```

### Middleware Development Guidelines

#### Creating Custom Middleware

```python
def create_custom_middleware(config):
    """Template for creating reusable middleware."""
    
    def middleware(request, next_handler):
        # 1. Pre-processing (request validation, modification)
        if not validate_request(request, config):
            return error_response(400, "Invalid request")
        
        # 2. Add data to context for downstream handlers
        request.context["custom_data"] = process_request(request)
        
        # 3. Call next handler
        response = next_handler(request)
        
        # 4. Post-processing (response modification, logging)
        response = modify_response(response, config)
        
        return response
    
    return middleware

# Usage
srv.use(create_custom_middleware({"option": "value"}))
```

#### Middleware Categories & Order

```python
# Recommended order for maximum compatibility:

# 1. INFRASTRUCTURE (request setup)
srv.use(request_id_middleware())
srv.use(request_size_middleware())

# 2. SECURITY (early filtering)  
srv.use(security_headers_middleware())
srv.use(rate_limit_middleware())

# 3. OBSERVABILITY (logging/monitoring)
srv.use(logging_middleware())
srv.use(timing_middleware())
srv.use(metrics_middleware())

# 4. CONTENT PROCESSING (parsing)
srv.use(compression_middleware())
srv.use(json_middleware())
srv.use(form_middleware())

# 5. CROSS-ORIGIN (after content processing)
srv.use(cors_middleware())

# 6. AUTHENTICATION (path-specific)
srv.use_for("/api/*", jwt_middleware())
srv.use_for("/admin/*", session_middleware())

# 7. BUSINESS LOGIC (routes)
# Your route handlers here

# 8. ERROR HANDLING (catch-all)
srv.use(error_middleware())
```

## Usage Examples

### 1. Basic Web Server

```python
load("web", "create_server", "response", "json_response")

def main():
    srv = create_server(host="0.0.0.0", port=8080)
    
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
        return html_response(html)
    
    srv.get("/", home)
    srv.get("/about", about)
    srv.route(["GET", "HEAD"], "/api/info", api_info)
    
    print("Server starting on http://{}:{}".format("0.0.0.0", 8080))
    srv.run()

main()
```

### 2. RESTful API with CRUD Operations

```python
load("web", "create_server", "json_response", "error_response")
load("time")

def main():
    srv = create_server(port=8080)
    
    # In-memory database (use shared_dict for thread safety)
    users = shared_dict()
    next_id = [1]  # Use list to allow modification
    
    # List all users
    def list_users(req):
        user_list = [users[user_id] for user_id in users]
        return json_response(user_list)
    
    # Get single user
    def get_user(req):
        user_id_str = req.param("id")
        if user_id_str == None:
            return error_response(400, "User ID required")
        
        user = users.get(int(user_id_str))
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
            "id": next_id[0],
            "name": name,
            "email": email,
            "created_at": time.now().format(time.RFC3339)
        }
        users[next_id[0]] = user
        next_id[0] = next_id[0] + 1
        
        return json_response(user, status=201)
    
    # Update user
    def update_user(req):
        user_id_str = req.param("id")
        if user_id_str == None:
            return error_response(400, "User ID required")
        
        user_id = int(user_id_str)
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
        users[user_id] = user
        return json_response(user)
    
    # Delete user
    def delete_user(req):
        user_id_str = req.param("id")
        if user_id_str == None:
            return error_response(400, "User ID required")
        
        user_id = int(user_id_str)
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
load("web", "create_server", "basic_auth", "response", "json_response")

def main():
    srv = create_server(port=8080)
    
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
load("web", "create_server", "bearer_auth", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    # Valid tokens (in production, check against database)
    valid_tokens = shared_dict({
        "token-123": {"user": "alice", "role": "admin"},
        "token-456": {"user": "bob", "role": "user"}
    })
    
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

### 4. File Upload and Static Files

```python
load("web", "create_server", "response", "json_response", "send_file", "send_data")
load("time")

def main():
    srv = create_server(port=8080)
    
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
        return html_response(html)
    
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
    
    # Generate and send data as file
    def generate_csv(req):
        csv_data = "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n"
        return send_data(csv_data, "users.csv", "text/csv")
    
    # Setup routes
    srv.get("/", upload_form)
    srv.post("/upload", handle_upload)
    srv.get("/download/{filename}", download)
    srv.get("/generate/csv", generate_csv)
    
    # Serve static files
    srv.static("/static", "./static")
    srv.static("/uploads", "./uploads")
    
    # SPA support
    srv.spa("/app", "./dist", fallback="index.html")
    
    srv.run()

main()
```

### 5. Advanced Routing and Route Groups

```python
load("web", "create_server", "json_response", "basic_auth")

def main():
    srv = create_server(port=8080)
    
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
    
    # Multiple error handlers
    def not_found_handler(req):
        return json_response({"error": "Not Found"}, status=404)
    
    def server_error_handler(req):
        return json_response({"error": "Server Error"}, status=500)
    
    # Register error handlers for multiple status codes
    srv.error_handler([404, 405], not_found_handler)  # Handle both 404 and 405
    srv.error_handler(500, server_error_handler)
    
    srv.run()

main()
```

## Complex Examples

For more advanced usage examples, see the separate example files:

- **[examples/blog_app.star](examples/blog_app.star)**: Complete blog application with admin functionality, sessions, and CRUD operations
- **[examples/session_management.star](examples/session_management.star)**: Session handling, user login/logout, and state management
- **[examples/middleware_auth.star](examples/middleware_auth.star)**: Advanced middleware patterns and authentication systems

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
├── examples/           # Example Starlark files
│   ├── blog_app.star
│   ├── session_management.star
│   └── middleware_auth.star
└── README.md           # User documentation
```

### Core Components

#### 1. Server Structure

```go
type Server struct {
    config         *Config
    httpServer     *http.Server
    router         *Router
    middleware     []MiddlewareFunc
    sessionManager *SessionManager
    running        atomic.Bool
    mu             sync.RWMutex
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
srv = create_server(port=8080)
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
session_manager = create_session_manager(secret="key")
srv = create_server(session_manager=session_manager)

def protected(request):
    session = session_manager.get_session(request)
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
4. **Enable compression**: For text-based responses
5. **Use shared_dict**: For thread-safe shared state

## Migration Guide from Flask

| Flask | Starlark Web |
|-------|--------------|
| `app = Flask(__name__)` | `srv = create_server()` |
| `@app.route("/")` | `srv.get("/", handler)` |
| `f"Hello {name}"` | `"Hello {}".format(name)` |
| `try/except` | Use `fail()` or check for `None` |
| `if x is None` | `if x == None` |
| `request` global | `request` parameter in handler |
| `session` global | `session_manager.get_session(request)` |

## Success Metrics

1. **Performance**: Handle 10,000+ requests/second on modest hardware
2. **Memory**: <100MB memory usage under normal load
3. **Latency**: <10ms response time for simple routes
4. **Reliability**: 99.9% uptime in production deployments
5. **Adoption**: Easy migration from Flask/Express applications
