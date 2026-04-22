# admin usage-logs minimal baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/usage-logs`

## 最小语义

- 复用 admin token 鉴权
- 调用 `MessageRequestRepository.ListRecent(limit)`
- 返回 action envelope：
  - `{"ok":true,"data":{"logs":[...],"count":N}}`
- 支持最小 `limit` 查询参数
- 非法 `limit` 返回 `400`

## 边界

本轮只补最小 recent logs 列表，不做复杂 filters、stats、export、session chain、error detail drilling 或 my-usage 视图。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
