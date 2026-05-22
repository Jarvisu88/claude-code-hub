package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestWarmupGuard_Name(t *testing.T) {
	guard := NewWarmupGuard(&mockSystemSettingsRepo{})
	assert.Equal(t, "WarmupGuard", guard.Name())
}

func TestWarmupGuard_Check_NonMessagesEndpoint(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/chat/completions",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Non /v1/messages endpoint should pass")
}

func TestWarmupGuard_Check_MultipleMessages(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{Role: "user", Content: "first"},
			{Role: "user", Content: "second"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Multiple messages should not be warmup")
}

func TestWarmupGuard_Check_WarmupIntercepted(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.ErrorIs(t, err, ErrWarmupIntercepted)
}

func TestWarmupGuard_Check_InterceptionDisabled(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: false},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should pass when interception is disabled")
}

func TestWarmupGuard_Check_SettingsError(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		err: errors.New("db error"),
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should not intercept on settings error")
}

func TestWarmupGuard_Check_NotWarmupContent(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello world"},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Non-warmup content should pass")
}

func TestWarmupGuard_Check_WarmupWithoutCacheControl(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup"},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Warmup without cache_control should not be intercepted")
}

func TestWarmupGuard_Check_NonUserRole(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Non-user role should not be detected as warmup")
}

func TestWarmupGuard_Check_StringContent(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{Role: "user", Content: "Warmup"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "String content without cache_control should not be warmup")
}

func TestWarmupGuard_Check_MultipleBlocks(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{InterceptAnthropicWarmupRequests: true},
	}
	guard := NewWarmupGuard(repo)

	req := &Request{
		Endpoint: "/v1/messages",
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Warmup", CacheControl: map[string]string{"type": "ephemeral"}},
					{Type: "text", Text: "Extra"},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Multiple content blocks should not be warmup")
}
