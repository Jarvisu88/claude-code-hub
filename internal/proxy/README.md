# 供应商选择器 (Provider Selector)

## 概述

供应商选择器负责根据多种条件（权重、优先级、模型匹配、分组标签、熔断器状态等）智能选择合适的供应商。支持故障转移，最多 3 次尝试。

## 功能特性

- ✅ **权重分配**: 加权随机选择，支持自定义权重
- ✅ **优先级调度**: 优先选择高优先级供应商
- ✅ **模型匹配**: 支持 exact/prefix/suffix/contains/regex 匹配
- ✅ **分组调度**: 支持 GroupTag 分组选择
- ✅ **熔断器集成**: 自动跳过熔断的供应商
- ✅ **故障转移**: 最多 3 次尝试，自动排除失败供应商
- ✅ **模型重定向**: 支持模型名称重定向
- ✅ **排除列表**: 支持排除已失败的供应商

## 使用示例

```go
import (
    "context"
    "github.com/ding113/claude-code-hub/internal/proxy"
)

// 创建选择器
selector := proxy.NewProviderSelector(circuitBreaker)

// 基础选择
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    Providers: providers,
})

if err != nil {
    log.Fatal(err)
}

fmt.Printf("Selected: %s\n", result.Provider.Name)
fmt.Printf("Model: %s\n", result.RedirectedModel)

// 分组选择
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    GroupTag:  "premium",
    Providers: providers,
})

// 故障转移（最多 3 次）
results, err := selector.SelectWithRetry(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    Providers: providers,
}, 3)

// 使用第一个供应商，失败后自动切换到第二个
for _, result := range results {
    err := callProvider(result.Provider)
    if err == nil {
        break // 成功
    }
    // 失败，尝试下一个
}
```

## API 文档

### NewProviderSelector

创建供应商选择器。

```go
func NewProviderSelector(circuitBreaker CircuitBreakerChecker) *ProviderSelector
```

**参数**:
- `circuitBreaker`: 熔断器检查接口（可选，传 nil 则不检查熔断器）

**返回**: `*ProviderSelector`

---

### Select

选择供应商。

```go
func (s *ProviderSelector) Select(ctx context.Context, req SelectRequest) (*SelectResult, error)
```

**参数**:
- `ctx`: 上下文
- `req`: 选择请求

**返回**:
- `*SelectResult`: 选择结果
- `error`: 错误信息

**SelectRequest 结构**:
```go
type SelectRequest struct {
    Model              string              // 模型名称
    GroupTag           string              // 分组标签（可选）
    ExcludeProviderIDs []int               // 排除的供应商 ID
    Providers          []*model.Provider   // 候选供应商列表
}
```

**SelectResult 结构**:
```go
type SelectResult struct {
    Provider        *model.Provider  // 选中的供应商
    RedirectedModel string           // 重定向后的模型名称
}
```

---

### SelectWithRetry

选择供应商（支持故障转移）。

```go
func (s *ProviderSelector) SelectWithRetry(ctx context.Context, req SelectRequest, maxAttempts int) ([]*SelectResult, error)
```

**参数**:
- `ctx`: 上下文
- `req`: 选择请求
- `maxAttempts`: 最大尝试次数（默认 3）

**返回**:
- `[]*SelectResult`: 选择结果列表（按优先级排序）
- `error`: 错误信息

**使用场景**: 需要故障转移时，一次性获取多个候选供应商。

---

## 选择算法

### 1. 过滤阶段

按以下条件过滤供应商：

```
1. 排除列表检查
   ↓
2. 启用状态检查 (IsEnabled)
   ↓
3. 分组标签检查 (GroupTag)
   ↓
4. 模型匹配检查 (AllowedModels)
   ↓
5. 熔断器状态检查
   ↓
候选供应商列表
```

### 2. 优先级分组

将候选供应商按优先级分组（降序）：

```
Priority 10: [Provider A, Provider B]
Priority 5:  [Provider C]
Priority 0:  [Provider D, Provider E, Provider F]
```

### 3. 加权随机选择

从最高优先级组开始，使用加权随机算法选择：

```
Provider A (weight=1): 10% 概率
Provider B (weight=9): 90% 概率
```

**算法**:
```go
totalWeight = sum(weights)
r = random(0, totalWeight)
cumulative = 0
for each provider:
    cumulative += provider.weight
    if r < cumulative:
        return provider
```

### 4. 模型重定向

如果供应商配置了模型重定向规则，返回重定向后的模型名称：

```
Input:  claude-opus-4
Redirect: claude-opus-4 → claude-opus-20250514
Output: claude-opus-20250514
```

---

## 模型匹配规则

### 支持的匹配类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `exact` | 精确匹配 | `claude-opus-4` |
| `prefix` | 前缀匹配 | `claude-*` |
| `suffix` | 后缀匹配 | `*-4` |
| `contains` | 包含匹配 | `*opus*` |
| `regex` | 正则匹配 | `^claude-.*-4$` |

### 配置示例

```json
{
  "allowedModels": [
    {"matchType": "exact", "pattern": "claude-opus-4"},
    {"matchType": "prefix", "pattern": "claude-"},
    {"matchType": "regex", "pattern": "^gpt-4.*"}
  ]
}
```

---

## 优先级和权重

### 优先级 (Priority)

- **值越大，优先级越高**
- 默认值: 0
- 范围: 任意整数

**示例**:
```
Priority 10 (最高) → 优先选择
Priority 5  (中等)
Priority 0  (默认) → 最后选择
```

### 权重 (Weight)

- **同一优先级内，权重越大，被选中概率越高**
- 默认值: 1
- 范围: 正整数

**示例**:
```
同一优先级:
  Provider A (weight=1): 10% 概率
  Provider B (weight=9): 90% 概率
```

---

## 故障转移

### 工作流程

```
1. 第一次选择
   ↓
2. 调用失败
   ↓
3. 将失败的供应商加入排除列表
   ↓
4. 第二次选择（排除已失败的）
   ↓
5. 调用失败
   ↓
6. 第三次选择（排除前两个）
   ↓
7. 成功或全部失败
```

### 使用示例

```go
results, err := selector.SelectWithRetry(ctx, SelectRequest{
    Model:     "claude-opus-4",
    Providers: providers,
}, 3)

for i, result := range results {
    log.Printf("Attempt %d: %s", i+1, result.Provider.Name)

    err := callProvider(result.Provider, result.RedirectedModel)
    if err == nil {
        log.Printf("Success with %s", result.Provider.Name)
        break
    }

    log.Printf("Failed with %s: %v", result.Provider.Name, err)
}
```

---

## 性能

### 基准测试结果

```
goos: windows
goarch: amd64
pkg: github.com/ding113/claude-code-hub/internal/proxy
cpu: 13th Gen Intel(R) Core(TM) i5-13500HX

BenchmarkSelect-20    	 3990514	       995.9 ns/op	     704 B/op	      22 allocs/op
```

**性能指标**:
- ⚡ 延迟: **996 ns/op** (~1 微秒)
- 💾 内存: **704 B/op**
- 🔄 分配: **22 allocs/op**
- 🚀 吞吐: **~1,000,000 次/秒**

**性能分析**:
- 10 个供应商的选择耗时 ~1 微秒
- 对请求延迟影响可忽略不计
- 内存占用小，适合高并发场景

---

## 测试覆盖

### 测试用例 (11 个)

- ✅ `TestSelect_BasicSelection` - 基础选择
- ✅ `TestSelect_ModelMatching` - 模型匹配
- ✅ `TestSelect_Priority` - 优先级选择
- ✅ `TestSelect_Weight` - 权重选择
- ✅ `TestSelect_GroupTag` - 分组标签
- ✅ `TestSelect_CircuitBreaker` - 熔断器集成
- ✅ `TestSelect_ExcludeProviders` - 排除供应商
- ✅ `TestSelect_ModelRedirect` - 模型重定向
- ✅ `TestSelect_NoProvidersAvailable` - 无可用供应商
- ✅ `TestSelectWithRetry` - 故障转移
- ✅ `TestSelectWithRetry_InsufficientProviders` - 供应商不足

**测试覆盖率**: 93.8% ✅

---

## 使用场景

### 1. 代理核心

```go
// 在 proxy.go 中使用
selector := proxy.NewProviderSelector(circuitBreaker)

// 获取候选供应商（使用缓存）
providers, err := providerCache.GetActiveProviders(ctx)

// 选择供应商
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     requestModel,
    Providers: providers,
})

// 调用上游
response, err := callUpstream(result.Provider, result.RedirectedModel)
```

### 2. 分组调度

```go
// Premium 用户使用高质量供应商
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    GroupTag:  "premium",
    Providers: providers,
})

// Standard 用户使用标准供应商
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    GroupTag:  "standard",
    Providers: providers,
})
```

### 3. 故障转移

```go
// 获取 3 个候选供应商
results, err := selector.SelectWithRetry(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    Providers: providers,
}, 3)

// 依次尝试
for _, result := range results {
    if err := tryProvider(result); err == nil {
        break
    }
}
```

---

## 最佳实践

### 1. 合理配置优先级

```
高优先级 (10): 官方 API、高质量供应商
中优先级 (5):  第三方 API、备用供应商
低优先级 (0):  测试供应商、降级方案
```

### 2. 合理配置权重

```
同一优先级内:
  主供应商: weight=9 (90%)
  备用供应商: weight=1 (10%)
```

### 3. 使用分组标签

```
premium: 高质量、低延迟供应商
standard: 标准质量供应商
fallback: 降级方案
```

### 4. 集成熔断器

```go
// 创建选择器时传入熔断器
selector := proxy.NewProviderSelector(circuitBreaker)

// 选择器会自动跳过熔断的供应商
```

### 5. 使用缓存

```go
// 使用缓存减少数据库查询
providers, err := providerCache.GetActiveProviders(ctx)

// 传入选择器
result, err := selector.Select(ctx, proxy.SelectRequest{
    Model:     "claude-opus-4",
    Providers: providers,
})
```

---

## 注意事项

### 1. 权重为 0

- ⚠️ 权重为 0 的供应商会被随机选择
- 建议使用权重 1 作为最小值

### 2. 无可用供应商

- ⚠️ 所有供应商都被排除/禁用/熔断时返回错误
- 建议配置降级方案

### 3. 模型匹配

- ⚠️ 如果 `AllowedModels` 为空，则匹配所有模型
- 建议明确配置允许的模型列表

### 4. 故障转移次数

- ⚠️ 默认最多 3 次尝试
- 可根据供应商数量调整

---

## 与 Node.js 版本对比

| 特性 | Node.js | Go | 说明 |
|------|---------|-----|------|
| **选择算法** | 加权随机 | 加权随机 | 一致 |
| **优先级** | 支持 | 支持 | 一致 |
| **模型匹配** | 5 种类型 | 5 种类型 | 一致 |
| **故障转移** | 最多 3 次 | 最多 3 次 | 一致 |
| **性能** | ~10 μs | ~1 μs | **10x** 🚀 |
| **并发安全** | 单线程 | 多线程安全 | Go 更强 |

---

## 依赖

- `internal/model` - 数据模型
- `context` - 上下文
- `math/rand` - 随机数生成

---

## 下一步

- [x] 集成到代理核心 (proxy.go)
- [x] 集成熔断器检查
- [x] 集成供应商缓存
- [ ] 添加选择统计和监控
- [ ] 支持自定义选择策略

---

## 参考

- Node.js 版本: `src/app/v1/_lib/proxy/provider-selector.ts`
- 供应商模型: `internal/model/provider.go`
- 熔断器服务: `internal/service/circuitbreaker/`
