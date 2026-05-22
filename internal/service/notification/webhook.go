package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

const (
	defaultWebhookTimeout   = 10 * time.Second
	webhookMaxRetries       = 3
	webhookRetryBaseWait    = 1 * time.Second
	telegramAPIBase         = "https://api.telegram.org/bot"
	maxResponseBodyReadSize = 4096
)

// WebhookPayload is the platform-agnostic notification payload that gets
// formatted per-platform before sending.
type WebhookPayload struct {
	EventType EventType   `json:"event_type"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
	Extra     interface{} `json:"extra,omitempty"`
}

// WebhookSender delivers notifications to multi-platform webhook endpoints.
type WebhookSender struct {
	httpClient *http.Client
}

// NewWebhookSender creates a new WebhookSender with default settings.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		httpClient: &http.Client{
			Timeout: defaultWebhookTimeout,
		},
	}
}

// NewWebhookSenderWithClient creates a WebhookSender using a custom HTTP client.
// This is primarily useful for testing.
func NewWebhookSenderWithClient(client *http.Client) *WebhookSender {
	return &WebhookSender{httpClient: client}
}

// Send delivers a webhook payload to the specified target. It automatically
// formats the payload for the target's platform and retries on transient failures.
func (s *WebhookSender) Send(ctx context.Context, target *model.WebhookTarget, payload WebhookPayload) error {
	var lastErr error

	for attempt := 0; attempt <= webhookMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := webhookRetryBaseWait * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := s.sendOnce(ctx, target, payload)
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on retryable errors (5xx, network)
		if !isRetryable(err) {
			return err
		}

		logger.Warn().Err(err).
			Int("target_id", target.ID).
			Str("platform", target.ProviderType).
			Int("attempt", attempt+1).
			Msg("[Webhook] Delivery attempt failed, will retry")
	}

	return fmt.Errorf("webhook delivery failed after %d attempts: %w", webhookMaxRetries+1, lastErr)
}

// sendOnce performs a single delivery attempt.
func (s *WebhookSender) sendOnce(ctx context.Context, target *model.WebhookTarget, payload WebhookPayload) error {
	url, body, err := s.formatForPlatform(target, payload)
	if err != nil {
		return fmt.Errorf("format error: %w", err)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Apply custom headers for custom webhook targets
	if target.ProviderType == string(model.WebhookProviderTypeCustom) && target.CustomHeaders != nil {
		for k, v := range target.CustomHeaders {
			if sv, ok := v.(string); ok {
				req.Header.Set(k, sv)
			}
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &webhookNetworkError{err: err}
	}
	defer resp.Body.Close()

	// Read a limited amount of the response body for logging
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyReadSize))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	werr := &webhookHTTPError{
		statusCode: resp.StatusCode,
		body:       string(respBody),
	}

	logger.Error().
		Int("target_id", target.ID).
		Str("platform", target.ProviderType).
		Int("status_code", resp.StatusCode).
		Str("response", string(respBody)).
		Msg("[Webhook] Delivery returned non-2xx status")

	return werr
}

// formatForPlatform constructs the URL and request body based on the webhook platform.
func (s *WebhookSender) formatForPlatform(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	switch model.WebhookProviderType(target.ProviderType) {
	case model.WebhookProviderTypeWechat:
		return s.formatWechat(target, payload)
	case model.WebhookProviderTypeFeishu:
		return s.formatFeishu(target, payload)
	case model.WebhookProviderTypeDingtalk:
		return s.formatDingtalk(target, payload)
	case model.WebhookProviderTypeTelegram:
		return s.formatTelegram(target, payload)
	case model.WebhookProviderTypeCustom:
		return s.formatCustom(target, payload)
	default:
		return "", nil, fmt.Errorf("unsupported webhook platform: %s", target.ProviderType)
	}
}

// formatWechat builds the WeChat Work (企业微信) webhook body.
func (s *WebhookSender) formatWechat(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	if target.WebhookUrl == nil || *target.WebhookUrl == "" {
		return "", nil, fmt.Errorf("wechat webhook URL is not configured")
	}

	content := fmt.Sprintf("**%s**\n%s", payload.Title, payload.Content)
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": content,
		},
	}

	return *target.WebhookUrl, body, nil
}

// formatFeishu builds the Feishu (飞书) interactive card body.
func (s *WebhookSender) formatFeishu(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	if target.WebhookUrl == nil || *target.WebhookUrl == "" {
		return "", nil, fmt.Errorf("feishu webhook URL is not configured")
	}

	body := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": payload.Title,
				},
				"template": "blue",
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag":     "markdown",
					"content": payload.Content,
				},
				map[string]interface{}{
					"tag": "note",
					"elements": []interface{}{
						map[string]interface{}{
							"tag":     "plain_text",
							"content": payload.Timestamp.Format(time.RFC3339),
						},
					},
				},
			},
		},
	}

	return *target.WebhookUrl, body, nil
}

// formatDingtalk builds the DingTalk (钉钉) markdown body.
func (s *WebhookSender) formatDingtalk(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	if target.WebhookUrl == nil || *target.WebhookUrl == "" {
		return "", nil, fmt.Errorf("dingtalk webhook URL is not configured")
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": payload.Title,
			"text":  fmt.Sprintf("### %s\n\n%s\n\n> %s", payload.Title, payload.Content, payload.Timestamp.Format(time.RFC3339)),
		},
	}

	return *target.WebhookUrl, body, nil
}

// formatTelegram builds the Telegram Bot API sendMessage body.
func (s *WebhookSender) formatTelegram(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	if target.TelegramBotToken == nil || *target.TelegramBotToken == "" {
		return "", nil, fmt.Errorf("telegram bot token is not configured")
	}
	if target.TelegramChatId == nil || *target.TelegramChatId == "" {
		return "", nil, fmt.Errorf("telegram chat ID is not configured")
	}

	url := telegramAPIBase + *target.TelegramBotToken + "/sendMessage"
	text := fmt.Sprintf("*%s*\n\n%s", escapeMarkdownV1(payload.Title), payload.Content)
	body := map[string]interface{}{
		"chat_id":    *target.TelegramChatId,
		"text":       text,
		"parse_mode": "Markdown",
	}

	return url, body, nil
}

// formatCustom builds the raw JSON body for a custom webhook target.
func (s *WebhookSender) formatCustom(target *model.WebhookTarget, payload WebhookPayload) (string, interface{}, error) {
	if target.WebhookUrl == nil || *target.WebhookUrl == "" {
		return "", nil, fmt.Errorf("custom webhook URL is not configured")
	}

	// If a custom template is provided, use it as the base and inject payload fields
	if target.CustomTemplate != nil && len(target.CustomTemplate) > 0 {
		body := make(map[string]interface{})
		for k, v := range target.CustomTemplate {
			body[k] = v
		}
		body["event_type"] = string(payload.EventType)
		body["title"] = payload.Title
		body["content"] = payload.Content
		body["timestamp"] = payload.Timestamp.Format(time.RFC3339)
		if payload.Extra != nil {
			body["extra"] = payload.Extra
		}
		return *target.WebhookUrl, body, nil
	}

	// Default: send the payload as-is
	return *target.WebhookUrl, payload, nil
}

// --- Error types for retry logic ---

// webhookHTTPError represents a non-2xx HTTP response.
type webhookHTTPError struct {
	statusCode int
	body       string
}

func (e *webhookHTTPError) Error() string {
	return fmt.Sprintf("webhook HTTP %d: %s", e.statusCode, e.body)
}

// IsServerError returns true if the HTTP error is a 5xx status.
func (e *webhookHTTPError) IsServerError() bool {
	return e.statusCode >= 500
}

// webhookNetworkError wraps a network-level error.
type webhookNetworkError struct {
	err error
}

func (e *webhookNetworkError) Error() string {
	return fmt.Sprintf("webhook network error: %v", e.err)
}

func (e *webhookNetworkError) Unwrap() error {
	return e.err
}

// isRetryable determines if an error warrants a retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	if _, ok := err.(*webhookNetworkError); ok {
		return true
	}

	// Only 5xx HTTP errors are retryable
	if herr, ok := err.(*webhookHTTPError); ok {
		return herr.IsServerError()
	}

	return false
}

// escapeMarkdownV1 escapes special characters for Telegram MarkdownV1 format.
func escapeMarkdownV1(s string) string {
	// In Telegram MarkdownV1, *, _, `, [ are special.
	// We only escape underscores since we use * for bold intentionally.
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			result = append(result, '\\')
		}
		result = append(result, s[i])
	}
	return string(result)
}
