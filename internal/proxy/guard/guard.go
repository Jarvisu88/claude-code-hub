package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/model"
)

// Guard 守卫接口
// 每个 Guard 负责一个特定的检查（认证、权限、限流等）
type Guard interface {
	// Check 执行检查
	// 返回 error 表示检查失败，应该中断请求
	// 返回 nil 表示检查通过，继续执行下一个 Guard
	Check(ctx context.Context, req *Request) error

	// Name 返回 Guard 名称（用于日志和调试）
	Name() string
}

// Request Guard 请求上下文
type Request struct {
	// 用户信息
	User *model.User

	// API Key 信息
	APIKey *model.Key

	// 请求的模型
	Model string

	// 客户端信息
	ClientID string

	// 供应商信息（可选，某些 Guard 需要）
	Provider *model.Provider

	// 请求 ID（用于日志追踪）
	RequestID string

	// 额外的上下文数据（用于 Guard 之间传递信息）
	Context map[string]interface{}
}

// Chain Guard 链
type Chain struct {
	guards []Guard
}

// NewChain 创建 Guard 链
func NewChain(guards ...Guard) *Chain {
	return &Chain{
		guards: guards,
	}
}

// Execute 执行 Guard 链
// 按顺序执行所有 Guard，任何一个失败都会中断执行
func (c *Chain) Execute(ctx context.Context, req *Request) error {
	for _, guard := range c.guards {
		if err := guard.Check(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// Add 添加 Guard 到链末尾
func (c *Chain) Add(guard Guard) {
	c.guards = append(c.guards, guard)
}

// Guards 返回所有 Guard（用于调试）
func (c *Chain) Guards() []Guard {
	return c.guards
}
