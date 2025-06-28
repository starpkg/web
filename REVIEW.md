You're a Go and Starlark expert, and helping to complete a Starlark module in Go named "web" for server-side web handling and hosting.

The module should provide a simple and intuitive interface for handling web requests (supporting various http methods, common auth, sessions and cookies, etc.), serving static files, and hosting web applications. The module should be designed to be easy to use and integrate with existing Starlark code. You can refer to the Python Flask module for inspiration, but the implementation should be idiomatic Go and follow best practices, also with high performance. Your new `web` package is for server-side only, and client-side usage you should read the doc of `http` carefully, and both package should share the same spirit for API design and usages.

Your colleague has written down a full plan in a markdown file named `PLAN.md` in the root directory of the repository, and it should include enough information for another developer to understand and implement the module.

After reviewing this documentation, you have the following suggestions to improve the documentation:

1. Functions need to be documented, including their purpose, parameters and return values, and any other relevant information.

2. For helper function `send_file`, should provide a counterpart function that can be used to send a file from data instead of a file path.

3. For function `server`, it might be better to name it `create_server` to be consistent with the `http` module.

4. For session management, it might be better to have a function to create a session manager like `create_server`, and handlers can use the session manager to get the session object from the request, like `session_manager.get_session(req)`. For new users, the session manager handle it well. You should also consider that wether you need to write back keys into cookies of response or not.

5. The purpose or scenario for `server.before_request` and `server.after_request` is not clear, maybe it's duplicate with `server.use` or `server.use_for`?

6. What about `server.after_response` or something like that, to have a chance to modify the response before it's sent to the client? Just like another middleware function but for the response.

7. Maybe we can redesign the middleware system to be more flexible to support both request and response middleware?

8. For `server.error_handler`, the `status_code` parameter can be a list of status codes or just a single status code.

9. For various handler like request handler, error handler, etc., you should list all the possible parameters and their types as definition or interface for developers/users to use and follow.

10. For defintion of `request` object, `client_ip` and `context` should be added as properties.

11. For session object, `flash` and `get_flashes` is kinda confusing, I don't know what it is for, or whether it's a must-have feature.

12. For the sample in `2. RESTful API with CRUD Operations`, you should `shared_dict` instead `dict` for `users` variable to avoid race condition.

13. Since Starlark doesn't support `yield` keyword, the `Streaming Responses and Large Files` section is not applicable.

14. For complicated example like `Complete Blog Application` and other examples, you should put them in separate *.star files as attachment.

Please modify the `PLAN.md` file to follow these suggestions to make this plan more clear and complete.
