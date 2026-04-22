package api

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func (f *fakeModelPricesStore) ListAllLatestPricesPaginated(_ context.Context, page, pageSize int, search string) (*repository.PaginatedPrices, error) {
	f.page = page
	f.pageSize = pageSize
	f.search = search
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

	directReq := httptest.NewRequest(http.MethodGet, "/api/prices?page=2&pageSize=20&search=gpt", nil)
	directReq.Header.Set("Authorization", "Bearer admin-token")
	directResp := httptest.NewRecorder()
	router.ServeHTTP(directResp, directReq)
	if directResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", directResp.Code, directResp.Body.String())
	}
	if store.page != 2 || store.pageSize != 20 || store.search != "gpt" {
		t.Fatalf("unexpected pagination capture: %+v", store)
	}
	if !strings.Contains(directResp.Body.String(), "\"ok\":true") {
		t.Fatalf("expected direct prices envelope, got %s", directResp.Body.String())
	}
}
