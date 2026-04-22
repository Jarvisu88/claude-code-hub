# /v1/chat/completions minimal runnable proxy loop

更新时间：2026-04-22  
负责人：Codex

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/proxy-handler.ts`
- `../claude-code-hub plus/src/app/v1/_lib/url.ts`

## 本次新增

- `internal/handler/v1/proxy.go`
- `internal/handler/v1/proxy_test.go`
- `tests/go_parity/proxy_chat_completions_minimal_contract_test.go`
- `tests/go-parity/fixtures/proxy-chat-completions-minimal-cases.json`

## 本轮边界

交付 `/v1/chat/completions` 第一个真正可运行的最小代理闭环：

1. 鉴权
2. session lifecycle
3. 选择单个 `openai-compatible` provider
4. 构建上游 URL
5. 透传请求到上游
6. 原样回写响应

本轮**不包含**：

- provider failover / retry
- OpenAI streaming 处理
- usage/cost writeback
- `/v1/messages` 真正 runnable 化
- guard pipeline / observability / admin parity

## 对齐要点

- `/v1/chat/completions` 已不再 501
- 使用与 `/v1/responses` 同一 auth + session lifecycle 中间件链
- provider 仅接受 `openai-compatible`
- URL / header relay 复用同一最小代理逻辑

## 验证记录

- PASS `gofmt.exe -w internal/handler/v1/proxy.go internal/handler/v1/proxy_test.go tests/go_parity/proxy_chat_completions_minimal_contract_test.go`
- PASS `go test ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 当前完成定义

当前切片已满足最小 runnable loop：

- `/v1/chat/completions` 进入真实 handler
- 成功请求不再返回 501
- 有 handler 层 happy-path 测试
- 有 go_parity contract 测试

## 下一步建议

下一刀优先做：

1. `/v1/messages` 最小 runnable loop；或
2. 为 `/v1/responses` + `/v1/chat/completions` 补最小上游错误映射 / timeout handling
