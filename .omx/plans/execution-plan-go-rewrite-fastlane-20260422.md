# Fastlane Execution Plan — Go Rewrite Functional Parity

更新时间：2026-04-22
模式：Ralph + Team
上下文：沿用 `.omx/plans/prd-go-rewrite-functional-parity.md` 与 `.omx/plans/test-spec-go-rewrite-functional-parity.md`

## Deep-interview note

尝试通过 `omx question` 做一轮 deep-interview 快问以锁定本轮终局目标，但当前运行时返回：
`active_execution_mode_blocked: omx question is unavailable while auto-executing workflows are active: ralph`

因此本轮按已有 PRD / test-spec / Ralph context 作为已澄清规格继续执行。

## 总体目标

持续推进到 Go rewrite toward functional parity；为加速交付，本轮优先冲 **P0 proxy core 可运行闭环**，再外扩到剩余功能等价面。

## 已完成基线

1. `/v1` auth 最小接线
2. Codex session extraction
3. session manager baseline
4. hash -> session 最小复用
5. short-context concurrent tracker
6. `/v1` session lifecycle wiring

## 详细执行路线

### Phase A — P0 proxy core runnable loop
目标：打通第一个真正可工作的代理闭环（优先 `/v1/responses`，随后 `/v1/chat/completions`）

A1. provider 数据最小闭环
- provider / endpoint / group 的最小查询面
- 仅补支撑 proxy runnable loop 所需字段与 repository

A2. request shaping / session handoff
- 将当前 auth + session lifecycle 接入真正 proxy handler
- 保持 Node session 语义，不扩到 observability

A3. upstream forwarding
- 最小 forwarder（HTTP + stream/non-stream 区分）
- 错误映射保持 Node 兼容边界

A4. response handling
- `/v1/responses` usage/status 基础处理
- 为后续 message_request 持久化预留最小落点，但不先扩出大子系统

A5. verification
- contract tests
- targeted integration tests
- `go test ./...`
- `go build ./cmd/server`
- `go vet ./...`

### Phase B — supporting parity for proxy correctness
B1. message_request repository + minimal persistence
B2. provider selection 最小可用版本
B3. cost / basic rate-limit hooks（只补 proxy 必需部分）
B4. `/v1/chat/completions` parity pass

### Phase C — admin/api parity
C1. UI 强依赖 `/api/actions/*` 最小兼容面
C2. system settings / providers / keys / users
C3. OpenAPI/docs endpoints

### Phase D — full parity hardening
D1. replay tests / golden tests
D2. observability / usage / active session
D3. remaining admin surface
D4. staged cutover checks

## 本轮 Team 冲刺目标

### Sprint Goal
交付 **`/v1/responses` 第一个真正可运行的最小代理闭环**，并保留当前 session/auth 语义与验证链。

### Team staffing
- Worker 1 — proxy handler / forwarder lane
- Worker 2 — provider/repository support lane
- Worker 3 — regression / tests / contract lane

### 本轮明确排除
- active-session ZSET
- provider binding smart update
- session observability 全量面
- 完整 admin parity
- 非 P0 的 dashboard / notifications / webhook 系统

## Ralph completion gate for this sprint

- `/v1/responses` 不再返回 501，而是进入真实最小代理链
- session lifecycle 仍保持对称
- tests/build/vet 通过
- architect review 通过
