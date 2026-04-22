# PRD: Go Rewrite Completion Roadmap

## Requirements Summary

目标不是再做一份抽象愿景，而是把“从当前已有进度到功能等价完成”的剩余工作拆成一系列**可连续执行的最小切片**，并让每一轮都能被 team 并行执行、leader 汇总验证、干净提交。

## Current Verified Progress

### 已完成
1. 规划基线
   - `.omx/plans/prd-go-rewrite-functional-parity.md`
   - `.omx/plans/test-spec-go-rewrite-functional-parity.md`
2. Auth 最小切片
   - `internal/service/auth/*`
   - `internal/handler/v1/proxy.go`
   - `internal/handler/v1/proxy_test.go`
   - `reports/go-rewrite/auth-parity-scaffold.md`
3. 迁移参考文档
   - `docs/go-parity-matrix.md`
   - `docs/go-migration-notes.md`

### 当前工作区缺口
- `/v1/models` 最小兼容切片尚未回到此工作区
- Codex session extractor 尚未回到此工作区
- `/v1/responses`、request context、request_sequence、proxy core、admin API 仍未落地

## End-State Definition

当以下 4 类能力均达到可验证 parity 时，视为“完成重写”：
1. **Core proxy parity**：`/v1/messages`, `/v1/chat/completions`, `/v1/responses`, `/v1/models`
2. **Stateful behavior parity**：session / request sequence / rate-limit / circuit breaker / cost
3. **Data/admin parity**：repository 缺口补齐、关键 admin API 兼容
4. **Verification/cutover readiness**：contract/replay/integration/shadow evidence 齐全

## Delivery Principles

1. 先补“下一刀的共同前置能力”，再补更宽的代理功能
2. 一次只推进一个可验证切片，不同时做完整代理链
3. Team 只做并行、独立、边界清晰的 lane
4. Leader 统一汇总、验证、提交，避免 team 自动提交污染主线历史

## Phased Execution Plan

### Phase A — 回补当前工作区到已知最新基线
目标：把已在另一执行线完成的最小成果安全带回当前工作区
- Slice A1: `/v1/models` 最小兼容切片
- Slice A2: Codex session extraction 基线

### Phase B — `/v1/responses` 最小接线
目标：建立 Responses 路径最小 request parsing + session context
- Slice B1: request body parse + Codex session extractor 接线
- Slice B2: `/v1/responses` contract tests + report

### Phase C — Request/session state 基线
目标：让 session 从“提取”进入“可持有的请求上下文”
- Slice C1: request context struct / session metadata carrier
- Slice C2: request_sequence 分配最小基线（先纯逻辑，再 Redis）
- Slice C3: message_request repository 基线

### Phase D — Proxy core 初版
目标：从 notImplemented 迈向真实转发骨架
- Slice D1: provider selector 最小版（不做完整健康与熔断）
- Slice D2: forwarder 最小版
- Slice D3: response handler / non-streaming baseline
- Slice D4: SSE/stream baseline

### Phase E — Stateful parity
- Slice E1: session manager + tracker
- Slice E2: rate-limit / fail-open
- Slice E3: circuit breaker + retry
- Slice E4: cost calculation / usage writeback

### Phase F — Admin/API parity
- Slice F1: system settings / prices / provider groups / endpoints repository + handler
- Slice F2: `/api/actions/openapi.json|docs|scalar`
- Slice F3: users / keys / providers CRUD 兼容层
- Slice F4: usage/audit/logs 关键读路径

### Phase G — Cutover readiness
- Slice G1: contract snapshots
- Slice G2: replay harness
- Slice G3: DB-backed integration tests
- Slice G4: frontend smoke + shadow mode checklist

## Acceptance Criteria for This Roadmap

1. 每个 Phase 都能拆成独立 team lane，且每轮完成后有明确可验证产物。
2. 下一切片必须明确为 `/v1/responses` 最小接线，而不是直接跳完整 session manager / proxy core。
3. 路线图能解释“为什么先做这个，再做那个”。
4. 路线图要包含 team 执行前置条件：干净 workspace、依赖可拉取、验证命令可执行。

## Team Staffing Model

### 常规一轮 team（推荐 3 lanes）
1. `architect x1`（high）
   - 只做边界约束、review、共享接口守门
2. `executor x1`（high）
   - 主实现 lane
3. `test-engineer x1`（medium）
   - fixture / contract / report / verification lane

### leader 责任
- 固化计划
- 清理 workspace
- 启动/监控 team
- 汇总并做全量验证
- 干净提交

## Blocking Preconditions Before Team

1. `.git` 指向已修复并可正常 `git status`
2. 当前 dirty baseline 需被 checkpoint / 清理，否则 `omx team` worktree 模式会拒绝启动
3. Go modules 需可下载或缓存可用，否则验证无法跑通
4. `.gitignore` 需忽略本地运行态噪音（例如 `.omx/logs/`, `.omx/state/`, `.cache/`, `server.exe`, `.codex`）

## Recommended Immediate Next Slice

### Next Slice
`/v1/responses` minimal wiring with Codex session extraction

### Why this next
- auth 已有
- session extractor 已是最自然的前置
- 这一步能把“纯逻辑”推进到“真实入口接线”，但不会过早引入 Redis / provider selector 复杂度

## Launch Hint

在阻塞解除后，优先执行：
```bash
omx team 3:executor "Use .omx/plans/prd-go-rewrite-responses-minimal-wiring.md as the brief. Lane A owns /v1/responses minimal handler wiring and request parsing. Lane B owns fixtures, contract tests, and reports. Lane C acts as boundary-review/cleanup support. Keep diffs small and avoid unrelated files."
```
