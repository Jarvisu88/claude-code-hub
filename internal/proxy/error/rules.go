// Package proxyerror provides the error rule matching engine and error response
// handling for the proxy pipeline.
//
// ErrorRules are loaded from the database (error_rules table), cached with a
// configurable TTL, and matched against upstream error responses using three
// match types: regex, contains, and exact. When a rule matches it can override
// the response body and/or the HTTP status code.
package proxyerror

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// RuleLoader abstracts loading active error rules from the database.
type RuleLoader interface {
	ListActive() ([]*model.ErrorRule, error)
}

// RuleMatcher caches active error rules and matches them against error
// response bodies. It is safe for concurrent use.
type RuleMatcher struct {
	loader RuleLoader
	ttl    time.Duration

	mu      sync.RWMutex
	rules   []*model.ErrorRule
	expires time.Time
}

// NewRuleMatcher creates a rule matcher with the given loader and cache TTL.
// If ttl <= 0, a default of 30 seconds is used.
func NewRuleMatcher(loader RuleLoader, ttl time.Duration) *RuleMatcher {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &RuleMatcher{
		loader: loader,
		ttl:    ttl,
	}
}

// loadRules returns the cached rules, refreshing from the loader if expired.
func (m *RuleMatcher) loadRules() ([]*model.ErrorRule, error) {
	m.mu.RLock()
	if m.rules != nil && time.Now().Before(m.expires) {
		rules := m.rules
		m.mu.RUnlock()
		return rules, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if m.rules != nil && time.Now().Before(m.expires) {
		return m.rules, nil
	}

	rules, err := m.loader.ListActive()
	if err != nil {
		return nil, err
	}

	m.rules = rules
	m.expires = time.Now().Add(m.ttl)
	return rules, nil
}

// Invalidate clears the cached rules so the next Match call re-fetches them.
func (m *RuleMatcher) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = nil
	m.expires = time.Time{}
}

// MatchResult wraps a matched error rule.
type MatchResult struct {
	Rule *model.ErrorRule
}

// Match attempts to find the first active error rule whose pattern matches
// the error message or the raw response body. Rules are tried in priority order.
// Returns nil if no rule matches.
func (m *RuleMatcher) Match(errorMessage string, responseBody []byte) (*MatchResult, error) {
	rules, err := m.loadRules()
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	matchText := strings.TrimSpace(errorMessage)
	if matchText == "" {
		matchText = ExtractErrorMessage(responseBody)
	}

	bodyText := strings.TrimSpace(string(bytes.TrimSpace(responseBody)))

	for _, rule := range rules {
		if rule == nil || !rule.IsActive() {
			continue
		}
		if matchesRule(rule, matchText, bodyText) {
			return &MatchResult{Rule: rule}, nil
		}
	}

	return nil, nil
}

// matchesRule checks whether a single rule's pattern matches the error message
// or the response body, using the rule's match type (regex, contains, exact).
func matchesRule(rule *model.ErrorRule, errorMessage, responseBody string) bool {
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" {
		return false
	}

	matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
	if matchType == "" {
		matchType = "regex"
	}

	// Build candidate list: error message first, then full body (if different).
	candidates := []string{strings.TrimSpace(errorMessage)}
	if body := strings.TrimSpace(responseBody); body != "" &&
		!strings.EqualFold(body, strings.TrimSpace(errorMessage)) {
		candidates = append(candidates, body)
	}

	switch matchType {
	case "exact":
		for _, candidate := range candidates {
			if strings.EqualFold(strings.TrimSpace(candidate), pattern) {
				return true
			}
		}
	case "contains":
		patternLower := strings.ToLower(pattern)
		for _, candidate := range candidates {
			if strings.Contains(strings.ToLower(candidate), patternLower) {
				return true
			}
		}
	default: // regex
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		for _, candidate := range candidates {
			if re.MatchString(candidate) {
				return true
			}
		}
	}

	return false
}

// ApplyResult describes what changes were made by Apply.
type ApplyResult struct {
	// OverrideApplied is true if the response body was replaced.
	OverrideApplied bool

	// StatusCode is the final status code to use.
	StatusCode int

	// Body is the final response body to use.
	Body []byte

	// ErrorMessage is the extracted/overridden error message.
	ErrorMessage string

	// FallbackReason is derived from the rule category.
	FallbackReason string
}

// Apply applies a matched error rule to the original status code and response
// body, returning the possibly-overridden result.
func Apply(rule *model.ErrorRule, originalStatusCode int, originalBody []byte) ApplyResult {
	result := ApplyResult{
		StatusCode:   originalStatusCode,
		Body:         originalBody,
		ErrorMessage: ExtractErrorMessage(originalBody),
	}

	if rule == nil {
		result.FallbackReason = fallbackReasonFromStatus(originalStatusCode)
		return result
	}

	// Override status code if the rule specifies one.
	if rule.OverrideStatusCode != nil && *rule.OverrideStatusCode > 0 {
		result.StatusCode = *rule.OverrideStatusCode
	}

	// Override response body if the rule specifies one.
	if len(rule.OverrideResponse) > 0 {
		rewritten, err := json.Marshal(rule.OverrideResponse)
		if err == nil {
			result.Body = rewritten
			result.OverrideApplied = true
			if msg := ExtractErrorMessage(rewritten); msg != "" {
				result.ErrorMessage = msg
			}
		}
	}

	if result.ErrorMessage == "" {
		result.ErrorMessage = ExtractErrorMessage(result.Body)
	}

	// Derive fallback reason from rule category.
	switch NormalizeCategory(rule.Category) {
	case "resource_not_found":
		result.FallbackReason = "resource_not_found"
	case "system_error":
		result.FallbackReason = "system_error"
	default:
		result.FallbackReason = fallbackReasonFromStatus(result.StatusCode)
	}

	return result
}

// NormalizeCategory normalizes a rule category string.
func NormalizeCategory(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

// fallbackReasonFromStatus derives a fallback reason from the HTTP status code.
func fallbackReasonFromStatus(statusCode int) string {
	switch {
	case statusCode == 404:
		return "resource_not_found"
	case statusCode >= 500:
		return "system_error"
	default:
		return ""
	}
}

// ExtractErrorMessage extracts a human-readable error message from a JSON
// response body. It tries: .error.message, .error (string), .message.
// Falls back to the raw body as a string.
func ExtractErrorMessage(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if msg := nestedErrorMessage(payload); msg != "" {
			return msg
		}
	}

	return strings.TrimSpace(string(body))
}

// nestedErrorMessage drills into the typical error envelope structures.
func nestedErrorMessage(payload map[string]any) string {
	if payload == nil {
		return ""
	}

	if raw, ok := payload["error"]; ok {
		switch typed := raw.(type) {
		case string:
			return strings.TrimSpace(typed)
		case map[string]any:
			if message, ok := typed["message"].(string); ok {
				return strings.TrimSpace(message)
			}
		}
	}

	if message, ok := payload["message"].(string); ok {
		return strings.TrimSpace(message)
	}

	return ""
}

// ExtractRequestID extracts the request_id / requestId from a JSON response.
func ExtractRequestID(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return extractRequestIDFromPayload(payload)
}

func extractRequestIDFromPayload(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"request_id", "requestId"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if errorValue, ok := payload["error"].(map[string]any); ok {
		for _, key := range []string{"request_id", "requestId"} {
			if value, ok := errorValue[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}
