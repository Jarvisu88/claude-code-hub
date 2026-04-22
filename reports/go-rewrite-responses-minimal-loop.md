# /v1/responses minimal runnable proxy loop

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/proxy-handler.ts`
- `../claude-code-hub plus/src/app/v1/_lib/url.ts`
- `../claude-code-hub plus/src/lib/session-manager.ts`

## 本次新增

- `internal/handler/v1/proxy.go`
- `internal/handler/v1/proxy_test.go`
- `tests/go_parity/proxy_responses_minimal_contract_test.go`
- `tests/go-parity/fixtures/proxy-responses-minimal-cases.json`
- `cmd/server/main.go`

## 本轮边界

交付 `/v1/responses` 第一个真正可运行的最小代理闭环：

1. 鉴权
2. session lifecycle
3. 选择单个可用 provider
4. 构建上游 URL
5. 透传请求到上游
6. 原样回写响应状态 / headers / body

本轮**不包含**：

- guard pipeline
- provider failover / retry
- response usage / cost writeback
- message_request 持久化
- provider binding / active-session / observability
- `/v1/chat/completions` 真正 runnable 化

## 对齐要点

### handler 链路

- `/v1/responses` 已不再走 `501 not_implemented`
- 继续复用现有 `AuthMiddleware` + `SessionLifecycleMiddleware`
- session id 仍在 handler 前生成并做并发计数增减

### provider 选择

- 当前只从 `GetActiveProviders()` 中筛选 `codex` / `openai-compatible`
- 按 `priority ASC`、`weight DESC` 选择第一个可用 provider
- 若请求体包含 `model`，则复用 `Provider.SupportsModel()` 过滤

### URL / header relay

- 保留 Node `buildProxyUrl` 的最小语义：
  - 若 base URL 已包含 `/responses` 端点根路径，则不重复拼接
  - 否则 baseURL + requestPath
- 剥离客户端 auth headers，改为 provider token
- 回写响应时剥离传输层 headers（`content-length` / `transfer-encoding` / `connection`）

## 验证记录

- PASS `gofmt.exe -w cmd/server/main.go internal/handler/v1/proxy.go internal/handler/v1/proxy_test.go tests/go_parity/proxy_auth_contract_test.go tests/go_parity/proxy_responses_minimal_contract_test.go`
- PASS `go test ./internal/handler/v1 ./internal/service/session ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 当前完成定义

当前切片已满足“第一个最小 runnable loop”：

- `/v1/responses` 进入真实 handler
- 成功请求不再返回 501
- 有 handler 层 happy-path 测试
- 有 go_parity contract 测试

## 下一步建议

下一刀只做二选一：

1. 把 `/v1/chat/completions` 按同样模式最小 runnable 化；或
2. 在 `/v1/responses` 上补最小 provider error mapping / upstream timeout handling

不要在下一刀同时扩展到 guard、provider failover、usage/cost、admin parity。
