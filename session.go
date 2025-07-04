package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlight/convert"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// SessionManager manages user sessions
type SessionManager struct {
	secret     []byte
	cookieName string
	maxAge     int
	sessions   map[string]*SessionData
	mu         sync.RWMutex
}

// SessionData represents session data
type SessionData struct {
	ID        string
	Data      map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
	IsNew     bool
}

// Session represents a user session
type Session struct {
	manager *SessionManager
	data    *SessionData
	request *Request
}

// NewSessionManager creates a new session manager
func NewSessionManager(secret, cookieName string, maxAge int) *SessionManager {
	return &SessionManager{
		secret:     []byte(secret),
		cookieName: cookieName,
		maxAge:     maxAge,
		sessions:   make(map[string]*SessionData),
	}
}

// GetSession retrieves or creates a session for the request
func (sm *SessionManager) GetSession(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var request starlark.Value

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "request", &request); err != nil {
		return starlark.None, err
	}

	// Extract Request from Starlark value
	goValue, err := dataconv.Unmarshal(request)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to unmarshal request: %v", err)
	}

	if req, ok := goValue.(*Request); ok {
		session := sm.getSession(req)
		result, err := convert.ToValue(session)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to convert session: %v", err)
		}
		return result, nil
	}

	return starlark.None, fmt.Errorf("invalid request object")
}

// getSession is the internal method to get a session
func (sm *SessionManager) getSession(req *Request) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Try to get session ID from cookie
	sessionID := sm.getSessionIDFromCookie(req)

	var sessionData *SessionData
	isNew := false

	if sessionID != "" {
		// Check if session exists and is valid
		if data, exists := sm.sessions[sessionID]; exists && data.ExpiresAt.After(time.Now()) {
			sessionData = data
			sessionData.IsNew = false
		}
	}

	if sessionData == nil {
		// Create new session
		sessionID = sm.generateSessionID()
		sessionData = &SessionData{
			ID:        sessionID,
			Data:      make(map[string]interface{}),
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Duration(sm.maxAge) * time.Second),
			IsNew:     true,
		}
		sm.sessions[sessionID] = sessionData
		isNew = true
	}

	session := &Session{
		manager: sm,
		data:    sessionData,
		request: req,
	}

	// Set cookie if new session
	if isNew {
		sm.setSessionCookie(req, sessionID)
	}

	return session
}

// getSessionIDFromCookie extracts session ID from cookie
func (sm *SessionManager) getSessionIDFromCookie(req *Request) string {
	cookie, err := req.Request.Cookie(sm.cookieName)
	if err != nil {
		return ""
	}

	// Verify HMAC signature
	return sm.verifySessionID(cookie.Value)
}

// generateSessionID creates a new session ID
func (sm *SessionManager) generateSessionID() string {
	// Generate random bytes
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)

	// Create base64 encoded session ID
	sessionID := base64.URLEncoding.EncodeToString(randomBytes)

	// Add HMAC signature
	return sm.signSessionID(sessionID)
}

// signSessionID adds HMAC signature to session ID
func (sm *SessionManager) signSessionID(sessionID string) string {
	h := hmac.New(sha256.New, sm.secret)
	h.Write([]byte(sessionID))
	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return sessionID + "." + signature
}

// verifySessionID verifies HMAC signature and returns session ID
func (sm *SessionManager) verifySessionID(signedSessionID string) string {
	parts := strings.Split(signedSessionID, ".")
	if len(parts) != 2 {
		return ""
	}

	sessionID := parts[0]
	signature := parts[1]

	// Verify signature
	h := hmac.New(sha256.New, sm.secret)
	h.Write([]byte(sessionID))
	expectedSignature := base64.URLEncoding.EncodeToString(h.Sum(nil))

	if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return sessionID
	}

	return ""
}

// setSessionCookie sets the session cookie
func (sm *SessionManager) setSessionCookie(req *Request, sessionID string) {
	cookie := &http.Cookie{
		Name:     sm.cookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   sm.maxAge,
		HttpOnly: true,
		Secure:   req.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	}

	// Add to context for response
	req.SetContext("session_cookie", cookie)
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if session.ExpiresAt.Before(now) {
			delete(sm.sessions, id)
		}
	}
}

// Session methods

// ID returns the session ID
func (s *Session) ID(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(s.data.ID), nil
}

// IsNew returns whether this is a new session
func (s *Session) IsNew(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Bool(s.data.IsNew), nil
}

// Get retrieves a value from the session
func (s *Session) Get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key          starlark.String
		defaultValue = starlark.None
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"key", &key,
		"default?", &defaultValue,
	); err != nil {
		return starlark.None, err
	}

	s.manager.mu.RLock()
	defer s.manager.mu.RUnlock()

	if value, exists := s.data.Data[key.GoString()]; exists {
		starlarkValue, err := convert.ToValue(value)
		if err != nil {
			return defaultValue, nil
		}
		return starlarkValue, nil
	}

	return defaultValue, nil
}

// Set stores a value in the session
func (s *Session) Set(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key   starlark.String
		value starlark.Value
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"key", &key,
		"value", &value,
	); err != nil {
		return starlark.None, err
	}

	// Convert Starlark value to Go value
	goValue, err := dataconv.Unmarshal(value)
	if err != nil {
		return starlark.None, err
	}

	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	s.data.Data[key.GoString()] = goValue
	return starlark.None, nil
}

// Delete removes a value from the session
func (s *Session) Delete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key starlark.String

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key); err != nil {
		return starlark.None, err
	}

	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	delete(s.data.Data, key.GoString())
	return starlark.None, nil
}

// Clear removes all values from the session
func (s *Session) Clear(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	s.data.Data = make(map[string]interface{})
	return starlark.None, nil
}

// Save explicitly saves the session (usually automatic)
func (s *Session) Save(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Update expiration time
	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	s.data.ExpiresAt = time.Now().Add(time.Duration(s.manager.maxAge) * time.Second)
	return starlark.None, nil
}

// Configure updates session manager configuration
func (sm *SessionManager) Configure(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		cookieName = starlark.String(sm.cookieName)
		maxAge     = starlark.MakeInt(sm.maxAge)
		secure     = starlark.Bool(false)
		httpOnly   = starlark.Bool(true)
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"cookie_name?", &cookieName,
		"max_age?", &maxAge,
		"secure?", &secure,
		"http_only?", &httpOnly,
	); err != nil {
		return starlark.None, err
	}

	maxAgeInt, _ := maxAge.Int64()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.cookieName = cookieName.GoString()
	sm.maxAge = int(maxAgeInt)

	return starlark.None, nil
}

// Add new methods for session cookie integration

// applySessionCookie applies the session cookie to the response if needed
func (s *Session) applySessionCookie(resp *Response) {
	// Get the session cookie from the request context
	if s.request != nil && s.data.IsNew {
		// Create a cookie for the session
		cookie := &http.Cookie{
			Name:     s.manager.cookieName,
			Value:    s.data.ID,
			Path:     "/",
			MaxAge:   s.manager.maxAge,
			HttpOnly: true,
			Secure:   s.request.Request.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		}

		// Make sure the response has headers
		if resp.Headers == nil {
			resp.Headers = make(http.Header)
		}

		// Add the cookie to the response
		resp.Headers["Set-Cookie"] = append(resp.Headers["Set-Cookie"], cookie.String())
	}
}

// Add a middleware creator method to the SessionManager

// Middleware creates a middleware function for session handling
func (sm *SessionManager) Middleware(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sessionMiddlewareFunc := func(req *Request, next NextFunc) *Response {
		// Get session for this request
		session := sm.getSession(req)

		// Make session available in request context
		req.SetContext("session", session)

		// Process request
		response := next(req)

		// Save session and apply cookie if needed
		session.applySessionCookie(response)

		return response
	}

	// Return a Starlark builtin that implements the middleware
	return starlark.NewBuiltin("session_middleware", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var req, nextHandler starlark.Value

		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"request", &req,
			"next_handler", &nextHandler,
		); err != nil {
			return starlark.None, err
		}

		// Convert request to Go type
		goReq, err := dataconv.Unmarshal(req)
		if err != nil {
			return starlark.None, fmt.Errorf("invalid request object: %v", err)
		}

		request, ok := goReq.(*Request)
		if !ok {
			return starlark.None, fmt.Errorf("expected Request, got %T", goReq)
		}

		// Create next_handler wrapper
		nextFunc := func(r *Request) *Response {
			// Call the Starlark next_handler
			result, err := starlark.Call(thread, nextHandler.(starlark.Callable), starlark.Tuple{req}, nil)
			if err != nil {
				return &Response{
					StatusCode: 500,
					Headers:    make(http.Header),
					Body:       fmt.Sprintf("Next handler error: %v", err),
				}
			}

			// Convert result back to Response
			respObj, err := ResponseFromStarlarkStruct(result)
			if err != nil {
				// Try normal unmarshaling as fallback
				goValue, err := dataconv.Unmarshal(result)
				if err != nil {
					return &Response{
						StatusCode: 500,
						Headers:    make(map[string][]string),
						Body:       fmt.Sprintf("Failed to unmarshal response: %v", err),
					}
				}

				if resp, ok := goValue.(*Response); ok {
					respObj = resp
				} else {
					return &Response{
						StatusCode: 500,
						Headers:    make(map[string][]string),
						Body:       "Handler did not return a Response object",
					}
				}
			}

			return respObj
		}

		// Call the actual middleware function
		response := sessionMiddlewareFunc(request, nextFunc)

		// Convert response back to Starlark
		result := response.Struct()

		return result, nil
	}), nil
}

// Update the SessionManager's Struct method to include the middleware method

// Struct returns a Starlark struct representation of the SessionManager
func (sm *SessionManager) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"get_session": starlark.NewBuiltin("get_session", sm.GetSession),
		"configure":   starlark.NewBuiltin("configure", sm.Configure),
		"middleware":  starlark.NewBuiltin("middleware", sm.Middleware),
	}
	return starlarkstruct.FromStringDict(starlark.String("SessionManager"), sd)
}

// Add a goroutine to periodically clean up expired sessions

// StartCleanupTask starts a background task to clean up expired sessions
func (sm *SessionManager) StartCleanupTask() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // Run cleanup every 10 minutes
		defer ticker.Stop()

		for range ticker.C {
			sm.cleanupExpiredSessions()
		}
	}()
}

// Struct returns a Starlark struct representation of the Session
func (s *Session) Struct() *starlarkstruct.Struct {
	sd := starlark.StringDict{
		"id":     starlark.NewBuiltin("id", s.ID),
		"is_new": starlark.NewBuiltin("is_new", s.IsNew),
		"get":    starlark.NewBuiltin("get", s.Get),
		"set":    starlark.NewBuiltin("set", s.Set),
		"delete": starlark.NewBuiltin("delete", s.Delete),
		"clear":  starlark.NewBuiltin("clear", s.Clear),
		"save":   starlark.NewBuiltin("save", s.Save),
	}
	return starlarkstruct.FromStringDict(starlark.String("Session"), sd)
}
