package forwarder

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestApplyModelRedirect_NoRedirects(t *testing.T) {
	p := &model.Provider{}
	result := ApplyModelRedirect(p, "claude-3-opus")

	assert.Equal(t, "claude-3-opus", result.OriginalModel)
	assert.Equal(t, "claude-3-opus", result.RedirectedModel)
	assert.False(t, result.WasRedirected)
}

func TestApplyModelRedirect_EmptyModel(t *testing.T) {
	p := &model.Provider{
		ModelRedirects: model.ProviderModelRedirectRules{
			{MatchType: "exact", Source: "old", Target: "new"},
		},
	}
	result := ApplyModelRedirect(p, "")

	assert.Equal(t, "", result.OriginalModel)
	assert.Equal(t, "", result.RedirectedModel)
	assert.False(t, result.WasRedirected)
}

func TestApplyModelRedirect_ExactMatch(t *testing.T) {
	p := &model.Provider{
		ModelRedirects: model.ProviderModelRedirectRules{
			{MatchType: "exact", Source: "claude-3-opus", Target: "claude-3.5-sonnet"},
		},
	}
	result := ApplyModelRedirect(p, "claude-3-opus")

	assert.Equal(t, "claude-3-opus", result.OriginalModel)
	assert.Equal(t, "claude-3.5-sonnet", result.RedirectedModel)
	assert.True(t, result.WasRedirected)
}

func TestApplyModelRedirect_NoMatch(t *testing.T) {
	p := &model.Provider{
		ModelRedirects: model.ProviderModelRedirectRules{
			{MatchType: "exact", Source: "claude-3-opus", Target: "claude-3.5-sonnet"},
		},
	}
	result := ApplyModelRedirect(p, "gpt-4")

	assert.Equal(t, "gpt-4", result.OriginalModel)
	assert.Equal(t, "gpt-4", result.RedirectedModel)
	assert.False(t, result.WasRedirected)
}

func TestRedirectModelInBody(t *testing.T) {
	body := []byte(`{"model":"claude-3-opus","messages":[]}`)
	result := RedirectModelInBody(body, "claude-3-opus", "claude-3.5-sonnet")
	assert.Equal(t, `{"model":"claude-3.5-sonnet","messages":[]}`, string(result))
}

func TestRedirectModelInBody_NoChange(t *testing.T) {
	body := []byte(`{"model":"claude-3-opus"}`)

	// Same model
	result := RedirectModelInBody(body, "claude-3-opus", "claude-3-opus")
	assert.Equal(t, string(body), string(result))

	// Empty original
	result2 := RedirectModelInBody(body, "", "claude-3.5-sonnet")
	assert.Equal(t, string(body), string(result2))

	// Empty redirected
	result3 := RedirectModelInBody(body, "claude-3-opus", "")
	assert.Equal(t, string(body), string(result3))
}

func TestRedirectModelInBody_NotFound(t *testing.T) {
	body := []byte(`{"model":"gpt-4"}`)
	result := RedirectModelInBody(body, "claude-3-opus", "claude-3.5-sonnet")
	assert.Equal(t, string(body), string(result))
}

func TestRedirectModelInBody_OnlyFirstOccurrence(t *testing.T) {
	body := []byte(`{"model":"old","nested":{"model":"old"}}`)
	result := RedirectModelInBody(body, "old", "new")
	// Only the first occurrence should be replaced
	assert.Equal(t, `{"model":"new","nested":{"model":"old"}}`, string(result))
}

func TestIndexOf(t *testing.T) {
	assert.Equal(t, 0, indexOf([]byte("hello"), []byte("hello")))
	assert.Equal(t, 5, indexOf([]byte("hello world"), []byte(" world")))
	assert.Equal(t, -1, indexOf([]byte("hello"), []byte("xyz")))
	assert.Equal(t, 0, indexOf([]byte("hello"), []byte("")))
	assert.Equal(t, -1, indexOf([]byte("hi"), []byte("hello")))
}
