package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

type upstreamErrorDecision struct {
	StatusCode      int
	ErrorMessage    string
	ResponseBody    []byte
	FallbackReason  string
	RequestID       string
	OverrideApplied bool
}

func (h *Handler) evaluateUpstreamErrorDecision(ctx context.Context, statusCode int, responseBody []byte) (upstreamErrorDecision, error) {
	decision := upstreamErrorDecision{
		StatusCode:   statusCode,
		ErrorMessage: "",
		ResponseBody: responseBody,
		RequestID:    extractProxyErrorRequestID(responseBody),
	}

	inferredStatusCode, inferredErrorMessage, apparentError := inferApparentUpstreamError(statusCode, responseBody)
	if !apparentError {
		return decision, nil
	}
	decision.StatusCode = inferredStatusCode
	decision.ErrorMessage = inferredErrorMessage
	decision.FallbackReason = fallbackReasonFromStatusCode(inferredStatusCode)

	rule, err := h.matchUpstreamErrorRule(ctx, inferredErrorMessage, responseBody)
	if err != nil {
		return upstreamErrorDecision{}, appErrors.NewInternalError("加载错误规则失败").WithError(err)
	}
	if rule == nil {
		return decision, nil
	}

	if rule.OverrideStatusCode != nil && *rule.OverrideStatusCode > 0 {
		decision.StatusCode = *rule.OverrideStatusCode
	}
	if len(rule.OverrideResponse) > 0 {
		rewrittenBody, marshalErr := json.Marshal(rule.OverrideResponse)
		if marshalErr != nil {
			return upstreamErrorDecision{}, appErrors.NewInternalError("构建错误规则覆盖响应失败").WithError(marshalErr)
		}
		decision.ResponseBody = rewrittenBody
		decision.OverrideApplied = true
		if message := extractErrorMessageOrBody(rewrittenBody); message != "" {
			decision.ErrorMessage = message
		}
		if requestID := extractProxyErrorRequestID(rewrittenBody); requestID != "" {
			decision.RequestID = requestID
		}
	}
	if decision.ErrorMessage == "" {
		decision.ErrorMessage = extractErrorMessageOrBody(decision.ResponseBody)
	}
	switch normalizeErrorRuleCategory(rule.Category) {
	case "resource_not_found":
		decision.FallbackReason = "resource_not_found"
	case "system_error":
		decision.FallbackReason = "system_error"
	default:
		decision.FallbackReason = fallbackReasonFromStatusCode(decision.StatusCode)
	}

	return decision, nil
}

func inferApparentUpstreamError(statusCode int, responseBody []byte) (int, string, bool) {
	if statusCode >= 400 {
		return statusCode, extractErrorMessageOrBody(responseBody), true
	}
	if fakeStatusCode, fakeErrorMessage, ok := detectFake200HTMLResponse(statusCode, responseBody); ok {
		return fakeStatusCode, fakeErrorMessage, true
	}

	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err == nil {
		if fakeStatusCode, fakeErrorMessage, ok := detectFake200JSONResponse(statusCode, payload, responseBody); ok {
			return fakeStatusCode, fakeErrorMessage, true
		}
	}
	if fakeStatusCode, fakeErrorMessage, ok := detectFake200PlainTextResponse(statusCode, responseBody); ok {
		return fakeStatusCode, fakeErrorMessage, true
	}
	return statusCode, "", false
}

func fallbackReasonFromStatusCode(statusCode int) string {
	switch {
	case statusCode == 404:
		return "resource_not_found"
	case statusCode >= 500:
		return "system_error"
	default:
		return ""
	}
}

func (h *Handler) matchUpstreamErrorRule(ctx context.Context, errorMessage string, responseBody []byte) (*model.ErrorRule, error) {
	if h == nil || h.errorRules == nil {
		return nil, nil
	}
	rules, err := h.errorRules.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	matchText := strings.TrimSpace(errorMessage)
	if matchText == "" {
		matchText = extractErrorMessageOrBody(responseBody)
	}
	bodyText := strings.TrimSpace(string(bytes.TrimSpace(responseBody)))
	for _, rule := range rules {
		if rule == nil || !rule.IsActive() {
			continue
		}
		if matchesErrorRule(rule, matchText, bodyText) {
			return rule, nil
		}
	}
	return nil, nil
}

func matchesErrorRule(rule *model.ErrorRule, errorMessage, responseBody string) bool {
	if rule == nil {
		return false
	}
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" {
		return false
	}
	matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
	if matchType == "" {
		matchType = "regex"
	}
	candidates := []string{strings.TrimSpace(errorMessage)}
	if body := strings.TrimSpace(responseBody); body != "" && !strings.EqualFold(body, strings.TrimSpace(errorMessage)) {
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
	default:
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

func normalizeErrorRuleCategory(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

func (h *Handler) evaluateStreamingUpstreamDecision(ctx context.Context, statusCode int, streamBody []byte) (upstreamErrorDecision, bool, error) {
	decision := upstreamErrorDecision{
		StatusCode:   statusCode,
		ErrorMessage: "",
		ResponseBody: streamBody,
		RequestID:    extractProxyErrorRequestID(streamBody),
	}

	if inferredStatusCode, inferredErrorMessage, inferredRequestID, ok := inferApparentStreamingUpstreamError(statusCode, streamBody); ok {
		decision.StatusCode = inferredStatusCode
		decision.ErrorMessage = inferredErrorMessage
		decision.FallbackReason = fallbackReasonFromStatusCode(inferredStatusCode)
		if inferredRequestID != "" {
			decision.RequestID = inferredRequestID
		}
		rule, err := h.matchUpstreamErrorRule(ctx, inferredErrorMessage, streamBody)
		if err != nil {
			return upstreamErrorDecision{}, false, appErrors.NewInternalError("加载错误规则失败").WithError(err)
		}
		if rule != nil {
			if rule.OverrideStatusCode != nil && *rule.OverrideStatusCode > 0 {
				decision.StatusCode = *rule.OverrideStatusCode
			}
			if len(rule.OverrideResponse) > 0 {
				rewrittenBody, marshalErr := json.Marshal(rule.OverrideResponse)
				if marshalErr != nil {
					return upstreamErrorDecision{}, false, appErrors.NewInternalError("构建错误规则覆盖响应失败").WithError(marshalErr)
				}
				decision.ResponseBody = rewrittenBody
				decision.OverrideApplied = true
				if message := extractErrorMessageOrBody(rewrittenBody); message != "" {
					decision.ErrorMessage = message
				}
				if requestID := extractProxyErrorRequestID(rewrittenBody); requestID != "" {
					decision.RequestID = requestID
				}
			}
			switch normalizeErrorRuleCategory(rule.Category) {
			case "resource_not_found":
				decision.FallbackReason = "resource_not_found"
			case "system_error":
				decision.FallbackReason = "system_error"
			default:
				decision.FallbackReason = fallbackReasonFromStatusCode(decision.StatusCode)
			}
		}
		return decision, true, nil
	}

	return decision, false, nil
}

func inferApparentStreamingUpstreamError(statusCode int, streamBody []byte) (int, string, string, bool) {
	trimmed := bytes.TrimSpace(streamBody)
	if len(trimmed) == 0 {
		return statusCode, "", "", false
	}
	if isStreamingEnvelope(trimmed) {
		payloads := extractSSEDataPayloads(trimmed)
		for _, payload := range payloads {
			payload = bytes.TrimSpace(payload)
			if len(payload) == 0 {
				continue
			}
			if inferredStatusCode, inferredErrorMessage, ok := inferApparentUpstreamError(statusCode, payload); ok {
				return inferredStatusCode, inferredErrorMessage, extractProxyErrorRequestID(payload), true
			}
		}
		if statusCode >= 400 {
			return statusCode, extractSSEErrorMessageOrBody(trimmed), extractProxyErrorRequestIDFromSSE(trimmed), true
		}
		return statusCode, "", "", false
	}
	if inferredStatusCode, inferredErrorMessage, ok := inferApparentUpstreamError(statusCode, trimmed); ok {
		return inferredStatusCode, inferredErrorMessage, extractProxyErrorRequestID(trimmed), true
	}
	if statusCode >= 400 {
		return statusCode, extractSSEErrorMessageOrBody(trimmed), extractProxyErrorRequestIDFromSSE(trimmed), true
	}
	return statusCode, "", "", false
}

func isStreamingEnvelope(streamBody []byte) bool {
	lower := strings.ToLower(string(streamBody))
	return strings.Contains(lower, "\ndata:") || strings.HasPrefix(lower, "data:") || strings.Contains(lower, "event:")
}

func extractSSEDataPayloads(streamBody []byte) [][]byte {
	lines := bytes.Split(streamBody, []byte("\n"))
	payloads := make([][]byte, 0)
	for _, rawLine := range lines {
		line := bytes.TrimSpace(rawLine)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func extractSSEErrorMessageOrBody(streamBody []byte) string {
	for _, payload := range extractSSEDataPayloads(streamBody) {
		if message := extractErrorMessageOrBody(payload); strings.TrimSpace(message) != "" {
			return message
		}
	}
	return strings.TrimSpace(string(streamBody))
}

func extractProxyErrorRequestIDFromSSE(streamBody []byte) string {
	for _, payload := range extractSSEDataPayloads(streamBody) {
		if requestID := extractProxyErrorRequestID(payload); requestID != "" {
			return requestID
		}
	}
	return ""
}

func extractProxyErrorRequestID(responseBody []byte) string {
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return ""
	}
	return extractProxyErrorRequestIDFromPayload(payload)
}

func extractProxyErrorRequestIDFromPayload(payload map[string]any) string {
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
