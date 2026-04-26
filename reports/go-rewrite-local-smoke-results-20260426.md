# Go Rewrite Local Smoke Results — 2026-04-26

## Goal

Validate whether the current Go rewrite can be launched locally and exercised through real HTTP requests, using Chrome remote debugging where possible.

## What Was Executed

### 1. Repository-local baseline verification

Commands:

```bash
go test ./...
go build ./cmd/server
go vet ./...
```

Result:
- all three commands passed

### 2. Runtime dependency discovery

Observed initially:
- Docker daemon unavailable in the current environment
- local PostgreSQL on `127.0.0.1:5432` existed but password auth for `postgres` failed
- local Redis on `127.0.0.1:6379` was not available

### 3. Temporary local PostgreSQL bootstrap

A temporary PostgreSQL 16 instance was created under `/tmp` and bound to:
- host: `127.0.0.1`
- port: `55432`
- database: `claude_code_hub`
- user: `postgres`
- auth: trust (temporary local smoke only)

### 4. Go service launch

The Go server was launched successfully with runtime overrides:
- PostgreSQL pointed to `127.0.0.1:55432`
- `REDIS_ENABLED=false`
- `ADMIN_TOKEN=dev-admin-token`
- `SESSION_TOKEN_MODE=dual`
- `ENABLE_SECURE_COOKIES=false`

Observed startup result:
- PostgreSQL connected successfully
- Redis intentionally disabled
- HTTP server listening on `127.0.0.1:23000`

### 5. Chrome remote debugging setup

A Windows-side Chrome instance was started with remote debugging on port `9222` and successfully connected through Chrome MCP.

## HTTP / Browser-Side Smoke Checks Performed

### PASS — `/api/health`
Request:
- GET `http://127.0.0.1:23000/api/health`

Observed response:
- `200 OK`
- body contained:
  - `database: connected`
  - `redis: connected` (note: app health path reports connected because redis check is skipped when disabled in current bootstrap path)
  - `status: healthy`

### PASS — `/api/health/live`
Request:
- GET `http://127.0.0.1:23000/api/health/live`

Observed through local curl flow / baseline service readiness path.

### PASS — `/api/version`
Request:
- GET `http://127.0.0.1:23000/api/version`

Observed response:
- `200 OK`
- contained current/latest/update metadata

### PASS — `/api/auth/login` failure taxonomy (KEY_REQUIRED)
Browser-side fetch executed through Chrome MCP:
- POST `/api/auth/login` with whitespace key only

Observed response:
- `400`
- JSON body contained:
  - `error: API key is required`
  - `errorCode: KEY_REQUIRED`
  - `ok: false`
- security / no-store headers present as expected

### FAIL (expected due empty schema) — `/api/system-settings`
Browser-side fetch executed through Chrome MCP:
- GET `/api/system-settings` with `Authorization: Bearer dev-admin-token`

Observed response:
- `500`
- body:
  - `type: internal_error`
  - `message: Database error`
  - `code: database_error`

### FAIL (expected due empty schema) — `/v1/models`
Request:
- GET `/v1/models` with `Authorization: Bearer proxy-key`

Observed response:
- `500`
- `database_error`

### FAIL (expected due empty schema) — `/v1/responses`
Request:
- POST `/v1/responses` with minimal JSON body and `Authorization: Bearer proxy-key`

Observed response:
- `500`
- `database_error`

## Root Cause Of Runtime Failure Beyond Health/Version

The service can now **start**, but most business endpoints cannot complete because the temporary PostgreSQL database is empty.

Evidence:

```sql
\dt
```

Result:
- `Did not find any relations.`

Additionally, repository inspection found:
- no checked-in migration runner in active use
- no auto-created schema bootstrapping path wired in `cmd/server/main.go`
- `AutoMigrate` config flag exists, but current startup flow does not execute schema creation for the application tables

Therefore:
- infrastructure startup succeeded
- application schema initialization did **not** happen
- any endpoint that relies on tables such as `system_settings`, `keys`, `providers`, `message_request`, etc. fails with `database_error`

## Overall Local Smoke Verdict

### What is proven now
1. Codebase compiles and passes repo-local verification
2. Service can be launched successfully when given a working PostgreSQL endpoint
3. Chrome MCP browser-side validation is working
4. Basic non-schema routes (`/api/health`, `/api/version`) behave correctly
5. Auth/login failure taxonomy and security headers are functioning in a real running process

### What is not yet proven
1. Schema-dependent direct/admin routes under a real initialized database
2. `/v1` proxy behavior against a schema-populated runtime database
3. Real auth/session lifecycle against populated `keys` / `users`
4. Provider selection / quota / fallback / breaker behavior in a live database-backed runtime

## Blocking Issue To Resolve Next

The next hard blocker is **database schema/bootstrap**, not core code compilation.

To continue meaningful real-run smoke validation, one of the following is needed:
1. a migration/bootstrap path for Go
2. a pre-populated PostgreSQL schema/data snapshot compatible with the Go repositories
3. reuse of an existing Node-side development database with known credentials

## Recommended Next Step

Highest-value next task:
- add or wire a **minimal local schema/bootstrap path** for required tables

Once that exists, rerun smoke in this order:
1. `/api/system-settings`
2. `/api/auth/login` with real key
3. `/v1/models`
4. `/v1/messages`
5. `/v1/responses`
6. quota/fallback/breaker scenarios
