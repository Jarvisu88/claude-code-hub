# admin actions system-settings baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/system-settings`
- `PUT /api/actions/system-settings`

## 最小语义

- 复用 admin token 鉴权
- `GET` 以 action envelope 返回当前系统设置：`{"ok":true,"data":...}`
- `PUT` 以 action envelope 返回更新后的系统设置：`{"ok":true,"data":...}`
- 非法 JSON / 非法字段值返回 `400`

## 边界

本轮只补 action 风格 system-settings 读写，不引入 Node 完整的 cache invalidation、审计事件与全量 schema。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
