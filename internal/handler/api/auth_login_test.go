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

type fakeSessionRevoker struct {
	created []struct {
		apiKey string
		userID int
		role   string
	}
	createID  string
	createErr error
	revoked   []string
	err       error
}

func (f *fakeSessionRevoker) Create(_ context.Context, apiKey string, userID int, userRole string) (string, error) {
	f.created = append(f.created, struct {
		apiKey string
		userID int
		role   string
	}{apiKey: apiKey, userID: userID, role: userRole})
	return f.createID, f.createErr
}

func (f *fakeSessionRevoker) Revoke(_ context.Context, sessionID string) error {
	f.revoked = append(f.revoked, sessionID)
	return f.err
}

func TestAuthLoginAndLogoutRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	description := "dashboard user"
	oldSecureCookies := os.Getenv("ENABLE_SECURE_COOKIES")
	oldSessionMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("ENABLE_SECURE_COOKIES", oldSecureCookies) })
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldSessionMode) })
	_ = os.Setenv("ENABLE_SECURE_COOKIES", "true")
	_ = os.Setenv("SESSION_TOKEN_MODE", "legacy")
	router := gin.New()
	sessionRevoker := &fakeSessionRevoker{}

	NewAuthHandler(fakeLoginAuth{
		adminToken: "admin-token",
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 11, Name: "alice", Description: &description, Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 1, Key: "sk-user", Name: "User Key", CanLoginWebUi: &enabled},
			APIKey:  "sk-user",
		},
	}, sessionRevoker).RegisterRoutes(router)

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
	if !strings.Contains(strings.Join(loginResp.Result().Header.Values("Set-Cookie"), ";"), "Secure") {
		t.Fatalf("expected secure auth cookie when ENABLE_SECURE_COOKIES=true, got %+v", loginResp.Result().Header.Values("Set-Cookie"))
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
	logoutReq.AddCookie(&http.Cookie{Name: authCookieName, Value: "opaque-session-token"})
	logoutResp := httptest.NewRecorder()
	router.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK || !strings.Contains(logoutResp.Body.String(), `"ok":true`) {
		t.Fatalf("expected logout ok payload, got %d: %s", logoutResp.Code, logoutResp.Body.String())
	}
	if !strings.Contains(strings.Join(logoutResp.Result().Header.Values("Set-Cookie"), ";"), "Secure") {
		t.Fatalf("expected secure logout cookie when ENABLE_SECURE_COOKIES=true, got %+v", logoutResp.Result().Header.Values("Set-Cookie"))
	}
	if got := logoutResp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected no-cache pragma, got %q", got)
	}
	if len(sessionRevoker.revoked) != 0 {
		t.Fatalf("expected legacy mode logout not to revoke session, got %+v", sessionRevoker.revoked)
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

func TestAuthLogoutRevokesOpaqueSessionInNonLegacyModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldMode) })
	_ = os.Setenv("SESSION_TOKEN_MODE", "dual")

	router := gin.New()
	sessionRevoker := &fakeSessionRevoker{}
	NewAuthHandler(fakeLoginAuth{}, sessionRevoker).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "sid_test_123"})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(sessionRevoker.revoked) != 1 || sessionRevoker.revoked[0] != "sid_test_123" {
		t.Fatalf("expected opaque session revocation, got %+v", sessionRevoker.revoked)
	}
}

func TestAuthLoginCreatesOpaqueSessionInDualModeButKeepsLegacyCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldMode) })
	_ = os.Setenv("SESSION_TOKEN_MODE", "dual")

	enabled := true
	router := gin.New()
	sessionRevoker := &fakeSessionRevoker{createID: "sid_dual_123"}
	NewAuthHandler(fakeLoginAuth{
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 5, Name: "dual-user", Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 5, Key: "sk-user", Name: "Dual Key", CanLoginWebUi: &enabled},
			APIKey:  "sk-user",
		},
	}, sessionRevoker).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(sessionRevoker.created) != 1 || sessionRevoker.created[0].apiKey != "sk-user" || sessionRevoker.created[0].userID != 5 || sessionRevoker.created[0].role != "user" {
		t.Fatalf("expected opaque session creation in dual mode, got %+v", sessionRevoker.created)
	}
	if !strings.Contains(strings.Join(resp.Result().Header.Values("Set-Cookie"), ";"), authCookieName+"=sk-user") {
		t.Fatalf("expected legacy cookie to remain in dual mode, got %+v", resp.Result().Header.Values("Set-Cookie"))
	}
}

func TestAuthLoginUsesOpaqueSessionCookieInOpaqueMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldMode) })
	_ = os.Setenv("SESSION_TOKEN_MODE", "opaque")

	enabled := true
	router := gin.New()
	sessionRevoker := &fakeSessionRevoker{createID: "sid_opaque_123"}
	NewAuthHandler(fakeLoginAuth{
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 6, Name: "opaque-user", Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 6, Key: "sk-user", Name: "Opaque Key", CanLoginWebUi: &enabled},
			APIKey:  "sk-user",
		},
	}, sessionRevoker).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(strings.Join(resp.Result().Header.Values("Set-Cookie"), ";"), authCookieName+"=sid_opaque_123") {
		t.Fatalf("expected opaque session cookie, got %+v", resp.Result().Header.Values("Set-Cookie"))
	}
}

func TestAuthLoginFailsWhenOpaqueSessionCreationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldMode) })
	_ = os.Setenv("SESSION_TOKEN_MODE", "opaque")

	enabled := true
	router := gin.New()
	sessionRevoker := &fakeSessionRevoker{createErr: errors.New("redis down")}
	NewAuthHandler(fakeLoginAuth{
		proxyToken: "sk-user",
		proxyResult: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 7, Name: "opaque-user", Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 7, Key: "sk-user", Name: "Opaque Key", CanLoginWebUi: &enabled},
			APIKey:  "sk-user",
		},
	}, sessionRevoker).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"sk-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"errorCode":"SESSION_CREATE_FAILED"`) {
		t.Fatalf("expected SESSION_CREATE_FAILED error code, got %s", resp.Body.String())
	}
}
