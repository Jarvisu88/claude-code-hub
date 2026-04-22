package go_parity

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/ding113/claude-code-hub/internal/handler/v1"
	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type chatCompletionsFixtureCase struct {
	Name                     string            `json:"name"`
	Method                   string            `json:"method"`
	Path                     string            `json:"path"`
	Body                     map[string]any    `json:"body"`
	Headers                  map[string]string `json:"headers"`
	ExpectedStatus           int               `json:"expected_status"`
	ExpectedResponseContains string            `json:"expected_response_contains"`
	NodeEvidence             string            `json:"node_evidence"`
}

func TestProxyChatCompletionsMinimalLoopParity(t *testing.T) {
	fixtures := loadChatCompletionsFixtures(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if got := r.Header.Get("Authorization"); got != "Bearer provider-secret" {
			t.Fatalf("expected provider Authorization header, got %q", got)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected upstream path /v1/chat/completions, got %q", r.URL.Path)
		}
		if !strings.Contains(string(body), `"messages"`) {
			t.Fatalf("expected upstream body to include messages, got %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	router := buildChatCompletionsParityRouter(t, upstream.URL, upstream.Client())

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			body, err := json.Marshal(tc.Body)
			if err != nil {
				t.Fatalf("failed to encode fixture body: %v", err)
			}
			req := httptest.NewRequest(tc.Method, tc.Path, bytes.NewReader(body))
			for key, value := range tc.Headers {
				req.Header.Set(key, value)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.ExpectedStatus {
				t.Fatalf("node evidence %s: expected status %d, got %d, body=%s", tc.NodeEvidence, tc.ExpectedStatus, resp.Code, resp.Body.String())
			}
			if tc.ExpectedResponseContains != "" && !strings.Contains(resp.Body.String(), tc.ExpectedResponseContains) {
				t.Fatalf("node evidence %s: expected response %q to contain %q", tc.NodeEvidence, resp.Body.String(), tc.ExpectedResponseContains)
			}
		})
	}
}

func buildChatCompletionsParityRouter(t *testing.T, upstreamURL string, upstreamClient *http.Client) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	enabled := true
	svc := authsvc.NewService(&parityKeyRepo{keys: map[string]*model.Key{
		"proxy-key": {
			ID:        1,
			UserID:    100,
			Key:       "proxy-key",
			Name:      "proxy",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        100,
				Name:      "proxy-user",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}}, parityUserRepo{}, "")

	handler := v1.NewHandler(svc, paritySessionManager{}, &parityProviderRepo{providers: []*model.Provider{{
		ID:           99,
		Name:         "openai-upstream",
		URL:          upstreamURL,
		Key:          "provider-secret",
		ProviderType: string(model.ProviderTypeOpenAICompatible),
		IsEnabled:    &enabled,
	}}}, nil, upstreamClient)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))
	return router
}

func loadChatCompletionsFixtures(t *testing.T) []chatCompletionsFixtureCase {
	t.Helper()
	path := filepath.Join("..", "go-parity", "fixtures", "proxy-chat-completions-minimal-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var cases []chatCompletionsFixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}
	return cases
}

var _ = context.Background
