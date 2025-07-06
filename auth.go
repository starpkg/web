package web

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Authenticator represents an authentication handler that can be used as middleware
type Authenticator struct {
	validateFunc starlark.Callable
	authType     string
	config       map[string]interface{}
}

// AuthResult represents the result of authentication
type AuthResult struct {
	Success   bool
	UserInfo  interface{}
	ErrorCode int
	Message   string
}

// Middleware returns a middleware function for this authenticator
func (a *Authenticator) Middleware() MiddlewareFunc {
	return func(req *Request, next NextFunc) *Response {
		result := a.authenticate(req)
		if !result.Success {
			return &Response{
				StatusCode: result.ErrorCode,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: fmt.Sprintf(`{"error":%q}`, result.Message),
			}
		}

		// Store user info in request context
		if result.UserInfo != nil {
			req.Context["auth_user"] = result.UserInfo
		}

		return next(req)
	}
}

// authenticate performs the actual authentication
func (a *Authenticator) authenticate(req *Request) *AuthResult {
	switch a.authType {
	case "api_key":
		return a.authenticateAPIKey(req)
	case "bearer":
		return a.authenticateBearerToken(req)
	case "basic":
		return a.authenticateBasic(req)
	default:
		return &AuthResult{
			Success:   false,
			ErrorCode: 500,
			Message:   "Unknown authentication type",
		}
	}
}

// authenticateAPIKey handles API key authentication
func (a *Authenticator) authenticateAPIKey(req *Request) *AuthResult {
	keys := a.config["keys"].([]string)
	header := a.config["header"].(string)
	queryParam := a.config["query_param"].(string)

	// Try header first
	var apiKey string
	if headerValue, exists := req.Headers[header]; exists {
		apiKey = headerValue
	} else if queryParam != "" {
		// Try query parameter
		if queryValue, exists := req.Query[queryParam]; exists {
			apiKey = queryValue
		}
	}

	if apiKey == "" {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "API key required",
		}
	}

	// Check if key is valid
	for _, validKey := range keys {
		if apiKey == validKey {
			return &AuthResult{
				Success:  true,
				UserInfo: map[string]interface{}{"api_key": apiKey},
			}
		}
	}

	return &AuthResult{
		Success:   false,
		ErrorCode: 401,
		Message:   "Invalid API key",
	}
}

// authenticateBearerToken handles bearer token authentication
func (a *Authenticator) authenticateBearerToken(req *Request) *AuthResult {
	header := a.config["header"].(string)
	validateFunc := a.validateFunc

	authHeader, exists := req.Headers[header]
	if !exists {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Authorization header required",
		}
	}

	var token string
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = authHeader[7:] // Remove "Bearer " prefix
	} else {
		// If using custom header, use the value directly
		token = authHeader
	}

	if token == "" {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Bearer token required",
		}
	}

	// Call validation function
	thread := &starlark.Thread{Name: "auth"}
	args := starlark.Tuple{starlark.String(token)}
	result, err := starlark.Call(thread, validateFunc, args, nil)
	if err != nil {
		return &AuthResult{
			Success:   false,
			ErrorCode: 500,
			Message:   "Token validation error",
		}
	}

	// If result is None, authentication failed
	if result == starlark.None {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Invalid token",
		}
	}

	// Convert result to Go value
	userInfo, err := dataconv.Unmarshal(result)
	if err != nil {
		userInfo = map[string]interface{}{"token": token}
	}

	return &AuthResult{
		Success:  true,
		UserInfo: userInfo,
	}
}

// authenticateBasic handles basic authentication
func (a *Authenticator) authenticateBasic(req *Request) *AuthResult {
	users := a.config["users"].(map[string]string)
	realm := a.config["realm"].(string)

	authHeader, exists := req.Headers["Authorization"]
	if !exists {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   fmt.Sprintf("Basic realm=\"%s\"", realm),
		}
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Basic authentication required",
		}
	}

	// Decode base64 credentials
	encoded := authHeader[6:] // Remove "Basic " prefix
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Invalid credentials format",
		}
	}

	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return &AuthResult{
			Success:   false,
			ErrorCode: 401,
			Message:   "Invalid credentials format",
		}
	}

	username, password := parts[0], parts[1]

	// Check credentials
	if validPassword, exists := users[username]; exists && validPassword == password {
		return &AuthResult{
			Success:  true,
			UserInfo: map[string]interface{}{"username": username},
		}
	}

	return &AuthResult{
		Success:   false,
		ErrorCode: 401,
		Message:   "Invalid credentials",
	}
}

// AuthenticatorWrapper wraps the Authenticator for Starlark
type AuthenticatorWrapper struct {
	auth *Authenticator
}

// NewAuthenticatorWrapper creates a new wrapper
func NewAuthenticatorWrapper(auth *Authenticator) *AuthenticatorWrapper {
	return &AuthenticatorWrapper{auth: auth}
}

// String returns string representation
func (aw *AuthenticatorWrapper) String() string {
	return fmt.Sprintf("<web.Authenticator type=%s>", aw.auth.authType)
}

// Type returns the Starlark type name
func (aw *AuthenticatorWrapper) Type() string {
	return "web.Authenticator"
}

// Freeze makes the object immutable
func (aw *AuthenticatorWrapper) Freeze() {}

// Truth returns the truth value
func (aw *AuthenticatorWrapper) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash value
func (aw *AuthenticatorWrapper) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", aw.Type())
}

// Attr returns the named attribute
func (aw *AuthenticatorWrapper) Attr(name string) (starlark.Value, error) {
	switch name {
	case "middleware":
		return starlark.NewBuiltin("middleware", aw.middlewareMethod), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("%s has no .%s attribute", aw.Type(), name))
	}
}

// AttrNames returns available attributes
func (aw *AuthenticatorWrapper) AttrNames() []string {
	return []string{"middleware"}
}

// middlewareMethod returns the middleware function
func (aw *AuthenticatorWrapper) middlewareMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	// Return a middleware function wrapper
	middleware := &MiddlewareWrapper{middleware: aw.auth.Middleware()}
	return middleware, nil
}
