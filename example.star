load("web", "create_server", "response", "json_response", "html_response", "redirect", "error_response")

def main():
    # Create server
    srv = create_server(host="localhost", port=8080)
    
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
                <nav>
                    <a href="/">Home</a> |
                    <a href="/about">About</a> |
                    <a href="/api/info">API Info</a>
                </nav>
            </body>
        </html>
        """
        return html_response(html)
    
    # Route with parameters
    def user_profile(req):
        user_id = req.param("id")
        if user_id == None:
            return error_response(400, "User ID required")
        
        return json_response({
            "user_id": user_id,
            "name": "User {}".format(user_id),
            "profile_url": "/users/{}".format(user_id)
        })
    
    # Redirect example
    def old_api(req):
        return redirect("/api/info", status=301)
    
    # Register routes
    srv.get("/", home)
    srv.get("/about", about)
    srv.get("/api/info", api_info)
    srv.get("/users/{id}", user_profile)
    srv.get("/old-api", old_api)
    
    print("Server starting on http://{}:{}".format("localhost", 8080))
    print("Try these endpoints:")
    print("  http://localhost:8080/")
    print("  http://localhost:8080/about")
    print("  http://localhost:8080/api/info")
    print("  http://localhost:8080/users/123")
    print("  http://localhost:8080/old-api")
    
    # Start server (this would block in real usage)
    # srv.run()
    print("Server configured successfully!")

main() 