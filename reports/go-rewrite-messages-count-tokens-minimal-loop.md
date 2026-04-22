# /v1/messages/count_tokens minimal runnable proxy loop

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `POST /v1/messages/count_tokens`
- `tests/go_parity/proxy_messages_count_tokens_minimal_contract_test.go`
- `tests/go-parity/fixtures/proxy-messages-count-tokens-minimal-cases.json`

## 最小语义

- 鉴权
- 选择单个 `claude` / `claude-auth` provider
- 将 `/v1/messages/count_tokens` 透传到上游
- 复用最小 header relay 与 URL 拼接
- 返回上游原始响应

## 边界

- 仍不做 count_tokens 专用 guard / pricing / persistence
- 不把它纳入并发 session 跟踪例外之外的新复杂逻辑
- 只交付最小 runnable loop

## 验证记录

- PASS `go test ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
