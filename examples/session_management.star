# Authentication Demo
# This example demonstrates various authentication methods available in the web module
# Note: Session management is not implemented in this version, so we use in-memory storage

load("web", "create_server", "json_response", "html_response", "error_response", "redirect",
     "basic_auth", "bearer_auth", "api_key_auth", "cors_middleware", "logging_middleware")
load("time", "now")

def main():
    srv = create_server(port=8080, server_header="Auth-Demo/1.0")
    
    # Add middleware
    srv.use(logging_middleware())
    srv.use(cors_middleware())
    
    # In-memory user storage and visit tracking
    users = {
        "admin": {"password": "secret", "role": "admin", "visits": 0},
        "user": {"password": "pass", "role": "user", "visits": 0}
    }
    
    # Valid API keys
    api_keys = {
        "key123": {"user": "admin", "app": "web_app"},
        "key456": {"user": "user", "app": "mobile_app"}
    }
    
    # Valid bearer tokens
    bearer_tokens = {
        "token123": {"user": "admin", "expires": "2024-12-31T23:59:59Z"},
        "token456": {"user": "user", "expires": "2024-12-31T23:59:59Z"}
    }
    
    # Create authentication systems
    basic_auth_obj = basic_auth(users={"admin": "secret", "user": "pass"}, realm="Demo")
    api_auth = api_key_auth(keys=["key123", "key456"], header="X-API-Key")
    
    # Token validation function
    def validate_token(token):
        return bearer_tokens.get(token)
    
    bearer_auth_obj = bearer_auth(validate_func=validate_token)
    
    # Helper function to track visits
    def track_visit(username):
        if username in users:
            users[username]["visits"] = users[username]["visits"] + 1
    
    # Routes
    def home(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Authentication Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .section { margin-bottom: 30px; padding: 20px; border: 1px solid #ddd; border-radius: 5px; }
                .auth-method { background: #f8f9fa; padding: 15px; margin: 10px 0; border-radius: 5px; }
                .credentials { background: #e9ecef; padding: 10px; margin: 10px 0; border-radius: 3px; }
                .endpoint { margin: 5px 0; }
                .method { font-weight: bold; color: #007bff; }
                pre { background: #f8f9fa; padding: 10px; border-radius: 4px; overflow-x: auto; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <h1>Authentication Demo</h1>
            
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Form</a>
                <a href="/profile">Profile</a>
                <a href="/admin">Admin</a>
            </div>
            
            <div class="section">
                <h2>🔐 Authentication Methods</h2>
                <p>This demo shows three authentication methods available in the web module:</p>
                
                <div class="auth-method">
                    <h3>1. Basic Authentication</h3>
                    <p>HTTP Basic Auth with username/password</p>
                    <div class="credentials">
                        <strong>Credentials:</strong> admin/secret or user/pass
                    </div>
                    <div class="endpoint">
                        <span class="method">GET</span> /profile - Protected profile page
                    </div>
                    <div class="endpoint">
                        <span class="method">GET</span> /admin - Admin area
                    </div>
                </div>
                
                <div class="auth-method">
                    <h3>2. Bearer Token Authentication</h3>
                    <p>JWT-style token authentication</p>
                    <div class="credentials">
                        <strong>Tokens:</strong> token123 (admin) or token456 (user)
                    </div>
                    <div class="endpoint">
                        <span class="method">GET</span> /api/profile - API profile endpoint
                    </div>
                </div>
                
                <div class="auth-method">
                    <h3>3. API Key Authentication</h3>
                    <p>API key-based authentication</p>
                    <div class="credentials">
                        <strong>Keys:</strong> key123 (admin) or key456 (user)
                    </div>
                    <div class="endpoint">
                        <span class="method">GET</span> /api/data - API data endpoint
                    </div>
                </div>
            </div>
            
            <div class="section">
                <h2>🧪 Test Commands</h2>
                <pre>
# Basic Authentication
curl -u admin:secret http://localhost:8080/profile
curl -u user:pass http://localhost:8080/profile

# Bearer Token Authentication
curl -H "Authorization: Bearer token123" http://localhost:8080/api/profile
curl -H "Authorization: Bearer token456" http://localhost:8080/api/profile

# API Key Authentication
curl -H "X-API-Key: key123" http://localhost:8080/api/data
curl -H "X-API-Key: key456" http://localhost:8080/api/data

# Test unauthorized access
curl http://localhost:8080/profile
curl http://localhost:8080/api/profile
curl http://localhost:8080/api/data
                </pre>
            </div>
            
            <div class="section">
                <h2>📊 User Statistics</h2>
                <div id="stats">
                    <p>Admin visits: {}</p>
                    <p>User visits: {}</p>
                </div>
            </div>
        </body>
        </html>
        """.format(users["admin"]["visits"], users["user"]["visits"]))
    
    def login_form(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Login - Auth Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 400px; margin: 50px auto; padding: 20px; }
                .form-group { margin-bottom: 15px; }
                .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                .form-group input { width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; }
                .btn { padding: 10px 20px; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
                .btn:hover { background: #0056b3; }
                .credentials { background: #e9ecef; padding: 15px; margin: 20px 0; border-radius: 4px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Form</a>
                <a href="/profile">Profile</a>
                <a href="/admin">Admin</a>
            </div>
            
            <h2>Login Form</h2>
            <p>This is a demonstration login form. In a real application, you would process these credentials.</p>
            
            <form method="POST" action="/login">
                <div class="form-group">
                    <label for="username">Username:</label>
                    <input type="text" id="username" name="username" required>
                </div>
                <div class="form-group">
                    <label for="password">Password:</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <button type="submit" class="btn">Login</button>
            </form>
            
            <div class="credentials">
                <h3>Test Credentials:</h3>
                <p><strong>Admin:</strong> admin / secret</p>
                <p><strong>User:</strong> user / pass</p>
            </div>
            
            <p><em>Note: This demo uses Basic Auth for actual authentication. The form is for demonstration purposes only.</em></p>
        </body>
        </html>
        """)
    
    def process_login(req):
        form_data = req.form()
        if form_data == None:
            return error_response(400, "Form data required")
        
        username = form_data.get("username")
        password = form_data.get("password")
        
        if username == None or password == None:
            return error_response(400, "Username and password required")
        
        # Validate credentials
        if username in users and users[username]["password"] == password:
            # In a real app, you would create a session or token here
            track_visit(username)
            return html_response("""
            <!DOCTYPE html>
            <html>
            <head>
                <title>Login Success</title>
                <style>
                    body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; text-align: center; }
                    .success { background: #d4edda; padding: 20px; border-radius: 5px; margin: 20px 0; }
                    .nav { margin-bottom: 20px; }
                    .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                    .nav a:hover { text-decoration: underline; }
                </style>
            </head>
            <body>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/login">Login Form</a>
                    <a href="/profile">Profile</a>
                    <a href="/admin">Admin</a>
                </div>
                
                <div class="success">
                    <h2>Login Successful!</h2>
                    <p>Welcome, <strong>{}</strong>! You have successfully logged in.</p>
                    <p>Role: {}</p>
                    <p>Total visits: {}</p>
                </div>
                
                <p><em>Note: To access protected endpoints, use Basic Auth with your credentials.</em></p>
                <p><a href="/profile">Try accessing your profile</a></p>
            </body>
            </html>
            """.format(username, users[username]["role"], users[username]["visits"]))
        else:
            return error_response(401, "Invalid credentials")
    
    def profile(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        username = basic_info[0]
        track_visit(username)
        
        user_info = users.get(username, {})
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Profile - Auth Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
                .profile { background: #f8f9fa; padding: 20px; border-radius: 5px; margin: 20px 0; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .info { margin: 10px 0; }
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Form</a>
                <a href="/profile">Profile</a>
                <a href="/admin">Admin</a>
            </div>
            
            <h2>User Profile</h2>
            
            <div class="profile">
                <h3>Welcome, {}!</h3>
                <div class="info"><strong>Username:</strong> {}</div>
                <div class="info"><strong>Role:</strong> {}</div>
                <div class="info"><strong>Total visits:</strong> {}</div>
                <div class="info"><strong>Last access:</strong> {}</div>
            </div>
            
            <p><em>This page is protected by Basic Authentication.</em></p>
        </body>
        </html>
        """.format(
            username, 
            username, 
            user_info.get("role", "unknown"),
            user_info.get("visits", 0),
            now().format("2006-01-02 15:04:05")
        ))
    
    def admin_area(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        username = basic_info[0]
        user_info = users.get(username, {})
        
        if user_info.get("role") != "admin":
            return error_response(403, "Admin access required")
        
        track_visit(username)
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Admin Area - Auth Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
                .admin { background: #fff3cd; padding: 20px; border-radius: 5px; margin: 20px 0; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 15px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .user-list { margin: 20px 0; }
                .user { background: #f8f9fa; padding: 10px; margin: 5px 0; border-radius: 3px; }
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Form</a>
                <a href="/profile">Profile</a>
                <a href="/admin">Admin</a>
            </div>
            
            <div class="admin">
                <h2>Admin Area</h2>
                <p>Welcome, Administrator <strong>{}</strong>!</p>
                <p>This area is restricted to admin users only.</p>
                
                <div class="user-list">
                    <h3>User Statistics:</h3>
                    <div class="user">
                        <strong>Admin:</strong> Role: admin, Visits: {}
                    </div>
                    <div class="user">
                        <strong>User:</strong> Role: user, Visits: {}
                    </div>
                </div>
            </div>
        </body>
        </html>
        """.format(username, users["admin"]["visits"], users["user"]["visits"]))
    
    def api_profile(req):
        token = req.bearer_token()
        if token == None:
            return error_response(401, "Bearer token required")
        
        token_info = validate_token(token)
        if token_info == None:
            return error_response(401, "Invalid token")
        
        username = token_info["user"]
        track_visit(username)
        user_info = users.get(username, {})
        
        return json_response({
            "username": username,
            "role": user_info.get("role", "unknown"),
            "visits": user_info.get("visits", 0),
            "token_expires": token_info.get("expires"),
            "timestamp": now().format("2006-01-02T15:04:05Z")
        })
    
    def api_data(req):
        api_key = req.get_header("X-API-Key")
        if api_key == None:
            return error_response(401, "API key required")
        
        key_info = api_keys.get(api_key)
        if key_info == None:
            return error_response(401, "Invalid API key")
        
        username = key_info["user"]
        track_visit(username)
        user_info = users.get(username, {})
        
        return json_response({
            "message": "API data access granted",
            "user": username,
            "app": key_info.get("app"),
            "role": user_info.get("role", "unknown"),
            "visits": user_info.get("visits", 0),
            "data": {
                "items": ["item1", "item2", "item3"],
                "count": 3
            },
            "timestamp": now().format("2006-01-02T15:04:05Z")
        })
    
    # Register public routes
    srv.get("/", home)
    srv.get("/login", login_form)
    srv.post("/login", process_login)
    
    # Register protected routes with Basic Auth
    srv.use_for("/profile", basic_auth_obj.middleware())
    srv.use_for("/admin", basic_auth_obj.middleware())
    srv.get("/profile", profile)
    srv.get("/admin", admin_area)
    
    # Register API routes with token auth
    srv.use_for("/api/profile", bearer_auth_obj.middleware())
    srv.get("/api/profile", api_profile)
    
    # Register API routes with key auth
    srv.use_for("/api/data", api_auth.middleware())
    srv.get("/api/data", api_data)
    
    print("Authentication Demo running on http://localhost:8080")
    print("")
    print("Authentication methods:")
    print("  1. Basic Auth: admin/secret or user/pass")
    print("  2. Bearer Token: token123 (admin) or token456 (user)")
    print("  3. API Key: key123 (admin) or key456 (user)")
    print("")
    print("Protected endpoints:")
    print("  - GET /profile (Basic Auth)")
    print("  - GET /admin (Basic Auth + admin role)")
    print("  - GET /api/profile (Bearer Token)")
    print("  - GET /api/data (API Key)")
    print("")
    print("Public endpoints:")
    print("  - GET / (home page)")
    print("  - GET /login (login form)")
    print("  - POST /login (process login)")
    
    srv.run()

main() 