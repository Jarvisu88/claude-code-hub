package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func newTestPayload() WebhookPayload {
	return WebhookPayload{
		EventType: EventCircuitBreakerOpen,
		Title:     "Test Alert",
		Content:   "Provider X circuit breaker opened",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}
}

// --- WeChat Work ---

func TestWebhookSender_Wechat(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           1,
		Name:         "test-wechat",
		ProviderType: string(model.WebhookProviderTypeWechat),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Equal(t, "markdown", receivedBody["msgtype"])
	md, ok := receivedBody["markdown"].(map[string]interface{})
	require.True(t, ok)
	content, ok := md["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "Test Alert")
	assert.Contains(t, content, "Provider X circuit breaker opened")
}

// --- Feishu ---

func TestWebhookSender_Feishu(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           2,
		Name:         "test-feishu",
		ProviderType: string(model.WebhookProviderTypeFeishu),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Equal(t, "interactive", receivedBody["msg_type"])
	card, ok := receivedBody["card"].(map[string]interface{})
	require.True(t, ok)
	header, ok := card["header"].(map[string]interface{})
	require.True(t, ok)
	title, ok := header["title"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Test Alert", title["content"])
}

// --- DingTalk ---

func TestWebhookSender_Dingtalk(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           3,
		Name:         "test-dingtalk",
		ProviderType: string(model.WebhookProviderTypeDingtalk),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Equal(t, "markdown", receivedBody["msgtype"])
	md, ok := receivedBody["markdown"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Test Alert", md["title"])
	text, ok := md["text"].(string)
	require.True(t, ok)
	assert.Contains(t, text, "Test Alert")
	assert.Contains(t, text, "Provider X circuit breaker opened")
}

// --- Telegram ---

func TestWebhookSender_Telegram(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Override the telegram API base to use our test server
	// We construct the target so the URL will be server.URL + "/botTOKEN/sendMessage"
	// Since our formatTelegram builds: telegramAPIBase + token + "/sendMessage"
	// We need to make the constructed URL point to our server.
	// Easiest: set the token so that the full URL matches.
	// We use a custom client with the test server transport.

	// Actually, we can use a custom http.Client that redirects to our test server.
	// Simpler approach: test with a custom sender and mock target URL directly.
	sender := NewWebhookSenderWithClient(server.Client())

	// The formatTelegram method builds URL from telegramAPIBase + token + "/sendMessage"
	// We can't easily redirect that to our test server in unit tests.
	// Instead, let's test the format and send flow by extracting format logic.

	// For a proper integration-like test, we override by testing sendOnce directly
	// or by constructing a target whose token makes the URL point to our server.

	// Best approach: just verify the format is correct by checking what the server receives.
	// We need the URL to point at the test server. Let's use a trick:
	// Strip the telegramAPIBase prefix and use the server URL as the "bot token".
	// formatTelegram generates: "https://api.telegram.org/bot{token}/sendMessage"
	// We want: server.URL + "/sendMessage"
	// So token needs to be empty and we need to override the base.
	// Since we can't override the const, let's test it differently.

	// Actually, the simplest solution: use httptest to capture the outbound request.
	// The http.Client on the sender uses server.Client() which sends to the test server
	// only for requests to the test server. For Telegram it would try the real API.

	// Let's use a transport-level redirect:
	customTransport := &testTransport{handler: func(req *http.Request) (*http.Response, error) {
		receivedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		json.Unmarshal(body, &receivedBody)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	}}

	sender = NewWebhookSenderWithClient(&http.Client{Transport: customTransport})

	target := &model.WebhookTarget{
		ID:               4,
		Name:             "test-telegram",
		ProviderType:     string(model.WebhookProviderTypeTelegram),
		TelegramBotToken: strPtr("123456:ABCDEF"),
		TelegramChatId:   strPtr("-100123456789"),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Contains(t, receivedPath, "/sendMessage")
	assert.Equal(t, "-100123456789", receivedBody["chat_id"])
	assert.Equal(t, "Markdown", receivedBody["parse_mode"])
	text, ok := receivedBody["text"].(string)
	require.True(t, ok)
	assert.Contains(t, text, "Test Alert")
}

// --- Custom ---

func TestWebhookSender_Custom(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           5,
		Name:         "test-custom",
		ProviderType: string(model.WebhookProviderTypeCustom),
		WebhookUrl:   strPtr(server.URL),
		CustomHeaders: map[string]interface{}{
			"Authorization": "Bearer secret",
		},
		CustomTemplate: map[string]interface{}{
			"source": "cch",
		},
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Equal(t, "cch", receivedBody["source"])
	assert.Equal(t, "Test Alert", receivedBody["title"])
	assert.Equal(t, string(EventCircuitBreakerOpen), receivedBody["event_type"])
}

func TestWebhookSender_Custom_NoTemplate(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           6,
		Name:         "test-custom-raw",
		ProviderType: string(model.WebhookProviderTypeCustom),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)

	assert.Equal(t, "Test Alert", receivedBody["title"])
}

// --- Retry on 5xx ---

func TestWebhookSender_RetryOn5xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusBadGateway) // 502
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           7,
		Name:         "test-retry",
		ProviderType: string(model.WebhookProviderTypeWechat),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(attempts.Load()), 3)
}

// --- No retry on 4xx ---

func TestWebhookSender_NoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest) // 400
	}))
	defer server.Close()

	sender := NewWebhookSender()
	target := &model.WebhookTarget{
		ID:           8,
		Name:         "test-no-retry",
		ProviderType: string(model.WebhookProviderTypeWechat),
		WebhookUrl:   strPtr(server.URL),
	}

	err := sender.Send(context.Background(), target, newTestPayload())
	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "should not retry on 4xx")
}

// --- Missing URL ---

func TestWebhookSender_MissingURL(t *testing.T) {
	sender := NewWebhookSender()

	platforms := []model.WebhookProviderType{
		model.WebhookProviderTypeWechat,
		model.WebhookProviderTypeFeishu,
		model.WebhookProviderTypeDingtalk,
		model.WebhookProviderTypeCustom,
	}

	for _, pt := range platforms {
		target := &model.WebhookTarget{
			ID:           100,
			ProviderType: string(pt),
		}
		err := sender.Send(context.Background(), target, newTestPayload())
		assert.Error(t, err, "platform %s should error on missing URL", pt)
	}
}

func TestWebhookSender_TelegramMissingConfig(t *testing.T) {
	sender := NewWebhookSender()

	// Missing bot token
	target := &model.WebhookTarget{
		ID:             101,
		ProviderType:   string(model.WebhookProviderTypeTelegram),
		TelegramChatId: strPtr("-123"),
	}
	err := sender.Send(context.Background(), target, newTestPayload())
	assert.Error(t, err)

	// Missing chat ID
	target2 := &model.WebhookTarget{
		ID:               102,
		ProviderType:     string(model.WebhookProviderTypeTelegram),
		TelegramBotToken: strPtr("token"),
	}
	err = sender.Send(context.Background(), target2, newTestPayload())
	assert.Error(t, err)
}

// --- Escape helpers ---

func TestEscapeMarkdownV1(t *testing.T) {
	assert.Equal(t, "hello\\_world", escapeMarkdownV1("hello_world"))
	assert.Equal(t, "no special", escapeMarkdownV1("no special"))
	assert.Equal(t, "a\\_b\\_c", escapeMarkdownV1("a_b_c"))
}

// --- isRetryable ---

func TestIsRetryable(t *testing.T) {
	assert.False(t, isRetryable(nil))
	assert.True(t, isRetryable(&webhookNetworkError{err: assert.AnError}))
	assert.True(t, isRetryable(&webhookHTTPError{statusCode: 502}))
	assert.False(t, isRetryable(&webhookHTTPError{statusCode: 400}))
	assert.False(t, isRetryable(&webhookHTTPError{statusCode: 404}))
}

// --- Test transport for intercepting outbound HTTP ---

type testTransport struct {
	handler func(req *http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.handler(req)
}
