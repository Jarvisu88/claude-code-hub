# Go Rewrite Cutover Smoke Checklist — 2026-04-26

## Purpose

This checklist is the final-mile operational proof pack for the Go rewrite.
It is intentionally concrete and execution-oriented so the team can validate the current branch without rediscovering scope.

## Scope

Focus areas:
- direct/admin/auth web-facing routes
- `/v1/messages`
- `/v1/chat/completions`
- `/v1/responses`
- `/v1/models`
- session continuity
- quota/fallback behavior
- minimal breaker behavior

Out of scope for this checklist:
- UI pixel-perfect verification
- full performance benchmarking
- all historical replay datasets
- full multi-process breaker synchronization proof

## Preconditions

1. Branch is based on `go-rewrite-main` and pushed to `origin/go-rewrite-main-telemetry-parity`
2. PostgreSQL available with schema compatible to current repo
3. Redis available
4. Environment variables configured:
   - `ADMIN_TOKEN`
   - `REDIS_URL` or `REDIS_*`
   - `DATABASE_*` / `DATABASE_DSN`
5. Provider fixtures or test upstreams available for:
   - Claude/messages
   - OpenAI-compatible/chat completions
   - Codex/responses
6. `ENABLE_SECURE_COOKIES` and `SESSION_TOKEN_MODE` can be toggled during validation

## Repo-Local Proof Commands

Run these first and capture output:

```bash
go test ./...
go build ./cmd/server
go vet ./...
```

Expected result:
- all commands exit 0

## Direct/Admin/Auth Smoke

### 1. Version route
- GET `/api/version`
- Validate payload contains:
  - `current`
  - `latest` or `latest:null`
  - `hasUpdate`
- For dev build, verify `dev-<sha>` compare behavior if configured

### 2. Login / logout
- POST `/api/auth/login` with valid readonly key
- Expect:
  - `ok=true`
  - `user`
  - `loginType`
  - `redirectTo`
  - auth security headers present
- Repeat with `SESSION_TOKEN_MODE=legacy|dual|opaque`
- In `opaque` mode, verify cookie value is opaque session id (`sid_...`)
- POST `/api/auth/logout`
- In `dual/opaque` mode, verify Redis session revoke occurs

### 3. System settings direct route
- GET `/api/system-settings` with:
  - admin token
  - readonly/direct session cookie
- Expect success for authenticated read path
- PUT `/api/system-settings` with admin token only
- Expect admin-only update semantics

### 4. Prices direct route
- GET `/api/prices?page=1&size=20`
- Verify raw paginated shape:
  - `data`
  - `page`
  - `pageSize`
  - `total`
  - `totalPages`

### 5. Availability endpoints
- GET `/api/availability/endpoints`
- GET `/api/availability/endpoints/probe-logs`
- Verify invalid limit/offset => 400
- Verify response shape parity (no extra limit/offset echo)

## `/v1` Proxy Smoke

### 6. `/v1/messages`
- Valid Claude-style request forwards upstream
- Warmup request with intercept enabled:
  - should not call upstream
  - should return local minimal assistant body
  - should set:
    - `x-cch-intercepted=warmup`
    - `x-cch-intercepted-by=claude-code-hub`
  - should write `message_request.blockedBy=warmup`

### 7. `/v1/chat/completions`
- Valid request forwards to OpenAI-compatible provider
- Model filtering remains endpoint-compatible

### 8. `/v1/responses`
- Valid non-streaming request forwards to Codex provider
- Valid SSE request streams through transparently
- Response-side `prompt_cache_key` updates session continuity for:
  - non-streaming JSON body
  - SSE `data:` events

### 9. `/v1/models`
- Verify endpoint-scoped catalog behavior:
  - `/v1/models`
  - `/v1/responses/models`
  - `/v1/chat/completions/models`

## Stateful/Quota Smoke

### 10. Session continuity
- Claude/Codex request with client-provided session metadata
- Verify generated/reused session id behavior
- For Codex responses, verify `codex_<prompt_cache_key>` session promotion path

### 11. Concurrent session gates
- Key concurrent limit exceeded => 429
- User fallback concurrent limit exceeded => 429
- Provider concurrent limit causes candidate skip/fallback
- Redis lookup failure should fail open where implemented

### 12. Cost quota checks
Verify preflight behavior for:
- key/user total
- key/user 5h
- key/user daily
- key/user weekly
- key/user monthly
- provider total
- provider 5h
- provider daily
- provider weekly
- provider monthly

Expected:
- user/key limit breaches => 402
- provider limit breaches => candidate skip/fallback
- lookup errors => fail open where current implementation declares fail-open behavior

## Retry/Fallback/Breaker Smoke

### 13. Same-provider retry
- transport error on first attempt
- success on retry
- verify providerChain includes:
  - `retry_failed`
  - `retry_success`

### 14. Multi-provider transport fallback
- first provider transport failure
- second provider success
- verify fallback occurs and chain records `system_error` fallback intent

### 15. Resource-not-found fallback
- first provider returns 404
- second provider succeeds
- verify chain marks `resource_not_found`
- verify breaker does not open on 404 path

### 16. Minimal circuit breaker
- transport failure can open provider when enabled
- open provider skipped by selector
- expired open => half-open trial allowed
- half-open success => closes
- half-open transport failure => reopens
- upstream 500 can count toward breaker
- upstream 404 should not count toward breaker

## Evidence To Capture

For each scenario above, capture at least one of:
- terminal output
- curl/httpie transcript
- structured request/response snippet
- DB row excerpt (`message_request`)
- Redis key/value excerpt for session continuity

Recommended artifact locations:
- `reports/go-rewrite/`
- `reports/go-rewrite-cutover-smoke-results-*.md`

## Current Known Residual Risks

1. Breaker is still in-process/minimal, not fully Redis-synchronized
2. Retry/error-category behavior is still not fully equivalent to Node for every 4xx/5xx edge
3. Lease-aware quota system is not fully mirrored
4. Streaming edge cases still need broader real-upstream coverage

## Exit Criteria For “Ready To Attempt Cutover”

All of the following should be true:
1. repo-local proof commands green
2. direct/admin/auth smoke green
3. `/v1/messages`, `/v1/chat/completions`, `/v1/responses`, `/v1/models` smoke green
4. session continuity proof captured
5. quota/fallback/breaker smoke captured
6. no unexplained regressions in `message_request` terminal states
7. remaining risks are explicitly accepted for the cutover window
