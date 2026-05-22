package forwarder

import (
	"github.com/ding113/claude-code-hub/internal/model"
)

// ModelRedirect holds the original and redirected model names.
type ModelRedirect struct {
	// OriginalModel is the model name as requested by the client (used for billing).
	OriginalModel string

	// RedirectedModel is the model name to send to the upstream provider.
	RedirectedModel string

	// WasRedirected is true if the model was actually changed.
	WasRedirected bool
}

// ApplyModelRedirect applies a provider's model redirect rules.
// Returns the original model (for billing) and the redirected model (for upstream).
func ApplyModelRedirect(provider *model.Provider, requestModel string) ModelRedirect {
	if requestModel == "" || len(provider.ModelRedirects) == 0 {
		return ModelRedirect{
			OriginalModel:   requestModel,
			RedirectedModel: requestModel,
			WasRedirected:   false,
		}
	}

	if redirected, ok := provider.ModelRedirects.Match(requestModel); ok {
		return ModelRedirect{
			OriginalModel:   requestModel,
			RedirectedModel: redirected,
			WasRedirected:   true,
		}
	}

	return ModelRedirect{
		OriginalModel:   requestModel,
		RedirectedModel: requestModel,
		WasRedirected:   false,
	}
}

// RedirectModelInBody replaces the model field in a JSON body.
// This is a lightweight string replacement that avoids full JSON parsing.
// It replaces the first occurrence of "model":"<original>" with "model":"<redirected>".
func RedirectModelInBody(body []byte, original, redirected string) []byte {
	if original == redirected || original == "" || redirected == "" {
		return body
	}

	// Find and replace "model":"<original>" with "model":"<redirected>"
	// We search for the pattern to avoid replacing model names in other contexts
	search := `"model":"` + original + `"`
	replace := `"model":"` + redirected + `"`
	return replaceFirst(body, []byte(search), []byte(replace))
}

// replaceFirst replaces the first occurrence of old with new in s.
func replaceFirst(s, old, new []byte) []byte {
	idx := indexOf(s, old)
	if idx < 0 {
		return s
	}

	result := make([]byte, 0, len(s)-len(old)+len(new))
	result = append(result, s[:idx]...)
	result = append(result, new...)
	result = append(result, s[idx+len(old):]...)
	return result
}

// indexOf returns the index of the first occurrence of needle in haystack, or -1.
func indexOf(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	if len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
