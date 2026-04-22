# Go 重写 Node->Go 对齐矩阵

更新时间：2026-04-21  
适用范围：`F:\claudecode hub plus\claude-code-hub-go-rewrite`  
证据来源：

- Node 工作树：`F:\claudecode hub plus\claude-code-hub plus`
- Go 工作树：`F:\claudecode hub plus\claude-code-hub-go-rewrite`

## 目标

把当前 Node 版本的核心能力拆成可以直接落地到 Go 工作树的实现清单，优先覆盖：

- 代理入口（proxy ingress）
- session
- rate-limit
- provider selection
- response handling
- admin actions
- schema/repository mapping
- direct REST endpoints
- i18n/frontend 依赖说明

## 状态标记

- `已存在`：Go 工作树里已经有可复用的模型/仓储/基础设施
- `半成品`：Go 工作树里只有占位、部分字段、或只有数据层没有业务层
- `缺失`：Go 工作树里没有对应实现

## 当前 Go 基线（基于现有仓库事实）

- `cmd/server/main.go`
  - 已有 Gin 启动、Postgres/Redis 初始化、`/health`
  - `/v1/messages`、`/v1/chat/completions`、`/v1/responses`、`/v1/models`
  - `/api/actions/users|keys|providers`
  - 以上业务路由当前全部是 `notImplemented`
- `internal/model/*`
  - 已覆盖一部分数据库模型：`user`、`key`、`provider`、`message_request`、`model_price`、`error_rule`、`request_filter`、`sensitive_word`、`system_settings`、`notification_settings`、`webhook_target`、`notification_target_binding`
- `internal/repository/*`
  - 已有：`user_repo.go`、`key_repo.go`、`provider_repo.go`、`statistics_repo.go`、`price_repo.go`
  - 缺少 message/request-filter/error-rule/system-settings/webhook/audit/provider-endpoint 等仓储
- `go.mod`
  - 仅包含后端依赖：Gin、Bun、go-redis、resty、viper、zerolog、validator
  - 没有任何前端/i18n/runtime UI 依赖

结论：Go 仓库目前是“数据模型 + 部分仓储 + HTTP 占位路由”的阶段，还没有真正进入代理核心、会话、限流、响应处理、后台 API 的实现期。

## 高优先级模块 Node->Go 对齐矩阵

| 模块 | Node 当前实现（证据） | Go 当前证据 | Go 落点建议 | 下一步实现动作 |
|---|---|---|---|---|
| 代理入口（proxy ingress） | `src/app/v1/[...route]/route.ts` 统一挂载 `/v1`；`src/app/v1/_lib/proxy-handler.ts` 为总入口；实际链路再进入 `proxy/guard-pipeline.ts`、`forwarder.ts`、`response-handler.ts` | `cmd/server/main.go` 仅注册了 `/v1/messages`、`/v1/chat/completions`、`/v1/responses`、`/v1/models` 占位路由，全部返回 501 | `internal/handler/v1/*.go` + `internal/proxy/handler.go` + `internal/proxy/guard/*` | 先把 `/v1` 路由从 `main.go` 拆到 `internal/handler/v1`，再做 `ProxySession -> GuardPipeline -> Forwarder -> ResponseHandler` 主链 |
| Session | Node 关键在 `src/app/v1/_lib/proxy/session.ts`、`src/lib/session-manager.ts`，并和 `src/lib/session-tracker.ts`、active-session 相关 actions 协同 | Go 只有 `internal/model/message_request.go` 里的 `session_id`、`request_sequence` 字段，没有 session 服务 | `internal/service/session/manager.go`、`tracker.go`、`store.go` | 先移植 `extractClientSessionId / getOrCreateSessionId / bindSessionToProvider / requestSequence / active session tracking`，Redis key 方案尽量与 Node 保持语义一致 |
| Rate limit | Node 主实现为 `src/lib/rate-limit/service.ts`，并依赖 `lease.ts`、`lease-service.ts`、`time-utils.ts`、`concurrent-session-limit.ts` 与 Redis Lua | Go 只有 user/key/provider 模型里的限额字段，以及 `statistics_repo.go` 提供部分成本查询；没有限流服务 | `internal/service/ratelimit/service.go`、`lease.go`、`time_utils.go`、`lua_scripts.go` | 先做和代理链直接相关的三类校验：key/user/provider 成本窗口、并发 session、provider session 追踪；之后再补 dashboard/统计型查询 |
| Provider selection | Node 主实现为 `src/app/v1/_lib/proxy/provider-selector.ts`，依赖 `repository/provider.ts`、`repository/provider-groups.ts`、`repository/provider-endpoints.ts`、`lib/circuit-breaker.ts` | Go 已有 `internal/model/provider.go` + `internal/repository/provider_repo.go`，但没有 provider group/vendor/endpoint/probe/circuit breaker 实现 | `internal/proxy/provider_selector.go` + `internal/service/circuitbreaker/*` + provider endpoint/group 仓储 | 不要直接只移植 `provider_selector.ts`；先补齐 provider groups / endpoints / vendors / probe / circuit state 的数据层，否则调度逻辑会被迫降级 |
| Response handling | Node 主实现为 `src/app/v1/_lib/proxy/response-handler.ts`，并依赖 `responses.ts`、`response-fixer/*`、`stream-finalization.ts`、usage/cost 写回仓储 | Go 没有对应实现；只有 `internal/pkg/httpclient/client.go` 可做上游请求基础 | `internal/proxy/response_handler.go`、`responses.go`、`sse.go`、`response_fixer/*` | 优先实现：SSE/非流式统一收口、usage 解析、状态码映射、message_request 更新、成本写回；fake-200/stream-finalization 需要单独设计测试 |
| Admin actions | Node 后台主入口是 `src/app/api/actions/[...route]/route.ts`，聚合 `src/actions/*.ts`；用户、keys、providers、price、filters、sessions、audit、notifications 等全走这里 | Go 只有 `main.go` 里的 users/keys/providers CRUD 占位；没有 action adapter、没有 auth/session | `internal/handler/api/*`，按资源显式写 Gin handler，不建议照搬 “Server Actions -> OpenAPI 适配层” | 第一批只做和 Go 后端最相关的资源：users、keys、providers、provider-groups、provider-endpoints、system-settings、model-prices、request-filters、sensitive-words |
| Schema / repository | Node 来源于 `src/drizzle/schema.ts` 与 `src/repository/*.ts` | Go 只有部分 model/repository | 见下方“表结构与仓储对齐表” | 先把缺失的数据层补齐，再进入业务服务层；否则 proxy/admin 都要绕过仓储写 SQL |
| Direct REST endpoints | Node 直接 REST 入口除了 `/v1` 和 `/api/actions`，还有 `src/app/api/auth/*`、`health/*`、`availability/*`、`prices/*`、`proxy-status`、`version`、`system-settings`、`admin/database/*`、`admin/log-level` 等 | Go 只有 `/health` 与极少数占位 `/v1`、`/api/actions/*` | `internal/handler/rest/*` 或 `internal/handler/api/direct/*` | 第一批应该补 `health/live/ready`、`auth/login/logout`、`version`、`system-settings`、`prices`、`proxy-status`、`availability`；数据库导入导出与日志清理可放第二批 |
| i18n / frontend 依赖 | Node 使用 `next-intl`（`src/i18n/*`），支持 5 个 locale（`zh-CN`、`zh-TW`、`en`、`ru`、`ja`），并有完整 `messages/*` 与 `src/app/[locale]/**` dashboard | Go 仓库完全没有 frontend/i18n 代码，只有后端依赖 | 不建议第一阶段在 Go 仓库内重写前端 | 先保持 Node 前端消费 Go API；等 Go API 稳定后，再决定是继续保留 Node 前端，还是单独开新前端迁移项目 |

## 表结构与仓储对齐表

> 这里按“Node 数据源真相 -> Go 当前覆盖 -> 下一步文件”来排。  
> 判断标准：Node 以 `src/drizzle/schema.ts` + `src/repository/*.ts` 为准；Go 以 `internal/model/*` + `internal/repository/*` 为准。

| Node 表 / 资源 | Node 证据 | Go model | Go repository | 状态 | 建议下一步 |
|---|---|---|---|---|---|
| `users` | `src/drizzle/schema.ts`、`src/repository/user.ts` | `internal/model/user.go` | `internal/repository/user_repo.go` | 已存在 | 可直接作为 admin/users 与 auth 基础 |
| `keys` | `src/drizzle/schema.ts`、`src/repository/key.ts` | `internal/model/key.go` | `internal/repository/key_repo.go` | 已存在 | 可直接作为 API key 校验与 key 管理基础 |
| `providers` | `src/drizzle/schema.ts`、`src/repository/provider.ts` | `internal/model/provider.go` | `internal/repository/provider_repo.go` | 已存在 | 需要继续补充 provider 选择依赖的周边表 |
| `provider_groups` | `src/drizzle/schema.ts`、`src/repository/provider-groups.ts` | 缺失 | 缺失 | 缺失 | 新增 `internal/model/provider_group.go` + `internal/repository/provider_group_repo.go` |
| `provider_vendors` | `src/drizzle/schema.ts`、`src/repository/provider-endpoints.ts` | 缺失 | 缺失 | 缺失 | 新增 vendor model/repo，给 endpoint 探测与聚合视图用 |
| `provider_endpoints` | `src/drizzle/schema.ts`、`src/repository/provider-endpoints.ts` | 缺失 | 缺失 | 缺失 | 这是 provider selection / availability / probing 的前置表，优先级 P0 |
| `provider_endpoint_probe_logs` | `src/drizzle/schema.ts`、`src/repository/provider-endpoints.ts` | 缺失 | 缺失 | 缺失 | availability / circuit / endpoint health 都依赖它，建议与 endpoint 表一起补 |
| `message_request` | `src/drizzle/schema.ts`、`src/repository/message.ts` | `internal/model/message_request.go` | 缺失 | 半成品 | 新增 `internal/repository/message_request_repo.go`，给 response handling / usage logs / session trace 用 |
| `model_prices` | `src/drizzle/schema.ts`、`src/repository/model-price.ts` | `internal/model/model_price.go` | `internal/repository/price_repo.go` | 已存在 | 可先复用，但后续要补 upsert/manual/source 等 Node 语义 |
| `error_rules` | `src/drizzle/schema.ts`、`src/repository/error-rules.ts` | `internal/model/error_rule.go` | 缺失 | 半成品 | 新增 `error_rule_repo.go`，后面给 response rewrite / error interception 用 |
| `request_filters` | `src/drizzle/schema.ts`、`src/repository/request-filters.ts` | `internal/model/request_filter.go` | 缺失 | 半成品 | 新增 `request_filter_repo.go`，是 proxy guard 的前置 |
| `sensitive_words` | `src/drizzle/schema.ts`、`src/repository/sensitive-words.ts` | `internal/model/sensitive_word.go` | 缺失 | 半成品 | 新增 `sensitive_word_repo.go`，用于敏感词 guard 预热和刷新缓存 |
| `system_settings` | `src/drizzle/schema.ts`、`src/repository/system-config.ts` | `internal/model/system_settings.go` | 缺失 | 半成品 | 新增 `system_settings_repo.go`，高优先级；provider selector / warmup / verbose error 都会读 |
| `notification_settings` | `src/drizzle/schema.ts`、`src/repository/notifications.ts` | `internal/model/notification_settings.go` | 缺失 | 半成品 | 放到 P2，先不阻塞 proxy 核心 |
| `webhook_targets` | `src/drizzle/schema.ts`、`src/repository/webhook-targets.ts` | `internal/model/webhook_target.go` | 缺失 | 半成品 | 放到 P2，配合通知系统再做 |
| `notification_target_bindings` | `src/drizzle/schema.ts`、`src/repository/notification-bindings.ts` | `internal/model/notification_target_binding.go` | 缺失 | 半成品 | 放到 P2 |
| `usage_ledger` | `src/drizzle/schema.ts`、`src/repository/usage-ledger.ts` | 缺失 | 缺失 | 缺失 | 若 Go 先只做 proxy parity，可延后；若要对齐账单/审计，必须补 |
| `audit_log` | `src/drizzle/schema.ts`、`src/repository/audit-log.ts` | 缺失 | 缺失 | 缺失 | admin actions 与安全审计依赖，建议在 admin API 第二波补上 |

## Admin / Actions 模块拆分建议

Node 当前 `src/actions/*.ts` 已经按资源分块，Go 不需要复制 “Server Actions” 机制，但应该复制“资源边界”：

### 第一批（直接支撑 Go 后端上线）

- `users`
- `keys`
- `providers`
- `provider-groups`
- `provider-endpoints`
- `model-prices`
- `system-config`
- `request-filters`
- `sensitive-words`

### 第二批（观测与运营）

- `active-sessions`
- `session-response`
- `session-origin-chain`
- `proxy-status`
- `rate-limit-stats`
- `statistics`
- `overview`
- `usage-logs`
- `audit-logs`

### 第三批（附加能力）

- `notifications`
- `notification-bindings`
- `webhook-targets`
- `client-versions`
- `dashboard-realtime`
- `dispatch-simulator`

**建议做法：**

- Go 侧用显式 Gin handler，例如：
  - `internal/handler/api/users.go`
  - `internal/handler/api/keys.go`
  - `internal/handler/api/providers.go`
  - `internal/handler/api/provider_groups.go`
  - `internal/handler/api/provider_endpoints.go`
- 不要照搬 Node 的 `createActionRoute` / OpenAPI 自动适配层
- 可以等 handler 稳定后，再统一补 Swagger/OpenAPI

## Direct REST endpoints 对齐清单

Node 当前直接 REST 端点可分为 4 组：

### 1. 代理入口

- `/v1/messages`
- `/v1/chat/completions`
- `/v1/responses`
- `/v1/models`
- `/v1/responses/models`
- `/v1/chat/completions/models`

### 2. 管理聚合入口

- `/api/actions/[module]/[action]`（Node 通过 `src/app/api/actions/[...route]/route.ts` 聚合）

### 3. 独立 REST / 运维端点

- `/api/auth/login`
- `/api/auth/logout`
- `/api/health`
- `/api/health/live`
- `/api/health/ready`
- `/api/version`
- `/api/system-settings`
- `/api/prices`
- `/api/proxy-status`
- `/api/availability`
- `/api/availability/current`
- `/api/availability/endpoints`
- `/api/availability/endpoints/probe-logs`
- `/api/ip-geo/[ip]`
- `/api/leaderboard`

### 4. 管理员专用端点

- `/api/admin/database/export`
- `/api/admin/database/import`
- `/api/admin/database/status`
- `/api/admin/log-cleanup/manual`
- `/api/admin/log-level`
- `/api/admin/system-config`

### Go 建议落地顺序

1. `health/live/ready`
2. `version`
3. `auth/login/logout`
4. `system-settings`
5. `prices`
6. `proxy-status`
7. `availability/*`
8. 管理员数据库/日志端点

原因：前 1~7 组能先支撑 frontend 健康检查、配置页、价格页、可用性页、登录流程；数据库导入导出和日志维护不阻塞代理主链上线。

## i18n / 前端依赖备注

Node 前端不是“可忽略的小层”，而是独立的大块工作量：

- i18n 基础设施：
  - `src/i18n/config.ts`
  - `src/i18n/request.ts`
  - `src/i18n/routing.ts`
- 多语言消息：
  - `messages/zh-CN/*`
  - `messages/zh-TW/*`
  - `messages/en/*`
  - `messages/ru/*`
  - `messages/ja/*`
- Dashboard / settings / usage 页面大量依赖 `src/app/[locale]/**` 与 `next-intl`

Go 当前完全没有这一层对应物，因此推荐：

### 推荐策略（第一阶段）

- **保留现有 Node 前端**
- **只让 Go 接管后端 API 和代理能力**
- 前端通过兼容的 `/v1`、`/api/actions`、`/api/*` 合同切到 Go

### 不推荐策略（第一阶段）

- 在 Go 工作树里同步重写 dashboard + i18n
- 在 proxy/admin 核心尚未对齐前就尝试替换前端

### 实施含义

- Go 的首要目标是 **API/数据/代理行为对齐**
- 不是 UI/i18n parity
- 如果未来确实要把前端也从 Node 脱离，应该单开工作流，不要混入当前后端重写主线

## 建议的 Go 实施顺序（最短可上线路径）

1. **补数据层缺口**
   - `provider_groups`
   - `provider_vendors`
   - `provider_endpoints`
   - `provider_endpoint_probe_logs`
   - `message_request`
   - `system_settings`
   - `request_filters`
   - `sensitive_words`

2. **补核心服务层**
   - session manager / tracker
   - rate limit / lease / time window
   - circuit breaker
   - provider selector

3. **补代理核心**
   - proxy session
   - guard pipeline
   - forwarder
   - response handler
   - SSE / stream finalization

4. **补 `/v1` 端点**
   - `/v1/messages`
   - `/v1/chat/completions`
   - `/v1/responses`
   - `/v1/models`

5. **补 admin/API**
   - users / keys / providers / groups / endpoints / prices / filters / settings

6. **最后接 frontend**
   - 先让现有 Node frontend 对 Go API 做联调
   - 等 API 兼容稳定后，再考虑是否迁出前端

## 关键结论

- Go 仓库当前 **还没进入“代理核心移植完成”阶段**，而是停留在“模型/仓储部分完成 + 路由占位”的阶段。
- 真正阻塞 Go parity 的不是 Gin/Resty 这些基础库，而是 **session、rate-limit、provider endpoint 体系、response handling、message_request 写回** 这几块中间层。
- 要最短路径接近 Node 功能，应优先补：
  1. `provider_groups / provider_endpoints / probe_logs`
  2. `message_request repo`
  3. `system_settings repo`
  4. `session + ratelimit + provider selector`
  5. `proxy response handling`
- 前端/i18n 建议继续由 Node 保持，直到 Go API 合同稳定。
