# Middleware and Authentication Demo
# This example demonstrates various middleware and authentication features

load("web", "create_server", "json_response", "html_response", "error_response", "basic_auth", "bearer_auth", "api_key_auth",
     "cors_middleware", "rate_limit_middleware", "compression_middleware", "security_headers_middleware", 
     "logging_middleware", "timing_middleware", "request_size_middleware", "cache_middleware")
load("time", "now")

def main():
    srv = create_server(port=8080, server_header="Demo-Server/1.0")
    
    # Create authentication systems
    basic_auth_obj = basic_auth(users={"admin": "secret", "user": "pass"}, realm="Demo Area")
    api_auth = api_key_auth(keys=["key123", "key456"], header="X-API-Key")
    
    # Token validation function for bearer auth
    def validate_token(token):
        valid_tokens = {
            "bearer123": {"user": "admin", "role": "admin"},
            "bearer456": {"user": "user", "role": "user"}
        }
        return valid_tokens.get(token)
    
    bearer_auth_obj = bearer_auth(validate_func=validate_token)
    
    # Create middleware
    cors_mw = cors_middleware(
        origins=["http://localhost:3000", "https://example.com"],
        methods=["GET", "POST", "PUT", "DELETE"],
        headers=["Content-Type", "Authorization", "X-API-Key"],
        credentials=True
    )
    
    rate_limit_mw = rate_limit_middleware(
        requests=10,
        window=60  # 10 requests per minute
    )
    
    compression_mw = compression_middleware(
        level=6,
        min_size=1024,
        types=["text/html", "application/json", "text/css", "text/javascript"]
    )
    
    security_mw = security_headers_middleware(
        frame_options="DENY",
        content_type_options="nosniff",
        xss_protection="1; mode=block",
        hsts="max-age=31536000",
        csp="default-src 'self'",
        referrer_policy="strict-origin"
    )
    
    logging_mw = logging_middleware(
        format="{method} {path} - {status} ({duration}ms) - {user_agent}"
    )
    
    timing_mw = timing_middleware(header="X-Response-Time")
    
    size_limit_mw = request_size_middleware(
        max_content_length=1024 * 1024,  # 1MB
        max_url_length=2048,
        max_headers=50
    )
    
    cache_mw = cache_middleware(
        max_age=3600,  # 1 hour
        private=False,
        patterns=["/api/cache-me", "/static/*"]
    )
    
    # Custom middleware examples
    def custom_timing_middleware(req, next_handler):
        start_time = now()
        
        resp = next_handler(req)
        
        end_time = now()
        duration = end_time.unix() - start_time.unix()
        
        resp.set_header("X-Custom-Timer", "{}s".format(duration))
        print("Custom timing: {} took {}s".format(req.path, duration))
        
        return resp
    
    def request_id_middleware(req, next_handler):
        request_id = "req-{}".format(now().unix())
        
        print("Processing request {} for {}".format(request_id, req.path))
        
        resp = next_handler(req)
        resp.set_header("X-Request-ID", request_id)
        
        return resp
    
    def api_version_middleware(req, next_handler):
        if req.path.startswith("/api/"):
            resp = next_handler(req)
            resp.set_header("X-API-Version", "1.0")
            return resp
        return next_handler(req)
    
    # Apply global middleware
    srv.use(logging_mw)
    srv.use(timing_mw)
    srv.use(cors_mw)
    srv.use(security_mw)
    srv.use(compression_mw)
    srv.use(request_id_middleware)
    
    # Apply specific middleware to API routes
    srv.use_for("/api/*", size_limit_mw)
    srv.use_for("/api/*", api_version_middleware)
    srv.use_for("/api/limited/*", rate_limit_mw)
    srv.use_for("/api/cache-me", cache_mw)
    
    # Apply authentication to specific routes
    srv.use_for("/admin/*", basic_auth_obj.middleware())
    srv.use_for("/api/secure-basic/*", basic_auth_obj.middleware())
    srv.use_for("/api/secure-bearer/*", bearer_auth_obj.middleware())
    srv.use_for("/api/secure-key/*", api_auth.middleware())
    
    # Routes
    def home(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Middleware & Auth Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .section { margin-bottom: 30px; padding: 20px; border: 1px solid #ddd; }
                .endpoint { margin-bottom: 10px; padding: 10px; background: #f9f9f9; }
                .method { font-weight: bold; color: #007bff; }
                .auth { color: #dc3545; }
                .middleware { color: #28a745; }
                pre { background: #f8f9fa; padding: 10px; border-radius: 4px; }
            </style>
        </head>
        <body>
            <h1>Middleware & Authentication Demo</h1>
            
            <div class="section">
                <h2>Public Endpoints</h2>
                <div class="endpoint">
                    <span class="method">GET</span> / - This page
                </div>
                <div class="endpoint">
                    <span class="method">GET</span> /api/public - Public API endpoint
                </div>
                <div class="endpoint">
                    <span class="method">GET</span> /api/cache-me - <span class="middleware">Cached response</span>
                </div>
                <div class="endpoint">
                    <span class="method">POST</span> /api/limited/test - <span class="middleware">Rate limited</span>
                </div>
            </div>
            
            <div class="section">
                <h2>Basic Auth Protected</h2>
                <p><span class="auth">Credentials: admin/secret or user/pass</span></p>
                <div class="endpoint">
                    <span class="method">GET</span> /admin/dashboard - Basic auth protected
                </div>
                <div class="endpoint">
                    <span class="method">GET</span> /api/secure-basic/data - Basic auth API
                </div>
            </div>
            
            <div class="section">
                <h2>Bearer Token Protected</h2>
                <p><span class="auth">Tokens: bearer123 or bearer456</span></p>
                <div class="endpoint">
                    <span class="method">GET</span> /api/secure-bearer/profile - Bearer token protected
                </div>
            </div>
            
            <div class="section">
                <h2>API Key Protected</h2>
                <p><span class="auth">Headers: X-API-Key: key123 or key456</span></p>
                <div class="endpoint">
                    <span class="method">GET</span> /api/secure-key/data - API key protected
                </div>
            </div>
            
            <div class="section">
                <h2>Test Commands</h2>
                <pre>
# Basic auth
curl -u admin:secret http://localhost:8080/admin/dashboard

# Bearer token
curl -H "Authorization: Bearer bearer123" http://localhost:8080/api/secure-bearer/profile

# API key
curl -H "X-API-Key: key123" http://localhost:8080/api/secure-key/data

# Rate limited (try multiple times)
curl -X POST http://localhost:8080/api/limited/test
                </pre>
            </div>
        </body>
        </html>
        """)
    
    def public_api(req):
        return json_response({
            "message": "Public API endpoint",
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "method": req.method,
            "headers": dict(req.headers),
            "middleware_applied": ["logging", "timing", "cors", "security", "compression", "custom_request_id"]
        })
    
    def cached_endpoint(req):
        return json_response({
            "message": "This response is cached",
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "cache_headers": "Check response headers for cache control"
        })
    
    def rate_limited_endpoint(req):
        return json_response({
            "message": "Rate limited endpoint",
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "rate_limit": "10 requests per minute"
        })
    
    def admin_dashboard(req):
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Admin Dashboard</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .welcome { background: #d4edda; padding: 15px; border-radius: 4px; margin-bottom: 20px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <div class="welcome">
                <h1>Welcome, {}!</h1>
                <p>You are authenticated via Basic Auth</p>
            </div>
            
            <div class="nav">
                <a href="/">Home</a>
                <a href="/admin/users">Manage Users</a>
                <a href="/admin/settings">Settings</a>
            </div>
            
            <p>This is a protected admin area accessible only with valid credentials.</p>
        </body>
        </html>
        """.format(username))
    
    def secure_basic_api(req):
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        return json_response({
            "message": "Basic auth protected API",
            "authenticated_user": username,
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "auth_type": "basic"
        })
    
    def secure_bearer_api(req):
        token = req.bearer_token()
        user_info = validate_token(token) if token != None else None
        
        return json_response({
            "message": "Bearer token protected API",
            "user_info": user_info,
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "auth_type": "bearer"
        })
    
    def secure_key_api(req):
        api_key = req.get_header("X-API-Key")
        
        return json_response({
            "message": "API key protected endpoint",
            "api_key": api_key,
            "timestamp": now().format("2006-01-02T15:04:05Z"),
            "auth_type": "api_key"
        })
    
    def custom_middleware_demo(req):
        return json_response({
            "message": "Custom middleware demo",
            "custom_middlewares": [
                "request_id_middleware - adds X-Request-ID header",
                "custom_timing_middleware - adds X-Custom-Timer header",
                "api_version_middleware - adds X-API-Version header"
            ],
            "timestamp": now().format("2006-01-02T15:04:05Z")
        })
    
    # Register routes
    srv.get("/", home)
    srv.get("/api/public", public_api)
    srv.get("/api/cache-me", cached_endpoint)
    srv.post("/api/limited/test", rate_limited_endpoint)
    srv.get("/api/custom-middleware", custom_middleware_demo)
    
    # Protected routes
    srv.get("/admin/dashboard", admin_dashboard)
    srv.get("/api/secure-basic/data", secure_basic_api)
    srv.get("/api/secure-bearer/profile", secure_bearer_api)
    srv.get("/api/secure-key/data", secure_key_api)
    
    # Apply custom middleware to specific route
    srv.use_for("/api/custom-middleware", custom_timing_middleware)
    
    print("Middleware & Auth Demo running on http://localhost:8080")
    print("")
    print("Authentication:")
    print("  Basic Auth: admin/secret or user/pass")
    print("  Bearer Token: bearer123 or bearer456")
    print("  API Key: key123 or key456")
    print("")
    print("Middleware features:")
    print("  - CORS support")
    print("  - Rate limiting (10 req/min on /api/limited/*)")
    print("  - Compression (gzip)")
    print("  - Security headers")
    print("  - Request/response logging")
    print("  - Response timing")
    print("  - Request size limits")
    print("  - Response caching")
    print("  - Custom middleware examples")
    
    srv.run()

main() 