# Upstream Rate Sync 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用
> superpowers:subagent-driven-development（推荐）或
> superpowers:executing-plans 逐任务实现此计划。步骤使用复选框
> （`- [ ]`）语法来跟踪进度。

**目标：** 自动从 Sub2API / New API 上游读取指定 key/token
所属组倍率，并同步到 CCH 的 `providers.costMultiplier`。

**架构：** 每个 provider 绑定一条可选的上游倍率同步配置。同步客户端
负责上游协议和登录态续期，同步服务负责调用客户端、更新
`providers.costMultiplier`、记录同步结果；调度器只挑选到期配置并委托
服务执行。

**技术栈：** Next.js server actions、Hono 管理 API、Drizzle ORM、
Vitest、现有 provider repository。

---

## 文件结构

- 创建 `src/lib/upstream-rate-sync/clients.ts`：Sub2API/New API 协议客户端。
- 创建 `src/lib/upstream-rate-sync/service.ts`：同步编排和倍率应用。
- 创建 `src/lib/upstream-rate-sync/scheduler.ts`：后台定时触发到期配置。
- 创建 `src/repository/upstream-rate-sync.ts`：配置表读写与结果更新。
- 修改 `src/drizzle/schema.ts`：新增 `provider_upstream_rate_sync_configs`。
- 修改 `src/repository/provider.ts`：增加单 provider 成本倍率更新 helper。
- 修改 `src/actions/providers.ts`：增加配置保存、读取、手动同步 action。
- 修改 `src/app/api/v1/resources/providers/*`：暴露管理 API。
- 修改 `src/instrumentation.ts` 和 `src/lib/lifecycle/shutdown.ts`：启动/停止调度器。
- 测试：
  - `tests/unit/upstream-rate-sync/clients.test.ts`
  - `tests/unit/upstream-rate-sync/service.test.ts`

## 任务

### 任务 1：协议客户端

**文件：**
- 创建：`src/lib/upstream-rate-sync/clients.ts`
- 测试：`tests/unit/upstream-rate-sync/clients.test.ts`

- [ ] 编写失败测试：Sub2API refresh 后读取 key group 的
  `balance_charge_rate`。
- [ ] 编写失败测试：Sub2API 401 时 refresh 后重试一次。
- [ ] 编写失败测试：New API 根据 token 找 group，并读取 `ratio`。
- [ ] 实现最少客户端代码使测试通过。
- [ ] 运行：
  `bunx vitest run tests/unit/upstream-rate-sync/clients.test.ts`

### 任务 2：同步服务

**文件：**
- 创建：`src/lib/upstream-rate-sync/service.ts`
- 创建：`src/repository/upstream-rate-sync.ts`
- 修改：`src/repository/provider.ts`
- 测试：`tests/unit/upstream-rate-sync/service.test.ts`

- [ ] 编写失败测试：同步服务将上游倍率写入指定 provider。
- [ ] 编写失败测试：Sub2API 刷新 token 后持久化新登录态。
- [ ] 实现 repository 接口和同步服务。
- [ ] 运行：
  `bunx vitest run tests/unit/upstream-rate-sync/service.test.ts`

### 任务 3：持久化与 API

**文件：**
- 修改：`src/drizzle/schema.ts`
- 修改：`src/actions/providers.ts`
- 修改：`src/app/api/v1/resources/providers/handlers.ts`
- 修改：`src/app/api/v1/resources/providers/router.ts`
- 修改：`src/lib/api-client/v1/actions/providers.ts`

- [ ] 新增配置表字段，保存 source、baseUrl、目标 key/name、登录态、
  同步间隔和最近同步结果。
- [ ] 使用 `bun run db:generate` 生成迁移。
- [ ] 增加 action/API：读取配置、保存配置、立即同步。

### 任务 4：调度器

**文件：**
- 创建：`src/lib/upstream-rate-sync/scheduler.ts`
- 修改：`src/instrumentation.ts`
- 修改：`src/lib/lifecycle/shutdown.ts`

- [ ] 定时查询 due 且 enabled 的配置。
- [ ] 使用 leader lock 避免多实例重复同步。
- [ ] 接入启动和 shutdown。

### 任务 5：最小 UI

**文件：**
- 修改：`src/app/[locale]/settings/providers/_components/provider-rich-list-item.tsx`
- 新增或复用 provider 设置 dialog 组件。
- 修改 i18n messages。

- [ ] 在 provider 列表添加“倍率同步”入口。
- [ ] 支持保存同步源、登录态、间隔分钟。
- [ ] 支持手动“立即同步”。

### 任务 6：验证

- [ ] 运行新增单测。
- [ ] 运行相关 provider/API 单测。
- [ ] 运行 `bun run typecheck`。
- [ ] 如修改 OpenAPI，运行 `bun run openapi:check`。
