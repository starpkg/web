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
    
    # TODO: make something calling via http package to verify the response
    
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
