# Session manager baseline 最小切片

更新时间：2026-04-22
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/lib/session-manager.ts`
- `../claude-code-hub plus/src/lib/claude-code/metadata-user-id.ts`
- `../claude-code-hub plus/src/app/v1/_lib/codex/session-extractor.ts`

## 本次新增

- `internal/service/session/manager.go`
- `internal/service/session/manager_test.go`
- `tests/go-parity/fixtures/session-manager-extract-cases.json`
- `tests/go_parity/session_manager_contract_test.go`

## 本轮边界

只补 session manager 的最小公共语义：

- Claude/Codex client session_id 提取
- session_id 生成
- request sequence 递增
- request count 查询
- Redis 不可用时 fail-open fallback sequence

本轮**不包含**：

- `getOrCreateSessionId`
- hash-based session reuse
- provider binding
- active session tracking
- message_request 持久化
- `/v1` handler 接线

## 对齐要点

### 提取语义

- Codex 路径继续复用已有 `ExtractCodexSessionID`
- Claude 路径支持：
  - `metadata.user_id` JSON 格式
  - `metadata.user_id` legacy 格式
  - `metadata.session_id` 兜底

### request sequence

- Redis key: `session:{sessionId}:seq`
- 首次 `INCR` 成功后设置 TTL
- Redis 不可用或报错时，不返回固定 `1`，而是使用时间戳 + 随机数生成 fallback sequence，保持 fail-open

## 验证记录

- PASS `go test ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`

## 下一步建议

下一刀把 `getOrCreateSessionId` 与最小 `hash -> session` 映射补上，但在引入 provider binding / active session tracking 前先保持边界收敛。
