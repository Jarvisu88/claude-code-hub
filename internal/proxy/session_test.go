package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// TestNewSession 测试创建会话
func TestNewSession(t *testing.T) {
	ctx := context.Background()
	session := NewSession(ctx)

	if session.ID == "" {
		t.Error("Expected session ID to be set")
	}

	if session.RequestID == "" {
		t.Error("Expected request ID to be set")
	}

	if session.Context != ctx {
		t.Error("Expected context to be set")
	}

	if session.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}

	if session.Timing == nil {
		t.Error("Expected timing to be initialized")
	}

	if session.Cost == nil {
		t.Error("Expected cost to be initialized")
	}
}

// TestSetUser 测试设置用户
func TestSetUser(t *testing.T) {
	session := NewSession(context.Background())
	user := &model.User{ID: 1}

	session.SetUser(user)

	if session.User != user {
		t.Error("Expected user to be set")
	}

	if session.GetUserID() != 1 {
		t.Errorf("Expected user ID = 1, got %d", session.GetUserID())
	}
}

// TestSetAPIKey 测试设置 API Key
func TestSetAPIKey(t *testing.T) {
	session := NewSession(context.Background())
	key := &model.Key{ID: 1, Key: "test-key"}

	session.SetAPIKey(key)

	if session.APIKey != key {
		t.Error("Expected API key to be set")
	}
}

// TestSetProvider 测试设置供应商
func TestSetProvider(t *testing.T) {
	session := NewSession(context.Background())
	provider := &model.Provider{ID: 1, Name: "test-provider"}

	session.SetProvider(provider)

	if session.Provider != provider {
		t.Error("Expected provider to be set")
	}

	if session.GetProviderID() != 1 {
		t.Errorf("Expected provider ID = 1, got %d", session.GetProviderID())
	}
}

// TestSetRequest 测试设置请求
func TestSetRequest(t *testing.T) {
	session := NewSession(context.Background())
	req := &Request{
		Model:  "claude-3-opus",
		Client: "claude",
		Stream: true,
	}

	session.SetRequest(req)

	if session.Request != req {
		t.Error("Expected request to be set")
	}

	if session.GetModel() != "claude-3-opus" {
		t.Errorf("Expected model = claude-3-opus, got %s", session.GetModel())
	}

	if session.GetClient() != "claude" {
		t.Errorf("Expected client = claude, got %s", session.GetClient())
	}

	if !session.IsStream() {
		t.Error("Expected stream = true")
	}
}

// TestSetResponse 测试设置响应
func TestSetResponse(t *testing.T) {
	session := NewSession(context.Background())
	resp := &Response{
		StatusCode:       200,
		CompletionTokens: 100,
		PromptTokens:     50,
		TotalTokens:      150,
	}

	session.SetResponse(resp)

	if session.Response != resp {
		t.Error("Expected response to be set")
	}
}

// TestSetError 测试设置错误
func TestSetError(t *testing.T) {
	session := NewSession(context.Background())
	err := context.DeadlineExceeded

	session.SetError(err)

	if session.Error != err {
		t.Error("Expected error to be set")
	}
}

// TestMetadata 测试元数据
func TestMetadata(t *testing.T) {
	session := NewSession(context.Background())

	// 设置元数据
	session.SetMetadata("key1", "value1")
	session.SetMetadata("key2", 123)

	// 获取元数据
	value1, ok1 := session.GetMetadata("key1")
	if !ok1 || value1 != "value1" {
		t.Error("Expected key1 = value1")
	}

	value2, ok2 := session.GetMetadata("key2")
	if !ok2 || value2 != 123 {
		t.Error("Expected key2 = 123")
	}

	// 获取不存在的键
	_, ok3 := session.GetMetadata("key3")
	if ok3 {
		t.Error("Expected key3 to not exist")
	}
}

// TestTiming 测试时间统计
func TestTiming(t *testing.T) {
	session := NewSession(context.Background())

	// 模拟 Guard 链执行
	time.Sleep(10 * time.Millisecond)
	session.MarkGuardEnd()

	if session.Timing.GuardDuration < 10*time.Millisecond {
		t.Error("Expected guard duration >= 10ms")
	}

	// 模拟转发执行
	session.MarkForwardStart()
	time.Sleep(20 * time.Millisecond)
	session.MarkForwardEnd()

	if session.Timing.ForwardDuration < 20*time.Millisecond {
		t.Error("Expected forward duration >= 20ms")
	}

	if session.Timing.TotalDuration < 30*time.Millisecond {
		t.Error("Expected total duration >= 30ms")
	}

	if session.Timing.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}
}

// TestSetCost 测试设置成本
func TestSetCost(t *testing.T) {
	session := NewSession(context.Background())

	inputCost := udecimal.MustParse("0.50")
	outputCost := udecimal.MustParse("1.50")

	session.SetCost(inputCost, outputCost)

	if !session.Cost.InputCost.Equal(inputCost) {
		t.Errorf("Expected input cost = %s, got %s", inputCost, session.Cost.InputCost)
	}

	if !session.Cost.OutputCost.Equal(outputCost) {
		t.Errorf("Expected output cost = %s, got %s", outputCost, session.Cost.OutputCost)
	}

	expectedTotal := udecimal.MustParse("2.00")
	if !session.Cost.TotalCost.Equal(expectedTotal) {
		t.Errorf("Expected total cost = %s, got %s", expectedTotal, session.Cost.TotalCost)
	}

	if session.Cost.Currency != "USD" {
		t.Errorf("Expected currency = USD, got %s", session.Cost.Currency)
	}
}

// TestIsStream 测试流式判断
func TestIsStream(t *testing.T) {
	session := NewSession(context.Background())

	// 未设置请求
	if session.IsStream() {
		t.Error("Expected stream = false when request is nil")
	}

	// 非流式请求
	session.SetRequest(&Request{Stream: false})
	if session.IsStream() {
		t.Error("Expected stream = false")
	}

	// 流式请求
	session.SetRequest(&Request{Stream: true})
	if !session.IsStream() {
		t.Error("Expected stream = true")
	}
}

// TestGetters 测试 Getter 方法
func TestGetters(t *testing.T) {
	session := NewSession(context.Background())

	// 未设置时应返回零值
	if session.GetUserID() != 0 {
		t.Error("Expected user ID = 0 when user is nil")
	}

	if session.GetProviderID() != 0 {
		t.Error("Expected provider ID = 0 when provider is nil")
	}

	if session.GetModel() != "" {
		t.Error("Expected model = empty when request is nil")
	}

	if session.GetClient() != "" {
		t.Error("Expected client = empty when request is nil")
	}
}
