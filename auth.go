package web

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Authentication implementations

// BasicAuth handles HTTP Basic authentication
type BasicAuth struct {
	Users map[string]string
	Realm string
}

// NewBasicAuth creates a new basic authentication handler
func NewBasicAuth(users map[string]string, realm string) *BasicAuth {
	return &BasicAuth{
		Users: users,
		Realm: realm,
	}
}

// Validate checks if the provided credentials are valid
func (ba *BasicAuth) Validate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		username starlark.String
		password starlark.String
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"username", &username,
		"password", &password,
	); err != nil {
		return starlark.Bool(false), err
	}

	if expectedPassword, exists := ba.Users[username.GoString()]; exists {
		return starlark.Bool(expectedPassword == password.GoString()), nil
	}

	return starlark.Bool(false), nil
}

// Middleware returns a middleware function for basic authentication
func (ba *BasicAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Check for Authorization header
		authHeader := req.Request.Header.Get("Authorization")
		if authHeader == "" {
			return ba.unauthorizedResponse()
		}

		// Parse Basic auth
		if !strings.HasPrefix(authHeader, "Basic ") {
			return ba.unauthorizedResponse()
		}

		encoded := authHeader[6:] // Remove "Basic " prefix
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return ba.unauthorizedResponse()
		}

		credentials := strings.SplitN(string(decoded), ":", 2)
		if len(credentials) != 2 {
			return ba.unauthorizedResponse()
		}

		username := credentials[0]
		password := credentials[1]

		// Validate credentials
		if expectedPassword, exists := ba.Users[username]; exists && expectedPassword == password {
			// Add username to request context
			req.SetContext("username", username)
			return next(req)
		}

		return ba.unauthorizedResponse()
	}

	return dataconv.WrapGoValue(middleware), nil
}

// unauthorizedResponse creates a 401 Unauthorized response
func (ba *BasicAuth) unauthorizedResponse() *Response {
	response := &Response{
		StatusCode: 401,
		Headers:    make(http.Header),
		Body:       "Unauthorized",
	}
	response.Headers.Set("WWW-Authenticate", "Basic realm=\""+ba.Realm+"\"")
	return response
}

// BearerAuth handles Bearer token authentication
type BearerAuth struct {
	ValidateFunc starlark.Callable
}

// NewBearerAuth creates a new bearer authentication handler
func NewBearerAuth(validateFunc starlark.Callable) *BearerAuth {
	return &BearerAuth{
		ValidateFunc: validateFunc,
	}
}

// Validate checks if the provided token is valid
func (ba *BearerAuth) Validate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var token starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "token", &token); err != nil {
		return starlark.None, err
	}

	// Call the validation function
	result, err := starlark.Call(thread, ba.ValidateFunc, starlark.Tuple{token}, nil)
	if err != nil {
		return starlark.None, err
	}

	return result, nil
}

// Middleware returns a middleware function for bearer authentication
func (ba *BearerAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Check for Authorization header
		authHeader := req.Request.Header.Get("Authorization")
		if authHeader == "" {
			return &Response{
				StatusCode: 401,
				Headers:    make(http.Header),
				Body:       "Authorization header required",
			}
		}

		// Parse Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return &Response{
				StatusCode: 401,
				Headers:    make(http.Header),
				Body:       "Invalid authorization format",
			}
		}

		token := authHeader[7:] // Remove "Bearer " prefix

		// Validate token
		result, err := starlark.Call(&starlark.Thread{}, ba.ValidateFunc, starlark.Tuple{starlark.String(token)}, nil)
		if err != nil {
			return &Response{
				StatusCode: 401,
				Headers:    make(http.Header),
				Body:       "Token validation error",
			}
		}

		// Check if validation returned user info
		if result == starlark.None {
			return &Response{
				StatusCode: 401,
				Headers:    make(http.Header),
				Body:       "Invalid token",
			}
		}

		// Add token info to request context
		if tokenInfo, err := dataconv.Unmarshal(result); err == nil {
			req.SetContext("token_info", tokenInfo)
		}

		return next(req)
	}

	return dataconv.WrapGoValue(middleware), nil
}

// APIKeyAuth handles API key authentication
type APIKeyAuth struct {
	Keys   []string
	Header string
}

// NewAPIKeyAuth creates a new API key authentication handler
func NewAPIKeyAuth(keys []string, header string) *APIKeyAuth {
	return &APIKeyAuth{
		Keys:   keys,
		Header: header,
	}
}

// Validate checks if the provided API key is valid
func (aka *APIKeyAuth) Validate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key); err != nil {
		return starlark.Bool(false), err
	}

	keyStr := key.GoString()
	for _, validKey := range aka.Keys {
		if validKey == keyStr {
			return starlark.Bool(true), nil
		}
	}

	return starlark.Bool(false), nil
}

// Middleware returns a middleware function for API key authentication
func (aka *APIKeyAuth) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	middleware := func(req *Request, next NextFunc) *Response {
		// Check for API key in header
		apiKey := req.Request.Header.Get(aka.Header)
		if apiKey == "" {
			// Check in query parameters as fallback
			apiKey = req.Request.URL.Query().Get("api_key")
		}

		if apiKey == "" {
			return &Response{
				StatusCode: 401,
				Headers:    make(http.Header),
				Body:       "API key required",
			}
		}

		// Validate API key
		for _, validKey := range aka.Keys {
			if validKey == apiKey {
				// Add API key to request context
				req.SetContext("api_key", apiKey)
				return next(req)
			}
		}

		return &Response{
			StatusCode: 401,
			Headers:    make(http.Header),
			Body:       "Invalid API key",
		}
	}

	return dataconv.WrapGoValue(middleware), nil
}
