package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeAdminAuth struct {
	result *authsvc.AuthResult
	err    error
}

func (f fakeAdminAuth) AuthenticateAdminToken(_ string) (*authsvc.AuthResult, error) {
	return f.result, f.err
}

type fakeUserLister struct{ users []*model.User }

func (f fakeUserLister) List(_ context.Context, _ *repository.ListOptions) ([]*model.User, error) {
	return f.users, nil
}

func (f fakeUserLister) GetByID(_ context.Context, id int) (*model.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, appErrors.NewNotFoundError("User")
}

func (f fakeUserLister) Create(_ context.Context, user *model.User) (*model.User, error) {
	user.ID = 99
	return user, nil
}

func (f fakeUserLister) Update(_ context.Context, user *model.User) (*model.User, error) {
	return user, nil
}

func (f fakeUserLister) Delete(_ context.Context, _ int) error { return nil }

type fakeKeyLister struct{ keys []*model.Key }

func (f fakeKeyLister) List(_ context.Context, _ *repository.ListOptions) ([]*model.Key, error) {
	return f.keys, nil
}

func (f fakeKeyLister) GetByID(_ context.Context, id int) (*model.Key, error) {
	for _, key := range f.keys {
		if key.ID == id {
			return key, nil
		}
	}
	return nil, appErrors.NewNotFoundError("Key")
}

func (f fakeKeyLister) Create(_ context.Context, key *model.Key) (*model.Key, error) {
	key.ID = 99
	return key, nil
}

func (f fakeKeyLister) Update(_ context.Context, key *model.Key) (*model.Key, error) {
	return key, nil
}

func (f fakeKeyLister) Delete(_ context.Context, _ int) error { return nil }

type fakeProviderLister struct{ providers []*model.Provider }

func (f fakeProviderLister) List(_ context.Context, _ *repository.ListOptions) ([]*model.Provider, error) {
	return f.providers, nil
}

func (f fakeProviderLister) GetByID(_ context.Context, id int) (*model.Provider, error) {
	for _, provider := range f.providers {
		if provider.ID == id {
			return provider, nil
		}
	}
	return nil, appErrors.NewNotFoundError("Provider")
}

func (f fakeProviderLister) Create(_ context.Context, provider *model.Provider) (*model.Provider, error) {
	provider.ID = 99
	return provider, nil
}

func (f fakeProviderLister) Update(_ context.Context, provider *model.Provider) (*model.Provider, error) {
	return provider, nil
}

func (f fakeProviderLister) Delete(_ context.Context, _ int) error { return nil }

func TestAdminListRoutesRequireTokenAndReturnData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeUserLister{users: []*model.User{{ID: 1, Name: "alice"}}},
		fakeKeyLister{keys: []*model.Key{{ID: 1, Name: "key-1"}}},
		fakeProviderLister{providers: []*model.Provider{{ID: 1, Name: "provider-1"}}},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		path         string
		wantContains string
	}{
		{path: "/api/actions/users", wantContains: "alice"},
		{path: "/api/actions/keys", wantContains: "key-1"},
		{path: "/api/actions/providers", wantContains: "provider-1"},
		{path: "/api/actions/users/1", wantContains: "alice"},
		{path: "/api/actions/keys/1", wantContains: "key-1"},
		{path: "/api/actions/providers/1", wantContains: "provider-1"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer admin-token")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}
			if got := resp.Body.String(); !strings.Contains(got, tc.wantContains) {
				t.Fatalf("expected response to contain %q, got %s", tc.wantContains, got)
			}
		})
	}
}

func TestAdminDetailRoutesRejectInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeUserLister{users: []*model.User{{ID: 1, Name: "alice"}}},
		fakeKeyLister{keys: []*model.Key{{ID: 1, Name: "key-1"}}},
		fakeProviderLister{providers: []*model.Provider{{ID: 1, Name: "provider-1"}}},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/users/not-an-id", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"ok\":false") {
		t.Fatalf("expected error envelope, got %s", resp.Body.String())
	}
}

func TestAdminListRoutesRejectInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		fakeAdminAuth{err: appErrors.NewAuthenticationError("未提供管理员令牌。", appErrors.CodeTokenRequired)},
		fakeUserLister{},
		fakeKeyLister{},
		fakeProviderLister{},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/users", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"ok\":false") {
		t.Fatalf("expected error envelope, got %s", resp.Body.String())
	}
}

func TestOpenAPIDocsEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil, fakeUserLister{}, fakeKeyLister{}, fakeProviderLister{})
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		path         string
		wantStatus   int
		wantContains string
	}{
		{path: "/api/actions/openapi.json", wantStatus: http.StatusOK, wantContains: "openapi"},
		{path: "/api/actions/docs", wantStatus: http.StatusOK, wantContains: "/api/actions/openapi.json"},
		{path: "/api/actions/scalar", wantStatus: http.StatusOK, wantContains: "/api/actions/openapi.json"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestOpenAPIListsImplementedActionPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil, fakeUserLister{}, fakeKeyLister{}, fakeProviderLister{})
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/openapi.json", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	wantPaths := []string{
		"/api/actions/users",
		"/api/actions/users/{id}",
		"/api/actions/keys",
		"/api/actions/providers",
		"/api/actions/system-settings",
		"/api/actions/usage-logs",
		"/api/actions/usage-logs/summary",
		"/api/actions/usage-logs/filter-options",
		"/api/actions/usage-logs/session-id-suggestions",
		"/api/actions/session-origin-chain",
	}
	for _, want := range wantPaths {
		if !strings.Contains(resp.Body.String(), want) {
			t.Fatalf("expected openapi body to contain %q, got %s", want, resp.Body.String())
		}
	}
}

func TestAdminSystemSettingsActionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	router := gin.New()
	NewSystemSettingsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	getReq := httptest.NewRequest(http.MethodGet, "/api/actions/system-settings", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}
	if !strings.Contains(getResp.Body.String(), "\"ok\":true") {
		t.Fatalf("expected action envelope, got %s", getResp.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/actions/system-settings", strings.NewReader(`{"siteTitle":"CCH Action","enableHttp2":true}`))
	putReq.Header.Set("Authorization", "Bearer admin-token")
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if !strings.Contains(putResp.Body.String(), "CCH Action") {
		t.Fatalf("expected updated settings payload, got %s", putResp.Body.String())
	}
}

func TestAdminCreateRoutesReturnCreatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeUserLister{},
		fakeKeyLister{},
		fakeProviderLister{},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{name: "create user", path: "/api/actions/users", body: `{"name":"bob"}`, wantStatus: http.StatusCreated, wantContains: "bob"},
		{name: "create key", path: "/api/actions/keys", body: `{"userId":1,"key":"sk-123","name":"key-a"}`, wantStatus: http.StatusCreated, wantContains: "key-a"},
		{name: "create provider", path: "/api/actions/providers", body: `{"name":"provider-a","url":"https://example.com","key":"pk-123"}`, wantStatus: http.StatusCreated, wantContains: "provider-a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestAdminUpdateAndDeleteRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeUserLister{users: []*model.User{{ID: 1, Name: "alice"}}},
		fakeKeyLister{keys: []*model.Key{{ID: 1, Name: "key-1"}}},
		fakeProviderLister{providers: []*model.Provider{{ID: 1, Name: "provider-1"}}},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{name: "update user", method: http.MethodPut, path: "/api/actions/users/1", body: `{"name":"alice-updated"}`, wantStatus: http.StatusOK, wantContains: "alice-updated"},
		{name: "update key", method: http.MethodPut, path: "/api/actions/keys/1", body: `{"name":"key-updated"}`, wantStatus: http.StatusOK, wantContains: "key-updated"},
		{name: "update provider", method: http.MethodPut, path: "/api/actions/providers/1", body: `{"name":"provider-updated"}`, wantStatus: http.StatusOK, wantContains: "provider-updated"},
		{name: "delete user", method: http.MethodDelete, path: "/api/actions/users/1", wantStatus: http.StatusOK, wantContains: "\"deleted\":true"},
		{name: "delete key", method: http.MethodDelete, path: "/api/actions/keys/1", wantStatus: http.StatusOK, wantContains: "\"deleted\":true"},
		{name: "delete provider", method: http.MethodDelete, path: "/api/actions/providers/1", wantStatus: http.StatusOK, wantContains: "\"deleted\":true"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}
