# admin usage-logs session-id suggestions baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/usage-logs/session-id-suggestions`

## 最小语义

- 复用 admin token 鉴权
- 查询参数：
  - `term`
  - `limit`
- `term` 长度小于 2 时直接返回空数组
- 调用 `MessageRequestRepository.FindSessionIDSuggestions`
- 返回 action envelope：
  - `{"ok":true,"data":[...]}`

## 边界

本轮只补最小 prefix 联想，不做用户维度权限裁剪、provider/key 过滤或复杂排名规则。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
