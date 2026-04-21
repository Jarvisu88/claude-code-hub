package auth

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
)

type proxyKeyLookup interface {
	GetByKeyWithUser(ctx context.Context, key string) (*model.Key, error)
}

type userExpiryMarker interface {
	MarkUserExpired(ctx context.Context, userID int) (bool, error)
}

// Service 封装 API Key 与管理员令牌鉴权逻辑。
// 当前阶段只处理最核心的认证语义，不耦合限流、会话和 provider 选择。
type Service struct {
	keyRepo    proxyKeyLookup
	userRepo   userExpiryMarker
	adminToken string
	now        func() time.Time
}

// ProxyAuthInput 统一承载 /v1 代理链可接受的凭据输入。
type ProxyAuthInput struct {
	AuthorizationHeader string
	APIKeyHeader        string
	GeminiAPIKeyHeader  string
	GeminiAPIKeyQuery   string
}

// AuthResult 返回当前请求被鉴权后的 principal。
type AuthResult struct {
	User    *model.User
	Key     *model.Key
	APIKey  string
	IsAdmin bool
}

func NewService(keyRepo proxyKeyLookup, userRepo userExpiryMarker, adminToken string) *Service {
	return &Service{
		keyRepo:    keyRepo,
		userRepo:   userRepo,
		adminToken: adminToken,
		now:        time.Now,
	}
}

func NewServiceFromFactory(factory *repository.Factory, adminToken string) *Service {
	return NewService(factory.Key(), factory.User(), adminToken)
}

// AuthenticateProxy 校验 /v1 代理请求。
// 语义对齐当前 Node 版本：
// - 支持 Authorization / x-api-key / x-goog-api-key / Gemini query key
// - 多凭据冲突时报错
// - 仅接受数据库中的 API Key，不接受管理员令牌
func (s *Service) AuthenticateProxy(ctx context.Context, input ProxyAuthInput) (*AuthResult, error) {
	apiKey, err := resolveSingleProxyAPIKey(input)
	if err != nil {
		return nil, err
	}

	key, err := s.keyRepo.GetByKeyWithUser(ctx, apiKey)
	if err != nil {
		if appErrors.IsCode(err, appErrors.CodeNotFound) {
			return nil, appErrors.NewAuthenticationError(
				"API 密钥无效。提供的密钥不存在、已被删除、已被禁用或已过期。",
				appErrors.CodeInvalidAPIKey,
			)
		}
		return nil, err
	}

	if key == nil || key.User == nil {
		return nil, appErrors.NewInternalError("认证结果缺少关联用户")
	}

	if !key.IsActive() {
		return nil, mapInactiveKeyError(key)
	}

	user := key.User
	if !userEnabled(user) {
		return nil, appErrors.NewAuthenticationError("用户账户已被禁用。请联系管理员。", appErrors.CodeDisabledUser)
	}

	if user.IsExpired() {
		if s.userRepo != nil {
			_, _ = s.userRepo.MarkUserExpired(ctx, user.ID)
		}
		expiredOn := user.ExpiresAt.UTC().Format("2006-01-02")
		return nil, appErrors.NewAuthenticationError("用户账户已于 "+expiredOn+" 过期。请续费订阅。", appErrors.CodeUserExpired)
	}

	return &AuthResult{
		User:    user,
		Key:     key,
		APIKey:  apiKey,
		IsAdmin: false,
	}, nil
}

// AuthenticateAdminToken 仅处理管理面管理员令牌语义。
// 注意：不要在 /v1 代理链中调用它替代 API Key 鉴权。
func (s *Service) AuthenticateAdminToken(token string) (*AuthResult, error) {
	normalizedToken := strings.TrimSpace(token)
	normalizedAdminToken := strings.TrimSpace(s.adminToken)

	if normalizedToken == "" {
		return nil, appErrors.NewAuthenticationError("未提供管理员令牌。", appErrors.CodeTokenRequired)
	}
	if normalizedAdminToken == "" {
		return nil, appErrors.NewAuthenticationError("管理员令牌未配置。", appErrors.CodeUnauthorized)
	}
	if subtle.ConstantTimeCompare([]byte(normalizedToken), []byte(normalizedAdminToken)) != 1 {
		return nil, appErrors.NewAuthenticationError("管理员令牌无效。", appErrors.CodeInvalidToken)
	}

	now := s.now()
	enabled := true
	canLogin := true

	user := &model.User{
		ID:        -1,
		Name:      "Admin Token",
		Role:      "admin",
		IsEnabled: &enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := &model.Key{
		ID:            -1,
		UserID:        user.ID,
		Key:           normalizedToken,
		Name:          "ADMIN_TOKEN",
		IsEnabled:     &enabled,
		CanLoginWebUi: &canLogin,
		CreatedAt:     now,
		UpdatedAt:     now,
		User:          user,
	}

	return &AuthResult{
		User:    user,
		Key:     key,
		APIKey:  normalizedToken,
		IsAdmin: true,
	}, nil
}

func resolveSingleProxyAPIKey(input ProxyAuthInput) (string, error) {
	bearerKey := extractBearerToken(input.AuthorizationHeader)
	apiKeyHeader := normalizeKey(input.APIKeyHeader)
	geminiKey := firstNonEmpty(
		normalizeKey(input.GeminiAPIKeyHeader),
		normalizeKey(input.GeminiAPIKeyQuery),
	)

	providedKeys := make([]string, 0, 3)
	for _, key := range []string{bearerKey, apiKeyHeader, geminiKey} {
		if key != "" {
			providedKeys = append(providedKeys, key)
		}
	}

	if len(providedKeys) == 0 {
		return "", appErrors.NewAuthenticationError(
			"未提供认证凭据。请在 Authorization 头部、x-api-key 头部或 x-goog-api-key 头部中包含 API 密钥。",
			appErrors.CodeTokenRequired,
		)
	}

	firstKey := providedKeys[0]
	for _, key := range providedKeys[1:] {
		if key != firstKey {
			return "", appErrors.NewAuthenticationError(
				"提供了多个冲突的 API 密钥。请仅使用一种认证方式。",
				appErrors.CodeInvalidCredentials,
			)
		}
	}

	return firstKey, nil
}

func extractBearerToken(header string) string {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return ""
	}
	parts := strings.Fields(trimmed)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func normalizeKey(value string) string {
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func userEnabled(user *model.User) bool {
	if user == nil {
		return false
	}
	if user.IsEnabled == nil {
		return true
	}
	return *user.IsEnabled
}

func mapInactiveKeyError(key *model.Key) error {
	if key == nil {
		return appErrors.NewAuthenticationError("API 密钥无效。", appErrors.CodeInvalidAPIKey)
	}
	if key.IsEnabled != nil && !*key.IsEnabled {
		return appErrors.NewAuthenticationError("API 密钥已被禁用。", appErrors.CodeDisabledAPIKey)
	}
	if key.IsExpired() {
		return appErrors.NewAuthenticationError("API 密钥已过期。", appErrors.CodeExpiredAPIKey)
	}
	return appErrors.NewAuthenticationError("API 密钥无效。", appErrors.CodeInvalidAPIKey)
}
