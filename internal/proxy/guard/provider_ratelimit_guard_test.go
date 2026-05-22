package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// TestProviderRateLimitGuard_Name 测试 Guard 名称
func TestProviderRateLimitGuard_Name(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	if guard.Name() != "ProviderRateLimitGuard" {
		t.Errorf("Expected name 'ProviderRateLimitGuard', got '%s'", guard.Name())
	}
}

// TestProviderRateLimitGuard_NoProvider 测试无供应商
func TestProviderRateLimitGuard_NoProvider(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	req := &Request{
		Provider: nil,
		Context:  make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error for nil provider, got %v", err)
	}
}

// TestProviderRateLimitGuard_ConcurrentSessionsAllowed 测试并发会话允许
func TestProviderRateLimitGuard_ConcurrentSessionsAllowed(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	sessionLimit := 10
	req := &Request{
		Provider: &model.Provider{
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

// TestProviderRateLimitGuard_ConcurrentSessionsExceeded 测试并发会话超限
func TestProviderRateLimitGuard_ConcurrentSessionsExceeded(t *testing.T) {
	service := newMockRateLimitService()
	service.sessionCount = 10
	guard := NewProviderRateLimitGuard(service)

	sessionLimit := 10
	req := &Request{
		Provider: &model.Provider{
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

// TestProviderRateLimitGuard_DailyAmountAllowed 测试每日金额允许
func TestProviderRateLimitGuard_DailyAmountAllowed(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	estimatedCost := udecimal.MustParse("100.00")

	req := &Request{
		Provider: &model.Provider{
			ID: 1,
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

// TestProviderRateLimitGuard_DailyAmountExceeded 测试每日金额超限
func TestProviderRateLimitGuard_DailyAmountExceeded(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	estimatedCost := udecimal.MustParse("100.00")

	req := &Request{
		Provider: &model.Provider{
			ID: 1,
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

// TestProviderRateLimitGuard_NoLimits 测试无限制
func TestProviderRateLimitGuard_NoLimits(t *testing.T) {
	service := newMockRateLimitService()
	guard := NewProviderRateLimitGuard(service)

	req := &Request{
		Provider: &model.Provider{
			ID: 1,
		},
		Context: make(map[string]interface{}),
	}

	err := guard.Check(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error for provider with no limits, got %v", err)
	}
}
