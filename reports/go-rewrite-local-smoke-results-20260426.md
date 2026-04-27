# Go Rewrite Local Smoke Results — 2026-04-26 (Updated)

## Goal

Validate whether the current Go rewrite can be launched locally and exercised through real HTTP requests, using a disposable PostgreSQL instance plus the repo-local bootstrap path.

## What Was Executed

### 1. Repository-local baseline verification

```bash
go test ./...
go build ./cmd/server
go vet ./...
```

Result:
- all three commands passed

### 2. Temporary PostgreSQL bootstrap

A temporary PostgreSQL 16 instance was started under `/tmp` and bound to:
- host: `127.0.0.1`
- port: `55432`
- database: `claude_code_hub`
- user: `postgres`
- auth: trust (temporary local smoke only)

### 3. Go service launch

The Go server was launched successfully with runtime overrides:
- PostgreSQL pointed to `127.0.0.1:55432`
- `REDIS_ENABLED=false`
- `ADMIN_TOKEN=dev-admin-token`
- `SESSION_TOKEN_MODE=dual`
- `ENABLE_SECURE_COOKIES=false`
- `BOOTSTRAP_DEV_SEED=true`
- `BOOTSTRAP_PROVIDER_BASE_URL=http://127.0.0.1:23006`

Observed startup result:
- PostgreSQL connected successfully
- local dev bootstrap executed successfully
- Redis intentionally disabled
- HTTP server listening successfully

## Bootstrap Result

The current branch now provides a minimal runtime bootstrap path that:
- auto-creates the currently modeled schema tables
- seeds minimal local dev data when `BOOTSTRAP_DEV_SEED=true`
- updates local mock providers to the current app base URL
- exposes local mock upstream handlers for:
  - `/__mock__/v1/messages`
  - `/__mock__/v1/messages/count_tokens`
  - `/__mock__/v1/chat/completions`
  - `/__mock__/v1/responses`

## Verified HTTP Results

### PASS — `/api/health`
- `200 OK`
- database connected
- service healthy

### PASS — `/api/system-settings`
- `200 OK`
- seeded default system settings record returned successfully

### PASS — `/v1/models`
- `200 OK`
- returned seeded mock model catalog

### PASS — `/v1/messages`
- `200 OK`
- returned mock Claude-style response body

### PASS — `/v1/responses`
- `200 OK`
- returned mock Codex-style response body with `prompt_cache_key`

### PASS — `/v1/chat/completions`
- `200 OK`
- returned mock OpenAI-compatible completion payload

## Functional Conclusion

This local run proves a meaningful new milestone:

1. the Go rewrite no longer stops at health/version-only startup
2. schema/bootstrap is now sufficient for a local empty database
3. core `/v1` business routes can execute end-to-end against seeded local mock providers
4. direct/admin/business routes can execute against the seeded schema baseline

## What Is Proven Now

- repo-local verification is green
- service startup is green
- empty database no longer blocks execution
- seeded local smoke works for:
  - direct health
  - direct system settings
  - `/v1/models`
  - `/v1/messages`
  - `/v1/responses`
  - `/v1/chat/completions`

## Remaining Caveats

These results prove local bootability and basic route execution, but do **not** fully prove production parity yet:

1. Redis-disabled local run does not prove full stateful Redis behavior
2. local mock upstreams do not prove real provider compatibility
3. quota / fallback / breaker behavior still needs targeted live-path scenarios
4. cutover proof still needs replay/integration/shadow style evidence

## Updated Most Valuable Next Step

Highest-value next task after this milestone:
- execute targeted real-behavior smoke for quota / fallback / breaker paths against the now-working bootstrap baseline
