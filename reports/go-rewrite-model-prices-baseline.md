# model-prices baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/prices`
- `POST /api/actions/model-prices/getModelPrices`

## 最小语义

- 复用 admin token 鉴权
- `GET /api/prices` 支持：
  - `page`
  - `pageSize`
  - `search`
- 调用 `ModelPriceRepository.ListAllLatestPricesPaginated`
- `POST /api/actions/model-prices/getModelPrices` 调用 `ListAllLatestPrices`
- 返回最小 envelope 或分页结果

## 边界

本轮只补最小模型价格读取面，不做上传、同步、冲突检查、来源过滤或完整价格管理工作流。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
