# Execution-Ready Spec — Go Rewrite Functional Parity Auth Sprint

## Metadata
- profile: quick
- rounds: 0 interactive rounds (grounded from existing approved artifacts + codebase state)
- final ambiguity: 0.18
- threshold: 0.30
- context type: brownfield
- context snapshot: `.omx/context/go-rewrite-functional-parity-auth-sprint-20260421T122445Z.md`

## Clarity Breakdown
| Dimension | Score | Notes |
|---|---:|---|
| Intent | 0.95 | 终极目标与质量偏好明确 |
| Outcome | 0.90 | 要求 Go 功能等价重写并持续推进 |
| Scope | 0.82 | 当前 sprint 以 auth 接线纵向切片为主 |
| Constraints | 0.90 | 小步、可验证、无新依赖、保持整洁 |
| Success Criteria | 0.82 | 需要代码落地、测试通过、review、commit |
| Context | 0.86 | 已有 PRD/test-spec、parity matrix、auth 第一刀代码 |

## Intent
用 Go 逐步重写 Claude Code Hub 后端，严格保持 Node 行为语义，避免大爆炸式迁移和无边界重构。

## Desired Outcome
在 `go-rewrite-main` 上持续交付可验证的等价模块。当前最近目标是完成一个最小纵向切片：`/v1` 入口已接入 Node 对齐的 proxy auth 语义，并有测试/契约证据支撑后续 session、rate-limit、provider 相关实现。

## In-Scope
- 评估当前 auth 接线是否已形成可提交最小切片
- 识别该切片缺失的实现/测试/验证项
- 通过 team 模式并行推进当前切片所需的实现与验证工作
- 完成本轮代码 review、验证与提交

## Out-of-Scope / Non-goals
- 本轮不完成完整 proxy forwarder / SSE / provider selector
- 本轮不完成完整 admin API
- 本轮不重写前端或 i18n
- 本轮不引入新依赖或大规模目录改造

## Decision Boundaries
- 可在 Go 内部重构 handler/service 流程，但不得破坏既有外部 HTTP/error contract
- 可新增测试、夹具和小型辅助代码
- 不得为了“先跑通”而删除已有测试或弱化 parity 目标

## Constraints
- Node 行为为真相源
- 先验证再声称完成
- 每个切片保持可审阅、可回退、可提交
- 完成后需要 review 与 git 提交

## Testable Acceptance Criteria
1. `internal/service/auth` 与 `/v1` handler 的连接在代码中完整存在，且不是仅 main.go 的占位接线。
2. `go test ./internal/handler/v1 ./internal/service/auth ./tests/...` 通过。
3. `go build ./cmd/server` 通过。
4. 受影响文件的诊断为 0 error。
5. 本轮变更形成单独提交，提交信息遵循 Lore protocol。

## Assumptions Exposed
- 当前最小切片应围绕 auth，而不是立刻扩展到 provider/session。
- 现有 PRD/test-spec 已足以作为 Ralph 的 planning gate。

## Pressure-pass Findings
用户明确要求“最小执行单位、逐个击破、不要产生过多废物”，因此 team 任务拆分必须围绕当前 auth 纵向切片，而不是扩成完整代理核心。

## Brownfield Evidence vs Inference
- Evidence: `cmd/server/main.go` 已接入 `v1handler.NewHandler(proxyAuthService).RegisterRoutes(...)`；`internal/handler/v1/proxy.go` 已有 `AuthMiddleware`；`internal/service/auth/*` 已有测试。
- Inference: 当前最合理的 team sprint 是补齐 auth 切片的剩余验证/整洁度，并为下个模块预留清晰边界。
