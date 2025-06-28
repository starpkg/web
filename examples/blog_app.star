# Complete Blog Application Example
# This example demonstrates a full blog application with admin functionality,
# sessions, authentication, and basic CRUD operations for posts and comments.

load("web", "create_server", "response", "json_response", "redirect", 
     "create_session_manager", "basic_auth", "send_file")
load("time")

def main():
    # Create session manager
    session_manager = create_session_manager(secret="blog-secret-key")
    
    srv = create_server(
        port=8080,
        session_manager=session_manager
    )
    
    # Simple in-memory database (use shared_dict for thread safety)
    posts = shared_dict()
    comments = shared_dict()
    next_post_id = [1]  # Use list to allow modification
    next_comment_id = [1]
    
    # Admin auth
    admin_auth = basic_auth(users={"admin": "admin123"})
    
    # Helper to render HTML template
    def render_html(title, content):
        return """
        <html>
            <head>
                <title>{}</title>
                <link rel="stylesheet" href="/static/style.css">
            </head>
            <body>
                <header>
                    <h1>My Blog</h1>
                    <nav>
                        <a href="/">Home</a>
                        <a href="/admin">Admin</a>
                    </nav>
                </header>
                <main>
                    {}
                </main>
            </body>
        </html>
        """.format(title, content)
    
    # Home page - list posts
    def home(req):
        content = "<h2>Recent Posts</h2>"
        
        if len(posts) == 0:
            content = content + "<p>No posts yet.</p>"
        else:
            content = content + "<ul>"
            for post_id in posts:
                post = posts[post_id]
                content = content + '<li><a href="/post/{}">{}</a> - {}</li>'.format(
                    post["id"], 
                    post["title"],
                    post["created"]
                )
            content = content + "</ul>"
        
        html = render_html("Home", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # View single post
    def view_post(req):
        post_id_str = req.param("id")
        if post_id_str == None:
            return error_response(400, "Post ID required")
        
        post_id = int(post_id_str)
        post = posts.get(post_id)
        
        if post == None:
            return error_response(404, "Post not found")
        
        # Build post HTML
        content = "<article>"
        content = content + "<h2>{}</h2>".format(post["title"])
        content = content + "<p class='meta'>Posted on {}</p>".format(post["created"])
        content = content + "<div class='content'>{}</div>".format(post["content"])
        content = content + "</article>"
        
        # Add comments section
        content = content + "<h3>Comments</h3>"
        post_comments = comments.get(post_id, [])
        
        if len(post_comments) == 0:
            content = content + "<p>No comments yet.</p>"
        else:
            for comment in post_comments:
                content = content + "<div class='comment'>"
                content = content + "<strong>{}</strong>: {}".format(
                    comment["author"], 
                    comment["text"]
                )
                content = content + "</div>"
        
        # Comment form
        content = content + """
        <form method="post" action="/post/{}/comment">
            <input name="author" placeholder="Your name" required>
            <textarea name="text" placeholder="Your comment" required></textarea>
            <button type="submit">Post Comment</button>
        </form>
        """.format(post_id)
        
        html = render_html(post["title"], content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # Post comment
    def post_comment(req):
        post_id_str = req.param("id")
        if post_id_str == None:
            return error_response(400, "Post ID required")
        
        post_id = int(post_id_str)
        
        # Check post exists
        if posts.get(post_id) == None:
            return error_response(404, "Post not found")
        
        form = req.form()
        author = form.get("author", "").strip()
        text = form.get("text", "").strip()
        
        if author == "" or text == "":
            return error_response(400, "Author and text required")
        
        # Add comment
        if comments.get(post_id) == None:
            comments[post_id] = []
        
        comment = {
            "id": next_comment_id[0],
            "author": author,
            "text": text,
            "created": time.now().format(time.DateTime)
        }
        post_comments = comments[post_id]
        post_comments.append(comment)
        comments[post_id] = post_comments
        next_comment_id[0] = next_comment_id[0] + 1
        
        return redirect("/post/{}".format(post_id))
    
    # Admin dashboard
    def admin_dashboard(req):
        content = "<h2>Admin Dashboard</h2>"
        content = content + "<p><a href='/admin/new'>Create New Post</a></p>"
        content = content + "<h3>All Posts</h3>"
        
        if len(posts) == 0:
            content = content + "<p>No posts yet.</p>"
        else:
            content = content + "<ul>"
            for post_id in posts:
                post = posts[post_id]
                content = content + '<li>{} - <a href="/admin/edit/{}">Edit</a></li>'.format(
                    post["title"], 
                    post["id"]
                )
            content = content + "</ul>"
        
        html = render_html("Admin Dashboard", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # New post form
    def new_post_form(req):
        content = """
        <h2>Create New Post</h2>
        <form method="post" action="/admin/new">
            <input name="title" placeholder="Post title" required>
            <textarea name="content" placeholder="Post content" rows="10" required></textarea>
            <button type="submit">Create Post</button>
        </form>
        """
        
        html = render_html("New Post", content)
        return response(html, headers={"Content-Type": "text/html"})
    
    # Create post
    def create_post(req):
        form = req.form()
        title = form.get("title", "").strip()
        content_text = form.get("content", "").strip()
        
        if title == "" or content_text == "":
            return error_response(400, "Title and content required")
        
        post = {
            "id": next_post_id[0],
            "title": title,
            "content": content_text,
            "created": time.now().format(time.DateTime)
        }
        posts[next_post_id[0]] = post
        next_post_id[0] = next_post_id[0] + 1
        
        session = session_manager.get_session(req)
        session.flash("Post created successfully!", "success")
        
        return redirect("/admin")
    
    # API endpoints
    def api_posts(req):
        post_list = [posts[post_id] for post_id in posts]
        return json_response(post_list)
    
    def api_post_comments(req):
        post_id_str = req.param("id")
        if post_id_str == None:
            return error_response(400, "Post ID required")
        
        post_id = int(post_id_str)
        return json_response(comments.get(post_id, []))
    
    # Register routes
    srv.get("/", home)
    srv.get("/post/{id}", view_post)
    srv.post("/post/{id}/comment", post_comment)
    
    # Admin routes (protected)
    srv.use_for("/admin/*", admin_auth.middleware())
    srv.get("/admin", admin_dashboard)
    srv.get("/admin/new", new_post_form)
    srv.post("/admin/new", create_post)
    
    # API routes
    srv.get("/api/posts", api_posts)
    srv.get("/api/posts/{id}/comments", api_post_comments)
    
    # Static files
    srv.static("/static", "./static")
    
    print("Blog running on http://localhost:8080")
    srv.run()

main() 