package proxy

import (
	"context"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/google/uuid"
	"github.com/quagmt/udecimal"
)

// Session 代理会话
// 贯穿整个请求生命周期的上下文
type Session struct {
	// ID 会话 ID
	ID string

	// RequestID 请求 ID
	RequestID string

	// Context 上下文
	Context context.Context

	// User 用户信息
	User *model.User

	// APIKey API Key 信息
	APIKey *model.Key

	// Provider 选中的供应商
	Provider *model.Provider

	// Request 请求信息
	Request *Request

	// Response 响应信息
	Response *Response

	// Metadata 元数据
	Metadata map[string]interface{}

	// Timing 时间统计
	Timing *Timing

	// Cost 成本信息
	Cost *Cost

	// Error 错误信息
	Error error

	// CreatedAt 创建时间
	CreatedAt time.Time
}

// Request 请求信息
type Request struct {
	// Method HTTP 方法
	Method string

	// Path 请求路径
	Path string

	// Headers 请求头
	Headers map[string]string

	// Body 请求体
	Body []byte

	// Model 模型名称
	Model string

	// Client 客户端类型 (claude/openai/gemini/codex)
	Client string

	// Stream 是否流式
	Stream bool

	// MaxTokens 最大 Token 数
	MaxTokens int

	// Messages 消息列表 (for chat)
	Messages []Message

	// Prompt 提示词 (for completion)
	Prompt string
}

// Response 响应信息
type Response struct {
	// StatusCode HTTP 状态码
	StatusCode int

	// Headers 响应头
	Headers map[string]string

	// Body 响应体
	Body []byte

	// Stream 流式响应
	Stream chan []byte

	// Error 错误信息
	Error error

	// CompletionTokens 完成 Token 数
	CompletionTokens int

	// PromptTokens 提示 Token 数
	PromptTokens int

	// TotalTokens 总 Token 数
	TotalTokens int
}

// Message 消息
type Message struct {
	// Role 角色 (user/assistant/system)
	Role string

	// Content 内容
	Content string
}

// Timing 时间统计
type Timing struct {
	// StartTime 开始时间
	StartTime time.Time

	// EndTime 结束时间
	EndTime time.Time

	// GuardDuration Guard 链耗时
	GuardDuration time.Duration

	// ForwardDuration 转发耗时
	ForwardDuration time.Duration

	// TotalDuration 总耗时
	TotalDuration time.Duration
}

// Cost 成本信息
type Cost struct {
	// InputCost 输入成本
	InputCost udecimal.Decimal

	// OutputCost 输出成本
	OutputCost udecimal.Decimal

	// TotalCost 总成本
	TotalCost udecimal.Decimal

	// Currency 货币单位
	Currency string
}

// NewSession 创建新会话
func NewSession(ctx context.Context) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New().String(),
		RequestID: uuid.New().String(),
		Context:   ctx,
		Metadata:  make(map[string]interface{}),
		Timing: &Timing{
			StartTime: now,
		},
		Cost: &Cost{
			InputCost:  udecimal.Zero,
			OutputCost: udecimal.Zero,
			TotalCost:  udecimal.Zero,
			Currency:   "USD",
		},
		CreatedAt: now,
	}
}

// SetUser 设置用户
func (s *Session) SetUser(user *model.User) {
	s.User = user
}

// SetAPIKey 设置 API Key
func (s *Session) SetAPIKey(key *model.Key) {
	s.APIKey = key
}

// SetProvider 设置供应商
func (s *Session) SetProvider(provider *model.Provider) {
	s.Provider = provider
}

// SetRequest 设置请求
func (s *Session) SetRequest(req *Request) {
	s.Request = req
}

// SetResponse 设置响应
func (s *Session) SetResponse(resp *Response) {
	s.Response = resp
}

// SetError 设置错误
func (s *Session) SetError(err error) {
	s.Error = err
}

// SetMetadata 设置元数据
func (s *Session) SetMetadata(key string, value interface{}) {
	s.Metadata[key] = value
}

// GetMetadata 获取元数据
func (s *Session) GetMetadata(key string) (interface{}, bool) {
	value, ok := s.Metadata[key]
	return value, ok
}

// MarkGuardEnd 标记 Guard 链结束
func (s *Session) MarkGuardEnd() {
	s.Timing.GuardDuration = time.Since(s.Timing.StartTime)
}

// MarkForwardStart 标记转发开始
func (s *Session) MarkForwardStart() {
	// 转发开始时间就是 Guard 结束时间
}

// MarkForwardEnd 标记转发结束
func (s *Session) MarkForwardEnd() {
	now := time.Now()
	s.Timing.ForwardDuration = now.Sub(s.Timing.StartTime.Add(s.Timing.GuardDuration))
	s.Timing.EndTime = now
	s.Timing.TotalDuration = now.Sub(s.Timing.StartTime)
}

// SetCost 设置成本
func (s *Session) SetCost(inputCost, outputCost udecimal.Decimal) {
	s.Cost.InputCost = inputCost
	s.Cost.OutputCost = outputCost
	s.Cost.TotalCost = inputCost.Add(outputCost)
}

// IsStream 是否流式请求
func (s *Session) IsStream() bool {
	return s.Request != nil && s.Request.Stream
}

// GetModel 获取模型名称
func (s *Session) GetModel() string {
	if s.Request != nil {
		return s.Request.Model
	}
	return ""
}

// GetClient 获取客户端类型
func (s *Session) GetClient() string {
	if s.Request != nil {
		return s.Request.Client
	}
	return ""
}

// GetUserID 获取用户 ID
func (s *Session) GetUserID() int {
	if s.User != nil {
		return s.User.ID
	}
	return 0
}

// GetProviderID 获取供应商 ID
func (s *Session) GetProviderID() int {
	if s.Provider != nil {
		return s.Provider.ID
	}
	return 0
}
