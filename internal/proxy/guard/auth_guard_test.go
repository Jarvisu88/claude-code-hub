package guard

import (
	"context"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService 模拟认证服务
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) AuthenticateProxy(ctx context.Context, input auth.ProxyAuthInput) (*auth.AuthResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResult), args.Error(1)
}

func TestAuthGuard_Name(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	assert.Equal(t, "AuthGuard", guard.Name())
}

func TestAuthGuard_Check_Success(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	enabled := true
	now := time.Now()
	user := &model.User{
		ID:        1,
		Name:      "test-user",
		IsEnabled: &enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := &model.Key{
		ID:        1,
		UserID:    1,
		Key:       "test-api-key",
		Name:      "test-key",
		IsEnabled: &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		User:      user,
	}

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer test-api-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(&auth.AuthResult{
		User:    user,
		Key:     key,
		APIKey:  "test-api-key",
		IsAdmin: false,
	}, nil)

	req := &Request{
		APIKey: &model.Key{
			Key: "test-api-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, req.User)
	assert.Equal(t, 1, req.User.ID)
	assert.Equal(t, "test-user", req.User.Name)
	assert.NotNil(t, req.APIKey)
	assert.Equal(t, 1, req.APIKey.ID)
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_NoAPIKey(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	req := &Request{
		APIKey: nil,
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeTokenRequired))
	mockService.AssertNotCalled(t, "AuthenticateProxy")
}

func TestAuthGuard_Check_EmptyAPIKey(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	req := &Request{
		APIKey: &model.Key{
			Key: "",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeTokenRequired))
	mockService.AssertNotCalled(t, "AuthenticateProxy")
}

func TestAuthGuard_Check_InvalidAPIKey(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer invalid-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(
		nil,
		appErrors.NewAuthenticationError(
			"API 密钥无效。提供的密钥不存在、已被删除、已被禁用或已过期。",
			appErrors.CodeInvalidAPIKey,
		),
	)

	req := &Request{
		APIKey: &model.Key{
			Key: "invalid-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeInvalidAPIKey))
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_DisabledAPIKey(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer disabled-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(
		nil,
		appErrors.NewAuthenticationError(
			"API 密钥已被禁用。",
			appErrors.CodeDisabledAPIKey,
		),
	)

	req := &Request{
		APIKey: &model.Key{
			Key: "disabled-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeDisabledAPIKey))
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_ExpiredAPIKey(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer expired-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(
		nil,
		appErrors.NewAuthenticationError(
			"API 密钥已过期。",
			appErrors.CodeExpiredAPIKey,
		),
	)

	req := &Request{
		APIKey: &model.Key{
			Key: "expired-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeExpiredAPIKey))
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_DisabledUser(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer user-disabled-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(
		nil,
		appErrors.NewAuthenticationError(
			"用户账户已被禁用。请联系管理员。",
			appErrors.CodeDisabledUser,
		),
	)

	req := &Request{
		APIKey: &model.Key{
			Key: "user-disabled-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeDisabledUser))
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_ExpiredUser(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer user-expired-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", mock.Anything, expectedInput).Return(
		nil,
		appErrors.NewAuthenticationError(
			"用户账户已于 2024-01-01 过期。请续费订阅。",
			appErrors.CodeUserExpired,
		),
	)

	req := &Request{
		APIKey: &model.Key{
			Key: "user-expired-key",
		},
	}

	err := guard.Check(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, appErrors.IsCode(err, appErrors.CodeUserExpired))
	mockService.AssertExpectations(t)
}

func TestAuthGuard_Check_ContextPropagation(t *testing.T) {
	mockService := new(MockAuthService)
	guard := NewAuthGuard(mockService)

	enabled := true
	now := time.Now()
	user := &model.User{
		ID:        1,
		Name:      "test-user",
		IsEnabled: &enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := &model.Key{
		ID:        1,
		UserID:    1,
		Key:       "test-api-key",
		Name:      "test-key",
		IsEnabled: &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		User:      user,
	}

	ctx := context.WithValue(context.Background(), "test-key", "test-value")

	expectedInput := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer test-api-key",
		AllowSessionToken:   false,
	}

	mockService.On("AuthenticateProxy", ctx, expectedInput).Return(&auth.AuthResult{
		User:    user,
		Key:     key,
		APIKey:  "test-api-key",
		IsAdmin: false,
	}, nil)

	req := &Request{
		APIKey: &model.Key{
			Key: "test-api-key",
		},
	}

	err := guard.Check(ctx, req)

	assert.NoError(t, err)
	mockService.AssertExpectations(t)
}
