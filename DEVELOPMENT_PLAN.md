# Claude Code Hub Go 重写 - 开发计划

> 生成时间: 2026-05-04
> 当前完成度: **45%**
> 预计完成时间: **4-6 周**

---

## 📊 当前进度分析

### 已完成模块 ✅

| 模块 | 文件 | 行数 | 完成度 | 说明 |
|------|------|------|--------|------|
| **配置管理** | `internal/config/` | 260 | 100% | viper + 环境变量 + YAML |
| **数据库连接** | `internal/database/` | 235 | 100% | PostgreSQL + Redis |
| **数据模型** | `internal/model/` | ~3,000 | 100% | 15+ 模型定义 |
| **Repository 层** | `internal/repository/` | ~8,000 | 100% | 20+ Repository |
| **认证服务** | `internal/service/auth/` | 300 | 100% | API Key + Session |
| **会话管理** | `internal/service/session/` | 506 | 100% | Session Manager |
| **熔断器存储** | `internal/service/circuitbreaker/` | 80 | 80% | Redis 状态存储 |
| **实时链追踪** | `internal/service/livechain/` | 180 | 100% | LiveChain Store |
| **供应商追踪** | `internal/service/providertracker/` | 70 | 100% | Provider Tracker |
| **会话追踪** | `internal/service/sessiontracker/` | 100 | 100% | Session Tracker |
| **端点探测** | `internal/service/endpointprobe/` | 200 | 100% | Endpoint Probe |
| **代理核心** | `internal/handler/v1/proxy.go` | 2,410 | 70% | 92 个函数 |
| **管理 API** | `internal/handler/api/` | ~15,000 | 90% | 72 个文件 |

**总计**: 27,072 行代码，108 个 Go 文件，53 个测试文件

### 待实现模块 ❌

| 模块 | 优先级 | 预计工作量 | 说明 |
|------|--------|-----------|------|
| **限流服务** | P0 | 3-4 天 | 多维度限流 + Lua 脚本 |
| **成本计算** | P0 | 1-2 天 | Token 计费 |
| **供应商缓存** | P1 | 1 天 | 进程级缓存 (30s TTL) |
| **供应商选择器** | P0 | 2-3 天 | 权重、优先级、故障转移 |
| **Guard 链** | P0 | 2-3 天 | 认证、限流、过滤 |
| **格式转换器** | P1 | 2-3 天 | Claude/OpenAI/Codex 转换 |
| **SSE 流处理** | P0 | 2-3 天 | 流式响应完善 |
| **熔断器逻辑** | P1 | 1-2 天 | 状态机完善 |
| **通知系统** | P2 | 2-3 天 | Webhook 通知 |
| **测试覆盖** | P1 | 持续 | 目标 80% |

---

## 🎯 开发计划

### Week 1: 核心服务层 (P0 模块)

#### Day 1-2: 实现成本计算服务 ✅ Task #3

**目标**: 实现 Token 计费服务

**任务清单**:
- [ ] 创建 `internal/service/cost/calculator.go`
- [ ] 实现 `CalculateCost()` 函数
  - 支持 input/output/cache tokens 分别计费
  - 使用 `udecimal.Decimal` 精确计算
  - 支持 cost_multiplier
- [ ] 从 ModelPrice Repository 查询价格
- [ ] 编写单元测试 (覆盖率 > 90%)
- [ ] 对比 Node.js 版本验证计算精度

**参考**: `src/lib/utils/cost-calculation.ts`

**验收标准**:
```go
cost, err := calculator.CalculateCost(ctx, CalculateRequest{
    Model: "claude-opus-4",
    InputTokens: 1000,
    OutputTokens: 500,
    CacheCreationTokens: 100,
    CacheReadTokens: 200,
    CostMultiplier: udecimal.MustParse("1.2"),
})
// cost 应该精确到小数点后 6 位
```

---

#### Day 3-4: 实现限流服务 ✅ Task #1

**目标**: 实现多维度限流服务

**任务清单**:
- [ ] 创建 `internal/service/ratelimit/service.go`
- [ ] 实现 RPM 限流
  - Redis Lua 脚本实现滑动窗口
  - 支持 Key 级别和 User 级别
- [ ] 实现金额限流
  - 5小时限额 (limit_5h_usd)
  - 日限额 (limit_daily_usd) - 支持固定/滚动模式
  - 周限额 (limit_weekly_usd)
  - 月限额 (limit_monthly_usd)
  - 总额限额 (limit_total_usd)
- [ ] 实现并发 Session 限流
  - Redis 计数器
  - 原子递增/递减
- [ ] 编写 Lua 脚本 `lua_scripts.go`
- [ ] 编写单元测试和集成测试

**参考**: `src/lib/rate-limit/service.ts`

**验收标准**:
```go
result, err := rateLimiter.CheckLimit(ctx, CheckLimitRequest{
    KeyID: 123,
    UserID: 456,
    EstimatedCost: udecimal.MustParse("0.05"),
})
// result.Allowed == true/false
// result.Reason == "rpm_exceeded" / "daily_limit_exceeded" / etc.
```

---

#### Day 5: 实现供应商缓存服务 ✅ Task #2

**目标**: 实现进程级供应商缓存

**任务清单**:
- [ ] 创建 `internal/service/cache/provider_cache.go`
- [ ] 实现进程内缓存 (使用 `sync.Map` 或 `github.com/patrickmn/go-cache`)
- [ ] 30 秒 TTL
- [ ] 并发安全
- [ ] 支持缓存失效和刷新
- [ ] 编写单元测试

**参考**: `src/lib/cache/provider-cache.ts`

**验收标准**:
```go
cache := NewProviderCache(30 * time.Second)
providers, err := cache.GetActiveProviders(ctx, func() ([]*model.Provider, error) {
    return repo.GetActiveProviders(ctx)
})
// 第一次查询数据库，后续 30 秒内从缓存返回
```

---

### Week 2: 代理核心 (P0 模块)

#### Day 1-2: 实现供应商选择器 ✅ Task #7

**目标**: 实现供应商选择逻辑

**任务清单**:
- [ ] 创建 `internal/proxy/provider_selector.go`
- [ ] 实现权重分配算法
  - 加权随机选择
  - 支持 weight 和 priority
- [ ] 实现分组调度 (group_tag)
- [ ] 实现模型匹配 (allowed_models)
  - exact, prefix, suffix, contains, regex
- [ ] 实现熔断器状态检查
- [ ] 实现故障转移 (最多 3 次)
- [ ] 编写单元测试

**参考**: `src/app/v1/_lib/proxy/provider-selector.ts`

**验收标准**:
```go
selector := NewProviderSelector(providers, circuitBreaker)
selected, err := selector.Select(ctx, SelectRequest{
    Model: "claude-opus-4",
    GroupTag: "premium",
    ExcludeProviderIDs: []int{1, 2}, // 已失败的供应商
})
// 返回符合条件且未熔断的供应商
```

---

#### Day 3-4: 实现 Guard 链 ✅ Task #8

**目标**: 实现请求 Guard 链

**任务清单**:
- [ ] 创建 `internal/proxy/guard/` 目录
- [ ] 实现 Guard 接口
  ```go
  type Guard interface {
      Check(ctx context.Context, req *ProxyRequest) error
  }
  ```
- [ ] 实现各个 Guard:
  - `auth_guard.go` - 认证检查
  - `ratelimit_guard.go` - 限流检查
  - `provider_guard.go` - 供应商可用性检查
  - `request_filter_guard.go` - 请求过滤
  - `sensitive_word_guard.go` - 敏感词检测
- [ ] 实现 `pipeline.go` - Guard 链式执行
- [ ] 编写单元测试

**参考**: `src/app/v1/_lib/proxy/guard-pipeline.ts`

**验收标准**:
```go
pipeline := NewGuardPipeline(
    NewAuthGuard(authService),
    NewRateLimitGuard(rateLimiter),
    NewProviderGuard(providerRepo),
    NewRequestFilterGuard(filterRepo),
    NewSensitiveWordGuard(wordRepo),
)
err := pipeline.Execute(ctx, proxyRequest)
// 任何一个 Guard 失败则返回错误
```

---

#### Day 5: 完善 SSE 流处理 ✅ Task #6

**目标**: 完善流式响应处理

**任务清单**:
- [ ] 完善 `internal/proxy/sse.go`
- [ ] 实现流式数据转发
  - 逐块读取上游响应
  - 实时写入下游
- [ ] 实现超时控制
  - first_byte_timeout_streaming_ms
  - streaming_idle_timeout_ms
- [ ] 实现错误处理和重试
- [ ] 实现流式成本计算
  - 解析 SSE 事件中的 usage 字段
  - 累加 tokens
- [ ] 编写集成测试

**参考**: `src/lib/utils/sse.ts`

**验收标准**:
```go
err := streamResponse(ctx, upstreamResp, downstreamWriter, StreamConfig{
    FirstByteTimeout: 30 * time.Second,
    IdleTimeout: 60 * time.Second,
    OnUsage: func(usage Usage) {
        // 累加 tokens
    },
})
```

---

### Week 3: 格式转换与完善

#### Day 1-3: 实现格式转换器 ✅ Task #9

**目标**: 实现多协议格式转换

**任务清单**:
- [ ] 创建 `internal/proxy/converter/` 目录
- [ ] 实现 Converter 接口
  ```go
  type Converter interface {
      ConvertRequest(req *http.Request) (*UpstreamRequest, error)
      ConvertResponse(resp *http.Response) (*DownstreamResponse, error)
  }
  ```
- [ ] 实现各个转换器:
  - `claude.go` - Claude API 格式
  - `openai.go` - OpenAI 兼容格式
  - `codex.go` - Codex API 格式
  - `gemini.go` - Gemini CLI 格式
- [ ] 实现请求/响应双向转换
- [ ] 编写单元测试 (覆盖各种边界情况)

**参考**: `src/app/v1/_lib/converters/`

**验收标准**:
```go
converter := NewOpenAIConverter()
upstreamReq, err := converter.ConvertRequest(claudeRequest)
// 将 Claude 格式转换为 OpenAI 格式

downstreamResp, err := converter.ConvertResponse(upstreamResponse)
// 将 OpenAI 响应转换回 Claude 格式
```

---

#### Day 4-5: 完善熔断器逻辑 ✅ Task #10

**目标**: 完善熔断器状态机

**任务清单**:
- [ ] 完善 `internal/service/circuitbreaker/breaker.go`
- [ ] 实现状态机逻辑
  - CLOSED: 正常状态，记录失败次数
  - OPEN: 熔断状态，拒绝所有请求
  - HALF_OPEN: 半开状态，允许少量探测请求
- [ ] 实现失败阈值检测
  - circuit_breaker_failure_threshold (默认 5)
- [ ] 实现半开状态成功阈值
  - circuit_breaker_half_open_success_threshold (默认 2)
- [ ] 实现 Redis 状态持久化
- [ ] 实现智能探测支持
- [ ] 编写单元测试和集成测试

**参考**: `src/lib/circuit-breaker.ts`

**验收标准**:
```go
breaker := NewCircuitBreaker(config)
err := breaker.Call(ctx, providerID, func() error {
    return callUpstream()
})
// 根据失败次数自动切换状态
```

---

### Week 4: 通知系统与测试

#### Day 1-3: 实现通知系统 ✅ Task #5

**目标**: 实现 Webhook 通知系统

**任务清单**:
- [ ] 创建 `internal/service/notification/` 目录
- [ ] 实现通知队列 `queue.go`
  - 异步发送
  - 批量处理
- [ ] 实现 Webhook 发送 `webhook.go`
  - HTTP POST 请求
  - 重试机制 (指数退避)
  - 超时控制
- [ ] 实现通知模板渲染 `renderer.go`
  - 支持多种事件类型
  - 模板变量替换
- [ ] 编写单元测试

**参考**: `src/lib/notification/`

**验收标准**:
```go
notifier := NewNotificationService(webhookRepo)
err := notifier.Send(ctx, Notification{
    Event: "request_completed",
    Data: map[string]any{
        "session_id": "xxx",
        "cost": "0.05",
    },
})
```

---

#### Day 4-5: 提升测试覆盖率 ✅ Task #4

**目标**: 提升测试覆盖率到 80%

**任务清单**:
- [ ] 为核心服务编写单元测试
  - ratelimit, cost, cache
- [ ] 为 Repository 编写集成测试
  - 使用真实数据库
- [ ] 为 Handler 编写 E2E 测试
  - 完整请求流程
- [ ] 编写并发测试
  - 使用 `go test -race`
- [ ] 编写边界条件测试
  - 空值、极大值、错误输入

**验收标准**:
```bash
make test-coverage
# 总覆盖率 > 80%
```

---

## 🚀 实施策略

### 开发顺序

**优先级排序**:
1. **P0 - 核心功能** (Week 1-2)
   - 限流服务 → 成本计算 → 供应商选择器 → Guard 链
2. **P1 - 重要功能** (Week 3)
   - 格式转换器 → 熔断器完善 → 供应商缓存
3. **P2 - 辅助功能** (Week 4)
   - 通知系统 → 测试覆盖

### 并行开发

可以并行开发的模块：
- **成本计算** + **供应商缓存** (独立模块)
- **格式转换器** + **熔断器完善** (独立模块)
- **通知系统** + **测试覆盖** (持续进行)

### 每日工作流

```
1. 早上 9:00 - 10:00
   - Review 前一天代码
   - 更新任务状态

2. 上午 10:00 - 12:00
   - 核心功能开发

3. 下午 14:00 - 17:00
   - 继续开发
   - 编写单元测试

4. 下午 17:00 - 18:00
   - 代码 Review
   - 提交代码
   - 更新文档
```

---

## 📋 任务追踪

### 当前任务列表

| ID | 任务 | 优先级 | 状态 | 预计工作量 |
|----|------|--------|------|-----------|
| #1 | 实现限流服务 | P0 | ⏳ Pending | 3-4 天 |
| #2 | 实现供应商缓存服务 | P1 | ⏳ Pending | 1 天 |
| #3 | 实现成本计算服务 | P0 | ⏳ Pending | 1-2 天 |
| #4 | 提升测试覆盖率 | P1 | ⏳ Pending | 持续 |
| #5 | 实现通知系统 | P2 | ⏳ Pending | 2-3 天 |
| #6 | 完善 SSE 流处理 | P0 | ⏳ Pending | 2-3 天 |
| #7 | 实现供应商选择器 | P0 | ⏳ Pending | 2-3 天 |
| #8 | 实现 Guard 链 | P0 | ⏳ Pending | 2-3 天 |
| #9 | 实现格式转换器 | P1 | ⏳ Pending | 2-3 天 |
| #10 | 完善熔断器逻辑 | P1 | ⏳ Pending | 1-2 天 |

### 使用 TaskUpdate 更新状态

```bash
# 开始任务
TaskUpdate --taskId 1 --status in_progress

# 完成任务
TaskUpdate --taskId 1 --status completed
```

---

## 🎯 里程碑

### M3: 核心服务完成 (Week 1 结束)
- [x] 认证服务
- [x] 会话管理
- [ ] 限流服务
- [ ] 成本计算
- [ ] 供应商缓存

### M4: 代理核心完成 (Week 2 结束)
- [ ] 供应商选择器
- [ ] Guard 链
- [ ] SSE 流处理
- [x] 代理主逻辑 (70%)

### M5: 格式转换完成 (Week 3 结束)
- [ ] Claude 格式
- [ ] OpenAI 格式
- [ ] Codex 格式
- [ ] Gemini 格式
- [ ] 熔断器完善

### M6: 辅助功能完成 (Week 4 结束)
- [ ] 通知系统
- [ ] 测试覆盖率 > 80%

### M7: 准备灰度发布 (Week 5-6)
- [ ] API 兼容性验证
- [ ] 性能基准测试
- [ ] 文档完善
- [ ] 部署脚本

---

## 🔍 验收标准

### 功能验收

每个模块完成后需要：
1. ✅ 单元测试覆盖率 > 80%
2. ✅ 集成测试通过
3. ✅ 与 Node.js 版本行为一致
4. ✅ 代码 Review 通过
5. ✅ 文档更新

### 性能验收

- QPS > 10,000 req/s (单机)
- P99 延迟 < 50ms
- 内存占用 < 500MB (10,000 并发)
- 无内存泄漏 (运行 24 小时)

### 兼容性验收

- API 响应格式 100% 兼容
- 数据库表结构兼容
- Redis Key 命名兼容
- 配置文件兼容

---

## 📝 开发规范

### 代码规范

```go
// 1. 包注释
// Package ratelimit 实现多维度限流服务
package ratelimit

// 2. 接口定义
type RateLimiter interface {
    CheckLimit(ctx context.Context, req CheckLimitRequest) (*CheckLimitResult, error)
}

// 3. 结构体定义
type Service struct {
    redis *database.RedisClient
    stats StatisticsRepository
}

// 4. 构造函数
func NewService(redis *database.RedisClient, stats StatisticsRepository) *Service {
    return &Service{
        redis: redis,
        stats: stats,
    }
}

// 5. 方法实现
func (s *Service) CheckLimit(ctx context.Context, req CheckLimitRequest) (*CheckLimitResult, error) {
    // 实现逻辑
}
```

### 测试规范

```go
// 1. Table-driven tests
func TestCalculateCost(t *testing.T) {
    tests := []struct {
        name    string
        input   CalculateRequest
        want    udecimal.Decimal
        wantErr bool
    }{
        {
            name: "basic calculation",
            input: CalculateRequest{
                Model: "claude-opus-4",
                InputTokens: 1000,
                OutputTokens: 500,
            },
            want: udecimal.MustParse("0.05"),
        },
        // 更多测试用例...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := CalculateCost(ctx, tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("CalculateCost() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !got.Equal(tt.want) {
                t.Errorf("CalculateCost() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Git 提交规范

```
feat: 实现限流服务
fix: 修复会话管理并发问题
test: 添加成本计算单元测试
docs: 更新开发计划文档
refactor: 重构供应商选择器
perf: 优化 Redis 连接池
```

---

## 🚨 风险管理

### 技术风险

| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|---------|
| Redis Lua 脚本复杂 | 高 | 中 | 参考 Node.js 版本，充分测试 |
| SSE 流处理 Bug | 高 | 中 | 增加集成测试，使用 race detector |
| 并发安全问题 | 高 | 中 | 使用 `go test -race`，代码 Review |
| 性能不达标 | 中 | 低 | 性能基准测试，pprof 分析 |

### 进度风险

| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|---------|
| 任务估算不准 | 中 | 中 | 每日更新进度，及时调整 |
| 依赖阻塞 | 中 | 低 | 并行开发独立模块 |
| 测试覆盖不足 | 高 | 中 | 持续编写测试，CI 检查 |

---

## 📈 进度追踪

### 每日更新

```bash
# 查看任务列表
make tasks

# 更新任务状态
make task-update id=1 status=in_progress

# 查看进度
make progress
```

### 每周 Review

- 周一: 制定本周计划
- 周五: Review 本周进度，调整下周计划

---

## 🎉 完成标准

项目完成的标准：

1. ✅ 所有 P0 和 P1 任务完成
2. ✅ 测试覆盖率 > 80%
3. ✅ API 兼容性验证通过
4. ✅ 性能基准测试达标
5. ✅ 文档完善
6. ✅ 代码 Review 通过
7. ✅ 灰度发布成功

---

**开始时间**: 2026-05-04
**预计完成**: 2026-06-15 (6 周)
**当前状态**: 🟢 进行中

让我们开始吧！🚀
