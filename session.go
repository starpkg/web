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
	"go.starlark.net/starlark"
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
		result, err := dataconv.Marshal(session)
		if err != nil {
			return starlark.None, fmt.Errorf("failed to marshal session: %v", err)
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
		starlarkValue, err := dataconv.Marshal(value)
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
