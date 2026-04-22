# message_request minimal persistence baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `internal/repository/message_request_repo.go`
- `internal/repository/factory.go`
- `internal/handler/v1/proxy.go`
- `internal/handler/v1/proxy_test.go`

## 最小语义

在三条主 POST 代理端点的最小 runnable loop 上增加 best-effort request log 落库：

- `/v1/messages`
- `/v1/chat/completions`
- `/v1/responses`
- （附带 `/v1/messages/count_tokens` 仍走同一最小代理链，但不额外扩展更复杂 finalize 语义）

当前已形成最小 create + terminal update seam，并落以下稳定字段：

- `provider_id`
- `user_id`
- `key`
- `model`
- `original_model`
- `session_id`
- `request_sequence`
- `status_code`
- `duration_ms`
- `api_type`
- `endpoint`
- `user_agent`
- `messages_count`
- `error_message`（最小错误场景）

## 边界

本轮**不包含**：

- Node 式 create + response finalize/update 双阶段
- token / usage / cost 写回
- provider_chain / special_settings 完整更新
- async write buffer
- usage logs / admin read 面

## 对齐说明

- 持久化使用现有 session manager 提供的 `session_id` / `request_sequence`
- handler 中 best-effort 写库失败不会阻断代理响应
- 当前已从 one-shot create 演进到更接近 Node 的 `create -> terminal update` seam，但仍未进入完整 response finalize/update 子系统
- 该切片是 request log persistence 的基线 seam，而不是完整 response-handler 子系统

## 验证记录

- PASS `go test ./internal/handler/v1`
- PASS `go test ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 当前完成定义

- request log repository 已存在
- factory 已可注入 `MessageRequestRepository`
- `/v1` proxy handler 已在 happy path 上写入最小 request log
- 持久化失败不会破坏代理请求成功响应

## 下一步建议

下一刀优先做：

1. 把 message_request 的 create/update seam 扩展到更多响应细节（仍不带 usage/cost）；或
2. 开始补最小 admin / actions 兼容面。
