# admin actions update/delete baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `PUT /api/actions/users/:id`
- `PUT /api/actions/keys/:id`
- `PUT /api/actions/providers/:id`
- `DELETE /api/actions/users/:id`
- `DELETE /api/actions/keys/:id`
- `DELETE /api/actions/providers/:id`

## 最小语义

- 复用 admin token 鉴权
- `PUT` 绑定最小 JSON body 并调用 repository `Update`
- `DELETE` 调用 repository `Delete`
- 成功返回 `{"ok":true,"data":...}`
- 非法 `id` 或非法 JSON 返回 `400`

## 边界

本轮只补最小 update/delete 能力，不做复杂验证、幂等审计、批量操作与字段级 patch 语义。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
