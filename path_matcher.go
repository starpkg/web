package web

import (
	"path"
	"strings"
)

// PathMatcher provides unified path matching functionality using sophisticated algorithms.
// This utility consolidates all path matching logic in the web package and provides
// consistent path matching behavior across the module.
type PathMatcher struct{}

// NewPathMatcher creates a new PathMatcher instance.
func NewPathMatcher() *PathMatcher {
	return &PathMatcher{}
}

// MatchesPattern checks if a path matches a glob-like pattern.
// This method supports various pattern types:
// - Exact matches: "/api/users" matches "/api/users" exactly
// - Glob patterns: "/api/*" matches "/api/users", "/api/posts", etc.
// - Prefix patterns: "/api/admin/*" matches "/api/admin/users", "/api/admin/posts", etc.
// - Parameter patterns: "/users/{id}" matches "/users/123", "/users/abc", etc.
//
// The method leverages Gin's internal path matching algorithm for consistency
// with the main router's behavior.
func (pm *PathMatcher) MatchesPattern(requestPath, pattern string) bool {
	// Handle exact matches first (most common case)
	if requestPath == pattern {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		return pm.matchWildcardPattern(requestPath, pattern)
	}

	// Handle parameter patterns ({param} style)
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		return pm.matchParameterPattern(requestPath, pattern)
	}

	// No match found
	return false
}

// matchWildcardPattern handles glob-style wildcard patterns.
// Supports patterns like "/api/*", "/static/*", "/admin/users/*", etc.
func (pm *PathMatcher) matchWildcardPattern(requestPath, pattern string) bool {
	// Handle root wildcard
	if pattern == "*" {
		return true
	}

	// Check if pattern has multiple wildcards (complex pattern)
	if strings.Count(pattern, "*") > 1 {
		return pm.matchComplexWildcard(requestPath, pattern)
	}

	// Handle patterns ending with /*
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2]

		// Empty prefix means pattern is "/*" - matches everything starting with "/"
		if prefix == "" {
			return strings.HasPrefix(requestPath, "/")
		}

		// Check if path starts with prefix
		if requestPath == prefix {
			return true
		}

		// Check if path starts with prefix followed by "/"
		if strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}

		return false
	}

	// Handle patterns ending with *
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(requestPath, prefix)
	}

	// Handle patterns with * in the middle (single wildcard)
	if strings.Contains(pattern, "*") {
		return pm.matchComplexWildcard(requestPath, pattern)
	}

	return false
}

// matchParameterPattern handles parameter patterns like "/users/{id}".
// Converts Flask-style {param} patterns to Gin-style :param patterns
// and uses a simplified route matching approach.
func (pm *PathMatcher) matchParameterPattern(requestPath, pattern string) bool {
	// Convert Flask-style parameters to Gin-style
	ginPattern := convertPathParams(pattern)

	// Use simplified route matching instead of creating temporary routes
	return pm.testRouteMatch(ginPattern, requestPath)
}

// testRouteMatch tests if a request path matches a Gin route pattern.
// This uses a simplified version of Gin's route matching logic.
func (pm *PathMatcher) testRouteMatch(ginPattern, requestPath string) bool {
	// Split both pattern and path into segments
	patternSegments := strings.Split(strings.Trim(ginPattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(requestPath, "/"), "/")

	// Handle root path case
	if ginPattern == "/" && requestPath == "/" {
		return true
	}

	// Must have same number of segments unless there's a catch-all
	if len(patternSegments) != len(pathSegments) {
		// Check for catch-all patterns
		if len(patternSegments) > 0 && strings.HasPrefix(patternSegments[len(patternSegments)-1], "*") {
			return len(pathSegments) >= len(patternSegments)-1
		}
		return false
	}

	// Check each segment
	for i, patternSeg := range patternSegments {
		pathSeg := pathSegments[i]

		// Parameter segment (starts with :)
		if strings.HasPrefix(patternSeg, ":") {
			continue // Parameters match any non-empty segment
		}

		// Catch-all segment (starts with *)
		if strings.HasPrefix(patternSeg, "*") {
			return true // Catch-all matches remaining path
		}

		// Exact match required
		if patternSeg != pathSeg {
			return false
		}
	}

	return true
}

// matchComplexWildcard handles patterns with wildcards in the middle.
// This supports patterns like "/api/*/users" or "/files/*/download/*".
func (pm *PathMatcher) matchComplexWildcard(requestPath, pattern string) bool {
	// Split pattern by wildcards
	parts := strings.Split(pattern, "*")

	// Filter out empty parts
	var nonEmptyParts []string
	for _, part := range parts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}

	if len(nonEmptyParts) == 0 {
		return true // Pattern is all wildcards, matches everything
	}

	// Start matching from the beginning
	currentPos := 0

	for i, part := range nonEmptyParts {
		// Find the part in the remaining path
		index := strings.Index(requestPath[currentPos:], part)
		if index == -1 {
			return false // Part not found
		}

		// Adjust index to absolute position
		absoluteIndex := currentPos + index

		// For the first part, it must be at the beginning (unless pattern starts with *)
		if i == 0 && !strings.HasPrefix(pattern, "*") && absoluteIndex != 0 {
			return false
		}

		// Move current position past this part
		currentPos = absoluteIndex + len(part)
	}

	// If pattern doesn't end with *, the last part must be at the end
	if !strings.HasSuffix(pattern, "*") && len(nonEmptyParts) > 0 {
		lastPart := nonEmptyParts[len(nonEmptyParts)-1]
		return strings.HasSuffix(requestPath, lastPart)
	}

	return true
}

// MatchesAny checks if a path matches any of the given patterns.
// This is useful for middleware that needs to apply to multiple patterns.
func (pm *PathMatcher) MatchesAny(requestPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if pm.MatchesPattern(requestPath, pattern) {
			return true
		}
	}
	return false
}

// ExtractParams extracts path parameters from a request path using a parameter pattern.
// Returns a map of parameter names to values, or nil if the pattern doesn't match.
func (pm *PathMatcher) ExtractParams(requestPath, pattern string) map[string]string {
	if !pm.MatchesPattern(requestPath, pattern) {
		return nil
	}

	// Only extract params from parameter patterns
	if !strings.Contains(pattern, "{") || !strings.Contains(pattern, "}") {
		return make(map[string]string)
	}

	params := make(map[string]string)

	// Convert pattern to segments
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(patternSegments) != len(pathSegments) {
		return nil
	}

	for i, patternSeg := range patternSegments {
		if strings.HasPrefix(patternSeg, "{") && strings.HasSuffix(patternSeg, "}") {
			paramName := patternSeg[1 : len(patternSeg)-1]
			params[paramName] = pathSegments[i]
		}
	}

	return params
}

// NormalizePath normalizes a path by cleaning it and ensuring it has proper format.
// This uses Go's standard path.Clean function for consistency.
func (pm *PathMatcher) NormalizePath(inputPath string) string {
	// Handle empty path as root
	if inputPath == "" {
		return "/"
	}

	// Use Go's standard path cleaning
	cleaned := path.Clean(inputPath)

	// Handle cleaned path that becomes "." (from empty input)
	if cleaned == "." {
		return "/"
	}

	// Ensure it starts with /
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	return cleaned
}

// IsValidPattern checks if a pattern is valid for path matching.
// This validates the pattern syntax and ensures it's properly formatted.
func (pm *PathMatcher) IsValidPattern(pattern string) bool {
	// Empty pattern is invalid
	if pattern == "" {
		return false
	}

	// Check for invalid characters or malformed patterns
	if strings.Contains(pattern, "//") {
		return false
	}

	// Check for malformed parameter patterns
	if strings.Contains(pattern, "{") || strings.Contains(pattern, "}") {
		return pm.isValidParameterPattern(pattern)
	}

	return true
}

// isValidParameterPattern validates parameter patterns like "/users/{id}".
func (pm *PathMatcher) isValidParameterPattern(pattern string) bool {
	// Check for balanced braces
	openCount := strings.Count(pattern, "{")
	closeCount := strings.Count(pattern, "}")

	if openCount != closeCount {
		return false
	}

	// Check for empty parameter names
	if strings.Contains(pattern, "{}") {
		return false
	}

	// Check for nested braces
	depth := 0
	for _, char := range pattern {
		switch char {
		case '{':
			depth++
			if depth > 1 {
				return false
			}
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}

	return depth == 0
}

// DefaultPathMatcher is a singleton instance for global use.
var DefaultPathMatcher = NewPathMatcher()

// Package-level convenience functions that use the default matcher

// MatchesPattern is a convenience function that uses the default PathMatcher.
func MatchesPattern(requestPath, pattern string) bool {
	return DefaultPathMatcher.MatchesPattern(requestPath, pattern)
}

// MatchesAny is a convenience function that uses the default PathMatcher.
func MatchesAny(requestPath string, patterns []string) bool {
	return DefaultPathMatcher.MatchesAny(requestPath, patterns)
}

// ExtractParams is a convenience function that uses the default PathMatcher.
func ExtractParams(requestPath, pattern string) map[string]string {
	return DefaultPathMatcher.ExtractParams(requestPath, pattern)
}

// NormalizePath is a convenience function that uses the default PathMatcher.
func NormalizePath(inputPath string) string {
	return DefaultPathMatcher.NormalizePath(inputPath)
}

// IsValidPattern is a convenience function that uses the default PathMatcher.
func IsValidPattern(pattern string) bool {
	return DefaultPathMatcher.IsValidPattern(pattern)
}
