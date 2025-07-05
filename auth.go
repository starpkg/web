package web

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Authentication implementations

// authMiddlewareFactory creates a standardized authentication middleware builtin
// This eliminates code duplication across different authentication types
func authMiddlewareFactory(middlewareFunc MiddlewareFunc) (starlark.Value, error) {
	result, err := dataconv.Marshal(middlewareFunc)
	if err != nil {
		return none, fmt.Errorf("failed to marshal auth middleware: %v", err)
	}
	return result, nil
}

// BasicAuth represents a basic authentication handler
type BasicAuth struct {
	users map[string]string
	realm string
}

// NewBasicAuth creates a new basic auth handler
func NewBasicAuth(users map[string]string, realm string) *BasicAuth {
	return &BasicAuth{
		users: users,
		realm: realm,
	}
}

// Struct returns a Starlark struct representation of the BasicAuth
func (ba *BasicAuth) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"middleware":   starlark.NewBuiltin("middleware", ba.Middleware),
		"validate":     starlark.NewBuiltin("validate", ba.Validate),
		"authenticate": starlark.NewBuiltin("authenticate", ba.Authenticate),
	}
	return starlarkstruct.FromStringDict(starlark.String("BasicAuth"), sd)
}

// Authenticate validates basic authentication for a request and returns result
func (ba *BasicAuth) Authenticate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var request starlark.Value

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"request", &request,
	); err != nil {
		return none, err
	}

	// Extract authorization header using request's get_header method
	var authHeader string
	if requestStruct, ok := request.(*starlarkstruct.Struct); ok {
		if getHeaderFunc, err := requestStruct.Attr("get_header"); err == nil {
			if callable, ok := getHeaderFunc.(starlark.Callable); ok {
				result, err := starlark.Call(thread, callable, starlark.Tuple{starlark.String("Authorization")}, nil)
				if err == nil && result != starlark.None {
					authHeader = result.String()
					// Remove quotes if present
					if strings.HasPrefix(authHeader, "\"") && strings.HasSuffix(authHeader, "\"") {
						authHeader = strings.Trim(authHeader, "\"")
					}
				}
			}
		}
	}

	if authHeader == "" {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Parse basic auth
	if !strings.HasPrefix(authHeader, "Basic ") {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	encoded := authHeader[6:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	username, password := parts[0], parts[1]

	// Check credentials
	if correctPassword, exists := ba.users[username]; !exists || correctPassword != password {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Return success result
	result := starlark.NewDict(2)
	result.SetKey(starlark.String("authenticated"), starlark.True)
	result.SetKey(starlark.String("user"), starlark.String(username))
	return result, nil
}

// Validate validates username and password credentials
func (ba *BasicAuth) Validate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var username, password starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"username", &username,
		"password", &password,
	); err != nil {
		return none, err
	}

	correctPassword, exists := ba.users[username.GoString()]
	if !exists || correctPassword != password.GoString() {
		return starlark.False, nil
	}

	return starlark.True, nil
}

// Middleware returns a middleware function that enforces basic authentication
func (ba *BasicAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Check for authorization header
		authHeader := req.Request.Header.Get("Authorization")
		if authHeader == "" {
			return &Response{
				StatusCode: 401,
				Body:       "Authorization required",
				Headers:    map[string][]string{"WWW-Authenticate": {fmt.Sprintf("Basic realm=\"%s\"", ba.realm)}},
			}
		}

		// Parse basic auth
		if !strings.HasPrefix(authHeader, "Basic ") {
			return Unauthorized("Invalid authorization format")
		}

		encoded := authHeader[6:]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return Unauthorized("Invalid authorization format")
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return Unauthorized("Invalid authorization format")
		}

		username, password := parts[0], parts[1]

		// Check credentials
		if correctPassword, exists := ba.users[username]; !exists || correctPassword != password {
			return Unauthorized("Invalid credentials")
		}

		// Add user to request context
		req.SetContext("username", username)

		return next(req)
	}

	return authMiddlewareFactory(middleware)
}

// basic_auth creates a basic HTTP authentication handler
func basicAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		users *starlark.Dict
		realm = starlark.String("Restricted")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"users?", &users,
		"realm?", &realm,
	); err != nil {
		return none, err
	}

	// Convert users dict to Go map using helper
	usersMap := starlarkDictToStringMap(users)

	basicAuth := &BasicAuth{
		users: usersMap,
		realm: realm.GoString(),
	}
	return basicAuth.Struct(), nil
}

// BearerAuth represents a bearer token authentication handler
type BearerAuth struct {
	validateFunc starlark.Callable
}

// NewBearerAuth creates a new bearer auth handler
func NewBearerAuth(validateFunc starlark.Callable) *BearerAuth {
	return &BearerAuth{
		validateFunc: validateFunc,
	}
}

// Struct returns a Starlark struct representation of the BearerAuth
func (ba *BearerAuth) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"middleware":   starlark.NewBuiltin("middleware", ba.Middleware),
		"validate":     starlark.NewBuiltin("validate", ba.ValidateToken),
		"authenticate": starlark.NewBuiltin("authenticate", ba.Authenticate),
	}
	return starlarkstruct.FromStringDict(starlark.String("BearerAuth"), sd)
}

// Authenticate validates bearer authentication for a request and returns result
func (ba *BearerAuth) Authenticate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var request starlark.Value

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"request", &request,
	); err != nil {
		return none, err
	}

	// Extract authorization header using request's get_header method
	var authHeader string
	if requestStruct, ok := request.(*starlarkstruct.Struct); ok {
		if getHeaderFunc, err := requestStruct.Attr("get_header"); err == nil {
			if callable, ok := getHeaderFunc.(starlark.Callable); ok {
				result, err := starlark.Call(thread, callable, starlark.Tuple{starlark.String("Authorization")}, nil)
				if err == nil && result != starlark.None {
					authHeader = result.String()
					// Remove quotes if present
					if strings.HasPrefix(authHeader, "\"") && strings.HasSuffix(authHeader, "\"") {
						authHeader = strings.Trim(authHeader, "\"")
					}
				}
			}
		}
	}

	if authHeader == "" {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Check for Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	token := authHeader[7:]

	// Validate token
	tokenResult, err := starlark.Call(thread, ba.validateFunc, starlark.Tuple{starlark.String(token)}, nil)
	if err != nil {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Check if token is valid (not None)
	if tokenResult == starlark.None {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Return success result
	result := starlark.NewDict(2)
	result.SetKey(starlark.String("authenticated"), starlark.True)
	result.SetKey(starlark.String("user"), tokenResult)
	return result, nil
}

// ValidateToken validates a bearer token
func (ba *BearerAuth) ValidateToken(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var token starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"token", &token,
	); err != nil {
		return none, err
	}

	// Call the validate function
	result, err := starlark.Call(thread, ba.validateFunc, starlark.Tuple{token}, nil)
	if err != nil {
		return none, err
	}

	return result, nil
}

// Middleware returns a middleware function that enforces bearer authentication
func (ba *BearerAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Get authorization header
		authHeader := req.Request.Header.Get("Authorization")
		if authHeader == "" {
			return Unauthorized("Authorization required")
		}

		// Check for Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return Unauthorized("Invalid authorization format")
		}

		token := authHeader[7:]

		// Validate token
		result, err := starlark.Call(thread, ba.validateFunc, starlark.Tuple{starlark.String(token)}, nil)
		if err != nil {
			return Unauthorized("Token validation failed")
		}

		// Check if token is valid (not None)
		if result == starlark.None {
			return Unauthorized("Invalid token")
		}

		// Add token info to request context
		if tokenData, err := dataconv.Unmarshal(result); err == nil {
			req.SetContext("token_info", tokenData)
		}

		return next(req)
	}

	return authMiddlewareFactory(middleware)
}

// APIKeyAuth represents an API key authentication handler
type APIKeyAuth struct {
	keys   []string
	header string
}

// NewAPIKeyAuth creates a new API key auth handler
func NewAPIKeyAuth(keys []string, header string) *APIKeyAuth {
	return &APIKeyAuth{
		keys:   keys,
		header: header,
	}
}

// Struct returns a Starlark struct representation of the APIKeyAuth
func (aka *APIKeyAuth) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"middleware":   starlark.NewBuiltin("middleware", aka.Middleware),
		"validate":     starlark.NewBuiltin("validate", aka.ValidateKey),
		"authenticate": starlark.NewBuiltin("authenticate", aka.Authenticate),
	}
	return starlarkstruct.FromStringDict(starlark.String("APIKeyAuth"), sd)
}

// Authenticate validates API key authentication for a request and returns result
func (aka *APIKeyAuth) Authenticate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var request starlark.Value

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"request", &request,
	); err != nil {
		return none, err
	}

	// Extract API key header using request's get_header method
	var apiKey string
	if requestStruct, ok := request.(*starlarkstruct.Struct); ok {
		if getHeaderFunc, err := requestStruct.Attr("get_header"); err == nil {
			if callable, ok := getHeaderFunc.(starlark.Callable); ok {
				result, err := starlark.Call(thread, callable, starlark.Tuple{starlark.String(aka.header)}, nil)
				if err == nil && result != starlark.None {
					apiKey = result.String()
					// Remove quotes if present
					if strings.HasPrefix(apiKey, "\"") && strings.HasSuffix(apiKey, "\"") {
						apiKey = strings.Trim(apiKey, "\"")
					}
				}
			}
		}
	}

	if apiKey == "" {
		result := starlark.NewDict(1)
		result.SetKey(starlark.String("authenticated"), starlark.False)
		return result, nil
	}

	// Validate API key
	valid := false
	for _, key := range aka.keys {
		if key == apiKey {
			valid = true
			break
		}
	}

	result := starlark.NewDict(1)
	result.SetKey(starlark.String("authenticated"), starlark.Bool(valid))
	return result, nil
}

// ValidateKey validates an API key
func (aka *APIKeyAuth) ValidateKey(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var apiKey starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"api_key", &apiKey,
	); err != nil {
		return none, err
	}

	// Check if API key is valid
	keyStr := apiKey.GoString()
	for _, key := range aka.keys {
		if key == keyStr {
			return starlark.True, nil
		}
	}

	return starlark.False, nil
}

// Middleware returns a middleware function that enforces API key authentication
func (aka *APIKeyAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Get API key from header
		apiKey := req.Request.Header.Get(aka.header)
		if apiKey == "" {
			return Unauthorized("API key required")
		}

		// Validate API key
		valid := false
		for _, key := range aka.keys {
			if key == apiKey {
				valid = true
				break
			}
		}

		if !valid {
			return Unauthorized("Invalid API key")
		}

		// Add API key to request context
		req.SetContext("api_key", apiKey)

		return next(req)
	}

	return authMiddlewareFactory(middleware)
}

// bearer_auth creates a bearer token authentication handler
func bearerAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var validateFunc starlark.Callable

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"validate_func", &validateFunc,
	); err != nil {
		return none, err
	}

	bearerAuth := &BearerAuth{
		validateFunc: validateFunc,
	}

	return bearerAuth.Struct(), nil
}

// api_key_auth creates an API key authentication handler
func apiKeyAuth(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		keys   *starlark.List
		header = starlark.String("X-API-Key")
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"keys?", &keys,
		"header?", &header,
	); err != nil {
		return none, err
	}

	// Convert keys list to Go slice using helper
	var keysSlice []string
	if keys != nil {
		keysSlice = starlarkListToStringSlice(keys)
	}

	apiKeyAuth := &APIKeyAuth{
		keys:   keysSlice,
		header: header.GoString(),
	}

	return apiKeyAuth.Struct(), nil
}
