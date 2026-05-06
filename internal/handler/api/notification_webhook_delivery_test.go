package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
)

func TestHTTPWebhookDeliveryTesterCustomWebhook(t *testing.T) {
	var received struct {
		Header http.Header
		Body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Header = r.Header.Clone()
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&received.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tester := newHTTPWebhookDeliveryTester(server.Client())
	target := &model.WebhookTarget{
		ProviderType: "custom",
		WebhookUrl:   stringPtr(server.URL),
		CustomHeaders: map[string]any{
			"X-Test": "yes",
		},
	}
	result := tester.Test(t.Context(), target)
	if !result.Success {
		t.Fatalf("expected success, got %#v", result)
	}
	if received.Header.Get("X-Test") != "yes" {
		t.Fatalf("expected custom header to be forwarded, got %#v", received.Header)
	}
	if received.Body["event"] != "notification_test" {
		t.Fatalf("expected test payload, got %#v", received.Body)
	}
}

func TestHTTPWebhookDeliveryTesterRejectsInvalidWebhook(t *testing.T) {
	tester := newHTTPWebhookDeliveryTester(nil)
	result := tester.Test(t.Context(), &model.WebhookTarget{ProviderType: "custom", WebhookUrl: stringPtr("not-a-url")})
	if result.Success {
		t.Fatalf("expected failure for invalid url, got %#v", result)
	}
	if result.Message == nil || *result.Message == "" {
		t.Fatalf("expected failure message, got %#v", result)
	}
}
