# Test Spec: Claude Code Hub Go 重写功能等价验证

## Scope

验证 Go 重写版本与当前 Node 版本在以下层面保持功能等价：

1. 数据语义
2. 代理行为
3. 管理 API 契约
4. 前端继续使用时的兼容性

## Baseline Evidence

- Proxy routing surface：`src/app/v1/[...route]/route.ts:41-57`
- Proxy handler pipeline：`src/app/v1/_lib/proxy-handler.ts:17-119`
- Provider selection：`src/app/v1/_lib/proxy/provider-selector.ts:157-255`
- Response handling：`src/app/v1/_lib/proxy/response-handler.ts:1-260`
- Session semantics：`src/lib/session-manager.ts:101-258`
- Rate limit semantics：`src/lib/rate-limit/service.ts:115-260`
- OpenAPI / action adapter：`src/lib/api/action-adapter-openapi.ts:39-107`, `src/app/api/actions/[...route]/route.ts:21-37`
- Data model baseline：`src/drizzle/schema.ts:42-172`, `src/drizzle/schema.ts:175-260`, `src/drizzle/schema.ts:446-520`, `src/drizzle/schema.ts:706-818`, `src/drizzle/schema.ts:929-1047`

## Acceptance Test Matrix

### 1. Repository / Data parity

- [ ] Go repository layer can read/write `users`
- [ ] Go repository layer can read/write `keys`
- [ ] Go repository layer can read/write `providers`
- [ ] Go repository layer can read/write `provider_groups`
- [ ] Go repository layer can read/write `provider_endpoints`
- [ ] Go repository layer can read/write `message_request`
- [ ] Go repository layer can read/write `model_prices`
- [ ] Go repository layer can read/write `system_settings`
- [ ] Go repository layer can read/write `usage_ledger`
- [ ] Go repository layer can read/write `audit_log`

### 2. Session / Rate limit parity

- [ ] Session ID extraction matches Node rules for Claude/Codex paths
- [ ] Session request sequence semantics remain unique and Redis-backed
- [ ] 5h rolling quota behavior matches Node implementation
- [ ] daily fixed reset behavior matches Node implementation
- [ ] daily rolling window behavior matches Node implementation
- [ ] fail-open behavior when Redis unavailable matches current expectation

### 3. Proxy parity

- [ ] `/v1/chat/completions` can route and stream successfully
- [ ] `/v1/responses` can route and stream successfully
- [ ] `/v1/messages` behavior is implemented or explicitly mapped if staged later
- [ ] `/v1/models` returns compatible model catalog shape
- [ ] provider selection respects group / priority / model compatibility
- [ ] circuit breaker / retry path does not regress
- [ ] cost calculation and persistence semantics do not regress materially

### 4. Admin / API parity

- [ ] `/api/actions/openapi.json` exists
- [ ] `/api/actions/docs` exists
- [ ] `/api/actions/scalar` exists
- [ ] key CRUD endpoints preserve response shape
- [ ] user CRUD endpoints preserve response shape
- [ ] provider CRUD endpoints preserve response shape
- [ ] system settings read/write endpoints remain compatible
- [ ] direct REST endpoints used by UI remain available

### 5. Frontend compatibility smoke

- [ ] settings/config page can load and save
- [ ] settings/providers page can list providers
- [ ] dashboard/users page can list users
- [ ] dashboard/logs page can query logs
- [ ] notifications page can load settings

## Test Types

## Unit Tests

- repository method tests
- session ID extraction tests
- Redis key/TTL behavior tests
- provider selector tests
- response parser / SSE tests
- cost calculator tests

## Integration Tests

- PostgreSQL + Redis environment
- auth → session → rate-limit → provider-selection chain
- action/OpenAPI generation
- provider CRUD + endpoint probing

## Contract Tests

- Golden JSON snapshots comparing Node vs Go responses for:
  - users list
  - providers list
  - system settings payload
  - model list payload
  - error response shape

## Replay Tests

- capture representative upstream proxy requests/responses in Node
- replay through Go
- compare:
  - status code
  - response format family
  - critical fields
  - usage/cost persistence side effects

## E2E Tests

- create user -> create key -> send proxy request -> verify logs/usage/quota
- configure provider -> test provider -> call through provider
- export logs / import DB progress endpoints

## Exit Criteria

1. P0 repository/service/proxy/admin parity tests green
2. no known mismatches in action-result response envelope
3. no known Redis fail-open regressions
4. current Next.js frontend can complete smoke flows against Go API for covered modules

## Evidence Collection

- test reports under `reports/go-rewrite/`
- parity matrix under `docs/go-parity-matrix.md`
- request/response goldens under `tests/go-parity/fixtures/`
- migration notes under `docs/go-migration-notes.md`
