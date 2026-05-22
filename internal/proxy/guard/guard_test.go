package guard

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
)

// TestChain_Execute 测试 Guard 链执行
func TestChain_Execute(t *testing.T) {
	// 创建 mock guards
	guard1 := &mockGuard{name: "Guard1", shouldFail: false}
	guard2 := &mockGuard{name: "Guard2", shouldFail: false}
	guard3 := &mockGuard{name: "Guard3", shouldFail: false}

	chain := NewChain(guard1, guard2, guard3)

	req := &Request{
		Model:     "claude-opus-4",
		RequestID: "test-123",
		Context:   make(map[string]interface{}),
	}

	err := chain.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// 验证所有 guard 都被调用
	if !guard1.called {
		t.Error("Guard1 was not called")
	}
	if !guard2.called {
		t.Error("Guard2 was not called")
	}
	if !guard3.called {
		t.Error("Guard3 was not called")
	}
}

// TestChain_Execute_FailEarly 测试 Guard 链提前失败
func TestChain_Execute_FailEarly(t *testing.T) {
	guard1 := &mockGuard{name: "Guard1", shouldFail: false}
	guard2 := &mockGuard{name: "Guard2", shouldFail: true}
	guard3 := &mockGuard{name: "Guard3", shouldFail: false}

	chain := NewChain(guard1, guard2, guard3)

	req := &Request{
		Model:     "claude-opus-4",
		RequestID: "test-123",
		Context:   make(map[string]interface{}),
	}

	err := chain.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// 验证 Guard1 和 Guard2 被调用，Guard3 未被调用
	if !guard1.called {
		t.Error("Guard1 was not called")
	}
	if !guard2.called {
		t.Error("Guard2 was not called")
	}
	if guard3.called {
		t.Error("Guard3 should not be called after Guard2 failed")
	}
}

// TestChain_Add 测试添加 Guard
func TestChain_Add(t *testing.T) {
	chain := NewChain()

	if len(chain.Guards()) != 0 {
		t.Errorf("Expected 0 guards, got %d", len(chain.Guards()))
	}

	guard1 := &mockGuard{name: "Guard1"}
	chain.Add(guard1)

	if len(chain.Guards()) != 1 {
		t.Errorf("Expected 1 guard, got %d", len(chain.Guards()))
	}

	guard2 := &mockGuard{name: "Guard2"}
	chain.Add(guard2)

	if len(chain.Guards()) != 2 {
		t.Errorf("Expected 2 guards, got %d", len(chain.Guards()))
	}
}

// mockGuard 模拟 Guard
type mockGuard struct {
	name       string
	shouldFail bool
	called     bool
}

func (m *mockGuard) Name() string {
	return m.name
}

func (m *mockGuard) Check(ctx context.Context, req *Request) error {
	m.called = true
	if m.shouldFail {
		return &mockError{message: m.name + " failed"}
	}
	return nil
}

type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

// TestPermissionGuard_Check 测试权限检查
func TestPermissionGuard_Check(t *testing.T) {
	guard := NewPermissionGuard()

	tests := []struct {
		name    string
		req     *Request
		wantErr bool
	}{
		{
			name: "no user",
			req: &Request{
				Model: "claude-opus-4",
			},
			wantErr: true,
		},
		{
			name: "model allowed",
			req: &Request{
				User: &model.User{
					AllowedModels: []string{"claude-opus-4"},
				},
				Model: "claude-opus-4",
			},
			wantErr: false,
		},
		{
			name: "model not allowed",
			req: &Request{
				User: &model.User{
					AllowedModels: []string{"gpt-4"},
				},
				Model: "claude-opus-4",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.Check(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkChain_Execute 性能基准测试
func BenchmarkChain_Execute(b *testing.B) {
	guard1 := &mockGuard{name: "Guard1"}
	guard2 := &mockGuard{name: "Guard2"}
	guard3 := &mockGuard{name: "Guard3"}

	chain := NewChain(guard1, guard2, guard3)

	req := &Request{
		Model:     "claude-opus-4",
		RequestID: "test-123",
		Context:   make(map[string]interface{}),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chain.Execute(ctx, req)
	}
}
