package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeLoginAuth struct {
	adminToken  string
	proxyToken  string
	adminResult *authsvc.AuthResult
	adminErr    error
	proxyResult *authsvc.AuthResult
	proxyErr    error
}

func (f fakeLoginAuth) AuthenticateAdminToken(token string) (*authsvc.AuthResult, error) {
	if f.adminToken != "" && token != f.adminToken {
		return nil, f.adminErr
	}
	return f.adminResult, f.adminErr
}

func (f fakeLoginAuth) AuthenticateProxy(_ context.Context, input authsvc.ProxyAuthInput) (*authsvc.AuthResult, error) {
	if f.proxyToken != "" && input.APIKeyHeader != f.proxyToken {
		return nil, f.proxyErr
	}
	return f.proxyResult, f.proxyErr
}

func TestAuthLoginAndLogoutRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	NewAuthHandler(fakeLoginAuth{
		adminToken: "admin-token",
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    nil,
			Key:     &model.Key{ID: 1, Key: "sk-user", Name: "User Key", CanLoginWebUi: &enabled},
			APIKey:  "sk-user",
		},
	}).RegisterRoutes(router)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", loginResp.Code, loginResp.Body.String())
	}
	if !strings.Contains(loginResp.Body.String(), `"redirectTo":"/dashboard"`) || !strings.Contains(loginResp.Body.String(), `"loginType":"dashboard_user"`) {
		t.Fatalf("expected dashboard login payload, got %s", loginResp.Body.String())
	}
	if !strings.Contains(strings.Join(loginResp.Result().Header.Values("Set-Cookie"), ";"), authCookieName+"=sk-user") {
		t.Fatalf("expected auth cookie to be set, got %+v", loginResp.Result().Header.Values("Set-Cookie"))
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutResp := httptest.NewRecorder()
	router.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK || !strings.Contains(logoutResp.Body.String(), `"ok":true`) {
		t.Fatalf("expected logout ok payload, got %d: %s", logoutResp.Code, logoutResp.Body.String())
	}
}

func TestAuthLoginSupportsAdminAndReadonlyUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	disabledWebUI := false

	NewAuthHandler(fakeLoginAuth{
		adminToken:  "admin-token",
		proxyToken:  "sk-read",
		adminResult: &authsvc.AuthResult{IsAdmin: true, APIKey: "admin-token"},
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			Key:     &model.Key{ID: 2, Key: "sk-read", Name: "Read Key", CanLoginWebUi: &disabledWebUI},
			APIKey:  "sk-read",
		},
	}).RegisterRoutes(router)

	adminReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"admin-token"}`))
	adminReq.Header.Set("Content-Type", "application/json")
	adminResp := httptest.NewRecorder()
	router.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK || !strings.Contains(adminResp.Body.String(), `"loginType":"admin"`) || !strings.Contains(adminResp.Body.String(), `"redirectTo":"/dashboard"`) {
		t.Fatalf("expected admin login payload, got %d: %s", adminResp.Code, adminResp.Body.String())
	}

	readonlyReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-read"}`))
	readonlyReq.Header.Set("Content-Type", "application/json")
	readonlyResp := httptest.NewRecorder()
	router.ServeHTTP(readonlyResp, readonlyReq)
	if readonlyResp.Code != http.StatusOK || !strings.Contains(readonlyResp.Body.String(), `"loginType":"readonly_user"`) || !strings.Contains(readonlyResp.Body.String(), `"redirectTo":"/my-usage"`) {
		t.Fatalf("expected readonly login payload, got %d: %s", readonlyResp.Code, readonlyResp.Body.String())
	}
}

func TestAuthLoginRejectsInvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAuthHandler(fakeLoginAuth{
		adminToken: "admin-token",
		proxyToken: "sk-valid",
		adminErr:   appErrors.NewAuthenticationError("bad token", appErrors.CodeInvalidToken),
		proxyErr:   appErrors.NewAuthenticationError("bad key", appErrors.CodeInvalidAPIKey),
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", resp.Code, resp.Body.String())
	}
}
