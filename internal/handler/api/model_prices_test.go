package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeModelPricesStore struct {
	prices    []*model.ModelPrice
	paginated *repository.PaginatedPrices
	page      int
	pageSize  int
	search    string
	has       bool
	models    []string
}

func (f *fakeModelPricesStore) ListAllLatestPrices(_ context.Context) ([]*model.ModelPrice, error) {
	return f.prices, nil
}

func (f *fakeModelPricesStore) ListAllLatestPricesPaginated(_ context.Context, page, pageSize int, search, source, litellmProvider string) (*repository.PaginatedPrices, error) {
	f.page = page
	f.pageSize = pageSize
	if source != "" {
		f.search = search + "|source=" + source
	} else {
		f.search = search
	}
	if litellmProvider != "" {
		f.search += "|litellmProvider=" + litellmProvider
	}
	return f.paginated, nil
}

func (f *fakeModelPricesStore) HasAnyRecords(_ context.Context) (bool, error) {
	return f.has, nil
}

func (f *fakeModelPricesStore) GetAllModelNames(_ context.Context) ([]string, error) {
	return f.models, nil
}

func (f *fakeModelPricesStore) GetChatModelNames(_ context.Context) ([]string, error) {
	return f.models, nil
}

func TestModelPricesActionAndDirectRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	store := &fakeModelPricesStore{
		prices: []*model.ModelPrice{{ID: 1, ModelName: "gpt-5.4", CreatedAt: now, UpdatedAt: now}},
		paginated: &repository.PaginatedPrices{
			Data:       []*model.ModelPrice{{ID: 1, ModelName: "gpt-5.4", CreatedAt: now, UpdatedAt: now}},
			Total:      1,
			Page:       1,
			PageSize:   50,
			TotalPages: 1,
		},
		has:    true,
		models: []string{"gpt-5.4", "gpt-4o-mini"},
	}
	auth := fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}

	router := gin.New()
	NewModelPricesActionHandler(auth, store).RegisterActionRoutes(router.Group("/api/actions"))
	NewModelPricesActionHandler(auth, store).RegisterDirectRoutes(router.Group("/api/prices"))

	actionReq := httptest.NewRequest(http.MethodPost, "/api/actions/model-prices/getModelPrices", strings.NewReader(`{}`))
	actionReq.Header.Set("Authorization", "Bearer admin-token")
	actionReq.Header.Set("Content-Type", "application/json")
	actionResp := httptest.NewRecorder()
	router.ServeHTTP(actionResp, actionReq)
	if actionResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", actionResp.Code, actionResp.Body.String())
	}
	if !strings.Contains(actionResp.Body.String(), "gpt-5.4") {
		t.Fatalf("expected model prices payload, got %s", actionResp.Body.String())
	}

	hasReq := httptest.NewRequest(http.MethodPost, "/api/actions/model-prices/hasPriceTable", strings.NewReader(`{}`))
	hasReq.Header.Set("Authorization", "Bearer admin-token")
	hasReq.Header.Set("Content-Type", "application/json")
	hasResp := httptest.NewRecorder()
	router.ServeHTTP(hasResp, hasReq)
	if hasResp.Code != http.StatusOK || !strings.Contains(hasResp.Body.String(), "\"data\":true") {
		t.Fatalf("expected hasPriceTable payload, got %d: %s", hasResp.Code, hasResp.Body.String())
	}

	availableReq := httptest.NewRequest(http.MethodPost, "/api/actions/model-prices/getAvailableModelsByProviderType", strings.NewReader(`{}`))
	availableReq.Header.Set("Authorization", "Bearer admin-token")
	availableReq.Header.Set("Content-Type", "application/json")
	availableResp := httptest.NewRecorder()
	router.ServeHTTP(availableResp, availableReq)
	if availableResp.Code != http.StatusOK || !strings.Contains(availableResp.Body.String(), "gpt-4o-mini") {
		t.Fatalf("expected available models payload, got %d: %s", availableResp.Code, availableResp.Body.String())
	}

	paginatedReq := httptest.NewRequest(http.MethodPost, "/api/actions/model-prices/getModelPricesPaginated", strings.NewReader(`{"page":2,"pageSize":20,"search":"gpt"}`))
	paginatedReq.Header.Set("Authorization", "Bearer admin-token")
	paginatedReq.Header.Set("Content-Type", "application/json")
	paginatedResp := httptest.NewRecorder()
	router.ServeHTTP(paginatedResp, paginatedReq)
	if paginatedResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", paginatedResp.Code, paginatedResp.Body.String())
	}
	if store.page != 2 || store.pageSize != 20 || store.search != "gpt" {
		t.Fatalf("expected action paginated request to capture page/pageSize/search, got %+v", store)
	}

	directReq := httptest.NewRequest(http.MethodGet, "/api/prices?page=2&size=20&search=gpt&source=manual&litellmProvider=anthropic", nil)
	directReq.Header.Set("Authorization", "Bearer admin-token")
	directResp := httptest.NewRecorder()
	router.ServeHTTP(directResp, directReq)
	if directResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", directResp.Code, directResp.Body.String())
	}
	if store.page != 2 || store.pageSize != 20 || store.search != "gpt|source=manual|litellmProvider=anthropic" {
		t.Fatalf("unexpected pagination capture: %+v", store)
	}
	if strings.Contains(directResp.Body.String(), "\"ok\":true") || !strings.Contains(directResp.Body.String(), "\"pageSize\":50") {
		t.Fatalf("expected direct prices raw paginated shape, got %s", directResp.Body.String())
	}

	cloudReq := httptest.NewRequest(http.MethodGet, "/api/prices/cloud-model-count", nil)
	cloudReq.Header.Set("Authorization", "Bearer admin-token")
	cloudResp := httptest.NewRecorder()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[models]\n  [models.gpt_5_4]\n  [models.gpt_4o_mini]\n  [models.gemini_2_5_pro]\n"))
	}))
	defer upstream.Close()
	oldURL := os.Getenv("CLOUD_PRICE_TABLE_URL")
	t.Cleanup(func() { _ = os.Setenv("CLOUD_PRICE_TABLE_URL", oldURL) })
	_ = os.Setenv("CLOUD_PRICE_TABLE_URL", upstream.URL)
	router.ServeHTTP(cloudResp, cloudReq)
	if cloudResp.Code != http.StatusOK || !strings.Contains(cloudResp.Body.String(), "\"count\":3") {
		t.Fatalf("expected cloud model count payload, got %d: %s", cloudResp.Code, cloudResp.Body.String())
	}
}
