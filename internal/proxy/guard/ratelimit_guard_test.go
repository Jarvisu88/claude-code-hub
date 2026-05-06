package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/service/ratelimit"
	"github.com/quagmt/udecimal"
)

// mockRateLimitService 模拟限流服务
type mockRateLimitService struct {
	rpmCount        int
	sessionCount    int
	amountUsage     map[ratelimit.Period]udecimal.Decimal
	checkRPMError   error
	checkSessionErr error
	getUsageError   error
}

func newMockRateLimitService() *mockRateLimitService {
	return &mockRateLimitService{
		amountUsage: make(map[ratelimit.Period]udecimal.Decimal),
	}
}

func (m *mockRateLimitService) CheckRPM(ctx context.Context, userID int, limit int) (bool, error) {
	if m.checkRPMError != nil {
		return false, m.checkRPMError
	}
	return m.rpmCount < limit, nil
}

func (m *mockRateLimitService) CheckAmount(ctx context.Context, userID int, amount udecimal.Decimal, period ratelimit.Period) (bool, error) {
	return true, nil
}

func (m *mockRateLimitService) CheckConcurrentSessions(ctx context.Context, userID int, limit int) (bool, error) {
	if m.checkSessionErr != nil {
		return false, m.checkSessionErr
	}
	return m.sessionCount < limit, nil
}

func (m *mockRateLimitService) IncrementRPM(ctx context.Context, userID int) error {
	m.rpmCount++
	return nil
}

func (m *mockRateLimitService) IncrementAmount(ctx context.Context, userID int, amount udecimal.Decimal, period ratelimit.Period) error {
	current := m.amountUsage[period]
	m.amountUsage[period] = current.Add(amount)
	return nil
}

func (m *mockRateLimitService) AcquireSession(ctx context.Context, userID int, sessionID string) error {
	m.sessionCount++
	return nil
}

func (m *mockRateLimitService) ReleaseSession(ctx context.Context, userID int, sessionID string) error {
	m.sessionCount--
	return nil
}

func (m *mockRateLimitService) GetRPMUsage(ctx context.Context, userID int) (int, error) {
	return m.rpmCount, nil
}

func (m *mockRateLimitService) GetAmountUsage(ctx context.Context, userID int, period ratelimit.Period) (udecimal.Decimal, error) {
	if m.getUsageError != nil {
		return udecimal.Zero, m.getUsageError
	}
	usage, ok := m.amountUsage[period]
	if !ok {
		return udecimal.Zero, nil
	}
	return usage, nil
}

func (m *mockRateLimitService) GetSessionCount(ctx context.Context, userID int) (int, error) {
	return m.sessionCount, nil
}

func (m *mockRateLimitService) ResetRPM(ctx context.Context, userID int) error {
	m.rpmCount = 0
	return nil
}

func (m *mockRateLimitService) ResetAmount(ctx context.Context, userID int, period ratelimit.Period) error {
	delete(m.amountUsage, period)
	return nil
}

// TestRateLimitGuard_Name 测试 Guard 名称
func TestRateLimitGuard_Name(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	if guard.Name() != "RateLimitGuard" {
		t.Errorf("Expected name 'RateLimitGuard', got '%s'", guard.Name())
	}
}

// TestRateLimitGuard_NoUser 测试无用户
func TestRateLimitGuard_NoUser(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	req := &Request{
		User: nil,
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error for nil user, got %v", err)
	}
}

// TestRateLimitGuard_RPMAllowed 测试 RPM 允许
func TestRateLimitGuard_RPMAllowed(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	rpmLimit := 10
	req := &Request{
		User: &model.User{
			ID:       1,
			RPMLimit: &rpmLimit,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestRateLimitGuard_RPMExceeded 测试 RPM 超限
func TestRateLimitGuard_RPMExceeded(t *testing.T) {
	service := newMockRateLimitService()
	service.rpmCount = 10
	guard := NewRateLimitGuard(service)

	rpmLimit := 10
	req := &Request{
		User: &model.User{
			ID:       1,
			RPMLimit: &rpmLimit,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err == nil {
		t.Error("Expected RPM limit exceeded error")
	}
}

// TestRateLimitGuard_ConcurrentSessionsAllowed 测试并发会话允许
func TestRateLimitGuard_ConcurrentSessionsAllowed(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	sessionLimit := 5
	req := &Request{
		User: &model.User{
			ID:                      1,
			LimitConcurrentSessions: &sessionLimit,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestRateLimitGuard_ConcurrentSessionsExceeded 测试并发会话超限
func TestRateLimitGuard_ConcurrentSessionsExceeded(t *testing.T) {
	service := newMockRateLimitService()
	service.sessionCount = 5
	guard := NewRateLimitGuard(service)

	sessionLimit := 5
	req := &Request{
		User: &model.User{
			ID:                      1,
			LimitConcurrentSessions: &sessionLimit,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err == nil {
		t.Error("Expected concurrent sessions limit exceeded error")
	}
}

// TestRateLimitGuard_AmountAllowed 测试金额允许
func TestRateLimitGuard_AmountAllowed(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	dailyLimit := udecimal.MustParse("100.00")
	estimatedCost := udecimal.MustParse("10.00")

	req := &Request{
		User: &model.User{
			ID:            1,
			DailyLimitUSD: &dailyLimit,
		},
		Context: map[string]interface{}{
			"estimatedCost": estimatedCost,
		},
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestRateLimitGuard_AmountExceeded 测试金额超限
func TestRateLimitGuard_AmountExceeded(t *testing.T) {
	service := newMockRateLimitService()
	service.amountUsage[ratelimit.PeriodDaily] = udecimal.MustParse("95.00")
	guard := NewRateLimitGuard(service)

	dailyLimit := udecimal.MustParse("100.00")
	estimatedCost := udecimal.MustParse("10.00")

	req := &Request{
		User: &model.User{
			ID:            1,
			DailyLimitUSD: &dailyLimit,
		},
		Context: map[string]interface{}{
			"estimatedCost": estimatedCost,
		},
	}

	err := guard.Check(context.Background(), req)
	if err == nil {
		t.Error("Expected daily limit exceeded error")
	}
}

// TestRateLimitGuard_NoLimits 测试无限制
func TestRateLimitGuard_NoLimits(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewRateLimitGuard(service)

	req := &Request{
		User: &model.User{
			ID: 1,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error for user with no limits, got %v", err)
	}
}
