# admin usage-logs filters baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `/api/actions/usage-logs` 最小过滤能力

## 最小语义

支持以下查询参数：
- `limit`
- `model`
- `endpoint`
- `sessionId`
- `statusCode`

返回：
- `{"ok":true,"data":{"logs":[...],"count":N,"filters":...}}`

非法 `limit` / `statusCode` 返回 `400`。

## 边界

本轮只补最小精确过滤，不做分页游标、统计聚合、导出、session chain、error detail drilling。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
