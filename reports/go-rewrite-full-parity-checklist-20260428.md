# Go Rewrite Full Parity Checklist — 2026-04-28

## 目标

把“接近 Node”切换为“以 Node 为唯一标准的全量复刻执行模式”。

当前结论：

- **不是 100% 完全复刻**
- 当前更接近 **核心主链高覆盖 + 若干外围能力缺失/简化**
- 本清单作为后续执行与收口基线

## 已有证据

- 代码级验证通过：
  - `go test ./...`
  - `go build ./cmd/server`
  - `go vet ./...`
- 核心 `/v1` 主链已具备：
  - auth
  - session lifecycle
  - request rectifiers
  - request filters / sensitive words
  - provider fallback / retry
  - minimal breaker
  - non-streaming / streaming error normalization

## 明确未完成/未完全对齐

### P0 数据层缺口

以下资源在 Node 存在，但 Go 当前仍缺失或未完整实现：

1. `provider_groups`
2. `provider_vendors`
3. `provider_endpoints`
4. `provider_endpoint_probe_logs`
5. `usage_ledger`
6. `audit_log`

### P1 运行时语义仍简化

1. circuit breaker 仍偏最小实现
   - 缺 Redis/cross-process state sync
   - 缺更细 half-open 策略
   - 缺更完整 failure accounting

2. quota / lease 仍非 Node 完整语义
   - 缺 lease-aware quota
   - 缺更精确 rolling window parity
   - 缺 provider lease interaction

3. retry / fallback category 仍未完全收口
   - richer `resource_not_found` / `system_error` / `retry_failed`
   - 更完整 streaming retry edges

### P2 admin / docs / observability 仍非最终态

1. Swagger / Scalar 仍是 placeholder
2. 若干 cache stats 仍是 placeholder note
3. cutover proof / replay / shadow evidence 仍未完成

### P3 live proof 未闭环

当前还缺：

- DB-backed integration sweep
- realistic smoke
- shadow/replay-style evidence

## 本轮开始执行的第一刀

### 已开始补 `provider_groups`

本轮已落地：

- `internal/model/provider_group.go`
- `internal/repository/provider_group_repo.go`
- `internal/repository/provider_group_repo_test.go`
- `internal/repository/factory.go` 新增 `ProviderGroup()`
- `internal/database/bootstrap.go` / `AutoMigrate` 接入 `provider_groups`

当前能力：

- list
- get by id
- get by name
- create
- update fields
- delete（保护 default group）
- ensure exists
- group cost multiplier cache

### 已继续补第一批 P0 资源

本轮新增：

- `internal/model/provider_vendor.go`
- `internal/model/provider_endpoint.go`
- `internal/model/provider_endpoint_probe_log.go`
- `internal/model/usage_ledger.go`
- `internal/model/audit_log.go`
- `internal/repository/provider_vendor_repo.go`
- `internal/repository/provider_endpoint_repo.go`
- `internal/repository/provider_endpoint_probe_log_repo.go`
- `internal/repository/usage_ledger_repo.go`
- `internal/repository/audit_log_repo.go`
- `internal/repository/usage_ledger_repo_test.go`
- `internal/repository/audit_log_repo_test.go`

并接入：

- `internal/repository/factory.go`
- `internal/database/bootstrap.go`

当前状态：

- `provider_groups`：基础仓储已完成
- `provider_vendors`：基础仓储已完成
- `provider_endpoints`：基础仓储已完成
- `provider_endpoint_probe_logs`：基础仓储已完成
- `usage_ledger`：基础模型/仓储已完成
- `audit_log`：基础模型/仓储已完成

尚未完成：

- provider_endpoints / probe_logs 与 availability / selector / runtime probing 的深度接线
- usage_ledger / audit_log 与现有 message_request / admin action / telemetry 的生产语义接线

## 后续执行顺序

### 第一阶段：补齐 P0 资源表与仓储

1. `provider_vendors`
2. `provider_groups` ✅ started
3. `provider_endpoints`
4. `provider_endpoint_probe_logs`
5. `usage_ledger`
6. `audit_log`

### 第二阶段：把运行时最小实现补成 Node 语义

1. provider endpoint aware selection
2. probe log persistence / availability integration
3. stronger breaker fidelity
4. lease-aware quota

### 第三阶段：完整 live / cutover 证明

1. local smoke
2. DB + Redis realistic verification
3. replay / shadow evidence pack

## 当前状态标签

- **方向已切换到“全量复刻模式”**
- **第一刀已开始落在 provider_groups**
- **仍不能宣称 100% parity 完成**
