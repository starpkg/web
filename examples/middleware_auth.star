# Middleware and Authentication Demo
# This example demonstrates various middleware and authentication features

load("web", "create_server", "html_response", "json_response", "error_response", "basic_auth", "bearer_auth", "api_key_auth", "cors_middleware", "logging_middleware", "compression_middleware", "rate_limit_middleware", "security_headers_middleware", "timing_middleware")

def main():
    srv = create_server(port=8080, server_header="Auth-Demo/1.0")
    
    # Setup authentication providers
    basic_auth_provider = basic_auth(users={"admin": "secret", "user": "password"}, realm="Demo")
    
    # Bearer token validation function
    def validate_bearer_token(token):
        token_map = {"token123": "admin", "token456": "user"}
        return token_map.get(token, None)
    
    bearer_auth_provider = bearer_auth(validate_func=validate_bearer_token)
    
    # API Key authentication with list of keys
    api_key_provider = api_key_auth(keys=["key123", "key456"], header="X-API-Key")
    
    # Global middleware
    srv.use(logging_middleware())
    srv.use(cors_middleware())
    srv.use(compression_middleware())
    srv.use(security_headers_middleware())
    srv.use(timing_middleware())
    
    # Rate limiting for API endpoints
    srv.use_for("/api/*", rate_limit_middleware(requests=100, window=60))
    
    # Custom middleware example
    def custom_header_middleware():
        def middleware(req, next):
            # Add custom header to all responses
            resp = next(req)
            resp.set_header("X-Custom-Server", "Starlark-Web")
            return resp
        return middleware
    
    srv.use(custom_header_middleware())
    
    # PUBLIC ROUTES
    def home(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Middleware & Authentication Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .section { margin: 20px 0; padding: 20px; border: 1px solid #ddd; }
                .auth-type { margin: 10px 0; padding: 10px; background: #f5f5f5; }
                pre { background: #f0f0f0; padding: 10px; overflow-x: auto; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <h1>Middleware & Authentication Demo</h1>
            
            <div class="nav">
                <a href="/">Home</a>
                <a href="/public">Public</a>
                <a href="/basic-protected">Basic Auth</a>
                <a href="/bearer-protected">Bearer Auth</a>
                <a href="/api-key-protected">API Key</a>
                <a href="/api/public">Public API</a>
            </div>
            
            <div class="section">
                <h2>Authentication Types</h2>
                
                <div class="auth-type">
                    <h3>Basic Authentication</h3>
                    <p>Username/Password: admin/secret or user/password</p>
                    <p><a href="/basic-protected">Try Basic Auth Protected Route</a></p>
                    <pre>curl -u admin:secret http://localhost:8080/basic-protected</pre>
                </div>
                
                <div class="auth-type">
                    <h3>Bearer Token</h3>
                    <p>Tokens: token123 (admin) or token456 (user)</p>
                    <p><a href="/bearer-protected">Try Bearer Protected Route</a></p>
                    <pre>curl -H "Authorization: Bearer token123" http://localhost:8080/bearer-protected</pre>
                </div>
                
                <div class="auth-type">
                    <h3>API Key</h3>
                    <p>API Keys: key123 or key456</p>
                    <p><a href="/api-key-protected">Try API Key Protected Route</a></p>
                    <pre>curl -H "X-API-Key: key123" http://localhost:8080/api-key-protected</pre>
                </div>
            </div>
            
            <div class="section">
                <h2>Middleware Features</h2>
                <ul>
                    <li><strong>Logging:</strong> All requests are logged</li>
                    <li><strong>CORS:</strong> Cross-origin requests enabled</li>
                    <li><strong>Compression:</strong> Responses are compressed</li>
                    <li><strong>Security Headers:</strong> Common security headers added</li>
                    <li><strong>Timing:</strong> Response timing information</li>
                    <li><strong>Rate Limiting:</strong> API endpoints limited to 100 requests/minute</li>
                    <li><strong>Custom Headers:</strong> X-Custom-Server header added</li>
                </ul>
            </div>
        </body>
        </html>
        """)
    
    def public_page(req):
        return html_response("""
        <html>
        <head><title>Public Page</title></head>
        <body>
            <h1>Public Page</h1>
            <p>This page is accessible without authentication.</p>
            <p><a href="/">Back to Home</a></p>
        </body>
        </html>
        """)
    
    def public_api(req):
        return json_response({
            "message": "This is a public API endpoint",
            "timestamp": "2024-01-01T12:00:00Z",
            "middleware_features": [
                "logging",
                "cors",
                "compression",
                "security_headers",
                "timing",
                "rate_limiting",
                "custom_headers"
            ]
        })
    
    # PROTECTED ROUTES - each with different auth middleware
    def basic_protected(req):
        # This route is protected by basic auth middleware
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        return html_response("""
        <html>
        <head><title>Basic Auth Protected</title></head>
        <body>
            <h1>Basic Auth Protected Page</h1>
            <p>Welcome, {}!</p>
            <p>You successfully authenticated using Basic Authentication.</p>
            <p><a href="/">Back to Home</a></p>
        </body>
        </html>
        """.format(username))
    
    def bearer_protected(req):
        # This route is protected by bearer auth middleware
        token_info = req.bearer_token()
        username = token_info if token_info != None else "unknown"
        
        return html_response("""
        <html>
        <head><title>Bearer Auth Protected</title></head>
        <body>
            <h1>Bearer Token Protected Page</h1>
            <p>Welcome, {}!</p>
            <p>You successfully authenticated using Bearer Token.</p>
            <p><a href="/">Back to Home</a></p>
        </body>
        </html>
        """.format(username))
    
    def api_key_protected(req):
        # This route is protected by API key auth middleware
        api_key_info = req.get_header("X-API-Key")
        username = api_key_info if api_key_info != None else "unknown"
        
        return html_response("""
        <html>
        <head><title>API Key Protected</title></head>
        <body>
            <h1>API Key Protected Page</h1>
            <p>Welcome, {}!</p>
            <p>You successfully authenticated using API Key.</p>
            <p><a href="/">Back to Home</a></p>
        </body>
        </html>
        """.format(username))
    
    def protected_api(req):
        # This route is protected by bearer auth middleware
        token_info = req.bearer_token()
        username = token_info if token_info != None else "unknown"
        
        return json_response({
            "message": "This is a protected API endpoint",
            "authenticated_user": username,
            "auth_method": "bearer_token",
            "timestamp": "2024-01-01T12:00:00Z"
        })
    
    # REGISTER PUBLIC ROUTES
    srv.get("/", home)
    srv.get("/public", public_page)
    srv.get("/api/public", public_api)
    
    # REGISTER PROTECTED ROUTES with different auth middleware
    srv.use_for("/basic-protected", basic_auth_provider.middleware())
    srv.get("/basic-protected", basic_protected)
    
    srv.use_for("/bearer-protected", bearer_auth_provider.middleware())
    srv.get("/bearer-protected", bearer_protected)
    
    srv.use_for("/api-key-protected", api_key_provider.middleware())
    srv.get("/api-key-protected", api_key_protected)
    
    # Protected API with bearer auth
    srv.use_for("/api/protected", bearer_auth_provider.middleware())
    srv.get("/api/protected", protected_api)
    
    print("Middleware & Authentication Demo server running on http://localhost:8080")
    print("")
    print("Public endpoints:")
    print("  GET / - Home page with auth examples")
    print("  GET /public - Public page (no auth)")
    print("  GET /api/public - Public API endpoint")
    print("")
    print("Protected endpoints:")
    print("  GET /basic-protected - Basic auth (admin/secret or user/password)")
    print("  GET /bearer-protected - Bearer token (token123 or token456)")
    print("  GET /api-key-protected - API key (key123 or key456 via X-API-Key header)")
    print("  GET /api/protected - Protected API (bearer token required)")
    print("")
    print("Active middleware: logging, cors, compression, security_headers, timing, rate_limiting, custom_headers")
    
    srv.run()

main() 