package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
	description := "dashboard user"
	oldSecureCookies := os.Getenv("ENABLE_SECURE_COOKIES")
	t.Cleanup(func() { _ = os.Setenv("ENABLE_SECURE_COOKIES", oldSecureCookies) })
	_ = os.Setenv("ENABLE_SECURE_COOKIES", "true")
	router := gin.New()

	NewAuthHandler(fakeLoginAuth{
		adminToken: "admin-token",
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 11, Name: "alice", Description: &description, Role: "user", IsEnabled: &enabled},
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
	if !strings.Contains(loginResp.Body.String(), `"redirectTo":"/dashboard"`) || !strings.Contains(loginResp.Body.String(), `"loginType":"dashboard_user"`) || !strings.Contains(loginResp.Body.String(), `"user":{"description":"dashboard user","id":11,"name":"alice","role":"user"}`) {
		t.Fatalf("expected dashboard login payload, got %s", loginResp.Body.String())
	}
	if !strings.Contains(strings.Join(loginResp.Result().Header.Values("Set-Cookie"), ";"), authCookieName+"=sk-user") {
		t.Fatalf("expected auth cookie to be set, got %+v", loginResp.Result().Header.Values("Set-Cookie"))
	}
	if got := loginResp.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}
	if got := loginResp.Header().Get("Content-Security-Policy-Report-Only"); got != authCSPReportOnlyValue {
		t.Fatalf("expected auth CSP report-only header, got %q", got)
	}
	if got := loginResp.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("expected HSTS header, got %q", got)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutResp := httptest.NewRecorder()
	router.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK || !strings.Contains(logoutResp.Body.String(), `"ok":true`) {
		t.Fatalf("expected logout ok payload, got %d: %s", logoutResp.Code, logoutResp.Body.String())
	}
	if got := logoutResp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected no-cache pragma, got %q", got)
	}
}

func TestAuthLoginSupportsAdminAndReadonlyUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	disabledWebUI := false
	enabled := true

	NewAuthHandler(fakeLoginAuth{
		adminToken:  "admin-token",
		proxyToken:  "sk-read",
		adminResult: &authsvc.AuthResult{IsAdmin: true, APIKey: "admin-token", User: &model.User{ID: -1, Name: "Admin Token", Role: "admin", IsEnabled: &enabled}},
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 2, Name: "bob", Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 2, Key: "sk-read", Name: "Read Key", CanLoginWebUi: &disabledWebUI},
			APIKey:  "sk-read",
		},
	}).RegisterRoutes(router)

	adminReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"admin-token"}`))
	adminReq.Header.Set("Content-Type", "application/json")
	adminResp := httptest.NewRecorder()
	router.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK || !strings.Contains(adminResp.Body.String(), `"loginType":"admin"`) || !strings.Contains(adminResp.Body.String(), `"redirectTo":"/dashboard"`) || !strings.Contains(adminResp.Body.String(), `"user":{"description":null,"id":-1,"name":"Admin Token","role":"admin"}`) {
		t.Fatalf("expected admin login payload, got %d: %s", adminResp.Code, adminResp.Body.String())
	}

	readonlyReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-read"}`))
	readonlyReq.Header.Set("Content-Type", "application/json")
	readonlyResp := httptest.NewRecorder()
	router.ServeHTTP(readonlyResp, readonlyReq)
	if readonlyResp.Code != http.StatusOK || !strings.Contains(readonlyResp.Body.String(), `"loginType":"readonly_user"`) || !strings.Contains(readonlyResp.Body.String(), `"redirectTo":"/my-usage"`) || !strings.Contains(readonlyResp.Body.String(), `"user":{"description":null,"id":2,"name":"bob","role":"user"}`) {
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
	if !strings.Contains(resp.Body.String(), `"errorCode":"KEY_INVALID"`) {
		t.Fatalf("expected KEY_INVALID error code, got %s", resp.Body.String())
	}
	if got := resp.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected auth security headers on failure response, got %q", got)
	}
}

func TestAuthLoginRequiresKeyWithErrorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAuthHandler(fakeLoginAuth{}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"errorCode":"KEY_REQUIRED"`) {
		t.Fatalf("expected KEY_REQUIRED error code, got %s", resp.Body.String())
	}
}

func TestAuthLoginRejectsNilAuthResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAuthHandler(fakeLoginAuth{
		adminToken:  "admin-token",
		proxyToken:  "sk-user",
		adminResult: nil,
		adminErr:    nil,
		proxyResult: nil,
		proxyErr:    nil,
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"errorCode":"KEY_INVALID"`) {
		t.Fatalf("expected KEY_INVALID error code, got %s", resp.Body.String())
	}
}

func TestAuthLoginReturnsServerErrorForInternalAuthFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAuthHandler(fakeLoginAuth{
		adminToken: "admin-token",
		proxyToken: "sk-user",
		adminErr:   appErrors.NewAuthenticationError("bad token", appErrors.CodeInvalidToken),
		proxyErr:   errors.New("db unavailable"),
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"errorCode":"SERVER_ERROR"`) {
		t.Fatalf("expected SERVER_ERROR error code, got %s", resp.Body.String())
	}
}
