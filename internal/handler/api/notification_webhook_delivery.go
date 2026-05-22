package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

type webhookDeliveryTester interface {
	Test(ctx context.Context, target *model.WebhookTarget) model.WebhookTestResult
}

type httpWebhookDeliveryTester struct {
	client *http.Client
}

func newHTTPWebhookDeliveryTester(client *http.Client) webhookDeliveryTester {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &httpWebhookDeliveryTester{client: client}
}

func NewWebhookDeliveryTester(client *http.Client) webhookDeliveryTester {
	return newHTTPWebhookDeliveryTester(client)
}

func (t *httpWebhookDeliveryTester) Test(ctx context.Context, target *model.WebhookTarget) model.WebhookTestResult {
	result := model.WebhookTestResult{
		Success:   false,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if target == nil {
		message := "webhook target is required"
		result.Message = &message
		return result
	}

	requestURL, body, err := buildWebhookTestRequest(target)
	if err != nil {
		message := err.Error()
		result.Message = &message
		return result
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		message := err.Error()
		result.Message = &message
		return result
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range target.CustomHeaders {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || value == nil {
			continue
		}
		request.Header.Set(trimmedKey, fmt.Sprint(value))
	}

	response, err := t.client.Do(request)
	if err != nil {
		message := err.Error()
		result.Message = &message
		return result
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("unexpected status: %d", response.StatusCode)
		result.Message = &message
		return result
	}

	result.Success = true
	message := "webhook test delivered"
	result.Message = &message
	return result
}

func buildWebhookTestRequest(target *model.WebhookTarget) (string, []byte, error) {
	providerType := strings.TrimSpace(target.ProviderType)
	now := time.Now().UTC().Format(time.RFC3339)

	if strings.EqualFold(providerType, "telegram") {
		botToken := trimStringValue(target.TelegramBotToken)
		chatID := trimStringValue(target.TelegramChatId)
		if botToken == "" || chatID == "" {
			return "", nil, fmt.Errorf("telegramBotToken and telegramChatId are required")
		}
		payload, err := json.Marshal(map[string]any{
			"chat_id": chatID,
			"text":    fmt.Sprintf("Claude Code Hub webhook test @ %s", now),
		})
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken), payload, nil
	}

	webhookURL := trimStringValue(target.WebhookUrl)
	if webhookURL == "" {
		return "", nil, fmt.Errorf("webhookUrl is required")
	}
	parsedURL, err := url.Parse(webhookURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", nil, fmt.Errorf("invalid webhookUrl")
	}

	payload, err := json.Marshal(map[string]any{
		"event":     "notification_test",
		"provider":  providerType,
		"text":      "Claude Code Hub webhook test",
		"timestamp": now,
	})
	if err != nil {
		return "", nil, err
	}
	return webhookURL, payload, nil
}

func trimStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
