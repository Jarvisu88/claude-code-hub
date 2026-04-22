package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestActionStyleAliasRoutes(t *testing.T) {
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
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{name: "getUsers alias", path: "/api/actions/users/getUsers", body: `{}`, wantStatus: http.StatusOK, wantContains: "alice"},
		{name: "getUser alias", path: "/api/actions/users/getUser", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: "alice"},
		{name: "addUser alias", path: "/api/actions/users/addUser", body: `{"name":"bob"}`, wantStatus: http.StatusCreated, wantContains: "bob"},
		{name: "editUser alias", path: "/api/actions/users/editUser", body: `{"id":1,"name":"bob2"}`, wantStatus: http.StatusOK, wantContains: "bob2"},
		{name: "removeUser alias", path: "/api/actions/users/removeUser", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: "\"deleted\":true"},
		{name: "getKeys alias", path: "/api/actions/keys/getKeys", body: `{}`, wantStatus: http.StatusOK, wantContains: "key-1"},
		{name: "getProviders alias", path: "/api/actions/providers/getProviders", body: `{}`, wantStatus: http.StatusOK, wantContains: "provider-1"},
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
