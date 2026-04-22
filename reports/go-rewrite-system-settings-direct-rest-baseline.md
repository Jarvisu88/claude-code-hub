# system-settings direct REST baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `internal/repository/system_settings_repo.go`
- `internal/handler/api/system_settings.go`
- `internal/handler/api/system_settings_test.go`
- `GET /api/system-settings`
- `PUT /api/system-settings`

## 最小语义

- 使用 admin token 鉴权
- `GET` 返回当前 system settings；若数据库中不存在则创建默认记录
- `PUT` 支持最小字段更新：
  - `siteTitle`
  - `allowGlobalUsageView`
  - `currencyDisplay`
  - `billingModelSource`
  - `enableAutoCleanup`
  - `enableClientVersionCheck`
  - `verboseProviderError`
  - `enableHttp2`
  - `interceptAnthropicWarmupRequests`

## 边界

本轮不引入 Node 那套完整 system-config schema/cache/invalidation/action 审计，仅补 direct REST 最小读写能力。

## 验证记录

- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
