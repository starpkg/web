package web

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestRouteRegistrarInterface(t *testing.T) {
	// Test that both Server and RouteGroup implement RouteRegistrar
	config := &ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	server := NewServer(config)

	// Test that Server implements RouteRegistrar
	var serverRegistrar RouteRegistrar = server
	if serverRegistrar == nil {
		t.Error("Server should implement RouteRegistrar interface")
	}

	// Test that RouteGroup implements RouteRegistrar
	group := NewRouteGroup(server, "/api")
	var groupRegistrar RouteRegistrar = group
	if groupRegistrar == nil {
		t.Error("RouteGroup should implement RouteRegistrar interface")
	}
}

func TestHTTPMethodHandlers(t *testing.T) {
	// Test that the generic HTTP method handlers work correctly
	config := &ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	server := NewServer(config)
	thread := &starlark.Thread{}

	// Create a simple handler function
	handler := starlark.NewBuiltin("test_handler", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("OK"), nil
	})

	// Create a mock builtin for testing
	mockBuiltin := starlark.NewBuiltin("test", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	// Test GET method registration
	args := starlark.Tuple{starlark.String("/test"), handler}
	_, err := handleGet(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("GET method registration failed: %v", err)
	}

	// Test POST method registration
	_, err = handlePost(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("POST method registration failed: %v", err)
	}

	// Test PUT method registration
	_, err = handlePut(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("PUT method registration failed: %v", err)
	}

	// Test DELETE method registration
	_, err = handleDelete(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("DELETE method registration failed: %v", err)
	}

	// Test PATCH method registration
	_, err = handlePatch(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("PATCH method registration failed: %v", err)
	}

	// Test OPTIONS method registration
	_, err = handleOptions(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("OPTIONS method registration failed: %v", err)
	}

	// Test HEAD method registration
	_, err = handleHead(server, thread, mockBuiltin, args, nil)
	if err != nil {
		t.Errorf("HEAD method registration failed: %v", err)
	}
}

func TestPathTransformers(t *testing.T) {
	tests := []struct {
		name        string
		transformer pathTransformer
		input       string
		expected    string
	}{
		{
			name:        "Identity transformer",
			transformer: identityPathTransformer,
			input:       "/test/path",
			expected:    "/test/path",
		},
		{
			name:        "Prefix transformer with slash",
			transformer: prefixPathTransformer("/api"),
			input:       "/users",
			expected:    "/api/users",
		},
		{
			name:        "Prefix transformer without slash",
			transformer: prefixPathTransformer("api"),
			input:       "/users",
			expected:    "/api/users",
		},
		{
			name:        "Prefix transformer with trailing slash",
			transformer: prefixPathTransformer("/api/"),
			input:       "/users",
			expected:    "/api/users",
		},
		{
			name:        "Empty prefix",
			transformer: prefixPathTransformer(""),
			input:       "/users",
			expected:    "/users",
		},
		{
			name:        "Path without leading slash",
			transformer: prefixPathTransformer("/api"),
			input:       "users",
			expected:    "/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.transformer(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRouteRegistrarImpl(t *testing.T) {
	// Test the route registrar implementation
	router := NewRouter()
	registrar := newRouteRegistrarImpl(router, identityPathTransformer)

	// Create a simple handler
	handler := starlark.NewBuiltin("test_handler", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("OK"), nil
	})

	// Test route registration
	err := registrar.registerRoute("GET", "/test", handler)
	if err != nil {
		t.Errorf("Route registration failed: %v", err)
	}

	// Verify that the route was registered
	if len(router.routes) == 0 {
		t.Error("Route was not registered in router")
	}

	if tree, exists := router.routes["GET"]; !exists {
		t.Error("GET route tree was not created")
	} else if tree == nil {
		t.Error("GET route tree is nil")
	}
}

func TestCreateRouteBuiltins(t *testing.T) {
	// Test the route builtins creation
	config := &ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	server := NewServer(config)
	builtins := createRouteBuiltins(server)

	// Check that all expected builtins are present
	expectedBuiltins := []string{"get", "post", "put", "delete", "patch", "options", "head"}

	for _, expectedBuiltin := range expectedBuiltins {
		if builtin, exists := builtins[expectedBuiltin]; !exists {
			t.Errorf("Expected builtin %s not found", expectedBuiltin)
		} else if builtin == nil {
			t.Errorf("Builtin %s is nil", expectedBuiltin)
		}
	}

	// Check that the builtins are of the correct type
	for name, builtin := range builtins {
		expectedType := "builtin_function_or_method"
		if builtin.Type() != expectedType {
			t.Errorf("Builtin %s is not of correct type, expected %s, got %s", name, expectedType, builtin.Type())
		}
	}
}

func TestRouteGroupPrefixHandling(t *testing.T) {
	// Test that RouteGroup correctly handles prefix transformations
	config := &ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	server := NewServer(config)
	group := NewRouteGroup(server, "/api")

	// Create a test handler
	handler := starlark.NewBuiltin("test_handler", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("OK"), nil
	})

	// Test that RouteGroup's path transformer works correctly
	if group.pathTransformer == nil {
		t.Error("RouteGroup's path transformer is nil")
	}

	// Test prefix transformation
	transformedPath := group.pathTransformer("/users")
	expectedPath := "/api/users"
	if transformedPath != expectedPath {
		t.Errorf("Expected %s, got %s", expectedPath, transformedPath)
	}

	// Test route registration with prefix
	err := group.registerRoute("GET", "/users", handler)
	if err != nil {
		t.Errorf("Route registration with prefix failed: %v", err)
	}

	// Verify the route was registered with the correct path
	if tree, exists := server.router.routes["GET"]; exists {
		// The route should be registered with the full path including prefix
		// This is verified by checking that the route tree was created for GET
		if tree == nil {
			t.Error("Route tree is nil after registration")
		}
	} else {
		t.Error("GET route tree was not created")
	}
}
