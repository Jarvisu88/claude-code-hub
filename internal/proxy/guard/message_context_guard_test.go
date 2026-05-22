package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockMessageRequestRepo implements MessageRequestRepo for testing.
type mockMessageRequestRepo struct {
	createdRecord *model.MessageRequest
	nextID        int
	err           error
}

func (m *mockMessageRequestRepo) Create(ctx context.Context, record *model.MessageRequest) error {
	if m.err != nil {
		return m.err
	}
	m.createdRecord = record
	record.ID = m.nextID
	return nil
}

func TestMessageContextGuard_Name(t *testing.T) {
	guard := NewMessageContextGuard(&mockMessageRequestRepo{})
	assert.Equal(t, "MessageContextGuard", guard.Name())
}

func TestMessageContextGuard_Check_Success(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 42}
	guard := NewMessageContextGuard(repo)

	enabled := true
	req := &Request{
		Provider:        &model.Provider{ID: 10},
		User:            &model.User{ID: 5, IsEnabled: &enabled},
		APIKey:          &model.Key{Name: "test-key"},
		Model:           "claude-opus-4",
		SessionID:       "session-123",
		RequestSequence: 3,
		UserAgent:       "Claude-Code/1.5",
		ClientIP:        "192.168.1.1",
		Endpoint:        "/v1/messages",
		Messages: []RequestMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 42, req.MessageRequestID)

	// Verify the created record
	assert.NotNil(t, repo.createdRecord)
	assert.Equal(t, 10, repo.createdRecord.ProviderID)
	assert.Equal(t, 5, repo.createdRecord.UserID)
	assert.Equal(t, "test-key", repo.createdRecord.Key)
	assert.Equal(t, "claude-opus-4", repo.createdRecord.Model)
	assert.NotNil(t, repo.createdRecord.SessionID)
	assert.Equal(t, "session-123", *repo.createdRecord.SessionID)
	assert.Equal(t, 3, repo.createdRecord.RequestSequence)
	assert.NotNil(t, repo.createdRecord.UserAgent)
	assert.Equal(t, "Claude-Code/1.5", *repo.createdRecord.UserAgent)
	assert.NotNil(t, repo.createdRecord.ClientIP)
	assert.Equal(t, "192.168.1.1", *repo.createdRecord.ClientIP)
	assert.NotNil(t, repo.createdRecord.MessagesCount)
	assert.Equal(t, 2, *repo.createdRecord.MessagesCount)
	assert.NotNil(t, repo.createdRecord.Endpoint)
	assert.Equal(t, "/v1/messages", *repo.createdRecord.Endpoint)
}

func TestMessageContextGuard_Check_FailOpen(t *testing.T) {
	repo := &mockMessageRequestRepo{err: errors.New("db error")}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		User:  &model.User{ID: 1},
		Model: "claude-opus-4",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open on repo error")
	assert.Equal(t, 0, req.MessageRequestID, "ID should remain 0 on error")
}

func TestMessageContextGuard_Check_NilProvider(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 1}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		Provider: nil,
		User:     &model.User{ID: 1},
		Model:    "claude-opus-4",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 0, repo.createdRecord.ProviderID)
}

func TestMessageContextGuard_Check_NilUser(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 1}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		User:  nil,
		Model: "claude-opus-4",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 0, repo.createdRecord.UserID)
}

func TestMessageContextGuard_Check_NilAPIKey(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 1}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		User:   &model.User{ID: 1},
		APIKey: nil,
		Model:  "claude-opus-4",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "", repo.createdRecord.Key)
}

func TestMessageContextGuard_Check_EmptyOptionalFields(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 1}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		User:  &model.User{ID: 1},
		Model: "claude-opus-4",
		// SessionID, UserAgent, ClientIP, Endpoint all empty
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Nil(t, repo.createdRecord.SessionID)
	assert.Nil(t, repo.createdRecord.UserAgent)
	assert.Nil(t, repo.createdRecord.ClientIP)
	assert.Nil(t, repo.createdRecord.Endpoint)
}

func TestMessageContextGuard_Check_NoMessages(t *testing.T) {
	repo := &mockMessageRequestRepo{nextID: 1}
	guard := NewMessageContextGuard(repo)

	req := &Request{
		User:     &model.User{ID: 1},
		Model:    "claude-opus-4",
		Messages: nil,
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, repo.createdRecord.MessagesCount)
	assert.Equal(t, 0, *repo.createdRecord.MessagesCount)
}
