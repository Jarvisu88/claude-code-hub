# Session manager hash/session mapping 最小切片

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/lib/session-manager.ts`
- `../claude-code-hub plus/src/lib/claude-code/metadata-user-id.ts`
- `../claude-code-hub plus/src/app/v1/_lib/codex/session-extractor.ts`

## 本次新增

- `internal/service/session/manager.go`
- `internal/service/session/manager_test.go`
- `tests/go-parity/fixtures/session-manager-hash-cases.json`
- `tests/go_parity/session_manager_hash_contract_test.go`

## 本轮边界

只补 Node `SessionManager` 中最小可验证的 **hash -> session 复用链路**：

- `calculateMessagesHash`
- `getOrCreateSessionId`
- `hash:{contentHash}:session` 映射初始化
- `session:{sessionId}:key`
- `session:{sessionId}:last_seen`
- 客户端已传 `session_id` 时的最小 TTL 刷新

本轮**仍不包含**：

- 短上下文并发检测 / `SessionTracker`
- provider binding / smart update
- active session tracking
- session message 存储
- `/v1` handler 接线

## 对齐要点

### calculateMessagesHash

与 Node 保持一致：

1. 仅计算前 3 条 messages
2. `content` 为字符串时直接拼接
3. `content` 为数组时只提取 `type === "text"` 的 `text`
4. 使用 `|` 拼接后做 SHA-256
5. 截取前 16 个十六进制字符

### getOrCreateSessionId

与 Node 当前最小语义对齐：

1. 有客户端 `session_id` 时直接复用
2. 无客户端 `session_id` 时退化到内容 hash
3. Redis 中已有 `hash:{hash}:session` 时复用已有 session
4. 未命中时生成新 `sess_{timestamp}_{random}` 并写入最小映射
5. Redis 出错或 hash 不可计算时 fail-open 生成新 session

## 验证记录

- PASS `go test ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 下一步建议

下一刀优先补 `SessionTracker`/短上下文并发检测，避免在 `/v1/responses` 真正接线前把 provider binding 或 active session tracking 一起带进来，保持 session 服务边界收敛。
