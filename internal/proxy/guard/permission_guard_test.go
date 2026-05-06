package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestPermissionGuard_Name(t *testing.T) {
	guard := NewPermissionGuard()
	assert.Equal(t, "PermissionGuard", guard.Name())
}

func TestPermissionGuard_Check_NoUser(t *testing.T) {
	guard := NewPermissionGuard()
	req := &Request{
		User:     nil,
		Model:    "claude-opus-4",
		ClientID: "web",
	}

	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrorTypeAuthentication))
	assert.True(t, errors.IsCode(err, errors.CodeUnauthorized))
}

func TestPermissionGuard_Check_ModelAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:            1,
		AllowedModels: []string{"claude-opus-4", "claude-sonnet-4"},
	}
	req := &Request{
		User:  user,
		Model: "claude-opus-4",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestPermissionGuard_Check_ModelNotAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:            1,
		AllowedModels: []string{"claude-opus-4", "claude-sonnet-4"},
	}
	req := &Request{
		User:  user,
		Model: "gpt-4",
	}

	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrorTypePermissionDenied))
	assert.True(t, errors.IsCode(err, errors.CodeModelNotAllowed))
	assert.Contains(t, err.Error(), "Model not allowed: gpt-4")
}

func TestPermissionGuard_Check_EmptyModelList(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:            1,
		AllowedModels: []string{},
	}
	req := &Request{
		User:  user,
		Model: "any-model",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty model list should allow all models")
}

func TestPermissionGuard_Check_NilModelList(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:            1,
		AllowedModels: nil,
	}
	req := &Request{
		User:  user,
		Model: "any-model",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Nil model list should allow all models")
}

func TestPermissionGuard_Check_ClientAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedClients: []string{"web", "mobile"},
	}
	req := &Request{
		User:     user,
		ClientID: "web",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestPermissionGuard_Check_ClientNotAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedClients: []string{"web", "mobile"},
	}
	req := &Request{
		User:     user,
		ClientID: "desktop",
	}

	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrorTypePermissionDenied))
	assert.True(t, errors.IsCode(err, errors.CodeClientNotAllowed))
	assert.Contains(t, err.Error(), "Client not allowed: desktop")
}

func TestPermissionGuard_Check_EmptyClientList(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedClients: []string{},
	}
	req := &Request{
		User:     user,
		ClientID: "any-client",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty client list should allow all clients")
}

func TestPermissionGuard_Check_NilClientList(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedClients: nil,
	}
	req := &Request{
		User:     user,
		ClientID: "any-client",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Nil client list should allow all clients")
}

func TestPermissionGuard_Check_BothModelAndClientAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedModels:  []string{"claude-opus-4"},
		AllowedClients: []string{"web"},
	}
	req := &Request{
		User:     user,
		Model:    "claude-opus-4",
		ClientID: "web",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestPermissionGuard_Check_ModelAllowedButClientNotAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedModels:  []string{"claude-opus-4"},
		AllowedClients: []string{"web"},
	}
	req := &Request{
		User:     user,
		Model:    "claude-opus-4",
		ClientID: "desktop",
	}

	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeClientNotAllowed))
}

func TestPermissionGuard_Check_ClientAllowedButModelNotAllowed(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedModels:  []string{"claude-opus-4"},
		AllowedClients: []string{"web"},
	}
	req := &Request{
		User:     user,
		Model:    "gpt-4",
		ClientID: "web",
	}

	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeModelNotAllowed))
}

func TestPermissionGuard_Check_EmptyModelInRequest(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:            1,
		AllowedModels: []string{"claude-opus-4"},
	}
	req := &Request{
		User:  user,
		Model: "",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty model in request should skip model check")
}

func TestPermissionGuard_Check_EmptyClientInRequest(t *testing.T) {
	guard := NewPermissionGuard()
	user := &model.User{
		ID:             1,
		AllowedClients: []string{"web"},
	}
	req := &Request{
		User:     user,
		ClientID: "",
	}

	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty client in request should skip client check")
}
