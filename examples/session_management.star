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
        # Check if user is authenticated
        basic_info = req.basic_auth()
        is_authenticated = basic_info != None
        username = basic_info[0] if is_authenticated else None
        user_role = "admin" if username == "admin" else "user" if username else None
        
        if is_authenticated:
            # Show authenticated homepage
            visit_count = len(get_visit_history(username))
            return html_response("""
            <!DOCTYPE html>
            <html>
            <head>
                <title>Authentication Flow Demo - Welcome {}</title>
                <style>
                    body {{ font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }}
                    .section {{ margin: 20px 0; padding: 20px; border: 1px solid #ddd; border-radius: 4px; }}
                    .nav {{ margin-bottom: 20px; }}
                    .nav a {{ margin-right: 20px; color: #007bff; text-decoration: none; padding: 5px 10px; }}
                    .nav a:hover {{ background: #f0f0f0; text-decoration: underline; }}
                    .welcome-banner {{ background: #d4edda; color: #155724; padding: 20px; border-radius: 4px; margin: 20px 0; border: 1px solid #c3e6cb; }}
                    .user-info {{ background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }}
                    .quick-actions {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 15px; margin: 20px 0; }}
                    .action-card {{ background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 4px; text-align: center; }}
                    .action-card h3 {{ margin-top: 0; color: #007bff; }}
                    .action-card a {{ color: #007bff; text-decoration: none; font-weight: bold; }}
                    .action-card a:hover {{ text-decoration: underline; }}
                    .logout-btn {{ background: #dc3545; color: white; padding: 8px 15px; text-decoration: none; border-radius: 4px; display: inline-block; margin-left: 10px; }}
                    .logout-btn:hover {{ background: #c82333; text-decoration: none; color: white; }}
                    .status {{ padding: 10px; margin: 10px 0; border-radius: 4px; }}
                    .info {{ background: #d1ecf1; color: #0c5460; border: 1px solid #bee5eb; }}
                </style>
            </head>
            <body>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/protected">Protected Area</a>
                    <a href="/user-profile">User Profile</a>
                    <a href="/admin-only">Admin Only</a>
                    <a href="/api/status">API Status</a>
                    <a href="/logout" class="logout-btn">Logout</a>
                </div>
                
                <div class="welcome-banner">
                    <h1>Welcome back, {}!</h1>
                    <p>You are logged in as <strong>{}</strong> with <strong>{}</strong> privileges.</p>
                    <p>You have visited this application <strong>{}</strong> times.</p>
                </div>
                
                <div class="section">
                    <h2>Your Quick Actions</h2>
                    <div class="quick-actions">
                        <div class="action-card">
                            <h3>Protected Area</h3>
                            <p>Access your secure dashboard and view visit history</p>
                            <a href="/protected">Enter Protected Area →</a>
                        </div>
                        <div class="action-card">
                            <h3>User Profile</h3>
                            <p>View and manage your profile information</p>
                            <a href="/user-profile">View Profile →</a>
                        </div>
                        {}
                        <div class="action-card">
                            <h3>Protected API</h3>
                            <p>Access authenticated API endpoints</p>
                            <a href="/api/protected">API Access →</a>
                        </div>
                    </div>
                </div>
                
                <div class="section">
                    <h2>Authentication Status</h2>
                    <div class="user-info">
                        <p><strong>Current Session:</strong></p>
                        <ul>
                            <li><strong>Username:</strong> {}</li>
                            <li><strong>Role:</strong> {}</li>
                            <li><strong>Authentication Method:</strong> HTTP Basic Auth</li>
                            <li><strong>Session Status:</strong> Active</li>
                        </ul>
                    </div>
                </div>
                
                <div class="section">
                    <h2>About Logout</h2>
                    <div class="info">
                        <p><strong>Note:</strong> HTTP Basic Authentication stores credentials in your browser. 
                        The logout feature will prompt you for new credentials. To fully log out:</p>
                        <ol>
                            <li>Click the <strong>Logout</strong> button above</li>
                            <li>When prompted for credentials, click <strong>Cancel</strong></li>
                            <li>Or enter different credentials to switch users</li>
                        </ol>
                        <p>You can also close your browser to clear the authentication.</p>
                    </div>
                </div>
            </body>
            </html>
            """.format(
                username,
                username, user_role.upper(), user_role.upper(), visit_count,
                '<div class="action-card"><h3>Admin Dashboard</h3><p>Administrative functions and user management</p><a href="/admin-only">Admin Panel →</a></div>' if user_role == "admin" else "",
                username, user_role.upper()
            ))
        else:
            # Show anonymous homepage
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
                    .hero { background: linear-gradient(135deg, #007bff, #6c757d); color: white; padding: 40px 20px; border-radius: 8px; text-align: center; margin: 20px 0; }
                    .hero h1 { margin-top: 0; font-size: 2.5em; }
                    .hero p { font-size: 1.2em; margin-bottom: 30px; }
                    .cta-btn { background: #28a745; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block; }
                    .cta-btn:hover { background: #218838; text-decoration: none; color: white; }
                </style>
            </head>
            <body>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/login">Login Demo</a>
                    <a href="/api/status">API Status</a>
                </div>
                
                <div class="hero">
                    <h1>🔐 Authentication Flow Demo</h1>
                    <p>Experience secure authentication with HTTP Basic Auth</p>
                    <a href="/protected" class="cta-btn">Get Started - Login Required</a>
                </div>
                
                <div class="section">
                    <h2>Authentication System</h2>
                    <p>This demo shows a complete authentication flow using Basic Authentication with:</p>
                    <ul>
                        <li><strong>User Management:</strong> Multiple users with different roles</li>
                        <li><strong>Protected Routes:</strong> Routes that require authentication</li>
                        <li><strong>Visit Tracking:</strong> Track user access patterns</li>
                        <li><strong>Role-based Access:</strong> Different access levels for different users</li>
                        <li><strong>Logout Support:</strong> Basic auth logout workaround</li>
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
                    <p>Click any of these links to start the authentication process:</p>
                    <ul>
                        <li><a href="/protected">Protected Area</a> - Requires any valid user</li>
                        <li><a href="/user-profile">User Profile</a> - Shows user-specific information</li>
                        <li><a href="/admin-only">Admin Only</a> - Requires admin privileges</li>
                        <li><a href="/api/protected">Protected API</a> - API endpoint requiring auth</li>
                    </ul>
                    <p><em>Your browser will prompt you for username and password when you click these links.</em></p>
                </div>
                
                <div class="section">
                    <h2>How It Works</h2>
                    <div class="info">
                        <p><strong>HTTP Basic Authentication</strong> prompts your browser for credentials when accessing protected resources.</p>
                        <ul>
                            <li>Click any protected link above</li>
                            <li>Enter username and password when prompted</li>
                            <li>Your browser will remember the credentials for this session</li>
                            <li>Navigate freely between protected areas</li>
                            <li>Use the logout feature to clear credentials</li>
                        </ul>
                    </div>
                </div>
                
                <div class="section">
                    <h2>Manual Testing</h2>
                    <div class="login-form">
                        <p>You can also test with curl commands:</p>
                        <pre>curl -u admin:secret http://localhost:8080/protected
curl -u user:password http://localhost:8080/user-profile</pre>
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
                <a href="/logout">Logout</a>
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
            
            <h2>Logout</h2>
            <p>To log out, click <a href="/logout">Logout</a> and then cancel the authentication dialog.</p>
            
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
    
    def logout(req):
        # More effective logout by redirecting to URL with fake credentials
        # This forces the browser to replace cached credentials
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Logging Out...</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; text-align: center; }
                .spinner { border: 4px solid #f3f3f3; border-top: 4px solid #007bff; border-radius: 50%; width: 40px; height: 40px; animation: spin 1s linear infinite; margin: 20px auto; }
                @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
                .info { background: #d1ecf1; color: #0c5460; padding: 15px; border-radius: 4px; margin: 20px 0; }
            </style>
            <script>
                // Force logout by navigating to URL with invalid credentials
                setTimeout(function() {
                    // Try to clear credentials by accessing with fake ones
                    var logoutUrl = window.location.protocol + "//" + "logout:logout@" + window.location.host + "/logout-clear";
                    
                    // Create a hidden iframe to trigger the credential clearing
                    var iframe = document.createElement('iframe');
                    iframe.style.display = 'none';
                    iframe.src = logoutUrl;
                    document.body.appendChild(iframe);
                    
                    // After attempting logout, redirect to home
                    setTimeout(function() {
                        window.location.href = '/';
                    }, 2000);
                }, 1000);
            </script>
        </head>
        <body>
            <h1>Logging Out...</h1>
            <div class="spinner"></div>
            <p>Clearing your authentication credentials...</p>
            
            <div class="info">
                <p>If you are not automatically redirected:</p>
                <ul>
                    <li><a href="/">Click here to return to the home page</a></li>
                    <li>Close and reopen your browser to fully clear credentials</li>
                </ul>
            </div>
        </body>
        </html>
        """)
    
    def logout_clear(req):
        # This endpoint is accessed with invalid credentials to clear the cache
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Logout Complete</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; text-align: center; }
                .success { background: #d4edda; color: #155724; padding: 15px; border-radius: 4px; margin: 20px 0; }
            </style>
            <script>
                // Redirect to home page
                setTimeout(function() {
                    window.location.href = '/';
                }, 1000);
            </script>
        </head>
        <body>
            <h1>✅ Logout Successful</h1>
            <div class="success">
                <p>You have been successfully logged out.</p>
                <p>Redirecting to home page...</p>
            </div>
            <p><a href="/">Return to Home Page</a></p>
        </body>
        </html>
        """, status=401, headers={"WWW-Authenticate": "Basic realm=\"Logged Out\""})
    
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
                "/logout",
                "/logout-clear",
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
                body {{ font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }}
                .welcome {{ background: #d4edda; color: #155724; padding: 15px; border-radius: 4px; margin: 20px 0; }}
                .nav a {{ margin-right: 20px; color: #007bff; text-decoration: none; }}
                .nav a:hover {{ text-decoration: underline; }}
                .visit-history {{ background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }}
                .visit-history ul {{ list-style-type: none; padding: 0; }}
                .visit-history li {{ padding: 5px 0; border-bottom: 1px solid #dee2e6; }}
                .logout-btn {{ background: #dc3545; color: white; padding: 8px 15px; text-decoration: none; border-radius: 4px; display: inline-block; margin-left: 10px; }}
                .logout-btn:hover {{ background: #c82333; text-decoration: none; }}
            </style>
        </head>
            <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
                <a href="/logout" class="logout-btn">Logout</a>
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
                body {{ font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }}
                .profile {{ background: #f8f9fa; padding: 20px; border-radius: 4px; margin: 20px 0; }}
                .nav a {{ margin-right: 20px; color: #007bff; text-decoration: none; }}
                .nav a:hover {{ text-decoration: underline; }}
                .stats {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }}
                .stat-card {{ background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 4px; }}
                .stat-value {{ font-size: 24px; font-weight: bold; color: #007bff; }}
                .logout-btn {{ background: #dc3545; color: white; padding: 8px 15px; text-decoration: none; border-radius: 4px; display: inline-block; margin-left: 10px; }}
                .logout-btn:hover {{ background: #c82333; text-decoration: none; }}
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
                <a href="/logout" class="logout-btn">Logout</a>
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
                body {{ font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }}
                .admin-panel {{ background: #fff3cd; color: #856404; padding: 20px; border-radius: 4px; margin: 20px 0; }}
                .nav a {{ margin-right: 20px; color: #007bff; text-decoration: none; }}
                .nav a:hover {{ text-decoration: underline; }}
                .admin-actions {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 20px 0; }}
                .action-card {{ background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 4px; }}
                .users-list {{ background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }}
                .logout-btn {{ background: #dc3545; color: white; padding: 8px 15px; text-decoration: none; border-radius: 4px; display: inline-block; margin-left: 10px; }}
                .logout-btn:hover {{ background: #c82333; text-decoration: none; }}
            </style>
        </head>
        <body>
            <div class="nav">
                <a href="/">Home</a>
                <a href="/protected">Protected Area</a>
                <a href="/user-profile">User Profile</a>
                <a href="/admin-only">Admin Only</a>
                <a href="/logout" class="logout-btn">Logout</a>
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
    srv.get("/logout", logout)
    srv.get("/logout-clear", logout_clear) # Register the new logout_clear endpoint
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
    print("  GET / - Home page (shows different content based on auth status)")
    print("  GET /login - Login demo page")
    print("  GET /logout - Logout (forces credential clearing)")
    print("  GET /logout-clear - Logout completion endpoint")
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
    print("  - Logout functionality (workaround for Basic Auth)")
    
    srv.run()

main() 