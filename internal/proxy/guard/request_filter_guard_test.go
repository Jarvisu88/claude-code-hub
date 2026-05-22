package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockRequestFilterRepo implements RequestFilterRepo for testing.
type mockRequestFilterRepo struct {
	filters []model.RequestFilter
	err     error
}

func (m *mockRequestFilterRepo) GetActive(ctx context.Context) ([]model.RequestFilter, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.filters, nil
}

func TestRequestFilterGuard_Name(t *testing.T) {
	guard := NewRequestFilterGuard(&mockRequestFilterRepo{})
	assert.Equal(t, "RequestFilterGuard", guard.Name())
}

func TestRequestFilterGuard_Check_RepoError(t *testing.T) {
	repo := &mockRequestFilterRepo{err: errors.New("db error")}
	guard := NewRequestFilterGuard(repo)

	req := &Request{}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open on repo error")
}

func TestRequestFilterGuard_Check_NoFilters(t *testing.T) {
	repo := &mockRequestFilterRepo{filters: nil}
	guard := NewRequestFilterGuard(repo)

	req := &Request{}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestRequestFilterGuard_Check_RemoveHeader(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "remove",
				Target:         "x-custom-header",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{
		Headers: map[string]string{
			"x-custom-header": "some-value",
			"other-header":    "keep-me",
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Empty(t, req.Headers["x-custom-header"])
	assert.Equal(t, "keep-me", req.Headers["other-header"])
}

func TestRequestFilterGuard_Check_SetHeader(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "set",
				Target:         "x-injected",
				Replacement:    "injected-value",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{
		Headers: map[string]string{},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "injected-value", req.Headers["x-injected"])
}

func TestRequestFilterGuard_Check_TextReplace(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "body",
				Action:         "text_replace",
				Target:         "old-text",
				Replacement:    "new-text",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{
		RawBody: []byte(`{"message":"hello old-text world"}`),
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Contains(t, string(req.RawBody), "new-text")
	assert.NotContains(t, string(req.RawBody), "old-text")
}

func TestRequestFilterGuard_Check_JSONPathSet(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "body",
				Action:         "json_path",
				Target:         "temperature",
				Replacement:    0.5,
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{
		RawBody: []byte(`{"model":"claude-opus-4","temperature":1.0}`),
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Contains(t, string(req.RawBody), `"temperature":0.5`)
}

func TestRequestFilterGuard_Check_JSONPathRemove(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "body",
				Action:         "json_path",
				Target:         "temperature",
				Replacement:    nil,
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{
		RawBody: []byte(`{"model":"claude-opus-4","temperature":1.0}`),
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.NotContains(t, string(req.RawBody), "temperature")
}

func TestRequestFilterGuard_Check_SkipDisabledFilter(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "set",
				Target:         "x-should-not-appear",
				Replacement:    "value",
				IsEnabled:      false,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{Headers: map[string]string{}}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Empty(t, req.Headers["x-should-not-appear"])
}

func TestRequestFilterGuard_Check_SkipNonGuardPhase(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "set",
				Target:         "x-should-not-appear",
				Replacement:    "value",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "response",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{Headers: map[string]string{}}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Empty(t, req.Headers["x-should-not-appear"])
}

func TestRequestFilterGuard_Check_SkipNonGlobalBinding(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "set",
				Target:         "x-should-not-appear",
				Replacement:    "value",
				IsEnabled:      true,
				BindingType:    "providers",
				ExecutionPhase: "guard",
				ProviderIds:    []int{1},
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{Headers: map[string]string{}}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Empty(t, req.Headers["x-should-not-appear"])
}

func TestRequestFilterGuard_Check_EmptyBody(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "body",
				Action:         "text_replace",
				Target:         "old",
				Replacement:    "new",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{RawBody: nil}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty body should be handled gracefully")
}

func TestRequestFilterGuard_Check_NilHeaders(t *testing.T) {
	repo := &mockRequestFilterRepo{
		filters: []model.RequestFilter{
			{
				Scope:          "header",
				Action:         "set",
				Target:         "x-new",
				Replacement:    "value",
				IsEnabled:      true,
				BindingType:    "global",
				ExecutionPhase: "guard",
			},
		},
	}
	guard := NewRequestFilterGuard(repo)

	req := &Request{Headers: nil}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "value", req.Headers["x-new"])
}
