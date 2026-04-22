# Codex session extraction 最小切片

更新时间：2026-04-22
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/codex/session-extractor.ts`
- `../claude-code-hub plus/src/lib/session-manager.ts`

## 本次新增

- `internal/service/session/codex_extractor.go`
- `internal/service/session/codex_extractor_test.go`
- `tests/go-parity/fixtures/codex-session-extractor-cases.json`
- `tests/go_parity/codex_session_extractor_contract_test.go`

## 本轮边界

只锁定 **Codex 请求 session_id 提取规则**，暂不接入：

- Redis session store
- request sequence
- `/v1/responses` handler
- message_request 落库

## 对齐语义

提取优先级与 Node 保持一致：

1. `headers["session_id"]`
2. `headers["x-session-id"]`
3. `body.prompt_cache_key`
4. `body.metadata.session_id`
5. `body.previous_response_id`（加 `codex_prev_` 前缀）

同时保留 Node 的输入校验约束：

- 去除首尾空白
- 最短长度 `21`
- 最长长度 `256`
- 仅允许 `A-Z a-z 0-9 _ - . :`

## 验证记录

- PASS `go test ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`

## 续作建议

下一刀直接把该提取器接到后续 `session.Manager`/`/v1/responses` 最小链路里，不要在进入 Redis / request_sequence 之前扩散到更大范围。
