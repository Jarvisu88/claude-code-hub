package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

func (h *Handler) maybeFixResponseBody(headers http.Header, body []byte) ([]byte, bool) {
	settings := h.currentProxySystemSettings()
	if settings != nil && !settings.EnableResponseFixer {
		return body, false
	}
	contentType := strings.ToLower(strings.TrimSpace(headers.Get("Content-Type")))
	if !strings.Contains(contentType, "json") || json.Valid(body) {
		return body, false
	}
	fixed := fixTruncatedJSON(body)
	if len(fixed) == 0 || bytes.Equal(fixed, body) || !json.Valid(fixed) {
		return body, false
	}
	return fixed, true
}

func fixTruncatedJSON(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body
	}
	out := make([]byte, 0, len(trimmed)+4)
	inString := false
	escaped := false
	var braceCount int
	var bracketCount int
	for _, b := range trimmed {
		out = append(out, b)
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch b {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
	}
	for len(out) > 0 && (out[len(out)-1] == ',' || out[len(out)-1] == '\n' || out[len(out)-1] == '\r' || out[len(out)-1] == '\t' || out[len(out)-1] == ' ') {
		if out[len(out)-1] == ',' {
			out = out[:len(out)-1]
			break
		}
		out = out[:len(out)-1]
	}
	if inString {
		out = append(out, '"')
	}
	for bracketCount > 0 {
		out = append(out, ']')
		bracketCount--
	}
	for braceCount > 0 {
		out = append(out, '}')
		braceCount--
	}
	return out
}

func responseFixerEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableResponseFixer
}
