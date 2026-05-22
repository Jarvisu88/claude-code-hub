package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockSessionManager implements SessionManager for testing.
type mockSessionManager struct {
	sessionID       string
	requestSequence int
	err             error
}

func (m *mockSessionManager) GetOrCreateSessionID(ctx context.Context, rawSessionID string) (string, int, error) {
	if m.err != nil {
		return "", 0, m.err
	}
	return m.sessionID, m.requestSequence, nil
}

func TestSessionGuard_Name(t *testing.T) {
	guard := NewSessionGuard(&mockSessionManager{})
	assert.Equal(t, "SessionGuard", guard.Name())
}

func TestSessionGuard_Check_Success(t *testing.T) {
	sm := &mockSessionManager{
		sessionID:       "session-123",
		requestSequence: 5,
	}
	guard := NewSessionGuard(sm)

	req := &Request{}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "session-123", req.SessionID)
	assert.Equal(t, 5, req.RequestSequence)
}

func TestSessionGuard_Check_WithHeaderSessionID(t *testing.T) {
	sm := &mockSessionManager{
		sessionID:       "session-from-header",
		requestSequence: 1,
	}
	guard := NewSessionGuard(sm)

	req := &Request{
		Headers: map[string]string{
			"x-session-id": "raw-session-id",
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "session-from-header", req.SessionID)
}

func TestSessionGuard_Check_WithContextSessionID(t *testing.T) {
	sm := &mockSessionManager{
		sessionID:       "session-from-ctx",
		requestSequence: 2,
	}
	guard := NewSessionGuard(sm)

	req := &Request{
		Context: map[string]interface{}{
			"sessionId": "raw-from-context",
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "session-from-ctx", req.SessionID)
}

func TestSessionGuard_Check_FailOpen(t *testing.T) {
	sm := &mockSessionManager{
		err: errors.New("redis connection failed"),
	}
	guard := NewSessionGuard(sm)

	req := &Request{}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open")
	assert.NotEmpty(t, req.SessionID, "Should generate random session ID")
	assert.Equal(t, 1, req.RequestSequence)
}

func TestSessionGuard_Check_NoSessionInfo(t *testing.T) {
	sm := &mockSessionManager{
		sessionID:       "new-session",
		requestSequence: 1,
	}
	guard := NewSessionGuard(sm)

	req := &Request{}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "new-session", req.SessionID)
	assert.Equal(t, 1, req.RequestSequence)
}
