# Session short-context / tracker 最小切片

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/lib/session-manager.ts`
- `../claude-code-hub plus/src/lib/session-tracker.ts`

## 本次新增

- `internal/service/session/tracker.go`
- `internal/service/session/manager.go`
- `internal/service/session/manager_test.go`
- `internal/config/config.go`
- `internal/config/loader.go`
- `config.example.yaml`

## 本轮边界

只补短上下文并发检测所需的最小语义：

- `session:{sessionId}:concurrent_count`
- `GetOrCreateSessionID` 在短上下文下读取并发计数
- 有并发时强制新建 session
- 无并发或长上下文时维持既有复用逻辑

本轮**不包含**：

- active session ZSET 追踪
- provider binding / smart update
- session info / usage / messages 存储
- `/v1` handler 接线

## 对齐要点

### concurrent_count

- key: `session:{sessionId}:concurrent_count`
- `INCR` 后设置 TTL `600s`
- `DECR` 后若结果 `<= 0` 删除 key
- `GET` 失败或 Redis 不可用时 fail-open 返回 `0`

### short-context detection

- 默认阈值 `2`
- 仅当 `messages` 数量 `<= threshold` 时检查并发计数
- 若已有并发请求，则不复用客户端 session，直接生成新的 `sess_{timestamp}_{random}`
- 长上下文跳过并发判重，沿用客户端 session + TTL refresh

## 验证记录

- PASS `gofmt.exe -w internal/service/session/manager.go internal/service/session/tracker.go internal/service/session/manager_test.go internal/config/config.go internal/config/loader.go tests/go_parity/session_manager_hash_contract_test.go tests/go_parity/session_manager_contract_test.go tests/go_parity/codex_session_extractor_contract_test.go`
- PASS `go test ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 下一步建议

下一刀继续把 `increment/decrementConcurrentCount` 接到 `/v1` 请求生命周期，再决定是否进入 active session ZSET 追踪；不要提前把 provider/session 观测面一起带进来。
