package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

type stubKeyRepo struct {
	key        *model.Key
	keysByUser map[int][]*model.Key
	err        error
}

func (s *stubKeyRepo) GetByKeyWithUser(_ context.Context, _ string) (*model.Key, error) {
	return s.key, s.err
}

func (s *stubKeyRepo) ListByUserID(_ context.Context, userID int) ([]*model.Key, error) {
	if s.keysByUser == nil {
		return nil, nil
	}
	return s.keysByUser[userID], nil
}

type stubSessionReader struct {
	session *SessionTokenData
	err     error
}

func (s stubSessionReader) Read(_ context.Context, _ string) (*SessionTokenData, error) {
	return s.session, s.err
}

type stubUserRepo struct {
	markedUserIDs []int
	err           error
}

func (s *stubUserRepo) MarkUserExpired(_ context.Context, userID int) (bool, error) {
	s.markedUserIDs = append(s.markedUserIDs, userID)
	if s.err != nil {
		return false, s.err
	}
	return true, nil
}

func TestAuthenticateProxySuccess(t *testing.T) {
	svc := NewService(
		&stubKeyRepo{
			key: buildActiveKey("proxy-key"),
		},
		&stubUserRepo{},
		"admin-token",
	)

	result, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		AuthorizationHeader: "Bearer proxy-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.User == nil || result.Key == nil {
		t.Fatalf("expected auth result with user and key")
	}
	if result.IsAdmin {
		t.Fatalf("proxy auth must not mark result as admin")
	}
	if result.APIKey != "proxy-key" {
		t.Fatalf("expected api key to be preserved, got %q", result.APIKey)
	}
}

func TestAuthenticateProxyRejectsConflictingCredentials(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "")

	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		AuthorizationHeader: "Bearer key-a",
		APIKeyHeader:        "key-b",
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !appErrors.IsCode(err, appErrors.CodeInvalidCredentials) {
		t.Fatalf("expected invalid_credentials, got %v", err)
	}
}

func TestAuthenticateProxyRejectsMissingCredentials(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "")

	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{})
	if err == nil {
		t.Fatal("expected missing credential error")
	}
	if !appErrors.IsCode(err, appErrors.CodeTokenRequired) {
		t.Fatalf("expected token_required, got %v", err)
	}
}

func TestAuthenticateProxyRejectsInvalidAPIKey(t *testing.T) {
	svc := NewService(&stubKeyRepo{err: appErrors.NewNotFoundError("Key")}, &stubUserRepo{}, "")

	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		APIKeyHeader: "missing-key",
	})
	if err == nil {
		t.Fatal("expected invalid key error")
	}
	if !appErrors.IsCode(err, appErrors.CodeInvalidAPIKey) {
		t.Fatalf("expected invalid_api_key, got %v", err)
	}
}

func TestAuthenticateProxyRejectsDisabledUser(t *testing.T) {
	key := buildActiveKey("proxy-key")
	disabled := false
	key.User.IsEnabled = &disabled

	svc := NewService(&stubKeyRepo{key: key}, &stubUserRepo{}, "")
	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		APIKeyHeader: "proxy-key",
	})
	if err == nil {
		t.Fatal("expected disabled user error")
	}
	if !appErrors.IsCode(err, appErrors.CodeDisabledUser) {
		t.Fatalf("expected user_disabled, got %v", err)
	}
}

func TestAuthenticateProxyMarksExpiredUser(t *testing.T) {
	key := buildActiveKey("proxy-key")
	expiredAt := time.Now().Add(-time.Hour)
	key.User.ExpiresAt = &expiredAt
	userRepo := &stubUserRepo{}

	svc := NewService(&stubKeyRepo{key: key}, userRepo, "")
	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		APIKeyHeader: "proxy-key",
	})
	if err == nil {
		t.Fatal("expected expired user error")
	}
	if !appErrors.IsCode(err, appErrors.CodeUserExpired) {
		t.Fatalf("expected user_expired, got %v", err)
	}
	if got := err.Error(); got == "" || !containsSubstring(got, "过期") || !containsSubstring(got, expiredAt.UTC().Format("2006-01-02")) {
		t.Fatalf("expected expired message to include UTC date, got %q", got)
	}
	if len(userRepo.markedUserIDs) != 1 || userRepo.markedUserIDs[0] != key.User.ID {
		t.Fatalf("expected expired user to be marked once, got %+v", userRepo.markedUserIDs)
	}
}

func TestAuthenticateAdminTokenSuccess(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "admin-token")
	fixedNow := time.Date(2026, 4, 21, 14, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	result, err := svc.AuthenticateAdminToken("admin-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || !result.IsAdmin {
		t.Fatalf("expected admin auth result")
	}
	if result.User == nil || !result.User.IsAdmin() {
		t.Fatalf("expected synthetic admin user")
	}
	if result.Key == nil || result.Key.Name != "ADMIN_TOKEN" {
		t.Fatalf("expected synthetic admin key")
	}
	if !result.Key.CreatedAt.Equal(fixedNow) {
		t.Fatalf("expected fixed timestamp, got %v", result.Key.CreatedAt)
	}
}

func TestAuthenticateAdminTokenRejectsInvalidToken(t *testing.T) {
	svc := NewService(&stubKeyRepo{}, &stubUserRepo{}, "admin-token")

	_, err := svc.AuthenticateAdminToken("wrong-token")
	if err == nil {
		t.Fatal("expected invalid admin token error")
	}
	if !appErrors.IsCode(err, appErrors.CodeInvalidToken) {
		t.Fatalf("expected invalid_token, got %v", err)
	}
}

func TestAuthenticateAdminTokenAcceptsOpaqueAdminSession(t *testing.T) {
	svc := NewService(
		&stubKeyRepo{},
		&stubUserRepo{},
		"admin-token",
		stubSessionReader{session: &SessionTokenData{
			SessionID:      "sid_admin_123",
			KeyFingerprint: keyFingerprint("admin-token"),
			UserID:         -1,
			UserRole:       "admin",
			CreatedAt:      time.Now().Add(-time.Minute).UnixMilli(),
			ExpiresAt:      time.Now().Add(time.Hour).UnixMilli(),
		}},
	)
	t.Setenv("SESSION_TOKEN_MODE", "opaque")

	result, err := svc.AuthenticateAdminToken("sid_admin_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || !result.IsAdmin || result.User == nil || result.User.Role != "admin" {
		t.Fatalf("expected opaque admin session auth result, got %+v", result)
	}
}

func TestAuthenticateProxyAllowsOpaqueSessionWhenExplicitlyEnabled(t *testing.T) {
	key := buildActiveKey("proxy-key")
	svc := NewService(
		&stubKeyRepo{
			key: key,
			keysByUser: map[int][]*model.Key{
				key.User.ID: {key},
			},
		},
		&stubUserRepo{},
		"admin-token",
		stubSessionReader{session: &SessionTokenData{
			SessionID:      "sid_user_123",
			KeyFingerprint: keyFingerprint("proxy-key"),
			UserID:         key.User.ID,
			UserRole:       key.User.Role,
			CreatedAt:      time.Now().Add(-time.Minute).UnixMilli(),
			ExpiresAt:      time.Now().Add(time.Hour).UnixMilli(),
		}},
	)
	t.Setenv("SESSION_TOKEN_MODE", "opaque")

	result, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		APIKeyHeader:      "sid_user_123",
		AllowSessionToken: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.User == nil || result.User.ID != key.User.ID || result.APIKey != "proxy-key" {
		t.Fatalf("expected opaque session to resolve underlying key auth, got %+v", result)
	}
}

func TestAuthenticateProxyDoesNotAcceptOpaqueSessionByDefault(t *testing.T) {
	svc := NewService(
		&stubKeyRepo{err: appErrors.NewNotFoundError("Key")},
		&stubUserRepo{},
		"admin-token",
		stubSessionReader{session: &SessionTokenData{
			SessionID:      "sid_user_123",
			KeyFingerprint: keyFingerprint("proxy-key"),
			UserID:         10,
			UserRole:       "user",
			CreatedAt:      time.Now().Add(-time.Minute).UnixMilli(),
			ExpiresAt:      time.Now().Add(time.Hour).UnixMilli(),
		}},
	)
	t.Setenv("SESSION_TOKEN_MODE", "opaque")

	_, err := svc.AuthenticateProxy(context.Background(), ProxyAuthInput{
		APIKeyHeader: "sid_user_123",
	})
	if err == nil {
		t.Fatal("expected opaque session token to be rejected without explicit opt-in")
	}
	if !appErrors.IsCode(err, appErrors.CodeInvalidAPIKey) {
		t.Fatalf("expected invalid_api_key, got %v", err)
	}
}

func buildActiveKey(rawKey string) *model.Key {
	enabled := true
	canLogin := true
	return &model.Key{
		ID:            1,
		UserID:        10,
		Key:           rawKey,
		Name:          "key-1",
		IsEnabled:     &enabled,
		CanLoginWebUi: &canLogin,
		User: &model.User{
			ID:        10,
			Name:      "tester",
			Role:      "user",
			IsEnabled: &enabled,
		},
	}
}

func containsSubstring(value, sub string) bool {
	return len(sub) == 0 || (len(value) >= len(sub) && strings.Contains(value, sub))
}
