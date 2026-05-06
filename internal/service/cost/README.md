# 成本计算服务 (Cost Calculator)

## 概述

成本计算服务负责根据模型价格和 Token 使用量计算精确的成本。支持多种计费维度，包括输入/输出 tokens、缓存 tokens、图片生成、搜索上下文等。

## 功能特性

- ✅ **精确计算**: 使用 `udecimal.Decimal` 进行零分配的精确金额计算
- ✅ **多维度计费**: 支持 input/output/cache tokens 分别计费
- ✅ **缓存支持**: 支持缓存创建和读取的分层计费
- ✅ **成本倍率**: 支持 Provider 级别的 cost_multiplier
- ✅ **图片生成**: 支持图片生成模型的按图片计费
- ✅ **搜索上下文**: 支持 Gemini 等模型的搜索上下文计费
- ✅ **200K 分层**: 支持 200K tokens 以上的分层价格

## 使用示例

```go
import (
    "context"
    "github.com/ding113/claude-code-hub/internal/service/cost"
    "github.com/quagmt/udecimal"
)

// 创建计算器
calculator := cost.NewCalculator(priceRepo)

// 计算成本
result, err := calculator.Calculate(ctx, cost.CalculateRequest{
    Model:               "claude-opus-4",
    InputTokens:         10000,
    OutputTokens:        5000,
    CacheCreationTokens: 20000,
    CacheReadTokens:     30000,
    CostMultiplier:      udecimal.MustParse("1.5"),
})

if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total Cost: $%s\n", result.TotalCost.String())
fmt.Printf("Input Cost: $%s\n", result.InputCost.String())
fmt.Printf("Output Cost: $%s\n", result.OutputCost.String())
fmt.Printf("Cache Creation Cost: $%s\n", result.CacheCreationCost.String())
fmt.Printf("Cache Read Cost: $%s\n", result.CacheReadCost.String())
```

## API 文档

### CalculateRequest

| 字段 | 类型 | 说明 |
|------|------|------|
| `Model` | string | 模型名称 |
| `InputTokens` | int | 输入 tokens 数量 |
| `OutputTokens` | int | 输出 tokens 数量 |
| `CacheCreationTokens` | int | 缓存创建 tokens（基础） |
| `CacheReadTokens` | int | 缓存读取 tokens（基础） |
| `CacheCreationTokens1h` | int | 1小时以上缓存创建 tokens |
| `CacheCreationTokens200k` | int | 200K 以上缓存创建 tokens |
| `CacheReadTokens200k` | int | 200K 以上缓存读取 tokens |
| `CostMultiplier` | udecimal.Decimal | 成本倍率（默认 1.0） |
| `OutputImages` | int | 输出图片数量 |
| `SearchContextQueries` | int | 搜索上下文查询次数 |
| `SearchContextSize` | string | 搜索上下文大小（low/medium/high） |

### CalculateResult

| 字段 | 类型 | 说明 |
|------|------|------|
| `TotalCost` | udecimal.Decimal | 总成本（已应用倍率） |
| `InputCost` | udecimal.Decimal | 输入成本（未应用倍率） |
| `OutputCost` | udecimal.Decimal | 输出成本（未应用倍率） |
| `CacheCreationCost` | udecimal.Decimal | 缓存创建成本（未应用倍率） |
| `CacheReadCost` | udecimal.Decimal | 缓存读取成本（未应用倍率） |
| `ImageCost` | udecimal.Decimal | 图片成本（未应用倍率） |
| `SearchContextCost` | udecimal.Decimal | 搜索上下文成本（未应用倍率） |
| `PriceData` | *model.PriceData | 使用的价格信息 |

## 计算公式

### 基础计算

```
InputCost = InputTokens × InputCostPerToken
OutputCost = OutputTokens × OutputCostPerToken
```

### 缓存计算

```
CacheCreationCost =
    CacheCreationTokens × CacheCreationInputTokenCost +
    CacheCreationTokens1h × CacheCreationInputTokenCostAbove1h +
    CacheCreationTokens200k × CacheCreationInputTokenCostAbove200kTokens

CacheReadCost =
    CacheReadTokens × CacheReadInputTokenCost +
    CacheReadTokens200k × CacheReadInputTokenCostAbove200kTokens
```

### 总成本

```
BaseCost = InputCost + OutputCost + CacheCreationCost + CacheReadCost + ImageCost + SearchContextCost
TotalCost = BaseCost × CostMultiplier
```

## 价格示例

### Claude Opus 4

```
Input: $3.00 per million tokens
Output: $15.00 per million tokens
Cache Creation: $3.75 per million tokens
Cache Read: $0.30 per million tokens
```

### 计算示例

```
输入: 10,000 tokens
输出: 5,000 tokens
缓存创建: 20,000 tokens
缓存读取: 30,000 tokens
成本倍率: 1.5

计算:
Input Cost = 10,000 × 0.000003 = $0.03
Output Cost = 5,000 × 0.000015 = $0.075
Cache Creation Cost = 20,000 × 0.00000375 = $0.075
Cache Read Cost = 30,000 × 0.0000003 = $0.009
Base Cost = $0.03 + $0.075 + $0.075 + $0.009 = $0.189
Total Cost = $0.189 × 1.5 = $0.2835
```

## 测试覆盖

- ✅ 基础输入输出计算
- ✅ 成本倍率应用
- ✅ 缓存 tokens 计算
- ✅ 图片生成计算
- ✅ 搜索上下文计算
- ✅ 零 tokens 处理
- ✅ 默认倍率处理
- ✅ 复杂场景测试

**测试覆盖率**: 76.6%

## 性能

基准测试结果（3秒运行，Intel i5-13500HX）：
- **每次计算耗时**: ~820 ns/op (0.82 微秒)
- **内存分配**: 320 B/op
- **分配次数**: 9 allocs/op
- **吞吐量**: ~1,220,000 次计算/秒

性能表现优异，单核可支持百万级 QPS 的成本计算。

## 与 Node.js 版本对比

| 特性 | Node.js | Go |
|------|---------|-----|
| 精度 | JavaScript Number | udecimal.Decimal |
| 性能 | ~XXX ops/s | ~XXX ops/s |
| 类型安全 | TypeScript | Go 类型系统 |
| 测试覆盖 | XX% | 76.6% |

## 依赖

- `github.com/quagmt/udecimal` - 零分配精确金额计算
- `internal/model` - 数据模型
- `internal/repository` - 价格查询

## 下一步

- [ ] 集成到代理核心 (proxy.go)
- [ ] 添加成本追踪和统计
- [ ] 支持批量计算优化
- [ ] 添加成本预估功能

## 参考

- Node.js 版本: `src/lib/utils/cost-calculation.ts`
- 价格模型: `internal/model/model_price.go`
- 价格仓库: `internal/repository/price_repo.go`
