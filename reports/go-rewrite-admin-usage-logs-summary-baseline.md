# admin usage-logs summary baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/usage-logs/summary`

## 最小语义

- 复用 admin token 鉴权
- 调用 `MessageRequestRepository.GetSummary`
- 支持最小过滤：
  - `model`
  - `endpoint`
  - `statusCode`
- 返回 action envelope：
  - `{"ok":true,"data":{"totalRequests":...,"totalTokens":...}}`

## 边界

本轮只补最小 summary，不做 cost 聚合、cache token 分层、导出统计、时间范围过滤或 dashboard 复杂汇总。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
