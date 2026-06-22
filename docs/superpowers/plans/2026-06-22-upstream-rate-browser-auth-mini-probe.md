# 上游倍率登录态与 Mini 探针实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用
> superpowers:subagent-driven-development（推荐）或
> superpowers:executing-plans 逐任务实现此计划。步骤使用复选框
> （`- [ ]`）语法来跟踪进度。

**目标：** 在 CCH 中加入类似 `F:\money` 的本机 Chrome 登录态导入，
并让 Mini 探针的首字节/延迟均值成为现有“速度排序”策略的数据源。

**架构：** 上游倍率同步继续放在 `src/lib/upstream-rate-sync` 与
provider API 边界内；Chrome 登录态导入作为独立 server-only 模块，
只读取临时 Chrome profile 的 DevTools 页面状态。Mini 探针复用现有
`provider-sort` 的 latency cache、scheduler 与用户/密钥 `sortStrategy`
字段，不引入第二套排序机制。

**技术栈：** Next.js server actions/API、Drizzle、Redis、Node/Bun
`net`/`child_process`、Vitest。

---

## 文件职责

- `src/lib/upstream-rate-sync/browser-auth.ts`
  - 启动本机 Chrome 登录页。
  - 通过 DevTools 读取目标 host 的 localStorage、sessionStorage 与 cookie。
  - 按 Sub2API / new-api 解析登录态字段。
- `src/lib/upstream-rate-sync/clients.ts`
  - 补齐 new-api cookie、masked token 匹配与余额读取。
  - 保持现有单渠道倍率同步调用兼容。
- `src/repository/upstream-rate-sync.ts`
  - 修复 due 查询 Date 参数绑定问题。
  - 保存浏览器导入的 token/cookie 字段。
- `src/actions/providers.ts`
  - 增加“打开上游登录页”和“读取上游登录态”管理动作。
- `src/app/api/v1/resources/providers/router.ts`
  - 暴露对应 REST API。
- `src/app/[locale]/settings/providers/_components/upstream-rate-sync-dialog.tsx`
  - 在现有单渠道配置弹窗中加入打开登录/读取登录态按钮。
- `src/lib/provider-sort/*`
  - 将现有 latency probe 标记为 Mini 探针语义。
  - 继续支持 `none`、`price`、`latency`，不改变成本排序。

## 任务 1：修复上游倍率定时同步 due 查询

- [ ] 修改 `findDueProviderUpstreamRateSyncConfigs`，避免把 JS `Date`
      直接传给 postgres.js。
- [ ] 增加仓储单测或服务单测覆盖 due SQL 生成/调用路径。
- [ ] 运行上游倍率同步相关 Vitest。

## 任务 2：实现 Chrome 登录态导入核心

- [ ] 新建 `browser-auth.ts`，移植 `F:\money\chrome_auth.go` 的核心流程。
- [ ] 仅支持本机 `127.0.0.1` DevTools 端口和独立临时 profile。
- [ ] 支持 Windows Chrome 路径查找。
- [ ] 支持 Sub2API 与 new-api 登录态解析。
- [ ] 增加解析逻辑单测。

## 任务 3：接入 provider 管理动作和 API

- [ ] 为单个 provider 的上游同步配置增加打开登录动作。
- [ ] 为单个 provider 的上游同步配置增加读取登录态动作。
- [ ] 保存时不暴露敏感字段到 audit 日志。
- [ ] API schema 更新后运行 OpenAPI 检查。

## 任务 4：补齐 new-api cookie 与余额读取

- [ ] `NewApiRateConfig` 增加 `cookie`。
- [ ] `newApiHeaders` 写入 Cookie。
- [ ] 支持 masked token 后四位匹配。
- [ ] 增加余额读取接口，为后续独立余额刷新页复用。

## 任务 5：Mini 探针排序接入

- [ ] 保留现有 `sortStrategy: latency` 作为“按速度排序”入口。
- [ ] 将 probe 结果写入同一 latency cache。
- [ ] 确认 provider selector 只在同一最高 priority 档内按速度排序，
      不和分组过滤、成本倍率、熔断策略冲突。
- [ ] 补充测试覆盖 price/latency 互不影响。

## 任务 6：前端最小可用入口

- [ ] 在上游倍率同步弹窗加入“打开登录”“读取登录态”。
- [ ] 读取成功后刷新配置状态。
- [ ] 文案走现有 i18n 文件。

## 任务 7：验证

- [ ] `bunx vitest run tests/unit/upstream-rate-sync`
- [ ] `bunx vitest run src/lib/provider-sort/*.test.ts`
- [ ] `bun run typecheck`
- [ ] 若本机 Chrome 可用，手动验证打开登录和读取登录态接口。
