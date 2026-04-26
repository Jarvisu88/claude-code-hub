# Go Rewrite Local Run Baseline — 2026-04-26

## Purpose

Provide the smallest repo-local setup for starting the Go service against disposable local PostgreSQL + Redis.

## Added Assets

- `docker-compose.local.yml`
- `.env.local.example`
- `Makefile` targets:
  - `local-stack-up`
  - `local-stack-down`
  - `local-run`

## Recommended Local Flow

```bash
cp .env.local.example .env.local
make local-stack-up
make local-run
```

Then verify:

```bash
curl http://127.0.0.1:23000/api/health
curl http://127.0.0.1:23000/api/health/live
curl http://127.0.0.1:23000/api/version
```

## Notes

1. Environment variable for database name is `DATABASE_NAME` (not `DATABASE_DBNAME`).
2. The example defaults to:
   - PostgreSQL user/password: `postgres/postgres`
   - database: `claude_code_hub`
   - Redis: `127.0.0.1:6379`
   - admin token: `dev-admin-token`
3. `SESSION_TOKEN_MODE=dual` is chosen as the local default because it exercises more auth/session paths than pure legacy mode.
4. `ENABLE_SECURE_COOKIES=false` is chosen for localhost convenience.
