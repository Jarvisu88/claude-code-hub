# admin openapi/docs minimal surface

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/openapi.json`
- `GET /api/actions/docs`
- `GET /api/actions/scalar`

## 最小语义

- `openapi.json` 返回最小 OpenAPI 文档骨架
- `docs` 返回指向 OpenAPI JSON 的最小 HTML 占位页
- `scalar` 返回指向 OpenAPI JSON 的最小 HTML 占位页

## 边界

本轮只补最小文档面，不引入完整 Swagger/Scalar 运行时集成，也不自动生成 schema/components。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
