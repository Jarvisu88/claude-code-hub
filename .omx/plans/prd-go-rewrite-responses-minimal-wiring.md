# PRD: Go Rewrite — /v1/responses Minimal Wiring

## Goal

在当前 auth 基线之上，完成 `/v1/responses` 的最小接线切片：
- 解析 request body
- 调用 Codex session extractor
- 把 session 提取结果放入统一 request context / 临时 request struct
- 补充 contract tests 与 slice report

## Scope

### In scope
- `internal/handler/v1/` 下 `/v1/responses` 最小 handler
- request payload decode
- 调用 `internal/service/session/codex_extractor.go`
- 最小 request metadata struct
- fixtures / contract tests / report

### Out of scope
- Redis session manager
- request_sequence 分配
- provider selector
- upstream forwarder
- SSE / streaming
- usage/cost writeback

## Acceptance Criteria

1. `/v1/responses` 不再只是纯 501 stub，而是至少能完成：鉴权通过 -> body parse -> session extraction -> 返回受控占位响应或内部 request state。
2. Go 侧有面向 `/v1/responses` 的 fixture / contract tests。
3. 提取到的 Codex session 信息可被 handler 层读取，而不是停留在 service 内不可消费。
4. `go test ./...` 与 `go build ./cmd/server` 通过。

## Minimum Viable Tasks

### MVT-1 — handler/request wiring
- Owner: implementation lane
- Files:
  - `internal/handler/v1/*responses*.go`
  - 可能新增 `internal/handler/v1/request_context.go` 或相似 carrier
- Done when:
  - handler 能 parse + extract + expose session result

### MVT-2 — tests / fixtures / report
- Owner: verification lane
- Files:
  - `tests/go-parity/fixtures/*responses*.json`
  - `tests/go_parity/*responses*_contract_test.go`
  - `reports/go-rewrite/*responses*.md`
- Done when:
  - request parse + session extraction 行为被 fixture 固定

### MVT-3 — boundary review / cleanup
- Owner: support lane
- Files:
  - 仅限共享 request structs / small cleanup
- Done when:
  - 共享边界清晰，未引入 provider/session manager 复杂度

## Team Staffing
- `executor x1` — handler wiring
- `test-engineer x1` — fixtures/tests/report
- `architect x1` — boundary review and cleanup support

## Verification Steps
- `go test ./...`
- `go build ./cmd/server`
- review `/v1/responses` 是否仍保持最小切片边界
