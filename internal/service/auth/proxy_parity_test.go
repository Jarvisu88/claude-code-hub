package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

type proxyAuthFixture struct {
	Source string                 `json:"source"`
	Notes  []string               `json:"notes"`
	Cases  []proxyAuthFixtureCase `json:"cases"`
}

type proxyAuthFixtureCase struct {
	Name             string       `json:"name"`
	Input            fixtureInput `json:"input"`
	WantKey          string       `json:"want_key"`
	WantErrorCode    string       `json:"want_error_code"`
	WantErrorMessage string       `json:"want_error_message"`
}

type fixtureInput struct {
	Authorization  string `json:"authorization"`
	APIKey         string `json:"x_api_key"`
	GeminiAPIKey   string `json:"x_goog_api_key"`
	GeminiQueryKey string `json:"gemini_query_key"`
}

func TestResolveSingleProxyAPIKeyParityFixture(t *testing.T) {
	fixture := loadProxyAuthFixture(t)
	t.Logf("node parity source: %s", fixture.Source)
	for _, note := range fixture.Notes {
		t.Logf("fixture note: %s", note)
	}

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			key, err := resolveSingleProxyAPIKey(ProxyAuthInput{
				AuthorizationHeader: tc.Input.Authorization,
				APIKeyHeader:        tc.Input.APIKey,
				GeminiAPIKeyHeader:  tc.Input.GeminiAPIKey,
				GeminiAPIKeyQuery:   tc.Input.GeminiQueryKey,
			})

			if tc.WantErrorCode != "" {
				if err == nil {
					t.Fatalf("expected error code %s, got nil", tc.WantErrorCode)
				}
				if !appErrors.IsCode(err, appErrors.ErrorCode(tc.WantErrorCode)) {
					t.Fatalf("expected error code %s, got %v", tc.WantErrorCode, err)
				}
				if tc.WantErrorMessage != "" && err.Error() != "authentication_error: "+tc.WantErrorMessage {
					t.Fatalf("expected error message %q, got %q", tc.WantErrorMessage, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if key != tc.WantKey {
				t.Fatalf("expected key %q, got %q", tc.WantKey, key)
			}
		})
	}
}

func TestAuthenticateProxySupportsGeminiQueryKey(t *testing.T) {
	svc := NewService(
		&stubKeyRepo{key: buildActiveKey("gemini-query-key")},
		&stubUserRepo{},
		"admin-token",
	)

	result, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		GeminiAPIKeyQuery: "gemini-query-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.APIKey != "gemini-query-key" {
		t.Fatalf("expected query key auth result, got %+v", result)
	}
}

func TestAuthenticateProxyAllowsSameCredentialAcrossHeaders(t *testing.T) {
	svc := NewService(
		&stubKeyRepo{key: buildActiveKey("shared-key")},
		&stubUserRepo{},
		"admin-token",
	)

	result, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		AuthorizationHeader: "Bearer shared-key",
		APIKeyHeader:        "shared-key",
		GeminiAPIKeyHeader:  "shared-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.APIKey != "shared-key" {
		t.Fatalf("expected shared key auth result, got %+v", result)
	}
}

func TestAuthenticateProxyRejectsDisabledKey(t *testing.T) {
	key := buildActiveKey("disabled-key")
	disabled := false
	key.IsEnabled = &disabled

	svc := NewService(&stubKeyRepo{key: key}, &stubUserRepo{}, "")
	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{APIKeyHeader: "disabled-key"})
	if err == nil {
		t.Fatal("expected disabled key error")
	}
	if !appErrors.IsCode(err, appErrors.CodeDisabledAPIKey) {
		t.Fatalf("expected disabled_api_key, got %v", err)
	}
}

func TestAuthenticateProxyRejectsExpiredKey(t *testing.T) {
	key := buildActiveKey("expired-key")
	expiredAt := time.Now().Add(-time.Hour)
	key.ExpiresAt = &expiredAt

	svc := NewService(&stubKeyRepo{key: key}, &stubUserRepo{}, "")
	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{APIKeyHeader: "expired-key"})
	if err == nil {
		t.Fatal("expected expired key error")
	}
	if !appErrors.IsCode(err, appErrors.CodeExpiredAPIKey) {
		t.Fatalf("expected expired_api_key, got %v", err)
	}
}

func TestAuthenticateAdminTokenRejectsMissingToken(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "admin-token")

	_, err := svc.AuthenticateAdminToken("   ")
	if err == nil {
		t.Fatal("expected missing admin token error")
	}
	if !appErrors.IsCode(err, appErrors.CodeTokenRequired) {
		t.Fatalf("expected token_required, got %v", err)
	}
}

func TestAuthenticateAdminTokenRejectsMissingConfig(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "")

	_, err := svc.AuthenticateAdminToken("admin-token")
	if err == nil {
		t.Fatal("expected unauthorized error when admin token is not configured")
	}
	if !appErrors.IsCode(err, appErrors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func loadProxyAuthFixture(t *testing.T) proxyAuthFixture {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current test file")
	}

	fixturePath := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../../tests/go-parity/fixtures/auth/proxy_auth_inputs.json"))
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", fixturePath, err)
	}

	var fixture proxyAuthFixture
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatalf("failed to decode fixture %s: %v", fixturePath, err)
	}

	return fixture
}
