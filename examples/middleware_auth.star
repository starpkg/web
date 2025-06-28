# Middleware and Authentication Example
# This example demonstrates various middleware functions including
# timing, logging, CORS, and authentication middleware.

load("web", "create_server", "response", "json_response", "create_session_manager")
load("time")

def main():
    session_manager = create_session_manager(secret="demo-secret")
    srv = create_server(port=8080, debug=True, session_manager=session_manager)
    
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
        session = session_manager.get_session(req)
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
        session = session_manager.get_session(req)
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
    
    print("Middleware demo running on http://localhost:8080")
    srv.run()

main() 