# PRD: Claude Code Hub Go 重写（功能等价）

## Requirements Summary

- 目标：将当前 Claude Code Hub 的后端能力重写为 Go，实现与现有产品**功能等价**，并保留现有运营/管理能力、代理能力、数据语义与主要接口契约。
- 当前系统证据：
  - 当前产品是 Next.js + Hono + PostgreSQL + Redis 的模块化单体，承担 AI API 代理、管理后台、监控、价格治理与 OpenAPI 文档职责（`README.en.md:17-18`, `README.en.md:231-250`, `docs/architecture-claude-code-hub-2025-11-29.md:25-33`, `docs/architecture-claude-code-hub-2025-11-29.md:64-110`）。
  - 代理入口位于 `src/app/v1/[...route]/route.ts:37-67`，核心处理链位于 `src/app/v1/_lib/proxy-handler.ts:17-119`。
  - 管理 API / OpenAPI 暴露位于 `src/app/api/actions/[...route]/route.ts:21-37`, `src/app/api/actions/[...route]/route.ts:52-71`。
  - 关键持久化模型位于 `src/drizzle/schema.ts:42-172`, `src/drizzle/schema.ts:175-260`, `src/drizzle/schema.ts:446-520`, `src/drizzle/schema.ts:706-818`, `src/drizzle/schema.ts:929-1047`。
  - Session 与限流 heavily 依赖 Redis，且采用 fail-open 降级策略（`README.en.md:100`, `README.en.md:246-249`, `src/lib/session-manager.ts:101-170`, `src/lib/rate-limit/service.ts:115-170`）。
  - UI 是 locale 前缀的 Next.js App Router，后台交互重度依赖 `src/actions/*`、OpenAPI action adapter 和 i18n 资源（`src/app/[locale]/layout.tsx:67-89`, `src/i18n/config.ts:7-10`, `src/lib/api/action-adapter-openapi.ts:39-107`）。
  - 上游仓库已存在部分 Go 重写分支 `upstream/go-rewrite`，其 `docs/REWRITE.md:287-370` 说明基础设施与数据访问层已完成，核心服务 / 代理核心 / HTTP 层仍未完成。

## Explicit Assumptions

1. 本阶段以**后端 Go 重写**为主，前端暂不整体重写；现有 Next.js 前端可以先继续作为管理 UI 使用。
2. 功能等价优先于实现等价：允许内部实现从 Node/Hono 改为 Go/Gin，但外部行为、数据语义和关键接口必须对齐。
3. 优先复用已有 `upstream/go-rewrite` 成果，而不是从零开始再造第二套 Go 骨架。
4. 当前 fork 仍以 `main` 为开发主线，但 Go 重写执行可通过工作树 / team worktree 进行并行开发。

## Acceptance Criteria

### A. 数据与配置等价

1. Go 版本能读写与现有 PostgreSQL 表结构兼容的数据，至少覆盖：
   - `users`, `keys`, `provider_groups`, `providers`, `provider_endpoints`, `message_request`, `model_prices`, `system_settings`, `usage_ledger`, `audit_log`
   - 依据：`src/drizzle/schema.ts:42-172`, `src/drizzle/schema.ts:175-260`, `src/drizzle/schema.ts:369-520`, `src/drizzle/schema.ts:607-818`, `src/drizzle/schema.ts:929-1047`
2. Redis 中的 session / rate-limit / circuit / cache 语义在 Go 版本中可复现，至少不能破坏现有 fail-open 设计。

### B. 代理行为等价

3. Go 版本提供 `/v1/messages`, `/v1/chat/completions`, `/v1/responses`, `/v1/models` 等关键代理端点，并保留当前格式检测、guard pipeline、provider 选择、forward、response handling 的行为边界。
   - 依据：`src/app/v1/[...route]/route.ts:41-57`, `src/app/v1/_lib/proxy-handler.ts:31-99`, `src/app/v1/_lib/proxy/provider-selector.ts:119-255`
4. Session 复用、并发跟踪、限额校验、成本计算、供应商重试/熔断不能回退为“仅能跑通”的简化版本。

### C. 管理 API 与前端兼容

5. 现有前端继续可用所需的关键 HTTP 契约必须被保留：
   - `/api/actions/openapi.json`, `/api/actions/docs`, `/api/actions/scalar`
   - 关键 action-result 响应形状 `{ok,data}` / `{ok:false,error,...}`
   - 直接 REST 端点如 `/api/system-settings`、数据库导入导出、日志清理、管理配置端点
   - 依据：`src/lib/api/action-adapter-openapi.ts:39-107`, `src/app/api/actions/[...route]/route.ts:2042-2266`, `src/app/api/system-settings/route.ts:13-25`

### D. 验证与迁移策略

6. 必须提供 parity 测试矩阵，把当前 Node 版本的关键行为映射到 Go 版本验证项。
7. 必须能以增量方式部署 / 切换，而不是只能一次性替换全站。

## RALPLAN-DR Summary

### Principles

1. **Reuse before rewrite**：优先接管已有 `upstream/go-rewrite` 成果。
2. **Parity over novelty**：先保行为一致，再追求 Go 风格优化。
3. **Incremental cutover**：按数据层 → 服务层 → 代理层 → 管理 API 层逐步落地。
4. **Evidence-backed migration**：每个模块都要用当前代码与测试面来定义验收。
5. **Keep UI stable**：除非用户另行要求，前端先不做大规模替换。

### Decision Drivers

1. 现有系统代理链复杂、状态重（Redis + PG + 流式响应），不适合大爆炸重写。
2. 上游已有 Go 重写分支，直接复用可显著降低重复劳动。
3. 前端对 action/OpenAPI/REST/i18n 契约依赖很深，后端必须尽量兼容而不是强迫前端同步重写。

### Viable Options

#### Option A: 复用 `upstream/go-rewrite`，在其基础上补齐服务层/代理层/HTTP 层
- Pros:
  - 已有基础设施与 repository 层
  - 最快进入高价值代理链实现
  - 减少重复设计与目录重建
- Cons:
  - 需先审计已有 Go 分支与当前 main 的功能差距
  - 可能需要较大规模补齐缺失模块

#### Option B: 在当前 `main` 上从零新增第二套 Go 服务目录
- Pros:
  - 完全按当前需求定制
  - 不受上游半成品限制
- Cons:
  - 重复搭骨架，耗时最高
  - 容易出现两套 Go 方案并存、决策发散

#### Option C: 先只用 Go 重写代理层，管理 API 暂时保留 Node
- Pros:
  - 最快替换性能/并发敏感部分
  - 风险相对可控
- Cons:
  - 只能实现“部分 Go 化”，不是完整目标
  - 会长期维持双后端复杂度

## ADR

### Decision
采用 **Option A + Option C 的阶段化组合**：
- 主路径：以 `upstream/go-rewrite` 为基础继续推进 Go 服务；
- 切换策略：优先完成 Go 代理核心与关键管理 API，再按模块逐步替代 Node 后端。

### Drivers

- 上游已完成 Go 基础设施与 repository 层（`upstream/go-rewrite:docs/REWRITE.md:287-313`）
- 现有系统最难、最关键的是服务层与代理链，而这正是上游仍未完成的部分（`upstream/go-rewrite:docs/REWRITE.md:317-359`）
- 前端契约耦合深，必须保留兼容入口而非同步推翻前端

### Alternatives considered

- 全量绿地 Go 重写：被否决，重复劳动太多
- 永久双栈（Node 管理 API + Go 代理层）：被否决，只能作为过渡，不满足最终目标

### Why chosen

- 能最快利用现有成果
- 能把并行执行集中到最关键的未完模块
- 允许分阶段验证并逐步切流

### Consequences

- 需要先建立 **Node → Go parity matrix**
- 需要同时维护 Go 分支与现有 Node 主线一段时间
- 需要定义明确的模块切换顺序与接口适配层

### Follow-ups

1. 合并/跟踪 `upstream/go-rewrite` 到本 fork 的执行分支
2. 建立 parity inventory
3. 以 team 方式并行推进 service/proxy/http 三大块

## Deliberate Pre-mortem

1. **代理链看似可跑，实际语义不等价**
   - 风险：限流、会话复用、响应流处理、供应商重试行为出现细微偏差
   - 缓解：先做 parity matrix，再补 golden tests / replay tests

2. **前端继续使用时，管理 API 契约被破坏**
   - 风险：现有 React UI 直接请求的 REST/API actions 失配
   - 缓解：优先保留响应形状与关键端点，新增兼容层

3. **并行执行导致 Go 目录设计分叉**
   - 风险：多个 worker 各自建自己的 service/proxy 风格
   - 缓解：先固定目录/边界/接口约定，再分工实现

## Implementation Steps

### Step 1 — 接管已有 Go rewrite 基线
- 基于 `refs/remotes/upstream/go-rewrite` 创建本地执行基线，审计其已完成内容：
  - `cmd/server/main.go`
  - `internal/config/*`
  - `internal/database/*`
  - `internal/model/*`
  - `internal/repository/*`
- 目标：确认可直接复用的目录、模型、repository 覆盖率

### Step 2 — 建立 Node → Go parity inventory
- 从当前 Node 代码生成模块映射表：
  - Proxy ingress / pipeline：`src/app/v1/[...route]/route.ts`, `src/app/v1/_lib/proxy-handler.ts`
  - Session / rate-limit：`src/lib/session-manager.ts`, `src/lib/rate-limit/service.ts`
  - OpenAPI / actions：`src/app/api/actions/[...route]/route.ts`, `src/actions/*`
  - Repository / schema：`src/repository/*`, `src/drizzle/schema.ts`
  - i18n/frontend dependencies：`src/app/[locale]/**`, `messages/**`
- 产物：`docs/go-parity-matrix.md`

### Step 3 — 补齐 Go service layer（P0）
- 新增并实现：
  - `internal/service/auth/*`
  - `internal/service/session/*`
  - `internal/service/ratelimit/*`
  - `internal/service/circuitbreaker/*`
  - `internal/service/cost/*`
  - `internal/service/cache/*`
- 以 Node 对应实现为语义基准：
  - `src/lib/session-manager.ts`
  - `src/lib/rate-limit/service.ts`
  - `src/lib/circuit-breaker*`
  - `src/lib/utils/cost-calculation.ts`

### Step 4 — 补齐 Go proxy core（P0）
- 新增并实现：
  - `internal/proxy/session.go`
  - `internal/proxy/guard/*`
  - `internal/proxy/provider_selector.go`
  - `internal/proxy/forwarder.go`
  - `internal/proxy/sse.go`
  - `internal/proxy/handler.go`
  - `internal/proxy/converter/*`
- 以以下 Node 文件为行为基线：
  - `src/app/v1/_lib/proxy-handler.ts`
  - `src/app/v1/_lib/proxy/provider-selector.ts`
  - `src/app/v1/_lib/proxy/response-handler.ts`
  - `src/app/v1/_lib/proxy/forwarder.ts`
  - `src/app/v1/_lib/proxy/session.ts`

### Step 5 — 补齐 Go HTTP / admin API layer（P1）
- 新增并实现：
  - `internal/handler/v1/*`
  - `internal/handler/api/*`
  - `internal/handler/middleware/*`
- 先覆盖当前前端强依赖的最小兼容面：
  - users / keys / providers / provider-endpoints / system-settings / usage-logs / notifications
  - OpenAPI JSON 与 docs endpoints

### Step 6 — 增量接入与回归验证（P0）
- 通过 replay / contract tests / DB-backed integration tests 验证：
  - 代理请求路径
  - 限流与会话
  - 后台关键 CRUD
  - 导入导出 / 流式接口

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Go 分支与当前 main 功能差距大 | 返工 | 先做 parity matrix，再分批推进 |
| Redis 语义实现不一致 | 高 | Lua / key naming / TTL 行为逐项对齐 |
| 前端继续用时接口不兼容 | 高 | 保留 action-result 形状与关键 REST 兼容层 |
| 流式响应与 SSE 处理偏差 | 高 | 单独建立 golden tests 与代理录制回放 |
| 团队并行导致目录和接口漂移 | 中 | 先定边界，再按文件 ownership 分 lane |

## Verification Steps

1. `go test ./...` 覆盖 Go 服务新增模块
2. 契约测试：对 Node 与 Go 的关键 API 响应进行 shape 对比
3. DB-backed integration：连接 PostgreSQL / Redis 验证 users/providers/keys/usage/session/limit
4. Proxy replay：回放 Claude/OpenAI/Codex/Gemini 样例请求，比较结果语义
5. Frontend smoke：现有前端指向 Go API 后，验证设置页 / 用户页 / providers / logs / notifications 关键流程

## Expanded Test Plan

### Unit
- service/auth
- service/session
- service/ratelimit
- service/cost
- proxy/converter
- proxy/provider selector

### Integration
- PostgreSQL repository parity
- Redis limit/session parity
- API auth + provider routing
- action/OpenAPI compatibility

### E2E
- `/v1/chat/completions`
- `/v1/responses`
- provider CRUD + test
- user/key CRUD + quota check
- usage log export / DB import-progress

### Observability
- structured logs
- health endpoints
- request duration / error counters
- provider selection decision trace parity

## Available-Agent-Types Roster

- `planner`
- `architect`
- `critic`
- `explore`
- `executor`
- `debugger`
- `test-engineer`
- `verifier`
- `writer`
- `researcher`

## Follow-up Staffing Guidance

### Ralph path
- 1 x `executor` (high): single-owner sequential integration
- 1 x `verifier` (high): completion evidence
- Use when scope narrows to one module family

### Team path
- 1 x `architect` (high): hold boundaries for Go service/proxy/api layering
- 2 x `executor` (high): implementation lanes
- 1 x `test-engineer` (medium): parity matrix + automated verification
- 1 x `writer` (medium): docs / migration notes / parity inventory maintenance

## Launch Hints

### Recommended first team run

```bash
omx team 4:executor "Continue the Go rewrite from upstream/go-rewrite with functional parity. Lane 1: build docs/go-parity-matrix.md from current Node code. Lane 2: implement missing internal/service packages for auth/session/ratelimit/cost. Lane 3: scaffold internal/proxy core to mirror the Node proxy pipeline. Lane 4: prepare verification harness for contract and replay tests. Keep file ownership explicit and do not overwrite others' work."
```

### Recommended second team run

```bash
omx team 4:executor "Continue Go rewrite HTTP/admin layer. Lane 1: v1 handlers. Lane 2: admin API compatibility handlers. Lane 3: OpenAPI/docs compatibility. Lane 4: integration tests and smoke verification."
```

## Team Verification Path

Before shutdown, team must prove:
1. parity matrix exists and maps Node modules to Go modules
2. Go service layer compiles and tests pass
3. proxy core package compiles with explicit TODO-free public interfaces for implemented paths
4. verification harness exists for contract/replay testing
5. no worker leaves uncommitted or ambiguous partial ownership in shared files

## Changelog

- Initial consensus-style plan created from current repository evidence plus `upstream/go-rewrite` status.
