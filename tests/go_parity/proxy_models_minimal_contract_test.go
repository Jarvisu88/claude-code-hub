package go_parity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/ding113/claude-code-hub/internal/handler/v1"
	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestProxyModelsMinimalParity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	svc := authsvc.NewService(&parityKeyRepo{keys: map[string]*model.Key{
		"proxy-key": {
			ID:        1,
			UserID:    100,
			Key:       "proxy-key",
			Name:      "proxy",
			IsEnabled: &enabled,
			User:      &model.User{ID: 100, Name: "proxy-user", Role: "user", IsEnabled: &enabled},
		},
	}}, parityUserRepo{}, "")

	handler := v1.NewHandler(svc, paritySessionManager{}, &parityProviderRepo{providers: []*model.Provider{
		{ID: 1, Name: "claude", ProviderType: string(model.ProviderTypeClaude), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("claude-sonnet-4")},
		{ID: 2, Name: "codex", ProviderType: string(model.ProviderTypeCodex), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gpt-5.4")},
		{ID: 3, Name: "openai", ProviderType: string(model.ProviderTypeOpenAICompatible), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gpt-4o-mini")},
	}}, nil, http.DefaultClient)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	tests := []struct {
		name       string
		path       string
		wantModels []string
		notModels  []string
	}{
		{name: "all models", path: "/v1/models", wantModels: []string{"claude-sonnet-4", "gpt-5.4", "gpt-4o-mini"}},
		{name: "responses models", path: "/v1/responses/models", wantModels: []string{"gpt-5.4"}, notModels: []string{"claude-sonnet-4", "gpt-4o-mini"}},
		{name: "chat completions models", path: "/v1/chat/completions/models", wantModels: []string{"gpt-4o-mini"}, notModels: []string{"gpt-5.4", "claude-sonnet-4"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer proxy-key")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode models payload: %v", err)
			}
			data, ok := payload["data"].([]any)
			if !ok {
				t.Fatalf("expected data array, got %#v", payload["data"])
			}
			serialized := resp.Body.String()
			for _, want := range tc.wantModels {
				if !contains(serialized, want) {
					t.Fatalf("expected %s to contain model %q", serialized, want)
				}
			}
			for _, forbidden := range tc.notModels {
				if contains(serialized, forbidden) {
					t.Fatalf("expected %s to not contain model %q", serialized, forbidden)
				}
			}
			_ = data
		})
	}
}
