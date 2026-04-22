# admin usage-logs detail minimal baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/usage-logs/:id`

## 最小语义

- 复用 admin token 鉴权
- 调用 `MessageRequestRepository.GetByID`
- 返回 action envelope：
  - `{"ok":true,"data":...}`
- 非法 `:id` 返回 `400`

## 边界

本轮只补 usage log detail 的最小读取能力，不做 error detail drilling、session chain 拼接或 provider display 格式化。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
