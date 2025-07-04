package web

import (
	"reflect"
	"testing"

	"go.starlark.net/starlark"
)

func TestStarlarkListToStringSlice(t *testing.T) {
	// Test nil list
	result := starlarkListToStringSlice(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// Test normal list
	list := starlark.NewList([]starlark.Value{
		starlark.String("hello"),
		starlark.String("world"),
	})

	result = starlarkListToStringSlice(list)
	expected := []string{"hello", "world"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestStarlarkListToIntSlice(t *testing.T) {
	// Test nil list
	result := starlarkListToIntSlice(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// Test normal list
	list := starlark.NewList([]starlark.Value{
		starlark.MakeInt(42),
		starlark.MakeInt(100),
	})

	result = starlarkListToIntSlice(list)
	expected := []int{42, 100}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestStarlarkDictToStringMap(t *testing.T) {
	// Test nil dict
	result := starlarkDictToStringMap(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// Test normal dict
	dict := starlark.NewDict(2)
	dict.SetKey(starlark.String("key1"), starlark.String("value1"))
	dict.SetKey(starlark.String("key2"), starlark.String("value2"))

	result = starlarkDictToStringMap(dict)
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestStarlarkDictToHeaders(t *testing.T) {
	// Test nil dict
	result := starlarkDictToHeaders(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// Test normal dict
	dict := starlark.NewDict(2)
	dict.SetKey(starlark.String("Content-Type"), starlark.String("application/json"))
	dict.SetKey(starlark.String("Authorization"), starlark.String("Bearer token123"))

	result = starlarkDictToHeaders(dict)
	expected := map[string][]string{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer token123"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestCreateMultiValueDict(t *testing.T) {
	// Test with single values
	data := map[string][]string{
		"Content-Type": {"application/json"},
		"Accept":       {"text/html"},
	}

	result := createMultiValueDict(data)

	// Check Content-Type
	contentType, _, err := result.Get(starlark.String("Content-Type"))
	if err != nil {
		t.Errorf("Failed to get Content-Type: %v", err)
	}
	if contentType.(starlark.String).GoString() != "application/json" {
		t.Errorf("Expected 'application/json', got %v", contentType.(starlark.String).GoString())
	}

	// Test with multiple values
	multiData := map[string][]string{
		"Accept-Encoding": {"gzip", "deflate", "br"},
	}

	result = createMultiValueDict(multiData)

	acceptEncoding, _, err := result.Get(starlark.String("Accept-Encoding"))
	if err != nil {
		t.Errorf("Failed to get Accept-Encoding: %v", err)
	}

	// Should be a list
	list, ok := acceptEncoding.(*starlark.List)
	if !ok {
		t.Errorf("Expected list, got %T", acceptEncoding)
	}

	if list.Len() != 3 {
		t.Errorf("Expected list length 3, got %d", list.Len())
	}
}

func TestErrorHelpers(t *testing.T) {
	tests := []struct {
		name     string
		helper   func(string) *Response
		expected int
	}{
		{"BadRequest", BadRequest, 400},
		{"Unauthorized", Unauthorized, 401},
		{"Forbidden", Forbidden, 403},
		{"NotFound", NotFound, 404},
		{"MethodNotAllowed", MethodNotAllowed, 405},
		{"InternalServerError", InternalServerError, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.helper("test message")
			if resp.StatusCode != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, resp.StatusCode)
			}
			if resp.Body != "test message" {
				t.Errorf("Expected body 'test message', got '%s'", resp.Body)
			}
		})
	}
}

func TestJSONErrorHelpers(t *testing.T) {
	tests := []struct {
		name     string
		helper   func(string) *Response
		expected int
	}{
		{"BadRequestJSON", BadRequestJSON, 400},
		{"UnauthorizedJSON", UnauthorizedJSON, 401},
		{"ForbiddenJSON", ForbiddenJSON, 403},
		{"NotFoundJSON", NotFoundJSON, 404},
		{"InternalServerErrorJSON", InternalServerErrorJSON, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.helper("test error")
			if resp.StatusCode != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, resp.StatusCode)
			}

			// Check content type
			contentType := resp.Headers["Content-Type"]
			if len(contentType) == 0 || contentType[0] != "application/json" {
				t.Errorf("Expected JSON content type, got %v", contentType)
			}

			// Check JSON data
			jsonData, ok := resp.JSONData.(map[string]interface{})
			if !ok {
				t.Errorf("Expected map[string]interface{}, got %T", resp.JSONData)
			}

			if jsonData["error"] != "test error" {
				t.Errorf("Expected error message 'test error', got '%v'", jsonData["error"])
			}
		})
	}
}
