# Go Rewrite Cutover Readiness Snapshot — 2026-04-26

## Summary

当前 Go rewrite 已进入**切换前验证阶段**，核心 direct/admin/auth 与 `/v1` 主链都已具备可验证的功能闭环。

基于当前仓库状态的务实判断：

- direct/admin/telemetry: 95%~97%
- auth/session (web/direct): 94%~96%
- `/v1` proxy 主线: 93%~96%
- overall rewrite: 94%~96%

这不是“所有工作都完成”的声明，而是说明：
1. 大多数用户可感知接口已具备 Node 风格语义；
2. 代理主链已具备 session / quota / fallback / minimal breaker 等关键行为；
3. 剩余工作集中在高阶策略与最终切换证据，而不是大面积缺功能。

## Recently Completed High-Value Parity Slices

### Direct/Admin/Auth
- `/api/system-settings` / `/api/actions/system-settings` cleanup 配置更新对齐
- `/api/prices` 直接路由 shape + `size` alias 对齐
- `/api/availability/endpoints/probe-logs` query 校验与 response shape 对齐
- `/api/ip-geo/:ip` cache header + lang 默认值对齐
- `/api/auth/login` success payload / failure taxonomy / secure cookie / no-store headers / dual & opaque modes
- `/api/auth/logout` dual & opaque session revoke 语义
- opaque auth-token cookie 直连读取（direct/web）
- `/api/version` release/no-release/error/dev-branch compare 语义对齐

### `/v1` Proxy / Stateful
- `/v1/messages` warmup intercept
- warmup request logging with `blockedBy=warmup`
- session lifecycle wiring
- Codex request-side session extraction
- non-streaming `prompt_cache_key` session promotion
- streaming SSE `prompt_cache_key` session promotion
- key/user concurrent session gate
- provider concurrent fallback filter
- key/user total / 5h / daily / weekly / monthly preflight checks
- provider total / 5h / daily / weekly / monthly candidate filtering
- same-provider transport retry
- multi-provider transport fallback
- resource-not-found (404) fallback to next provider
- minimal provider circuit-breaker: open skip, half-open recovery, failure accounting

## Verification Evidence

### Repeated green verification loop
The following commands are still green after the latest slices:

```bash
go test ./...
go build ./cmd/server
go vet ./...
```

### Repo-local contract coverage already present
Representative coverage exists for:
- `tests/go_parity/proxy_messages_minimal_contract_test.go`
- `tests/go_parity/proxy_messages_count_tokens_minimal_contract_test.go`
- `tests/go_parity/proxy_chat_completions_minimal_contract_test.go`
- `tests/go_parity/proxy_responses_minimal_contract_test.go`
- `tests/go_parity/proxy_models_minimal_contract_test.go`
- `tests/go_parity/session_manager_contract_test.go`
- `tests/go_parity/codex_session_extractor_contract_test.go`

### High-value local handler coverage
Representative stateful/proxy handler coverage exists in:
- `internal/handler/v1/proxy_test.go`
- `internal/handler/api/*_test.go`

## Remaining Gaps Before Strong Cutover Confidence

### 1. Retry / fallback error categories still not fully matched
Still worth tightening:
- more nuanced 4xx/5xx category splits
- richer `resource_not_found` / `system_error` / `retry_failed` parity
- streaming-specific retry edges

### 2. Circuit breaker still intentionally minimal
Still missing or simplified:
- Redis-backed / cross-process state sync
- richer half-open thresholds
- more complete non-network failure accounting
- stronger provider selection integration with circuit metadata

### 3. Quota / lease system still simplified
Still missing or intentionally deferred:
- lease-aware quotas
- more exact rolling-window cache parity
- provider lease interactions

### 4. Cutover proof is not yet complete
Still recommended before calling the rewrite “done”:
- refresh/expand replay evidence
- DB-backed integration sweep
- shadow or smoke checklist against a realistic environment

## Recommended Next Steps

1. Strengthen retry/fallback category parity
2. Expand circuit-breaker fidelity (state sync / half-open policy)
3. Produce a focused cutover checklist + replay/integration evidence pack

## Current Branch State

- local branch: `go-rewrite-main`
- push target: `origin/go-rewrite-main-telemetry-parity`

## Working Tree Note

Do not commit:
- `.omx/context/go-rewrite-telemetry-admin-parity-20260425T031333Z.md`
