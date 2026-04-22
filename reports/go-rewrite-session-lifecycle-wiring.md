# Session lifecycle wiring 最小切片

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/proxy-handler.ts`
- `../claude-code-hub plus/src/lib/session-manager.ts`
- `../claude-code-hub plus/src/lib/session-tracker.ts`

## 本次新增

- `internal/handler/v1/proxy.go`
- `internal/handler/v1/proxy_test.go`
- `cmd/server/main.go`
- `internal/service/session/manager.go`

## 本轮边界

只补 `/v1` 请求生命周期里的最小 session tracker 接线：

- POST `/v1/messages`
- POST `/v1/chat/completions`
- POST `/v1/responses`

在鉴权通过后：

1. 解析请求体
2. 提取 client session id
3. 生成/复用当前 session id
4. 请求开始前 `IncrementConcurrentCount`
5. 请求结束后 `DecrementConcurrentCount`

本轮**不包含**：

- active session ZSET
- provider binding
- session info / usage / messages 观测面
- 真正的 proxy forward / response handling

## 对齐要点

### 生命周期位置

与 Node `handleProxyRequest` 保持同类边界：

- 并发计数增加发生在鉴权/请求解析之后、handler 执行之前
- 并发计数减少发生在 `defer`，确保无论成功失败都执行

### 路由范围

只对会产生代理请求的 POST 路由启用并发计数；`GET /v1/models` 不接入。

### session 输入

- Claude 路径优先取 `body.messages`
- Codex/Responses 路径优先取 `body.input`
- 通过 `GetOrCreateSessionID` 统一生成最终 session id

## 验证记录

- PASS `gofmt.exe -w cmd/server/main.go internal/handler/v1/proxy.go internal/handler/v1/proxy_test.go tests/go_parity/proxy_auth_contract_test.go`
- PASS `go test ./internal/handler/v1 ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 下一步建议

下一刀再把这条 session lifecycle 接线移入真正的 proxy handler；在那之前不要扩展到 provider / active session / dashboard。
