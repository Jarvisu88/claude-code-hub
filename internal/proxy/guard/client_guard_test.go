package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestClientGuard_Name(t *testing.T) {
	guard := NewClientGuard()
	assert.Equal(t, "ClientGuard", guard.Name())
}

func TestClientGuard_Check_NilUser(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{User: nil, UserAgent: "SomeClient/1.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestClientGuard_Check_EmptyUserAgent(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{"web"}},
		UserAgent: "",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty User-Agent should pass through")
}

func TestClientGuard_Check_EmptyAllowlist(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{}},
		UserAgent: "AnyClient/1.0",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty allowlist should allow all clients")
}

func TestClientGuard_Check_NilAllowlist(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: nil},
		UserAgent: "AnyClient/1.0",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Nil allowlist should allow all clients")
}

func TestClientGuard_Check_Allowed(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{"claude-code", "web"}},
		UserAgent: "Claude-Code/1.5.0",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestClientGuard_Check_NotAllowed(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{"claude-code", "web"}},
		UserAgent: "Postman/9.0",
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeClientNotAllowed))
	assert.Contains(t, err.Error(), "Client not allowed")
}

func TestClientGuard_Check_CaseInsensitiveMatch(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{"Claude-Code"}},
		UserAgent: "claude-code/1.0",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should match case-insensitively")
}

func TestClientGuard_Check_SubstringMatch(t *testing.T) {
	guard := NewClientGuard()
	req := &Request{
		User:      &model.User{AllowedClients: []string{"claude"}},
		UserAgent: "Claude-Code/1.5.0 (Linux x86_64)",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should match as substring")
}
