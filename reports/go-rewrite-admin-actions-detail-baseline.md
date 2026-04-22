# admin actions detail baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/users/:id`
- `GET /api/actions/keys/:id`
- `GET /api/actions/providers/:id`

## 最小语义

- 复用已有 admin token 鉴权
- 使用仓储层 `GetByID` 获取实体
- 成功返回：`{"ok":true,"data":...}`
- 错误返回：`{"ok":false,"error":...}`
- 非法 `:id` 返回 `400`
- 资源不存在沿用 repository / AppError 语义

## 边界

本轮不包含详情页以外的更新、删除、搜索、分页扩展或 action-style POST 包装。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
