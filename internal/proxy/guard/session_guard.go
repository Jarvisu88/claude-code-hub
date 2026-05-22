package guard

import (
	"context"

	"github.com/google/uuid"
)

// SessionManager manages session IDs and request sequences.
type SessionManager interface {
	// GetOrCreateSessionID returns the session ID and request sequence
	// for the given raw session identifier.
	// If the session does not exist, it creates a new one.
	GetOrCreateSessionID(ctx context.Context, rawSessionID string) (sessionID string, requestSequence int, err error)
}

// SessionGuard extracts or creates a session ID and sets it on the request.
type SessionGuard struct {
	sessionManager SessionManager
}

// NewSessionGuard creates a new SessionGuard.
func NewSessionGuard(sessionManager SessionManager) *SessionGuard {
	return &SessionGuard{sessionManager: sessionManager}
}

// Name returns the guard name.
func (g *SessionGuard) Name() string {
	return "SessionGuard"
}

// Check extracts/creates a session ID and sets it on the request.
// Fail-open: on error, generates a random session ID.
func (g *SessionGuard) Check(ctx context.Context, req *Request) error {
	// Try to extract raw session ID from headers or context
	rawSessionID := extractRawSessionID(req)

	sessionID, requestSequence, err := g.sessionManager.GetOrCreateSessionID(ctx, rawSessionID)
	if err != nil {
		// Fail-open: generate random session ID
		req.SessionID = uuid.New().String()
		req.RequestSequence = 1
		return nil
	}

	req.SessionID = sessionID
	req.RequestSequence = requestSequence
	return nil
}

// extractRawSessionID extracts the raw session identifier from the request.
func extractRawSessionID(req *Request) string {
	// Check headers for session ID
	if req.Headers != nil {
		if sid := req.Headers["x-session-id"]; sid != "" {
			return sid
		}
		if sid := req.Headers["X-Session-Id"]; sid != "" {
			return sid
		}
	}

	// Check context map
	if req.Context != nil {
		if sid, ok := req.Context["sessionId"].(string); ok && sid != "" {
			return sid
		}
	}

	return ""
}
