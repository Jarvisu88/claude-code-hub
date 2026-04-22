# /v1/messages minimal runnable proxy loop

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/proxy-handler.ts`
- `../claude-code-hub plus/src/app/v1/_lib/url.ts`
- `../claude-code-hub plus/src/app/v1/_lib/proxy/forwarder.ts`

## 本次新增

- `internal/handler/v1/proxy.go`
- `internal/handler/v1/proxy_test.go`
- `tests/go_parity/proxy_messages_minimal_contract_test.go`
- `tests/go-parity/fixtures/proxy-messages-minimal-cases.json`

## 本轮边界

交付 `/v1/messages` 第一个真正可运行的最小代理闭环：

1. 鉴权
2. session lifecycle
3. 选择单个 `claude` / `claude-auth` provider
4. 构建上游 URL
5. 透传请求到上游
6. 原样回写响应

本轮**不包含**：

- guard pipeline
- rate limit / circuit breaker / failover
- stream finalization
- usage/cost writeback
- `/v1/messages/count_tokens`
- admin parity

## 对齐要点

- `/v1/messages` 已不再 501
- 继续复用现有 `AuthMiddleware` + `SessionLifecycleMiddleware`
- Claude 类型 provider 最小 header 语义：
  - `Authorization: Bearer <provider-key>`
  - `x-api-key: <provider-key>`
  - `claude-auth` provider 会移除 `x-api-key`
- `buildProxyURL` 追加支持已有 `/messages` 端点根路径时不重复拼接

## 验证记录

- PASS `gofmt.exe -w internal/handler/v1/proxy.go internal/handler/v1/proxy_test.go tests/go_parity/proxy_messages_minimal_contract_test.go`
- PASS `go test ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 当前完成定义

当前切片已满足最小 runnable loop：

- `/v1/messages` 进入真实 handler
- 成功请求不再返回 501
- 有 handler 层 happy-path 测试
- 有 go_parity contract 测试

## 下一步建议

下一刀优先补：

1. `/v1/models` / `/responses/models` / `/chat/completions/models` 最小可用返回；或
2. 为三个 runnable 端点统一补最小 provider error mapping / timeout handling
