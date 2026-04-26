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
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
)

type responsesFixtureCase struct {
	Name                     string            `json:"name"`
	Method                   string            `json:"method"`
	Path                     string            `json:"path"`
	Body                     map[string]any    `json:"body"`
	Headers                  map[string]string `json:"headers"`
	ExpectedStatus           int               `json:"expected_status"`
	ExpectedResponseContains string            `json:"expected_response_contains"`
	NodeEvidence             string            `json:"node_evidence"`
}

type parityProviderRepo struct {
	providers []*model.Provider
}

func (r *parityProviderRepo) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	return r.providers, nil
}

type paritySessionManager struct{}

func (paritySessionManager) ExtractClientSessionID(_ map[string]any, _ http.Header) sessionsvc.ClientSessionExtractionResult {
	return sessionsvc.ClientSessionExtractionResult{SessionID: "sess_client_123"}
}

func (paritySessionManager) GetOrCreateSessionID(_ context.Context, _ int, _ any, _ string) string {
	return "sess_generated_123"
}

func (paritySessionManager) GetNextRequestSequence(_ context.Context, _ string) int { return 1 }
func (paritySessionManager) BindProvider(_ context.Context, _ string, _ int)        {}
func (paritySessionManager) UpdateCodexSessionWithPromptCacheKey(_ context.Context, currentSessionID, _ string, _ int) string {
	return currentSessionID
}
func (paritySessionManager) IncrementConcurrentCount(_ context.Context, _ string) {}
func (paritySessionManager) DecrementConcurrentCount(_ context.Context, _ string) {}

func TestProxyResponsesMinimalLoopParity(t *testing.T) {
	fixtures := loadResponsesFixtures(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if got := r.Header.Get("Authorization"); got != "Bearer provider-secret" {
			t.Fatalf("expected provider Authorization header, got %q", got)
		}
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("expected upstream path /v1/responses, got %q", r.URL.Path)
		}
		if !strings.Contains(string(body), `"input"`) {
			t.Fatalf("expected upstream body to include input, got %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resp_123","status":"completed"}`))
	}))
	defer upstream.Close()

	router := buildResponsesParityRouter(t, upstream.URL, upstream.Client())

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

func buildResponsesParityRouter(t *testing.T, upstreamURL string, upstreamClient *http.Client) *gin.Engine {
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
		Name:         "codex-upstream",
		URL:          upstreamURL,
		Key:          "provider-secret",
		ProviderType: string(model.ProviderTypeCodex),
		IsEnabled:    &enabled,
	}}}, nil, upstreamClient)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))
	return router
}

func loadResponsesFixtures(t *testing.T) []responsesFixtureCase {
	t.Helper()
	path := filepath.Join("..", "go-parity", "fixtures", "proxy-responses-minimal-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var cases []responsesFixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}
	return cases
}

var _ = appErrors.CodeNoProviderAvailable
