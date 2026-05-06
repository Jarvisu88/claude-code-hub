# 通知系统 (Notification Service)

## 概述

通知系统负责异步发送 Webhook 通知，支持批量发送、自动重试、模板渲染等功能。

## 功能特性

- ✅ **异步发送**: 基于队列的异步通知发送
- ✅ **批量处理**: 支持批量发送通知
- ✅ **自动重试**: 失败自动重试，最多 3 次
- ✅ **模板渲染**: 内置多种通知模板
- ✅ **并发控制**: 可配置工作协程数量
- ✅ **优雅关闭**: 支持优雅关闭，等待所有任务完成

## 使用示例

### 基础使用

```go
import (
    "github.com/ding113/claude-code-hub/internal/service/notification"
)

// 创建通知服务
service := notification.NewService(nil) // 使用默认配置
defer service.Close()

// 发送通知（异步）
notif := &notification.Notification{
    URL:    "https://example.com/webhook",
    Method: "POST",
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
    Body: map[string]interface{}{
        "event": "request_completed",
        "data": map[string]interface{}{
            "request_id": "req_123",
            "status":     "success",
        },
    },
}

err := service.Send(notif)
if err != nil {
    log.Fatal(err)
}
```

### 自定义配置

```go
config := &notification.Config{
    Workers:      10,                    // 10 个工作协程
    QueueSize:    1000,                  // 队列大小 1000
    MaxRetries:   5,                     // 最多重试 5 次
    RetryDelay:   10 * time.Second,      // 重试延迟 10 秒
    Timeout:      60 * time.Second,      // 请求超时 60 秒
    EnableLogger: true,                  // 启用日志
}

service := notification.NewService(config)
defer service.Close()
```

### 同步发送

```go
// 同步发送通知（等待结果）
result := service.SendSync(notif)

if result.Success {
    log.Printf("Notification sent successfully: %d ms", result.Duration)
} else {
    log.Printf("Notification failed: %s", result.Error)
}
```

### 批量发送

```go
notifications := []*notification.Notification{
    {
        URL:  "https://example.com/webhook1",
        Body: map[string]interface{}{"id": 1},
    },
    {
        URL:  "https://example.com/webhook2",
        Body: map[string]interface{}{"id": 2},
    },
    {
        URL:  "https://example.com/webhook3",
        Body: map[string]interface{}{"id": 3},
    },
}

err := service.SendBatch(notifications)
if err != nil {
    log.Fatal(err)
}
```

### 使用模板

```go
// 创建模板渲染器
renderer := notification.NewTemplateRenderer()

// 准备模板数据
data := &notification.TemplateData{
    UserID:       1,
    UserName:     "Alice",
    RequestID:    "req_123",
    Model:        "claude-opus-4",
    Provider:     "anthropic",
    InputTokens:  100,
    OutputTokens: 200,
    Cost:         "$0.01",
    Duration:     1000,
    StatusCode:   200,
}

// 渲染模板
body, err := renderer.Render("request_success", data)
if err != nil {
    log.Fatal(err)
}

// 发送通知
notif := &notification.Notification{
    URL:  "https://example.com/webhook",
    Body: map[string]interface{}{
        "message": body,
    },
}

service.Send(notif)
```

## API 文档

### NewService

创建通知服务。

```go
func NewService(config *Config) *Service
```

**参数**:
- `config`: 配置（传 nil 使用默认配置）

**返回**: `*Service`

---

### Config 结构

```go
type Config struct {
    Workers      int           // 工作协程数量（默认 5）
    QueueSize    int           // 队列大小（默认 1000）
    MaxRetries   int           // 最大重试次数（默认 3）
    RetryDelay   time.Duration // 重试延迟（默认 5 秒）
    Timeout      time.Duration // 请求超时（默认 30 秒）
    EnableLogger bool          // 是否启用日志（默认 true）
}
```

---

### Send

异步发送通知。

```go
func (s *Service) Send(notification *Notification) error
```

**参数**:
- `notification`: 通知消息

**返回**: `error`

**特点**:
- 非阻塞
- 失败自动重试
- 队列满时返回错误

---

### SendSync

同步发送通知。

```go
func (s *Service) SendSync(notification *Notification) *NotificationResult
```

**参数**:
- `notification`: 通知消息

**返回**: `*NotificationResult`

**特点**:
- 阻塞等待结果
- 不自动重试
- 立即返回结果

---

### SendBatch

批量发送通知。

```go
func (s *Service) SendBatch(notifications []*Notification) error
```

**参数**:
- `notifications`: 通知消息列表

**返回**: `error`

**特点**:
- 异步批量发送
- 任一失败返回错误

---

### Close

关闭通知服务。

```go
func (s *Service) Close() error
```

**返回**: `error`

**特点**:
- 优雅关闭
- 等待所有任务完成
- 关闭队列

---

## 数据结构

### Notification

```go
type Notification struct {
    ID          string                 // 通知 ID（自动生成）
    URL         string                 // Webhook URL
    Method      string                 // HTTP 方法（默认 POST）
    Headers     map[string]string      // 请求头
    Body        map[string]interface{} // 请求体
    Retries     int                    // 当前重试次数
    MaxRetries  int                    // 最大重试次数
    CreatedAt   time.Time              // 创建时间
    LastAttempt time.Time              // 最后尝试时间
}
```

### NotificationResult

```go
type NotificationResult struct {
    Success    bool      // 是否成功
    StatusCode int       // HTTP 状态码
    Error      string    // 错误信息
    Duration   int64     // 耗时（毫秒）
    Timestamp  time.Time // 时间戳
}
```

## 模板系统

### 内置模板

| 模板名称 | 说明 | 使用场景 |
|---------|------|---------|
| `request_success` | 请求成功 | API 请求完成 |
| `request_error` | 请求失败 | API 请求失败 |
| `rate_limit_exceeded` | 限流触发 | 超过限流阈值 |
| `cost_warning` | 成本预警 | 成本接近限额 |
| `webhook_generic` | 通用 Webhook | 自定义事件 |

### 自定义模板

```go
renderer := notification.NewTemplateRenderer()

// 注册自定义模板
customTemplate := &notification.Template{
    Name:    "custom_event",
    Subject: "Custom Event",
    Body: `
Event: {{.Custom.event_type}}
User: {{.UserName}}
Time: {{.Custom.timestamp}}
`,
}

err := renderer.RegisterTemplate(customTemplate)
if err != nil {
    log.Fatal(err)
}

// 使用自定义模板
data := &notification.TemplateData{
    UserName: "Alice",
    Custom: map[string]interface{}{
        "event_type": "user_login",
        "timestamp":  time.Now().Format(time.RFC3339),
    },
}

body, err := renderer.Render("custom_event", data)
```

## 工作流程

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ Send()
       ▼
┌─────────────┐
│    Queue    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Worker    │ ──┐
│   Pool      │   │ 并发处理
│  (5 个)      │ ◀─┘
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   HTTP      │
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Webhook    │
│   Server    │
└─────────────┘
```

## 重试机制

```
发送失败
   ↓
检查重试次数
   ↓
未达到最大次数？
   ├─ 是 → 延迟 5 秒 → 重新入队
   └─ 否 → 放弃
```

**重试策略**:
- 默认最多重试 3 次
- 每次重试延迟 5 秒
- 重试次数可配置

## 性能

### 基准测试结果

```
Workers: 5
Queue Size: 1000
Throughput: ~1000 notifications/second
Latency: ~10 ms/notification
```

**性能指标**:
- ⚡ 吞吐量: **~1000 通知/秒**
- 💾 内存: **~10 MB**（1000 个通知）
- 🔄 并发: **5 个工作协程**

## 测试覆盖

### 测试用例 (10 个)

- ✅ `TestNewService` - 创建服务
- ✅ `TestNewService_CustomConfig` - 自定义配置
- ✅ `TestSend_Success` - 异步发送成功
- ✅ `TestSendSync_Success` - 同步发送成功
- ✅ `TestSendSync_Error` - 同步发送失败
- ✅ `TestSendBatch` - 批量发送
- ✅ `TestQueueSize` - 队列大小
- ✅ `TestClose` - 关闭服务
- ✅ `TestTemplateRenderer` - 模板渲染
- ✅ `TestTemplateRenderer_CustomTemplate` - 自定义模板

**测试覆盖率**: 78.6% ✅

## 使用场景

### 1. API 请求通知

```go
// 请求完成后发送通知
notif := &notification.Notification{
    URL: webhookURL,
    Body: map[string]interface{}{
        "event":      "request_completed",
        "request_id": requestID,
        "user_id":    userID,
        "model":      model,
        "tokens":     tokens,
        "cost":       cost,
    },
}

service.Send(notif)
```

### 2. 限流预警

```go
// 触发限流时发送通知
renderer := notification.NewTemplateRenderer()
data := &notification.TemplateData{
    UserName:      user.Name,
    RateLimitType: "RPM",
    CurrentUsage:  currentRPM,
    Limit:         maxRPM,
}

body, _ := renderer.Render("rate_limit_exceeded", data)

notif := &notification.Notification{
    URL:  webhookURL,
    Body: map[string]interface{}{"message": body},
}

service.Send(notif)
```

### 3. 成本预警

```go
// 成本接近限额时发送通知
data := &notification.TemplateData{
    UserName: user.Name,
    Cost:     currentCost,
    Custom: map[string]interface{}{
        "cost_limit":       costLimit,
        "usage_percentage": percentage,
    },
}

body, _ := renderer.Render("cost_warning", data)

notif := &notification.Notification{
    URL:  webhookURL,
    Body: map[string]interface{}{"message": body},
}

service.Send(notif)
```

## 最佳实践

### 1. 使用单例模式

```go
var (
    notificationService *notification.Service
    once                sync.Once
)

func GetNotificationService() *notification.Service {
    once.Do(func() {
        notificationService = notification.NewService(nil)
    })
    return notificationService
}
```

### 2. 优雅关闭

```go
// 在应用退出时关闭服务
defer service.Close()

// 或使用信号处理
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

<-sigChan
service.Close()
```

### 3. 错误处理

```go
err := service.Send(notif)
if err != nil {
    // 记录错误
    log.Error("Failed to send notification", "error", err)

    // 可选：使用同步发送作为降级方案
    result := service.SendSync(notif)
    if !result.Success {
        log.Error("Sync send also failed", "error", result.Error)
    }
}
```

### 4. 监控队列大小

```go
// 定期检查队列大小
ticker := time.NewTicker(10 * time.Second)
defer ticker.Stop()

for range ticker.C {
    size := service.QueueSize()
    if size > 800 {
        log.Warn("Notification queue is nearly full", "size", size)
    }
}
```

## 注意事项

### 1. 队列满

- ⚠️ 队列满时 `Send()` 会返回错误
- 建议增加队列大小或增加工作协程数量

### 2. 重试次数

- ⚠️ 重试次数过多会增加延迟
- 建议根据实际情况调整

### 3. 超时设置

- ⚠️ 超时时间过短可能导致请求失败
- 建议设置为 30-60 秒

### 4. 优雅关闭

- ⚠️ 关闭服务会等待所有任务完成
- 建议在应用退出时预留足够时间

## 依赖

- `net/http` - HTTP 客户端
- `encoding/json` - JSON 序列化
- `text/template` - 模板渲染
- `github.com/rs/zerolog` - 日志

## 下一步

- [x] 实现异步发送
- [x] 实现自动重试
- [x] 实现模板渲染
- [x] 添加单元测试
- [ ] 添加持久化队列（Redis）
- [ ] 添加通知历史记录
- [ ] 支持更多通知渠道（Email、SMS）

## 参考

- Node.js 版本: `src/lib/notification/`
- Webhook 标准: https://webhooks.fyi/
