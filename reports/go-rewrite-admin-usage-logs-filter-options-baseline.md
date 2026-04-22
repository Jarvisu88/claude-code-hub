# admin usage-logs filter-options baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/usage-logs/filter-options`

## 最小语义

返回 usage logs 的最小筛选选项：
- `models`
- `statusCodes`
- `endpoints`

返回 action envelope：
- `{"ok":true,"data":{"models":[...],"statusCodes":[...],"endpoints":[...]}}`

## 边界

本轮只补最小筛选选项，不做缓存、用户维度过滤或更复杂联想逻辑。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
