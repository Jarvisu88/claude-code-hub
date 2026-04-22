# platform direct REST baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/health`
- `GET /api/health/live`
- `GET /api/health/ready`
- `GET /api/version`

## 最小语义

- `/api/health`：检查 DB / Redis 状态，返回健康摘要
- `/api/health/live`：返回存活状态
- `/api/health/ready`：检查依赖是否 ready
- `/api/version`：返回最小版本信息

## 边界

本轮只补 direct REST 最小面，不引入更复杂的 runtime metadata、build hash、依赖细分指标或 metrics 聚合。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
