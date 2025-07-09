load("web", "create_server", "html_response", "json_response", "error_response", "redirect", "basic_auth", "cors_middleware", "logging_middleware", "compression_middleware")
load("time", "now")

def main():
    srv = create_server(port=8080, server_header="Blog-Server/1.0")
    
    # Add middleware
    srv.use(logging_middleware())
    srv.use(cors_middleware())
    srv.use(compression_middleware())
    
    # Create authentication for admin
    auth = basic_auth(users={"admin": "blogpass"}, realm="Blog Admin")
    
    # In-memory data store
    posts = []
    next_id = [1]
    
    # Sample data
    posts.append({
        "id": 1,
        "title": "Welcome to My Blog",
        "content": "This is my first blog post! Welcome to my simple blog application built with Starlark.",
        "author": "admin",
        "created": "2024-01-01T12:00:00Z",
        "updated": "2024-01-01T12:00:00Z"
    })
    next_id[0] = 2
    
    # Helper function to get post by ID
    def get_post_by_id(post_id):
        for post in posts:
            if post["id"] == post_id:
                return post
        return None
    
    # Helper function to format date
    def format_date(date_str):
        return date_str.split("T")[0]
    
    # Routes
    def home(req):
        posts_html = ""
        for post in posts:
            posts_html = posts_html + """
                <div class="post">
                    <h3><a href="/post/{}">{}</a></h3>
                    <p class="meta">By {} on {}</p>
                    <p>{}</p>
                </div>
            """.format(
                post["id"],
                post["title"],
                post["author"],
                format_date(post["created"]),
                post["content"][:200] + "..." if len(post["content"]) > 200 else post["content"]
            )
        
        if posts_html == "":
            posts_html = "<p>No posts yet. <a href='/admin/new'>Create one</a>!</p>"
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>My Blog</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; }
                .post { margin-bottom: 30px; padding: 20px; border: 1px solid #eee; }
                .meta { color: #666; font-size: 0.9em; margin-bottom: 10px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .form-group { margin-bottom: 15px; }
                .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                .form-group input, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ccc; }
                .btn { padding: 10px 20px; background: #007bff; color: white; border: none; cursor: pointer; }
                .btn:hover { background: #0056b3; }
                .error { color: red; margin-bottom: 15px; }
                .success { color: green; margin-bottom: 15px; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>My Blog</h1>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/admin/new">New Post</a>
                    <a href="/admin/list">Admin</a>
                </div>
            </div>
            
            <div class="content">
                {}
            </div>
        </body>
        </html>
        """.format(posts_html))
    
    def view_post(req):
        post_id_str = req.param("id")
        post_id = int(post_id_str) if post_id_str else 0
        
        post = get_post_by_id(post_id)
        if post == None:
            return error_response(404, "Post not found")
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>{} - My Blog</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; }
                .post { margin-bottom: 30px; }
                .meta { color: #666; font-size: 0.9em; margin-bottom: 20px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                .content { line-height: 1.6; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>My Blog</h1>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/admin/new">New Post</a>
                    <a href="/admin/list">Admin</a>
                </div>
            </div>
            
            <article class="post">
                <h2>{}</h2>
                <p class="meta">By {} on {} (Updated: {})</p>
                <div class="content">{}</div>
            </article>
        </body>
        </html>
        """.format(
            post["title"],
            post["title"],
            post["author"],
            format_date(post["created"]),
            format_date(post["updated"]),
            post["content"]
        ))
    
    def new_post_form(req):
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>New Post - My Blog</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; }
                .form-group { margin-bottom: 15px; }
                .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                .form-group input, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ccc; }
                .btn { padding: 10px 20px; background: #007bff; color: white; border: none; cursor: pointer; }
                .btn:hover { background: #0056b3; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>My Blog</h1>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/admin/new">New Post</a>
                    <a href="/admin/list">Admin</a>
                </div>
            </div>
            
            <h2>Create New Post</h2>
            <form method="POST" action="/admin/create">
                <div class="form-group">
                    <label for="title">Title:</label>
                    <input type="text" id="title" name="title" required>
                </div>
                <div class="form-group">
                    <label for="content">Content:</label>
                    <textarea id="content" name="content" rows="10" required></textarea>
                </div>
                <button type="submit" class="btn">Create Post</button>
            </form>
        </body>
        </html>
        """)
    
    def create_post(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        username = basic_info[0]
        
        form_data = req.form()
        if form_data == None:
            return error_response(400, "Form data required")
        
        title = form_data.get("title")
        content = form_data.get("content")
        
        if title == None or content == None or title == "" or content == "":
            return error_response(400, "Title and content are required")
        
        current_time = now().format("2006-01-02T15:04:05Z")
        post = {
            "id": next_id[0],
            "title": title,
            "content": content,
            "author": username,
            "created": current_time,
            "updated": current_time
        }
        
        posts.append(post)
        next_id[0] = next_id[0] + 1
        
        return redirect("/")
    
    def admin_list(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        posts_html = ""
        for post in posts:
            posts_html = posts_html + """
                <tr>
                    <td>{}</td>
                    <td><a href="/post/{}">{}</a></td>
                    <td>{}</td>
                    <td>{}</td>
                    <td>
                        <a href="/admin/edit/{}">Edit</a>
                        <a href="/admin/delete/{}" onclick="return confirm('Are you sure?')">Delete</a>
                    </td>
                </tr>
            """.format(
                post["id"],
                post["id"],
                post["title"],
                post["author"],
                format_date(post["created"]),
                post["id"],
                post["id"]
            )
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Admin - My Blog</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
                table { width: 100%; border-collapse: collapse; }
                th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
                th { background-color: #f2f2f2; }
                .actions a { margin-right: 10px; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>My Blog</h1>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/admin/new">New Post</a>
                    <a href="/admin/list">Admin</a>
                </div>
            </div>
            
            <h2>Manage Posts</h2>
            <table>
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Title</th>
                        <th>Author</th>
                        <th>Created</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {}
                </tbody>
            </table>
        </body>
        </html>
        """.format(posts_html))
    
    def edit_post_form(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        post_id_str = req.param("id")
        post_id = int(post_id_str) if post_id_str else 0
        
        post = get_post_by_id(post_id)
        if post == None:
            return error_response(404, "Post not found")
        
        return html_response("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Edit Post - My Blog</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
                .header { border-bottom: 1px solid #ccc; padding-bottom: 10px; margin-bottom: 20px; }
                .form-group { margin-bottom: 15px; }
                .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
                .form-group input, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ccc; }
                .btn { padding: 10px 20px; background: #007bff; color: white; border: none; cursor: pointer; }
                .btn:hover { background: #0056b3; }
                .nav { margin-bottom: 20px; }
                .nav a { margin-right: 20px; color: #007bff; text-decoration: none; }
                .nav a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>My Blog</h1>
                <div class="nav">
                    <a href="/">Home</a>
                    <a href="/admin/new">New Post</a>
                    <a href="/admin/list">Admin</a>
                </div>
            </div>
            
            <h2>Edit Post</h2>
            <form method="POST" action="/admin/update/{}">
                <div class="form-group">
                    <label for="title">Title:</label>
                    <input type="text" id="title" name="title" value="{}" required>
                </div>
                <div class="form-group">
                    <label for="content">Content:</label>
                    <textarea id="content" name="content" rows="10" required>{}</textarea>
                </div>
                <button type="submit" class="btn">Update Post</button>
            </form>
        </body>
        </html>
        """.format(post["id"], post["title"], post["content"]))
    
    def update_post(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        post_id_str = req.param("id")
        post_id = int(post_id_str) if post_id_str else 0
        
        post = get_post_by_id(post_id)
        if post == None:
            return error_response(404, "Post not found")
        
        form_data = req.form()
        if form_data == None:
            return error_response(400, "Form data required")
        
        title = form_data.get("title")
        content = form_data.get("content")
        
        if title == None or content == None or title == "" or content == "":
            return error_response(400, "Title and content are required")
        
        post["title"] = title
        post["content"] = content
        post["updated"] = now().format("2006-01-02T15:04:05Z")
        
        return redirect("/admin/list")
    
    def delete_post(req):
        basic_info = req.basic_auth()
        if basic_info == None:
            return error_response(401, "Authentication required")
        
        post_id_str = req.param("id")
        post_id = int(post_id_str) if post_id_str else 0
        
        for i, post in enumerate(posts):
            if post["id"] == post_id:
                posts.pop(i)
                return redirect("/admin/list")
        
        return error_response(404, "Post not found")
    
    # API endpoints
    def api_posts(req):
        if req.method == "GET":
            return json_response({"posts": posts})
        elif req.method == "POST":
            basic_info = req.basic_auth()
            if basic_info == None:
                return error_response(401, "Authentication required")
            
            username = basic_info[0]
            
            data = req.json()
            if data == None:
                return error_response(400, "JSON data required")
            
            title = data.get("title")
            content = data.get("content")
            
            if title == None or content == None:
                return error_response(400, "Title and content are required")
            
            current_time = now().format("2006-01-02T15:04:05Z")
            post = {
                "id": next_id[0],
                "title": title,
                "content": content,
                "author": username,
                "created": current_time,
                "updated": current_time
            }
            
            posts.append(post)
            next_id[0] = next_id[0] + 1
            
            return json_response(post, status=201)
        else:
            return error_response(405, "Method not allowed")
    
    def api_post(req):
        post_id_str = req.param("id")
        post_id = int(post_id_str) if post_id_str else 0
        
        post = get_post_by_id(post_id)
        if post == None:
            return error_response(404, "Post not found")
        
        return json_response(post)
    
    # Register public routes
    srv.get("/", home)
    srv.get("/post/{id}", view_post)
    
    # Register admin routes with authentication
    srv.use_for("/admin/*", auth.middleware())
    srv.get("/admin/new", new_post_form)
    srv.post("/admin/create", create_post)
    srv.get("/admin/list", admin_list)
    srv.get("/admin/edit/{id}", edit_post_form)
    srv.post("/admin/update/{id}", update_post)
    srv.get("/admin/delete/{id}", delete_post)
    
    # Register API routes with authentication
    srv.use_for("/api/*", auth.middleware())
    srv.route(["GET", "POST"], "/api/posts", api_posts)
    srv.get("/api/posts/{id}", api_post)
    
    print("Blog server running on http://localhost:8080")
    print("Admin credentials: admin/blogpass")
    print("API endpoints:")
    print("  GET /api/posts - List all posts")
    print("  POST /api/posts - Create new post")
    print("  GET /api/posts/{id} - Get specific post")
    
    srv.run()

main() 