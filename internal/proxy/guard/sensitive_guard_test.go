package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockSensitiveWordRepo implements SensitiveWordRepo for testing.
type mockSensitiveWordRepo struct {
	words []model.SensitiveWord
	err   error
}

func (m *mockSensitiveWordRepo) GetAll(ctx context.Context) ([]model.SensitiveWord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.words, nil
}

func TestSensitiveWordGuard_Name(t *testing.T) {
	guard := NewSensitiveWordGuard(&mockSensitiveWordRepo{})
	assert.Equal(t, "SensitiveWordGuard", guard.Name())
}

func TestSensitiveWordGuard_Check_NoMessages(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "badword", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{Messages: nil}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestSensitiveWordGuard_Check_ContainsMatch(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "secret", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "This is a SECRET message"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Request contains sensitive word: secret")
}

func TestSensitiveWordGuard_Check_ContainsCaseInsensitive(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "BadWord", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "this has badword in it"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
}

func TestSensitiveWordGuard_Check_ExactMatch(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "forbidden", MatchType: "exact", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	// Exact match should trigger
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "Forbidden"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)

	// Substring should NOT trigger for exact match
	req2 := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "this is forbidden content"},
		},
	}
	err2 := guard.Check(context.Background(), req2)
	assert.NoError(t, err2)
}

func TestSensitiveWordGuard_Check_RegexMatch(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: `\d{3}-\d{3}-\d{4}`, MatchType: "regex", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "Call me at 123-456-7890"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)

	// No match
	req2 := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "No phone numbers here"},
		},
	}
	err2 := guard.Check(context.Background(), req2)
	assert.NoError(t, err2)
}

func TestSensitiveWordGuard_Check_InvalidRegex(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: `[invalid`, MatchType: "regex", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "some text"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Invalid regex should not match")
}

func TestSensitiveWordGuard_Check_DisabledWord(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "secret", MatchType: "contains", IsEnabled: false},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "This is a secret message"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Disabled word should not trigger")
}

func TestSensitiveWordGuard_Check_FailOpen_RepoError(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		err: errors.New("database error"),
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "any message"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open on repo error")
}

func TestSensitiveWordGuard_Check_ContentBlocks(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "password", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "My password is 12345"},
				},
			},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Request contains sensitive word: password")
}

func TestSensitiveWordGuard_Check_EmptyContent(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "test", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: ""},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestSensitiveWordGuard_Check_MultipleMessages(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "trigger", MatchType: "contains", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "safe message"},
			{Role: "assistant", Content: "safe reply"},
			{Role: "user", Content: "this has the trigger word"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
}

func TestSensitiveWordGuard_Check_NoMatch(t *testing.T) {
	repo := &mockSensitiveWordRepo{
		words: []model.SensitiveWord{
			{ID: 1, Word: "forbidden", MatchType: "contains", IsEnabled: true},
			{ID: 2, Word: "secret", MatchType: "exact", IsEnabled: true},
		},
	}
	guard := NewSensitiveWordGuard(repo)

	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}
