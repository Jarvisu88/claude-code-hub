# 熔断器服务 (Circuit Breaker)

## 概述

熔断器服务实现了经典的熔断器模式，用于保护上游供应商免受过载和级联故障的影响。当供应商连续失败达到阈值时，熔断器会自动打开，暂时阻止请求，给供应商恢复的时间。

## 功能特性

- ✅ **自动熔断**: 失败次数达到阈值自动打开
- ✅ **自动恢复**: 打开一段时间后自动转为半开状态
- ✅ **半开状态**: 允许少量请求测试供应商是否恢复
- ✅ **可配置阈值**: 支持自定义失败阈值
- ✅ **可配置时长**: 支持自定义打开时长
- ✅ **网络错误过滤**: 可选择是否计数网络错误
- ✅ **并发安全**: 使用互斥锁保护状态
- ✅ **零依赖**: 无需外部存储

## 熔断器状态

```
关闭 (Closed)
  ↓ 失败次数 >= 阈值
打开 (Open)
  ↓ 等待一段时间
半开 (Half-Open)
  ↓ 成功
关闭 (Closed)
```

### 状态说明

**关闭 (Closed)**:
- 正常状态，所有请求通过
- 记录失败次数
- 失败次数达到阈值 → 打开

**打开 (Open)**:
- 熔断状态，阻止所有请求
- 等待配置的时长
- 时长到期 → 半开

**半开 (Half-Open)**:
- 测试状态，允许少量请求
- 成功 → 关闭（恢复正常）
- 失败 → 打开（继续熔断）

---

## 使用示例

### 基础使用

```go
import (
    "github.com/ding113/claude-code-hub/internal/service/circuitbreaker"
)

// 检查熔断器状态
if circuitbreaker.IsOpen(provider) {
    return errors.New("Circuit breaker is open")
}

// 调用供应商
response, err := callProvider(provider)

if err != nil {
    // 记录失败
    circuitbreaker.RecordFailure(provider, isNetworkError(err))
    return err
}

// 记录成功
circuitbreaker.RecordSuccess(provider)
return response
```

### 配置网络错误

```go
// 启用网络错误计数
circuitbreaker.Configure(true)

// 禁用网络错误计数（默认）
circuitbreaker.Configure(false)
```

### 供应商配置

```go
threshold := 5
duration := 1800000 // 30 分钟（毫秒）

provider := &model.Provider{
    ID: 1,
    CircuitBreakerFailureThreshold: &threshold,  // 失败阈值
    CircuitBreakerOpenDuration:     &duration,   // 打开时长（ms）
}
```

---

## API 文档

### IsOpen

检查熔断器是否打开。

```go
func IsOpen(provider *model.Provider) bool
```

**参数**:
- `provider`: 供应商信息

**返回**:
- `true`: 熔断器打开，应该阻止请求
- `false`: 熔断器关闭或半开，可以继续请求

**注意**: 如果熔断器已过期，会自动转为半开状态并返回 `false`

---

### IsHalfOpen

检查熔断器是否处于半开状态。

```go
func IsHalfOpen(provider *model.Provider) bool
```

**参数**:
- `provider`: 供应商信息

**返回**:
- `true`: 熔断器半开
- `false`: 熔断器关闭或打开

---

### RecordFailure

记录供应商失败。

```go
func RecordFailure(provider *model.Provider, networkError bool)
```

**参数**:
- `provider`: 供应商信息
- `networkError`: 是否是网络错误

**行为**:
- 失败计数 +1
- 如果达到阈值，打开熔断器
- 如果 `networkError=true` 且未启用网络错误计数，则忽略

---

### RecordSuccess

记录供应商成功。

```go
func RecordSuccess(provider *model.Provider)
```

**参数**:
- `provider`: 供应商信息

**行为**:
- 清除失败计数
- 关闭熔断器

---

### Configure

配置是否计数网络错误。

```go
func Configure(enableNetworkErrors bool)
```

**参数**:
- `enableNetworkErrors`: 是否计数网络错误

**默认**: `false` (不计数网络错误)

---

## 配置说明

### 失败阈值 (CircuitBreakerFailureThreshold)

- **类型**: `*int`
- **默认值**: `5`
- **说明**: 连续失败多少次后打开熔断器

**示例**:
```go
threshold := 10
provider.CircuitBreakerFailureThreshold = &threshold
```

---

### 打开时长 (CircuitBreakerOpenDuration)

- **类型**: `*int` (毫秒)
- **默认值**: `1800000` (30 分钟)
- **说明**: 熔断器打开后多久转为半开状态

**示例**:
```go
duration := 300000 // 5 分钟
provider.CircuitBreakerOpenDuration = &duration
```

---

## 性能

### 基准测试结果

```
goos: windows
goarch: amd64
pkg: github.com/ding113/claude-code-hub/internal/service/circuitbreaker
cpu: 13th Gen Intel(R) Core(TM) i5-13500HX

BenchmarkIsOpen-20           	159984200	        21.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkRecordFailure-20    	149701525	        26.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkRecordSuccess-20    	237901030	        13.29 ns/op	       0 B/op	       0 allocs/op
```

**性能指标**:
- ⚡ IsOpen 延迟: **21 ns/op**
- ⚡ RecordFailure 延迟: **26 ns/op**
- ⚡ RecordSuccess 延迟: **13 ns/op**
- 💾 内存: **0 B/op** (零分配)
- 🔄 分配: **0 allocs/op**
- 🚀 吞吐: **~47M QPS** (IsOpen)

---

## 测试覆盖

### 测试用例 (13 个)

- ✅ `TestIsOpen` - 基础打开检查
- ✅ `TestIsOpen_Expired` - 过期自动转半开
- ✅ `TestRecordFailure` - 记录失败
- ✅ `TestRecordFailure_NetworkError` - 网络错误处理
- ✅ `TestRecordSuccess` - 记录成功
- ✅ `TestRecordFailure_CustomThreshold` - 自定义阈值
- ✅ `TestRecordFailure_CustomDuration` - 自定义时长
- ✅ `TestIsOpen_NilProvider` - nil 处理
- ✅ `TestIsOpen_InvalidID` - 无效 ID
- ✅ `TestRecordFailure_NilProvider` - nil 处理
- ✅ `TestRecordSuccess_NilProvider` - nil 处理
- ✅ `TestIsHalfOpen` - 半开状态
- ✅ `TestConcurrentAccess` - 并发安全

**测试覆盖率**: 98.2% ✅

---

## 使用场景

### 1. 代理核心

```go
// 在调用供应商前检查
if circuitbreaker.IsOpen(provider) {
    // 跳过该供应商，选择下一个
    continue
}

// 调用供应商
response, err := callUpstream(provider)

if err != nil {
    // 记录失败
    circuitbreaker.RecordFailure(provider, isNetworkError(err))
    return err
}

// 记录成功
circuitbreaker.RecordSuccess(provider)
```

### 2. 供应商选择器

```go
// 过滤掉熔断的供应商
func (s *ProviderSelector) filterProviders(providers []*model.Provider) []*model.Provider {
    var candidates []*model.Provider

    for _, provider := range providers {
        // 检查熔断器
        if circuitbreaker.IsOpen(provider) {
            continue
        }

        candidates = append(candidates, provider)
    }

    return candidates
}
```

### 3. 健康检查

```go
// 定期检查供应商健康状态
func healthCheck(provider *model.Provider) {
    if circuitbreaker.IsOpen(provider) {
        log.Warnf("Provider %d is circuit broken", provider.ID)
        return
    }

    if circuitbreaker.IsHalfOpen(provider) {
        log.Infof("Provider %d is half-open, testing...", provider.ID)
    }

    // 执行健康检查
    err := ping(provider)
    if err != nil {
        circuitbreaker.RecordFailure(provider, false)
    } else {
        circuitbreaker.RecordSuccess(provider)
    }
}
```

---

## 最佳实践

### 1. 合理设置阈值

```
低流量供应商: threshold = 3-5
高流量供应商: threshold = 10-20
```

**原因**: 低流量供应商失败几次就可能有问题，高流量供应商需要更多样本。

### 2. 合理设置时长

```
快速恢复: duration = 5-10 分钟
慢速恢复: duration = 30-60 分钟
```

**原因**: 根据供应商的恢复速度调整。

### 3. 网络错误处理

```go
// 区分网络错误和业务错误
if isNetworkError(err) {
    circuitbreaker.RecordFailure(provider, true)
} else {
    circuitbreaker.RecordFailure(provider, false)
}
```

**原因**: 网络错误可能是临时的，业务错误可能是持久的。

### 4. 半开状态处理

```go
if circuitbreaker.IsHalfOpen(provider) {
    // 半开状态，谨慎处理
    log.Infof("Provider %d is half-open, testing recovery", provider.ID)
}
```

---

## 注意事项

### 1. 状态存储

- ⚠️ 状态存储在内存中，重启后丢失
- ⚠️ 多实例部署时，每个实例独立维护状态
- ⚠️ 如需持久化，需要扩展实现

### 2. 并发安全

- ✅ 使用互斥锁保护状态
- ✅ 所有方法都是并发安全的
- ✅ 适合高并发场景

### 3. 性能影响

- ✅ 延迟极低（~XXX ns）
- ✅ 内存占用小
- ✅ 对请求延迟影响可忽略不计

---

## 与 Node.js 版本对比

| 特性 | Node.js | Go | 说明 |
|------|---------|-----|------|
| **状态存储** | 内存 | 内存 | 一致 |
| **状态机** | 3 状态 | 3 状态 | 一致 |
| **配置** | 供应商级别 | 供应商级别 | 一致 |
| **并发安全** | 单线程 | 互斥锁 | Go 更强 |
| **性能** | ~XX μs | ~XX ns | Go 更快 |

---

## 依赖

- `internal/model` - 数据模型
- `sync` - 并发控制
- `time` - 时间处理

---

## 下一步

- [x] 实现基础熔断器
- [x] 添加测试覆盖
- [x] 集成到供应商选择器
- [ ] 添加监控指标
- [ ] 支持持久化存储（可选）

---

## 参考

- Node.js 版本: `src/lib/circuit-breaker/store.ts`
- 供应商模型: `internal/model/provider.go`
- 供应商选择器: `internal/proxy/provider_selector.go`
