# admin actions create baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `POST /api/actions/users`
- `POST /api/actions/keys`
- `POST /api/actions/providers`

## 最小语义

- 复用 admin token 鉴权
- 绑定最小 JSON body
- 调用 repository `Create`
- 成功返回 `201` + `{"ok":true,"data":...}`
- 非法 JSON 或缺少必填字段返回 `400`

## 边界

本轮只做最小创建能力，不补更新/删除、复杂验证、搜索去重、批量操作或更深 action schema。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
