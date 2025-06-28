# Session Management Example
# This example demonstrates session handling, user login/logout,
# and tracking user state across requests.

load("web", "create_server", "response", "redirect", "json_response", 
     "create_session_manager")
load("time")

def main():
    # Create session manager with configuration
    session_manager = create_session_manager(
        secret="my-secret-key-change-in-production",
        cookie_name="app_session",
        max_age=3600  # 1 hour
    )
    
    srv = create_server(
        port=8080,
        session_manager=session_manager
    )
    
    # Home page
    def home(req):
        session = session_manager.get_session(req)
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
            session = session_manager.get_session(req)
            session.set("username", username)
            session.set("login_time", time.now().format(time.RFC3339))
            return redirect("/")
        
        # Invalid credentials - redirect back with error
        return redirect("/")
    
    # API endpoint requiring session
    def api_status(req):
        session = session_manager.get_session(req)
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
        session = session_manager.get_session(req)
        session.clear()
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
    
    print("Session demo running on http://localhost:8080")
    srv.run()

main() 