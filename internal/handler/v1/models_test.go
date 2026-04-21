package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type testProviderRepo struct {
	providers []*model.Provider
	err       error
}

func (r *testProviderRepo) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	return r.providers, r.err
}

func TestModelsHandlerReturnsOpenAIFormatAndDedupesModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{key: buildHandlerActiveKey("proxy-key", nil)}, testUserRepo{}, ""),
		&testProviderRepo{
			providers: []*model.Provider{
				{
					ID:           1,
					Name:         "codex-a",
					ProviderType: string(model.ProviderTypeCodex),
					AllowedModels: []string{
						"gpt-4o-mini",
						"gpt-4.1",
					},
					IsEnabled: boolPtr(true),
				},
				{
					ID:           2,
					Name:         "openai-b",
					ProviderType: string(model.ProviderTypeOpenAICompatible),
					AllowedModels: []string{
						"gpt-4.1",
						"o3-mini",
					},
					IsEnabled: boolPtr(true),
				},
			},
		},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Object != "list" {
		t.Fatalf("expected object=list, got %q", payload.Object)
	}
	if len(payload.Data) != 3 {
		t.Fatalf("expected 3 deduped models, got %d", len(payload.Data))
	}
	if payload.Data[0].ID != "gpt-4.1" || payload.Data[1].ID != "gpt-4o-mini" || payload.Data[2].ID != "o3-mini" {
		t.Fatalf("unexpected model order: %+v", payload.Data)
	}
	if payload.Data[0].OwnedBy != "openai" {
		t.Fatalf("expected openai owner inference, got %+v", payload.Data[0])
	}
}

func TestModelsHandlerReturnsGeminiFormatWhenGeminiCredentialIsUsed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{key: buildHandlerActiveKey("gemini-key", nil)}, testUserRepo{}, ""),
		&testProviderRepo{
			providers: []*model.Provider{
				{
					ID:            1,
					Name:          "gemini-a",
					ProviderType:  string(model.ProviderTypeGemini),
					AllowedModels: []string{"gemini-2.5-pro"},
					IsEnabled:     boolPtr(true),
				},
				{
					ID:            2,
					Name:          "codex-b",
					ProviderType:  string(model.ProviderTypeCodex),
					AllowedModels: []string{"o3-mini"},
					IsEnabled:     boolPtr(true),
				},
			},
		},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/models?key=gemini-key", nil)
	req.Header.Set("x-goog-api-key", "gemini-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode gemini response: %v", err)
	}
	if len(payload.Models) != 1 {
		t.Fatalf("expected 1 gemini model, got %d", len(payload.Models))
	}
	if payload.Models[0].Name != "models/gemini-2.5-pro" {
		t.Fatalf("unexpected gemini model payload: %+v", payload.Models[0])
	}
}

func TestModelsHandlerUsesOpenAIShapeForQueryKeyOnlyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{key: buildHandlerActiveKey("gemini-query-key", nil)}, testUserRepo{}, ""),
		&testProviderRepo{
			providers: []*model.Provider{
				{
					ID:            1,
					Name:          "gemini-a",
					ProviderType:  string(model.ProviderTypeGemini),
					AllowedModels: []string{"gemini-2.5-pro"},
					IsEnabled:     boolPtr(true),
				},
			},
		},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/models?key=gemini-query-key", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := payload["data"]; !ok {
		t.Fatalf("expected query-key-only request to keep OpenAI-compatible shape, got %s", resp.Body.String())
	}
	if _, ok := payload["models"]; ok {
		t.Fatalf("did not expect Gemini shape for query-key-only request, got %s", resp.Body.String())
	}
}

func TestModelsHandlerFiltersByEffectiveProviderGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group := "vip"
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{key: buildHandlerActiveKey("vip-key", &group)}, testUserRepo{}, ""),
		&testProviderRepo{
			providers: []*model.Provider{
				{
					ID:            1,
					Name:          "default-provider",
					ProviderType:  string(model.ProviderTypeCodex),
					AllowedModels: []string{"o3-mini"},
					IsEnabled:     boolPtr(true),
				},
				{
					ID:            2,
					Name:          "vip-provider",
					ProviderType:  string(model.ProviderTypeCodex),
					GroupTag:      &group,
					AllowedModels: []string{"gpt-4.1"},
					IsEnabled:     boolPtr(true),
				},
			},
		},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/models?api_type=response", nil)
	req.Header.Set("x-api-key", "vip-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "gpt-4.1" {
		t.Fatalf("expected only vip codex/openai-filtered model, got %+v", payload.Data)
	}
}

func buildHandlerActiveKey(rawKey string, providerGroup *string) *model.Key {
	enabled := true
	return &model.Key{
		ID:            1,
		UserID:        10,
		Key:           rawKey,
		Name:          "key-1",
		IsEnabled:     &enabled,
		ProviderGroup: providerGroup,
		User: &model.User{
			ID:            10,
			Name:          "tester",
			Role:          "user",
			IsEnabled:     &enabled,
			ProviderGroup: providerGroup,
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}
