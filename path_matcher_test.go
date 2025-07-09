package web

import (
	"testing"
)

func TestPathMatcher_MatchesPattern(t *testing.T) {
	pm := NewPathMatcher()

	tests := []struct {
		name        string
		requestPath string
		pattern     string
		expected    bool
	}{
		// Exact matches
		{"exact_match_root", "/", "/", true},
		{"exact_match_simple", "/api/users", "/api/users", true},
		{"exact_match_with_trailing_slash", "/api/users/", "/api/users/", true},
		{"exact_no_match", "/api/users", "/api/posts", false},
		{"exact_no_match_partial", "/api/users", "/api", false},
		{"exact_no_match_longer_pattern", "/api", "/api/users", false},

		// Wildcard patterns - global
		{"wildcard_global_root", "/", "*", true},
		{"wildcard_global_simple", "/api/users", "*", true},
		{"wildcard_global_complex", "/api/v1/users/123/posts", "*", true},

		// Wildcard patterns - suffix
		{"wildcard_suffix_match", "/api/users", "/api/*", true},
		{"wildcard_suffix_match_longer", "/api/users/123", "/api/*", true},
		{"wildcard_suffix_match_exact_prefix", "/api", "/api/*", true},
		{"wildcard_suffix_no_match", "/dashboard", "/api/*", false},
		{"wildcard_suffix_no_match_partial", "/ap", "/api/*", false},

		// Wildcard patterns - prefix
		{"wildcard_prefix_match", "/static/css/style.css", "/static*", true},
		{"wildcard_prefix_match_exact", "/static", "/static*", true},
		{"wildcard_prefix_no_match", "/public/css/style.css", "/static*", false},

		// Complex wildcard patterns
		{"complex_wildcard_middle", "/files/images/download/pic.jpg", "/files/*/download/*", true},
		{"complex_wildcard_middle_no_match", "/files/images/view/pic.jpg", "/files/*/download/*", false},
		{"complex_wildcard_multiple", "/api/v1/users/123/posts/456", "/api/*/users/*/posts/*", true},

		// Parameter patterns
		{"parameter_single", "/users/123", "/users/{id}", true},
		{"parameter_single_alpha", "/users/abc", "/users/{id}", true},
		{"parameter_single_no_match_missing", "/users", "/users/{id}", false},
		{"parameter_single_no_match_extra", "/users/123/posts", "/users/{id}", false},
		{"parameter_multiple", "/users/123/posts/456", "/users/{id}/posts/{post_id}", true},
		{"parameter_multiple_no_match", "/users/123/comments/456", "/users/{id}/posts/{post_id}", false},

		// Edge cases
		{"empty_path_empty_pattern", "", "", true},
		{"empty_path_root_pattern", "", "/", false},
		{"root_path_empty_pattern", "/", "", false},
		{"double_slash_path", "//api//users", "/api/users", false},
		{"trailing_slash_difference", "/api/users", "/api/users/", false},
		{"case_sensitive", "/API/USERS", "/api/users", false},

		// Special characters
		{"path_with_query", "/api/users?page=1", "/api/users", false},
		{"path_with_fragment", "/api/users#section", "/api/users", false},
		{"path_with_encoded_chars", "/api/users%20test", "/api/users%20test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesPattern(tt.requestPath, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesPattern(%q, %q) = %v, want %v",
					tt.requestPath, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestPathMatcher_MatchesAny(t *testing.T) {
	pm := NewPathMatcher()

	tests := []struct {
		name        string
		requestPath string
		patterns    []string
		expected    bool
	}{
		{
			"match_first_pattern",
			"/api/users",
			[]string{"/api/*", "/dashboard/*", "/admin/*"},
			true,
		},
		{
			"match_middle_pattern",
			"/dashboard/stats",
			[]string{"/api/*", "/dashboard/*", "/admin/*"},
			true,
		},
		{
			"match_last_pattern",
			"/admin/users",
			[]string{"/api/*", "/dashboard/*", "/admin/*"},
			true,
		},
		{
			"no_match",
			"/public/files",
			[]string{"/api/*", "/dashboard/*", "/admin/*"},
			false,
		},
		{
			"empty_patterns",
			"/api/users",
			[]string{},
			false,
		},
		{
			"single_pattern_match",
			"/api/users",
			[]string{"/api/*"},
			true,
		},
		{
			"single_pattern_no_match",
			"/dashboard/stats",
			[]string{"/api/*"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesAny(tt.requestPath, tt.patterns)
			if result != tt.expected {
				t.Errorf("MatchesAny(%q, %v) = %v, want %v",
					tt.requestPath, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestPathMatcher_ExtractParams(t *testing.T) {
	pm := NewPathMatcher()

	tests := []struct {
		name        string
		requestPath string
		pattern     string
		expected    map[string]string
		shouldMatch bool
	}{
		{
			"single_parameter",
			"/users/123",
			"/users/{id}",
			map[string]string{"id": "123"},
			true,
		},
		{
			"multiple_parameters",
			"/users/123/posts/456",
			"/users/{id}/posts/{post_id}",
			map[string]string{"id": "123", "post_id": "456"},
			true,
		},
		{
			"parameter_with_alpha",
			"/users/alice",
			"/users/{username}",
			map[string]string{"username": "alice"},
			true,
		},
		{
			"no_parameters_exact",
			"/api/users",
			"/api/users",
			map[string]string{},
			true,
		},
		{
			"no_parameters_wildcard",
			"/api/users",
			"/api/*",
			map[string]string{},
			true,
		},
		{
			"no_match_returns_nil",
			"/users/123/comments",
			"/users/{id}/posts/{post_id}",
			nil,
			false,
		},
		{
			"pattern_no_match",
			"/api/posts",
			"/users/{id}",
			nil,
			false,
		},
		{
			"empty_parameter_value",
			"/users//posts/123",
			"/users/{id}/posts/{post_id}",
			map[string]string{"id": "", "post_id": "123"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.ExtractParams(tt.requestPath, tt.pattern)

			if !tt.shouldMatch {
				if result != nil {
					t.Errorf("ExtractParams(%q, %q) = %v, want nil",
						tt.requestPath, tt.pattern, result)
				}
				return
			}

			if result == nil && tt.expected != nil {
				t.Errorf("ExtractParams(%q, %q) = nil, want %v",
					tt.requestPath, tt.pattern, tt.expected)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ExtractParams(%q, %q) returned %d params, want %d",
					tt.requestPath, tt.pattern, len(result), len(tt.expected))
				return
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("ExtractParams(%q, %q) missing parameter %q",
						tt.requestPath, tt.pattern, key)
				} else if actualValue != expectedValue {
					t.Errorf("ExtractParams(%q, %q) parameter %q = %q, want %q",
						tt.requestPath, tt.pattern, key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestPathMatcher_NormalizePath(t *testing.T) {
	pm := NewPathMatcher()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already_normalized", "/api/users", "/api/users"},
		{"missing_leading_slash", "api/users", "/api/users"},
		{"double_slashes", "//api//users//", "/api/users"},
		{"trailing_slash", "/api/users/", "/api/users"},
		{"dot_segments", "/api/./users/../posts", "/api/posts"},
		{"root_path", "/", "/"},
		{"empty_path", "", "/"},
		{"complex_path", "///api/../v1/./users///", "/v1/users"},
		{"relative_path", "api/../users/./posts", "/users/posts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathMatcher_IsValidPattern(t *testing.T) {
	pm := NewPathMatcher()

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		// Valid patterns
		{"valid_exact", "/api/users", true},
		{"valid_wildcard", "/api/*", true},
		{"valid_parameter", "/users/{id}", true},
		{"valid_multiple_parameters", "/users/{id}/posts/{post_id}", true},
		{"valid_root", "/", true},
		{"valid_global_wildcard", "*", true},
		{"valid_complex_wildcard", "/files/*/download/*", true},

		// Invalid patterns
		{"invalid_empty", "", false},
		{"invalid_double_slash", "/api//users", false},
		{"invalid_unbalanced_braces_open", "/users/{id", false},
		{"invalid_unbalanced_braces_close", "/users/id}", false},
		{"invalid_empty_parameter", "/users/{}", false},
		{"invalid_nested_braces", "/users/{id{nested}}", false},
		{"invalid_multiple_open_braces", "/users/{{id}", false},
		{"invalid_multiple_close_braces", "/users/{id}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.IsValidPattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("IsValidPattern(%q) = %v, want %v",
					tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestPathMatcher_NoopPanics(t *testing.T) {
	pm := NewPathMatcher()

	// Test that methods don't panic with various inputs
	testCases := []struct {
		name        string
		requestPath string
		pattern     string
	}{
		{"nil_strings", "", ""},
		{"very_long_path", string(make([]byte, 10000)), "/api/*"},
		{"very_long_pattern", "/api/users", string(make([]byte, 10000))},
		{"unicode_path", "/api/用户", "/api/*"},
		{"unicode_pattern", "/api/用户/{用户名}", "/api/用户/test"},
		{"special_chars_path", "/api/users!@#$%^&*()", "/api/*"},
		{"special_chars_pattern", "/api/users!@#$%^&*()", "/api/users!@#$%^&*()"},
		{"malformed_pattern", "/users/{id", "/users/123"},
		{"deeply_nested", "/a/b/c/d/e/f/g/h/i/j", "/a/*"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// These operations should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PathMatcher operation panicked with %v", r)
				}
			}()

			_ = pm.MatchesPattern(tt.requestPath, tt.pattern)
			_ = pm.MatchesAny(tt.requestPath, []string{tt.pattern})
			_ = pm.ExtractParams(tt.requestPath, tt.pattern)
			_ = pm.NormalizePath(tt.requestPath)
			_ = pm.IsValidPattern(tt.pattern)
		})
	}
}

func TestPathMatcher_Performance(t *testing.T) {
	pm := NewPathMatcher()

	// Test performance with many patterns
	patterns := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		patterns[i] = "/api/v1/resource" + string(rune(i%10+'0')) + "/*"
	}

	requestPath := "/api/v1/resource5/items/123"

	// This should complete reasonably quickly
	result := pm.MatchesAny(requestPath, patterns)
	if !result {
		t.Error("Expected match in performance test")
	}
}

func TestPathMatcher_ConcurrentAccess(t *testing.T) {
	pm := NewPathMatcher()

	// Test concurrent access to the default matcher
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			path := "/api/users/" + string(rune(id%10+'0'))
			pattern := "/api/*"

			for j := 0; j < 100; j++ {
				_ = pm.MatchesPattern(path, pattern)
				_ = pm.ExtractParams(path, "/api/users/{id}")
				_ = pm.NormalizePath(path)
				_ = pm.IsValidPattern(pattern)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Test package-level convenience functions
func TestPackageLevelFunctions(t *testing.T) {
	// Test that package-level functions work correctly
	if !MatchesPattern("/api/users", "/api/*") {
		t.Error("Package-level MatchesPattern failed")
	}

	if !MatchesAny("/api/users", []string{"/api/*", "/dashboard/*"}) {
		t.Error("Package-level MatchesAny failed")
	}

	params := ExtractParams("/users/123", "/users/{id}")
	if params == nil || params["id"] != "123" {
		t.Error("Package-level ExtractParams failed")
	}

	normalized := NormalizePath("//api//users//")
	if normalized != "/api/users" {
		t.Error("Package-level NormalizePath failed")
	}

	if !IsValidPattern("/api/*") {
		t.Error("Package-level IsValidPattern failed")
	}
}
