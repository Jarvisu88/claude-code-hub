# admin actions list baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `internal/handler/api/actions.go`
- `internal/handler/api/actions_test.go`
- `cmd/server/main.go`

## 当前支持

在 `/api/actions` 下增加最小可用的管理 GET 列表端点：

- `GET /api/actions/users`
- `GET /api/actions/keys`
- `GET /api/actions/providers`

## 最小语义

- 使用管理员 token 鉴权
- 支持 `Authorization: Bearer <admin-token>` 与 `x-api-key: <admin-token>`
- 响应形状采用最小 envelope：
  - 成功：`{"ok":true,"data":...}`
  - 失败：`{"ok":false,"error":...}`

## 边界

本轮**不包含**：

- `/api/actions/openapi.json`
- `/api/actions/docs`
- `/api/actions/scalar`
- users / keys / providers 的详情 / 创建 / 更新 / 删除
- 复杂筛选、分页、搜索、统计、导入导出

## 验证记录

- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 下一步建议

下一刀优先做：

1. `/api/actions/openapi.json` 最小输出；或
2. `users / keys / providers` 的详情 GET 接口；或
3. 更接近 Node action-result 的 envelope / 错误码兼容收口。
