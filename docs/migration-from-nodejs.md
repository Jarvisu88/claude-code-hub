# Migration from Node.js to Go

This document describes how to migrate an existing Claude Code Hub deployment from the original Node.js (Next.js) version to the Go rewrite.

## Why Migrate

The Go rewrite provides:

- **Single binary deployment** -- No Node.js runtime, no `node_modules`. The frontend is embedded in the Go binary.
- **Lower resource usage** -- Significantly reduced memory footprint and faster startup.
- **Better concurrency** -- Go's goroutine-based model handles high connection counts more efficiently than Node.js.
- **Simplified operations** -- One binary, one config file, one container image.
- **Same functionality** -- All features from the Node.js version are preserved.

## What Changed

### Tech Stack Comparison

| Component | Node.js Version | Go Version |
|-----------|----------------|------------|
| Runtime | Node.js 20+ | Go 1.26+ (compiled binary) |
| Web Framework | Next.js 16 + Hono | Gin |
| ORM | Drizzle | Bun (uptrace/bun) |
| Database | PostgreSQL (same) | PostgreSQL (same) |
| Cache | ioredis | go-redis |
| Frontend | React 19 + shadcn/ui | React 19 + Vite (embedded) |
| Package Manager | Bun | Go modules |
| Config | `.env` + env vars | YAML file + env vars |
| Port | 13500 (default) | 23000 (default) |

### What Stayed the Same

- **Database schema** -- The Go version uses the same PostgreSQL tables and columns. No schema migration is needed.
- **API endpoints** -- All proxy endpoints (`/v1/messages`, `/v1/chat/completions`, `/v1/responses`, `/v1/models`) and management endpoints (`/api/actions/*`) are compatible.
- **Frontend** -- The dashboard has been rebuilt with Vite (instead of Next.js) but provides the same functionality and more.

## Database Compatibility

The Go version reads and writes the same database tables as the Node.js version. You can point the Go version at your existing database and it will work immediately.

The Go version adds new columns and tables incrementally via its own migration system. These additions are backward-compatible -- they do not break the Node.js version if you need to roll back.

**Important**: The Go version runs migrations automatically on startup (when `auto_migrate: true`). These migrations are additive only.

## API Compatibility

All proxy and management API endpoints maintain the same request/response format:

| Endpoint | Status |
|----------|--------|
| `POST /v1/messages` | Compatible |
| `POST /v1/chat/completions` | Compatible |
| `POST /v1/responses` | Compatible |
| `GET /v1/models` | Compatible |
| `POST /api/auth/login` | Compatible |
| `GET /api/actions/overview/*` | Compatible |
| `*/api/actions/users/*` | Compatible |
| `*/api/actions/keys/*` | Compatible |
| `*/api/actions/providers/*` | Compatible |
| `*/api/actions/usage-logs/*` | Compatible |
| `*/api/actions/statistics/*` | Compatible |
| `GET /health` | Compatible |

Clients using API keys do not need any changes.

## Configuration Mapping

The Node.js version uses `.env` files. The Go version uses a YAML config file plus environment variables with the `CCH_` prefix.

| Node.js `.env` | Go `config.yaml` | Go Env Var |
|-----------------|-------------------|------------|
| `ADMIN_TOKEN` | `auth.admin_token` | `CCH_AUTH_ADMIN_TOKEN` |
| `DSN` | `database.dsn` | `CCH_DATABASE_DSN` |
| `REDIS_URL` | `redis.url` | `CCH_REDIS_URL` |
| `ENABLE_RATE_LIMIT` | `rate_limit.enabled` | `CCH_RATE_LIMIT_ENABLED` |
| `SESSION_TTL` | `proxy.session_ttl` | `CCH_PROXY_SESSION_TTL` |
| `AUTO_MIGRATE` | `auto_migrate` | `CCH_AUTO_MIGRATE` |
| `PORT` (13500) | `server.port` (23000) | `CCH_SERVER_PORT` |
| `LOG_LEVEL` | `log.level` | `CCH_LOG_LEVEL` |

## Step-by-Step Migration

### 1. Back up your database

```bash
pg_dump -U postgres claude_code_hub > backup_before_migration.sql
```

### 2. Stop the Node.js application

```bash
# If using Docker Compose
docker compose down

# If running directly
# Stop the Node.js process
```

### 3. Prepare the Go version

```bash
# Pull the Go version
git fetch origin
git checkout go-rewrite-v2

# Or pull the Docker image
docker pull ghcr.io/ding113/claude-code-hub:latest
```

### 4. Create the Go configuration

Create a `config.yaml` or set environment variables. Map your existing `.env` values:

```yaml
server:
  port: 23000
  host: "0.0.0.0"
  mode: "release"

database:
  dsn: "postgres://postgres:YOUR_PASSWORD@localhost:5432/claude_code_hub?sslmode=disable"

redis:
  url: "redis://localhost:6379/0"

auth:
  admin_token: "YOUR_EXISTING_ADMIN_TOKEN"

auto_migrate: true

log:
  level: "info"
  format: "json"
```

### 5. Start the Go version

```bash
# Docker Compose
docker compose up -d

# Or directly
./claude-code-hub
```

The Go version will:
1. Connect to the existing database
2. Run any pending migrations (additive only)
3. Start serving on port 23000

### 6. Update your reverse proxy

If you had nginx/Caddy pointing to port 13500, update it to 23000:

```nginx
# Before
proxy_pass http://127.0.0.1:13500;

# After
proxy_pass http://127.0.0.1:23000;
```

### 7. Verify

- Open the dashboard and log in with your existing admin token.
- Check that users, keys, and providers are visible.
- Send a test proxy request through `/v1/messages` or `/v1/chat/completions`.
- Verify usage logs are being recorded.

### 8. Update client configurations

If any clients were hardcoded to port 13500, update them to 23000 (or keep using your reverse proxy URL, which should be unchanged).

## Rollback Plan

If you encounter issues after migrating:

1. **Stop the Go version**:
   ```bash
   docker compose down
   ```

2. **The database is backward-compatible**. The Go version only adds columns/tables; it does not remove or modify existing ones.

3. **Switch back to the Node.js version**:
   ```bash
   git checkout main
   docker compose up -d
   ```

4. **Restore from backup** (only if needed):
   ```bash
   psql -U postgres claude_code_hub < backup_before_migration.sql
   ```

The Node.js version will ignore any extra columns added by the Go version, so a database restore is typically not necessary for rollback.
