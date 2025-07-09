load("web", "create_server", "html_response", "json_response", "error_response", "redirect", "basic_auth", "cors_middleware", "logging_middleware", "compression_middleware")
load("time", "now")

def main():
    srv = create_server(port=8080, server_header="Auth-Flow-Demo/1.0")
    
    # Add middleware
    srv.use(logging_middleware())
    srv.use(cors_middleware())
    srv.use(compression_middleware())
    
    # Create authentication for protected areas
    auth = basic_auth(users={"admin": "secret", "user": "password"}, realm="Demo Area")
    
    # In-memory storage for demonstration
    user_visits = {}  # Track user visits
    login_attempts = {}  # Track login attempts
    
    # Helper to track user visits
    def track_visit(username):
        if username not in user_visits:
            user_visits[username] = []
        user_visits[username].append(now().format("2006-01-02T15:04:05Z"))
    
    # Helper to get user visit history
    def get_visit_history(username):
        return user_visits.get(username, [])
    
    # PUBLIC ROUTES
    def home(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Authentication Flow Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .section { margin: 20px 0; padding: 20px; border: 1px solid #ddd; border-radius: 4px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; padding: 5px 10px; }
                .nav a:hover { background: #f0f0f0; text-decoration: underline; }
                .login-form { background: #f8f9fa; padding: 20px; border-radius: 4px; margin: 20px 0; }
                .form-group { margin-bottom: 15px; }
                .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                .form-group input { width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; }
                .btn { padding: 10px 20px; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
                .btn:hover { background: #0056b3; }
                .status { padding: 10px; margin: 10px 0; border-radius: 4px; }
                .success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
                .error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
                .info { background: #d1ecf1; color: #0c5460; border: 1px solid #bee5eb; }
            </style>
        </head>
        <body>
            <h1>Authentication Flow Demo</h1>
            
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Demo</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
                <a href="/api/status">API Status</a>
            </div>
            
            <div class="section">
                <h2>Authentication System</h2>
                <p>This demo shows a complete authentication flow using Basic Authentication with:</p>
                <ul>
                    <li><strong>User Management:</strong> Multiple users with different roles</li>
                    <li><strong>Protected Routes:</strong> Routes that require authentication</li>
                    <li><strong>Visit Tracking:</strong> Track user access patterns</li>
                    <li><strong>Role-based Access:</strong> Different access levels for different users</li>
                </ul>
            </div>
            
            <div class="section">
                <h2>Available Users</h2>
                <div class="info">
                    <p><strong>Admin User:</strong> admin / secret</p>
                    <p><strong>Regular User:</strong> user / password</p>
                </div>
            </div>
            
            <div class="section">
                <h2>Test the Authentication</h2>
                <p>Try accessing the protected areas below:</p>
                <ul>
                    <li><a href="/protected">Protected Area</a> - Requires any valid user</li>
                    <li><a href="/user-profile">User Profile</a> - Shows user-specific information</li>
                    <li><a href="/admin-only">Admin Only</a> - Requires admin privileges</li>
                    <li><a href="/api/protected">Protected API</a> - API endpoint requiring auth</li>
                </ul>
            </div>
            
            <div class="section">
                <h2>Manual Login Test</h2>
                <div class="login-form">
                    <p>Use your browser's authentication dialog when accessing protected routes, or test with curl:</p>
                    <pre>curl -u admin:secret http://localhost:8080/protected</pre>
                    <pre>curl -u user:password http://localhost:8080/user-profile</pre>
                </div>
            </div>
        </body>
        </html>
        """)
    
    def login_demo(req):
        return html_response("""
        <!DOCTYPE html>
            <html>
        <head>
            <title>Login Demo</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; }
                .info { background: #d1ecf1; color: #0c5460; padding: 15px; border-radius: 4px; margin: 20px 0; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                pre { background: #f8f9fa; padding: 10px; border-radius: 4px; overflow-x: auto; }
            </style>
        </head>
                <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/login">Login Demo</a>
                <a href="/protected">Protected Area</a>
            </div>
            
            <h1>Login Demo</h1>
            
            <div class="info">
                <p>This application uses <strong>HTTP Basic Authentication</strong>. When you access a protected route, your browser will show a login dialog.</p>
            </div>
            
            <h2>Test Authentication</h2>
            <p>Click the links below to test authentication:</p>
            
            <ul>
                <li><a href="/protected">Protected Area</a> - Will prompt for login</li>
                <li><a href="/user-profile">User Profile</a> - User-specific content</li>
                <li><a href="/admin-only">Admin Only</a> - Admin-only content</li>
            </ul>
            
            <h2>API Testing</h2>
            <p>Test with curl commands:</p>
            <pre>
# Test without authentication (should fail)
curl http://localhost:8080/protected

# Test with valid credentials
curl -u admin:secret http://localhost:8080/protected
curl -u user:password http://localhost:8080/user-profile

# Test API endpoints
curl -u admin:secret http://localhost:8080/api/protected
            </pre>
                </body>
            </html>
        """)
    
    def api_status(req):
        return json_response({
            "status": "running",
            "authentication": "basic_auth",
            "available_users": ["admin", "user"],
            "protected_routes": [
                "/protected",
                "/user-profile", 
                "/admin-only",
                "/api/protected"
            ],
            "public_routes": [
                "/",
                "/login",
                "/api/status"
            ],
            "timestamp": now().format("2006-01-02T15:04:05Z")
        })
    
    # PROTECTED ROUTES
    def protected_area(req):
        # This route is protected by auth middleware
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        track_visit(username)
        visit_history = get_visit_history(username)
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Protected Area</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .welcome { background: #d4edda; color: #155724; padding: 15px; border-radius: 4px; margin: 20px 0; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .visit-history { background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }
                .visit-history ul { list-style-type: none; padding: 0; }
                .visit-history li { padding: 5px 0; border-bottom: 1px solid #dee2e6; }
            </style>
        </head>
            <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
            </div>
            
            <div class="welcome">
                <h1>Welcome to the Protected Area, {}!</h1>
                <p>You have successfully authenticated and can access this protected content.</p>
            </div>
            
            <div class="visit-history">
                <h2>Your Visit History</h2>
                <p>Recent visits to this protected area:</p>
                <ul>
                    {}
                </ul>
            </div>
            
            <div>
                <h2>Protected Features</h2>
                <p>This area is only accessible to authenticated users. Here you can:</p>
                <ul>
                    <li>View your visit history</li>
                    <li>Access user-specific content</li>
                    <li>Perform authenticated actions</li>
                </ul>
            </div>
            </body>
        </html>
        """.format(username, "".join(["<li>{}</li>".format(visit) for visit in visit_history[-5:]])))
    
    def user_profile(req):
        # This route is protected by auth middleware
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        track_visit(username)
        visit_history = get_visit_history(username)
        
        # Determine user role
        user_role = "admin" if username == "admin" else "user"
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>User Profile - {}</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .profile { background: #f8f9fa; padding: 20px; border-radius: 4px; margin: 20px 0; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
                .stat-card { background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 4px; }
                .stat-value { font-size: 24px; font-weight: bold; color: #007bff; }
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
            </div>
            
            <div class="profile">
                <h1>User Profile</h1>
                <p><strong>Username:</strong> {}</p>
                <p><strong>Role:</strong> {}</p>
                <p><strong>Status:</strong> Authenticated</p>
                <p><strong>Last Login:</strong> {}</p>
            </div>
            
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-value">{}</div>
                    <div>Total Visits</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{}</div>
                    <div>Access Level</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{}</div>
                    <div>Account Type</div>
                </div>
            </div>
            
            <div>
                <h2>Recent Activity</h2>
                <ul>
                    {}
                </ul>
            </div>
        </body>
        </html>
        """.format(
            username,
            username,
            user_role.upper(),
            visit_history[-1] if visit_history else "Never",
            len(visit_history),
            user_role.upper(),
            user_role.upper(),
            "".join(["<li>Visited on {}</li>".format(visit) for visit in visit_history[-5:]])
        ))
    
    def admin_only(req):
        # This route is protected by auth middleware
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        # Check if user is admin
        if username != "admin":
            return error_response(403, "Access denied. Admin privileges required.")
        
        track_visit(username)
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Admin Only Area</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .admin-panel { background: #fff3cd; color: #856404; padding: 20px; border-radius: 4px; margin: 20px 0; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .admin-actions { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 20px 0; }
                .action-card { background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 4px; }
                .users-list { background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
            </div>
            
            <div class="admin-panel">
                <h1>Admin Dashboard</h1>
                <p>Welcome, {}! This area is restricted to administrators only.</p>
            </div>
            
            <div class="admin-actions">
                <div class="action-card">
                    <h3>User Management</h3>
                    <p>Manage user accounts and permissions</p>
                </div>
                <div class="action-card">
                    <h3>System Settings</h3>
                    <p>Configure system-wide settings</p>
                </div>
                <div class="action-card">
                    <h3>Security Logs</h3>
                    <p>View authentication and access logs</p>
                </div>
                <div class="action-card">
                    <h3>Analytics</h3>
                    <p>View usage statistics and reports</p>
                </div>
            </div>
            
            <div class="users-list">
                <h2>User Activity Summary</h2>
                <ul>
                    {}
                </ul>
            </div>
        </body>
        </html>
        """.format(
            username,
            "".join(["<li><strong>{}:</strong> {} visits</li>".format(user, len(visits)) for user, visits in user_visits.items()])
        ))
    
    def protected_api(req):
        # This route is protected by auth middleware
        basic_info = req.basic_auth()
        username = basic_info[0] if basic_info != None else "unknown"
        
        track_visit(username)
        
        return json_response({
            "message": "Protected API endpoint accessed successfully",
            "authenticated_user": username,
            "user_role": "admin" if username == "admin" else "user",
            "visit_count": len(get_visit_history(username)),
            "timestamp": now().format("2006-01-02T15:04:05Z")
        })
    
    # REGISTER PUBLIC ROUTES
    srv.get("/", home)
    srv.get("/login", login_demo)
    srv.get("/api/status", api_status)
    
    # REGISTER PROTECTED ROUTES (with authentication middleware)
    srv.use_for("/protected", auth.middleware())
    srv.get("/protected", protected_area)
    
    srv.use_for("/user-profile", auth.middleware())
    srv.get("/user-profile", user_profile)
    
    srv.use_for("/admin-only", auth.middleware())
    srv.get("/admin-only", admin_only)
    
    srv.use_for("/api/protected", auth.middleware())
    srv.get("/api/protected", protected_api)
    
    print("Authentication Flow Demo server running on http://localhost:8080")
    print("")
    print("Available users:")
    print("  admin / secret - Full admin access")
    print("  user / password - Regular user access")
    print("")
    print("Public endpoints:")
    print("  GET / - Home page")
    print("  GET /login - Login demo page")
    print("  GET /api/status - API status (public)")
    print("")
    print("Protected endpoints (require authentication):")
    print("  GET /protected - Protected area (any user)")
    print("  GET /user-profile - User profile page (any user)")
    print("  GET /admin-only - Admin dashboard (admin only)")
    print("  GET /api/protected - Protected API (any user)")
    print("")
    print("Features demonstrated:")
    print("  - Basic Authentication with multiple users")
    print("  - Visit tracking and user history")
    print("  - Role-based access control")
    print("  - Protected API endpoints")
    print("  - User-specific content")
    
    srv.run()

main() 