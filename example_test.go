package web

import (
	"fmt"
	"testing"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// TestStarlarkScripts runs Starlark test scripts from the test directory.
// Scripts with "test-" prefix should succeed, "panic-" prefix should fail.
func TestStarlarkScripts(t *testing.T) {
	// Create a module factory function that returns a fresh module loader for each test
	moduleFactory := func() starlet.ModuleLoader {
		return NewModule().LoadModule()
	}
	extraModules := []string{"go_idiomatic", "http"}

	// Use the helper function from the base package
	base.RunStarlarkTests(t, ModuleName, moduleFactory, extraModules, "")
}

func Example_basicWebServer() {
	script := `
load("web", "create_server", "response", "json_response")

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
        })
    
    # Register routes
    srv.get("/", home)
    srv.get("/api/info", api_info)
    
    print("Server created successfully")
    
    # Start server
    srv.start()
    
    # ... do something calling via http package to verify the response ...
    
    # Stop server
    srv.stop()

main()
`

	// Create machine with web module
	machine := starlet.NewWithNames(starlet.StringAnyMap{}, []string{"go_idiomatic", "http"}, []string{})
	machine.SetPrintFunc(func(thread *starlark.Thread, msg string) {
		// Print function for testing
		fmt.Println(msg)
	})

	// Load web module
	webModule := NewModule()
	machine.AddLazyloadModules(starlet.ModuleLoaderMap{
		ModuleName: webModule.LoadModule(),
	})

	_, err := machine.RunScript([]byte(script), nil)
	if err != nil {
		panic(err)
	}

	// Output: Server created successfully
}

func Example_restfulAPI() {
	script := `
load("web", "create_server", "json_response", "error_response")

def main():
    srv = create_server(port=8080)
    
    # In-memory database
    users = {}
    next_id = [1]
    
    # List all users
    def list_users(req):
        user_list = []
        for user_id in users:
            user_list.append(users[user_id])
        return json_response(user_list)
    
    # Get single user
    def get_user(req):
        user_id_str = req.param("id")
        if user_id_str == None:
            return error_response(400, "User ID required")
        
        user_id = int(user_id_str)
        if user_id not in users:
            return error_response(404, "User not found")
        
        return json_response(users[user_id])
    
    # Create new user
    def create_user(req):
        data = req.json()
        if data == None:
            return error_response(400, "Invalid JSON")
        
        name = data.get("name")
        email = data.get("email")
        
        if name == None or email == None:
            return error_response(400, "Name and email required")
        
        user = {
            "id": next_id[0],
            "name": name,
            "email": email,
        }
        users[next_id[0]] = user
        next_id[0] = next_id[0] + 1
        
        return json_response(user, status=201)
    
    # Register routes
    srv.get("/api/users", list_users)
    srv.get("/api/users/{id}", get_user)
    srv.post("/api/users", create_user)
    
    print("RESTful API server created")

main()
`

	// Create machine with web module
	machine := starlet.NewWithNames(starlet.StringAnyMap{}, []string{"go_idiomatic"}, []string{})
	machine.SetPrintFunc(func(thread *starlark.Thread, msg string) {
		fmt.Println(msg)
	})

	// Load web module
	webModule := NewModule()
	machine.AddLazyloadModules(starlet.ModuleLoaderMap{
		ModuleName: webModule.LoadModule(),
	})

	_, err := machine.RunScript([]byte(script), nil)
	if err != nil {
		panic(err)
	}

	// Output: RESTful API server created
}

func Example_middleware() {
	script := `
load("web", "create_server", "response", "cors_middleware", "logging_middleware")

def main():
    srv = create_server(port=8080)
    
    # Add CORS middleware
    srv.use(cors_middleware(
        origins=["https://example.com"],
        methods=["GET", "POST", "PUT", "DELETE"],
        credentials=True
    ))
    
    # Add logging middleware
    srv.use(logging_middleware(
        format="{method} {path} {status} {duration}",
        skip_paths=["/health"]
    ))
    
    # Simple handler
    def hello(req):
        return response("Hello, World!")
    
    # Health check (skipped by logging)
    def health(req):
        return response("OK")
    
    srv.get("/hello", hello)
    srv.get("/health", health)
    
    print("Server with middleware created")

main()
`

	// Create machine with web module
	machine := starlet.NewDefault()
	machine.SetPrintFunc(func(thread *starlark.Thread, msg string) {
		fmt.Println(msg)
	})

	// Load web module
	webModule := NewModule()
	machine.AddLazyloadModules(starlet.ModuleLoaderMap{
		ModuleName: webModule.LoadModule(),
	})

	_, err := machine.RunScript([]byte(script), nil)
	if err != nil {
		panic(err)
	}

	// Output: Server with middleware created
}

func TestWebModule(t *testing.T) {
	// Test that the module can be created and loaded
	module := NewModule()
	if module == nil {
		t.Fatal("Failed to create web module")
	}

	loader := module.LoadModule()
	if loader == nil {
		t.Fatal("Failed to load web module")
	}
}
