# PRD: Go Rewrite — Codex Session Extraction 最小切片

## Requirements Summary

当前 `go-rewrite-main` 已完成两步：
1. `/v1` 代理鉴权接线（`internal/service/auth/*`, `internal/handler/v1/proxy.go`）
2. `/v1/models` 最小兼容目录输出（`internal/handler/v1/models.go`）

下一步最小可行切片应优先落在 **Codex session ID 提取基线**，原因：
- Node 已有独立、纯逻辑、可直接移植的基线：`src/app/v1/_lib/codex/session-extractor.ts`
- 该能力是后续 `/v1/responses`、session manager、request_sequence、message_request 落地的共同前置
- 当前 Go 已有 `message_request.session_id` / `request_sequence` 字段，但没有任何 session service：`internal/model/message_request.go`

## Goal

在 Go 中实现一个 Node 语义对齐的 Codex session extractor，作为后续代理链路的稳定前置基础能力。

## Non-goals

- 本轮**不**实现 Redis-backed session manager
- 本轮**不**实现 request_sequence 分配
- 本轮**不**接入 `/v1/responses` 完整 handler
- 本轮**不**扩展到 rate-limit / circuit breaker / provider selector

## Acceptance Criteria

1. 新增 `internal/service/session/` 下的纯逻辑提取器，实现与 Node `src/app/v1/_lib/codex/session-extractor.ts` 一致的优先级：
   - `session_id` header
   - `x-session-id` header
   - body `prompt_cache_key`
   - body `metadata.session_id`
   - body `previous_response_id`（带 `codex_prev_` 前缀）
2. 校验规则与 Node 当前逻辑对齐：
   - 最小长度 21
   - 最大长度 256
   - 允许字符集与 Node 一致
3. 新增 Go 单测覆盖 Node 现有关键场景：优先级、边界长度、非法字符、previous_response_id fallback。
4. 新增 parity fixture / contract 说明，明确该能力是未来 `/v1/responses` 接线的输入基线。
5. `go test ./internal/service/... ./tests/...`、`go test ./...`、`go build ./cmd/server` 全部通过。

## Evidence

- Node extractor: `src/app/v1/_lib/codex/session-extractor.ts`
- Node tests: `src/app/v1/_lib/codex/__tests__/session-extractor.test.ts`
- Go current services: `internal/service/auth/auth.go`
- Go current request log model: `internal/model/message_request.go`

## Implementation Steps

### Step 1 — 建立 Go session extractor 边界
- 新增 `internal/service/session/codex_extractor.go`
- 只做纯函数/轻结构，不碰 Redis、不碰 HTTP handler
- 导出最小 public API，便于后续 `/v1/responses` 直接复用

### Step 2 — 对齐 Node 提取/校验语义
- 严格复刻 Node 提取顺序与校验规则
- 保留 source 标识，方便后续日志与调试

### Step 3 — 建立回归测试与 parity 夹具
- 新增 Go 单测覆盖 Node 已有测试矩阵
- 新增 `tests/go-parity/fixtures/` 夹具与 contract test / 说明文档

### Step 4 — 收口验证
- 运行 targeted tests + full tests + build
- 输出本轮最小切片说明，记录不在本轮范围内的后续工作

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| 直接跳去做 session manager 导致范围膨胀 | 中 | 把本轮严格限制为 pure extractor |
| 提取规则与 Node 测试基线偏离 | 高 | 以 Node extractor + unit tests 为单一真相源 |
| 后续 handler 接线时接口不顺手 | 中 | 先导出清晰的 result/source 结构体 |

## Verification Steps

- `go test ./internal/service/... ./tests/...`
- `go test ./...`
- `go build ./cmd/server`
- 代码审查确认未把 Redis / handler / proxy 复杂度提前混入 extractor 层

## Minimum Viable Tasks (for team execution)

### MVT-1 — 提取器实现
- Owner lane: implementation
- Files: `internal/service/session/codex_extractor.go`
- Done when: 提取顺序与校验规则落地且 API 稳定

### MVT-2 — 单测与 parity fixture
- Owner lane: verification
- Files: `internal/service/session/codex_extractor_test.go`, `tests/go-parity/fixtures/*`, `tests/go_parity/*`
- Done when: Node 关键案例全部在 Go 被锁定

### MVT-3 — 切片说明与后续接口边界
- Owner lane: documentation/review
- Files: `reports/go-rewrite/*`
- Done when: 当前完成边界与后续未做项被明确记录

## Available-Agent-Types Roster
- `executor`
- `test-engineer`
- `verifier`
- `architect`
- `writer`
- `explore`

## Follow-up Staffing Guidance

### Recommended team lanes
1. `executor x1` — implementation lane（high）
   - 负责 `internal/service/session/*` 纯逻辑实现
2. `test-engineer x1` — verification lane（medium）
   - 负责 parity fixtures、contract tests、报告
3. leader verification
   - 最终由 leader 跑全量测试/build，并做集成/提交

### Launch hint
```bash
OMX_TEAM_WORKER_CLI=codex OMX_TEAM_WORKER_LAUNCH_ARGS='--model gpt-5.4 -c model_reasoning_effort=high' \
  omx team 2:executor "Use .omx/plans/prd-go-rewrite-codex-session-extraction.md as the brief. Lane A owns internal/service/session/* implementation for Codex session extraction parity. Lane B owns tests/go-parity fixtures, contract tests, and slice verification notes. Keep diffs small, avoid unrelated files, commit before reporting."
```

## Team Verification Path
- Team proves:
  - extractor logic exists
  - unit/parity tests exist and are green
  - slice report exists
- Leader proves before shutdown:
  - `go test ./...`
  - `go build ./cmd/server`
  - no unrelated dirty files
