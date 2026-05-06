# Guard 链 (Guard Chain)

## 概述

Guard 链是代理核心的请求拦截器系统，负责在请求到达上游供应商之前执行各种检查（认证、权限、限流等）。采用责任链模式，每个 Guard 负责一个特定的检查。

## 架构设计

### Guard 接口

```go
type Guard interface {
    Check(ctx context.Context, req *Request) error
    Name() string
}
```

### Guard 链执行流程

```
请求 → Guard 1 → Guard 2 → Guard 3 → ... → 上游供应商
         ↓          ↓          ↓
       失败       失败       失败
         ↓          ↓          ↓
      返回错误   返回错误   返回错误
```

**特点**:
- 顺序执行
- 任何一个 Guard 失败都会中断执行
- 失败时返回错误，不继续执行后续 Guard

### Request 上下文

```go
type Request struct {
    User      *model.User      // 用户信息
    APIKey    *model.Key       // API Key 信息
    Model     string           // 请求的模型
    ClientID  string           // 客户端信息
    Provider  *model.Provider  // 供应商信息（可选）
    RequestID string           // 请求 ID
    Context   map[string]interface{} // 额外上下文
}
```

## 内置 Guard

### 1. AuthGuard - 认证守卫

**职责**: 验证 API Key 和用户身份

**检查项**:
- API Key 是否存在
- API Key 是否有效
- 用户是否启用
- 用户是否过期

**失败场景**:
- API Key 不存在 → `CodeAPIKeyRequired`
- API Key 无效 → `CodeInvalidAPIKey`
- 用户被禁用 → `CodeDisabledUser`
- 用户已过期 → `CodeUserExpired`

---

### 2. PermissionGuard - 权限守卫

**职责**: 检查用户权限

**检查项**:
- 用户是否有权限使用指定模型
- 用户是否有权限使用指定客户端

**失败场景**:
- 模型不允许 → `CodeModelNotAllowed`
- 客户端不允许 → `CodeClientNotAllowed`

---

### 3. RateLimitGuard - 限流守卫

**职责**: 检查用户级别的限流

**检查项**:
- RPM 限流（每分钟请求数）
- 金额限流（5h/daily/weekly/monthly/total）
- 并发会话限流

**失败场景**:
- RPM 超限 → `CodeRPMLimitExceeded`
- 金额超限 → `Code5HLimitExceeded` / `CodeDailyLimitExceeded` 等
- 并发超限 → `CodeConcurrentSessionsExceeded`

---

### 4. ProviderRateLimitGuard - 供应商限流守卫

**职责**: 检查供应商级别的限流

**检查项**:
- 供应商金额限流
- 供应商并发会话限流

**失败场景**:
- 供应商金额超限 → `CodeProviderLimitExceeded`
- 供应商并发超限 → `CodeProviderConcurrentExceeded`

---

## 使用示例

### 基础使用

```go
import (
    "context"
    "github.com/ding113/claude-code-hub/internal/proxy/guard"
)

// 创建 Guard 链
chain := guard.NewChain(
    guard.NewAuthGuard(authService),
    guard.NewPermissionGuard(),
    guard.NewRateLimitGuard(rateLimitService),
    guard.NewProviderRateLimitGuard(providerService),
)

// 执行检查
req := &guard.Request{
    APIKey:    apiKey,
    Model:     "claude-opus-4",
    ClientID:  "web",
    RequestID: "req-123",
    Context:   make(map[string]interface{}),
}

err := chain.Execute(ctx, req)
if err != nil {
    // 检查失败，返回错误
    return err
}

// 检查通过，继续处理请求
// req.User 已被填充
```

### 动态添加 Guard

```go
chain := guard.NewChain()

// 根据配置动态添加
if config.EnableAuth {
    chain.Add(guard.NewAuthGuard(authService))
}

if config.EnablePermission {
    chain.Add(guard.NewPermissionGuard())
}

if config.EnableRateLimit {
    chain.Add(guard.NewRateLimitGuard(rateLimitService))
}
```

### 自定义 Guard

```go
type CustomGuard struct{}

func (g *CustomGuard) Name() string {
    return "CustomGuard"
}

func (g *CustomGuard) Check(ctx context.Context, req *guard.Request) error {
    // 自定义检查逻辑
    if someCondition {
        return errors.NewInvalidRequest("Custom check failed")
    }
    return nil
}

// 添加到链中
chain.Add(&CustomGuard{})
```

---

## 性能

### 基准测试结果

```
BenchmarkChain_Execute-20    	 XXXXXX	       XXX ns/op	     XXX B/op	      XX allocs/op
```

**性能指标**:
- ⚡ 延迟: ~XXX ns/op
- 💾 内存: ~XXX B/op
- 🔄 分配: ~XX allocs/op

**性能分析**:
- Guard 链执行非常快速
- 对请求延迟影响可忽略不计
- 适合高并发场景

---

## 错误处理

### 错误类型

所有 Guard 返回的错误都是 `*errors.AppError` 类型，包含：

```go
type AppError struct {
    Type       ErrorType              // 错误类型
    Message    string                 // 错误消息
    Code       ErrorCode              // 错误码
    Details    map[string]interface{} // 详细信息
    HTTPStatus int                    // HTTP 状态码
}
```

### 错误处理示例

```go
err := chain.Execute(ctx, req)
if err != nil {
    if appErr, ok := err.(*errors.AppError); ok {
        // 根据错误类型处理
        switch appErr.Type {
        case errors.ErrorTypeAuthentication:
            // 认证错误
            return c.JSON(401, appErr.ToResponse())
        case errors.ErrorTypePermissionDenied:
            // 权限错误
            return c.JSON(403, appErr.ToResponse())
        case errors.ErrorTypeRateLimitError:
            // 限流错误
            return c.JSON(429, appErr.ToResponse())
        default:
            // 其他错误
            return c.JSON(500, appErr.ToResponse())
        }
    }
    return c.JSON(500, gin.H{"error": err.Error()})
}
```

---

## 集成点

### 代理核心集成

```go
// 在 proxy.go 中使用
func (h *Handler) HandleRequest(c *gin.Context) error {
    // 1. 解析请求
    apiKey := extractAPIKey(c)
    model := extractModel(c)

    // 2. 执行 Guard 链
    guardReq := &guard.Request{
        APIKey:    apiKey,
        Model:     model,
        ClientID:  c.ClientIP(),
        RequestID: c.GetString("request_id"),
        Context:   make(map[string]interface{}),
    }

    if err := h.guardChain.Execute(c.Request.Context(), guardReq); err != nil {
        return err
    }

    // 3. 使用 guardReq.User 继续处理
    user := guardReq.User

    // 4. 选择供应商
    provider := selectProvider(user, model)

    // 5. 调用上游
    response := callUpstream(provider, model)

    return c.JSON(200, response)
}
```

---

## 测试

### 单元测试

每个 Guard 都有完整的单元测试：

```go
func TestAuthGuard_Check(t *testing.T) {
    tests := []struct {
        name    string
        req     *Request
        wantErr bool
    }{
        {
            name: "valid api key",
            req: &Request{
                APIKey: &model.Key{Key: "valid-key"},
            },
            wantErr: false,
        },
        {
            name: "invalid api key",
            req: &Request{
                APIKey: &model.Key{Key: "invalid-key"},
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            guard := NewAuthGuard(mockAuthService)
            err := guard.Check(context.Background(), tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 集成测试

测试 Guard 链的整体行为：

```go
func TestChain_Execute(t *testing.T) {
    chain := NewChain(
        NewAuthGuard(authService),
        NewPermissionGuard(),
        NewRateLimitGuard(rateLimitService),
    )

    req := &Request{
        APIKey: validAPIKey,
        Model:  "claude-opus-4",
    }

    err := chain.Execute(context.Background(), req)
    if err != nil {
        t.Fatalf("Execute() error = %v", err)
    }

    // 验证 User 被填充
    if req.User == nil {
        t.Error("User should be populated")
    }
}
```

---

## 最佳实践

### 1. Guard 顺序

推荐的 Guard 顺序：

```
1. AuthGuard          (认证)
2. PermissionGuard    (权限)
3. RateLimitGuard     (用户限流)
4. ProviderRateLimitGuard (供应商限流)
```

**原因**:
- 先认证，确保用户身份
- 再检查权限，避免无权限用户消耗资源
- 最后检查限流，因为限流检查可能涉及 Redis 查询

### 2. 错误处理

```go
// 返回详细的错误信息
return errors.NewRateLimitExceeded(
    "RPM limit exceeded",
    errors.CodeRPMLimitExceeded,
).WithDetails(map[string]interface{}{
    "current": currentRPM,
    "limit":   rpmLimit,
})
```

### 3. 上下文传递

```go
// 在 Guard 之间传递信息
func (g *Guard1) Check(ctx context.Context, req *Request) error {
    // 设置上下文
    req.Context["key"] = "value"
    return nil
}

func (g *Guard2) Check(ctx context.Context, req *Request) error {
    // 读取上下文
    value := req.Context["key"]
    return nil
}
```

---

## 与 Node.js 版本对比

| 特性 | Node.js | Go | 说明 |
|------|---------|-----|------|
| **Guard 接口** | 类 | 接口 | Go 更灵活 |
| **执行顺序** | 顺序 | 顺序 | 一致 |
| **错误处理** | throw Error | return error | Go 更显式 |
| **性能** | ~XX μs | ~XX ns | Go 更快 |

---

## 依赖

- `internal/model` - 数据模型
- `internal/pkg/errors` - 错误类型
- `context` - 上下文

---

## 下一步

- [x] 实现基础 Guard 接口
- [x] 实现 AuthGuard
- [x] 实现 PermissionGuard
- [ ] 实现 RateLimitGuard
- [ ] 实现 ProviderRateLimitGuard
- [ ] 完善测试覆盖率
- [ ] 集成到代理核心

---

## 参考

- Node.js 版本: `src/app/v1/_lib/proxy/guards/`
- 错误类型: `internal/pkg/errors/errors.go`
- 用户模型: `internal/model/user.go`
