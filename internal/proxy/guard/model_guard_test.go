package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestModelGuard_Name(t *testing.T) {
	guard := NewModelGuard()
	assert.Equal(t, "ModelGuard", guard.Name())
}

func TestModelGuard_Check_NilUser(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{User: nil, Model: "claude-opus-4"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestModelGuard_Check_EmptyModel(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{"claude-opus-4"}},
		Model: "",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty model should pass through")
}

func TestModelGuard_Check_EmptyAllowedModels(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{}},
		Model: "any-model",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty allowed models should pass all")
}

func TestModelGuard_Check_NilAllowedModels(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: nil},
		Model: "any-model",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Nil allowed models should pass all")
}

func TestModelGuard_Check_Allowed(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{"claude-opus-4", "claude-sonnet-4"}},
		Model: "claude-opus-4",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestModelGuard_Check_NotAllowed(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{"claude-opus-4", "claude-sonnet-4"}},
		Model: "gpt-4",
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeModelNotAllowed))
	assert.Contains(t, err.Error(), "Model not allowed: gpt-4")
}

func TestModelGuard_Check_CaseInsensitive(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{"Claude-Opus-4"}},
		Model: "claude-opus-4",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should match case-insensitively")
}

func TestModelGuard_Check_CaseInsensitiveReverse(t *testing.T) {
	guard := NewModelGuard()
	req := &Request{
		User:  &model.User{AllowedModels: []string{"claude-opus-4"}},
		Model: "Claude-Opus-4",
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should match case-insensitively regardless of direction")
}
