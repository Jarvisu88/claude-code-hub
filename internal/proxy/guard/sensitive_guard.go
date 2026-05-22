package guard

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// SensitiveWordRepo provides access to sensitive word rules.
type SensitiveWordRepo interface {
	GetAll(ctx context.Context) ([]model.SensitiveWord, error)
}

// SensitiveWordGuard checks request messages for sensitive words.
type SensitiveWordGuard struct {
	repo SensitiveWordRepo
}

// NewSensitiveWordGuard creates a new SensitiveWordGuard.
func NewSensitiveWordGuard(repo SensitiveWordRepo) *SensitiveWordGuard {
	return &SensitiveWordGuard{repo: repo}
}

// Name returns the guard name.
func (g *SensitiveWordGuard) Name() string {
	return "SensitiveWordGuard"
}

// Check scans request messages against sensitive word rules.
// Fail-open: returns nil on detection errors.
func (g *SensitiveWordGuard) Check(ctx context.Context, req *Request) error {
	if len(req.Messages) == 0 {
		return nil
	}

	words, err := g.repo.GetAll(ctx)
	if err != nil {
		// Fail-open: on detection error, pass through
		return nil
	}

	// Collect all text content from messages
	texts := extractTexts(req.Messages)
	if len(texts) == 0 {
		return nil
	}

	for _, sw := range words {
		if !sw.IsActive() {
			continue
		}
		for _, text := range texts {
			if matchesSensitiveWord(text, sw.Word, sw.MatchType) {
				return appErrors.NewInvalidRequest(
					fmt.Sprintf("Request contains sensitive word: %s", sw.Word),
				)
			}
		}
	}

	return nil
}

// extractTexts collects all text content from request messages.
func extractTexts(messages []RequestMessage) []string {
	var texts []string
	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			if content != "" {
				texts = append(texts, content)
			}
		case []ContentBlock:
			for _, block := range content {
				if block.Text != "" {
					texts = append(texts, block.Text)
				}
			}
		}
	}
	return texts
}

// matchesSensitiveWord checks if text matches the given word with the specified match type.
func matchesSensitiveWord(text, word, matchType string) bool {
	switch strings.ToLower(matchType) {
	case "exact":
		return strings.EqualFold(text, word)
	case "regex":
		re, err := regexp.Compile(word)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	case "contains", "":
		return strings.Contains(strings.ToLower(text), strings.ToLower(word))
	default:
		return false
	}
}
