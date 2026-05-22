package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/model"
)

// MessageRequestRepo provides access to message request records.
type MessageRequestRepo interface {
	Create(ctx context.Context, record *model.MessageRequest) error
}

// MessageContextGuard creates a message_request DB record for the request.
type MessageContextGuard struct {
	repo MessageRequestRepo
}

// NewMessageContextGuard creates a new MessageContextGuard.
func NewMessageContextGuard(repo MessageRequestRepo) *MessageContextGuard {
	return &MessageContextGuard{repo: repo}
}

// Name returns the guard name.
func (g *MessageContextGuard) Name() string {
	return "MessageContextGuard"
}

// Check creates a message_request record and sets the ID on the request.
// Fail-open: on error, logs and continues.
func (g *MessageContextGuard) Check(ctx context.Context, req *Request) error {
	providerID := 0
	if req.Provider != nil {
		providerID = req.Provider.ID
	}

	userID := 0
	if req.User != nil {
		userID = req.User.ID
	}

	keyStr := ""
	if req.APIKey != nil {
		keyStr = req.APIKey.Name
	}

	messagesCount := len(req.Messages)

	var sessionID *string
	if req.SessionID != "" {
		sid := req.SessionID
		sessionID = &sid
	}

	var userAgent *string
	if req.UserAgent != "" {
		ua := req.UserAgent
		userAgent = &ua
	}

	var clientIP *string
	if req.ClientIP != "" {
		ip := req.ClientIP
		clientIP = &ip
	}

	var endpoint *string
	if req.Endpoint != "" {
		ep := req.Endpoint
		endpoint = &ep
	}

	record := &model.MessageRequest{
		ProviderID:      providerID,
		UserID:          userID,
		Key:             keyStr,
		Model:           req.Model,
		SessionID:       sessionID,
		RequestSequence: req.RequestSequence,
		UserAgent:       userAgent,
		ClientIP:        clientIP,
		MessagesCount:   &messagesCount,
		Endpoint:        endpoint,
	}

	err := g.repo.Create(ctx, record)
	if err != nil {
		// Fail-open: log and continue
		return nil
	}

	req.MessageRequestID = record.ID
	return nil
}
