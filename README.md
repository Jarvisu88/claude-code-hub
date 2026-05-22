<p align="right">
  <strong>English</strong> | <a href="#chinese">中文</a>
</p>

<div align="center">

# Claude Code Hub

**Intelligent AI API Proxy and Management Platform -- Unified multi-provider access, elastic scheduling, and operational control for teams**

[![Container Image](https://img.shields.io/badge/ghcr.io-ding113%2Fclaude--code--hub-181717?logo=github)](https://github.com/ding113/claude-code-hub/pkgs/container/claude-code-hub)
[![License](https://img.shields.io/github/license/ding113/claude-code-hub)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/ding113/claude-code-hub)](https://github.com/ding113/claude-code-hub/stargazers)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ding113/claude-code-hub)](go.mod)
[![Telegram Group](https://img.shields.io/badge/Telegram-Chat-blue?logo=telegram)](https://t.me/ygxz_group)

</div>

---

## Features

- **Multi-format Proxy** -- Claude (Messages API), OpenAI (Chat Completions), OpenAI Responses, Codex CLI, and Gemini formats with bidirectional conversion
- **Load Balancing** -- Weight/priority-based provider selection with automatic failover
- **Circuit Breaker** -- Per-provider health tracking; degraded providers are temporarily bypassed
- **Rate Limiting** -- Multi-dimensional limits (user, key, model, global) backed by Redis
- **Session Affinity** -- Sticky provider binding for long-running sessions (configurable TTL)
- **Request Filters & Sensitive Words** -- Block, allow, or rewrite requests; content moderation
- **Error Rule Engine** -- Classify and rectify upstream errors with pattern matching
- **Usage Tracking & Cost Accounting** -- Per-request token and cost logging with model price management
- **Real-time Dashboard** -- Overview, statistics, leaderboard, and a full-screen monitoring big screen
- **Notifications** -- Circuit breaker alerts, cost alerts, leaderboard summaries via webhook (Slack, DingTalk, Lark, generic)
- **Audit Logging** -- Every admin action is recorded
- **Internationalization** -- Dashboard in 5 languages (English, Simplified Chinese, Traditional Chinese, Japanese, Russian)
- **Single Binary** -- Go binary with embedded frontend; no Node.js runtime needed in production

## Quick Start (Docker Compose)

```bash
# 1. Clone the repository
git clone https://github.com/ding113/claude-code-hub.git
cd claude-code-hub

# 2. Create an environment file
cp .env.example .env
# Edit .env -- at minimum set ADMIN_TOKEN to a secure value

# 3. Start all services
docker compose up -d

# 4. Open the dashboard
#    http://localhost:23000
```

The default port is **23000**. PostgreSQL and Redis are included in the Compose file.

## Development Setup

### Prerequisites

| Tool       | Version   |
|------------|-----------|
| Go         | 1.26+     |
| Node.js    | 20+       |
| PostgreSQL | 15+       |
| Redis      | 7+        |

### Running locally

```bash
# Install frontend dependencies and build
cd web && npm install && npm run build && cd ..

# Copy and edit configuration
cp config.example.yaml config.yaml

# Run the server (auto-migrates the database by default)
go run ./cmd/server
```

The server starts on `http://localhost:23000` by default.

### Frontend development

```bash
cd web
npm install
npm run dev       # Vite dev server with hot-reload (proxies /api to Go backend)
```

## Build from Source

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Build Go binary (frontend is embedded via go:embed)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o claude-code-hub ./cmd/server
```

The resulting binary is fully self-contained.

## Configuration

Claude Code Hub is configured via a YAML file (`config.yaml`) and/or environment variables. Environment variables use the `CCH_` prefix and underscore-separated paths.

| Environment Variable | Config Key | Default | Description |
|---|---|---|---|
| `CCH_SERVER_PORT` | `server.port` | `23000` | HTTP listen port |
| `CCH_SERVER_HOST` | `server.host` | `0.0.0.0` | Listen address |
| `CCH_SERVER_MODE` | `server.mode` | `release` | Gin mode (`debug`, `release`, `test`) |
| `CCH_DATABASE_DSN` | `database.dsn` | -- | PostgreSQL connection string |
| `CCH_DATABASE_MAX_OPEN_CONNS` | `database.max_open_conns` | `25` | Max open DB connections |
| `CCH_REDIS_URL` | `redis.url` | -- | Redis connection URL |
| `CCH_AUTH_ADMIN_TOKEN` | `auth.admin_token` | -- | Admin login token (required) |
| `CCH_AUTH_SESSION_TTL` | `auth.session_ttl` | `24h` | Admin session TTL |
| `CCH_PROXY_SESSION_TTL` | `proxy.session_ttl` | `300` | Provider session TTL (seconds) |
| `CCH_PROXY_MAX_RETRY_ATTEMPTS` | `proxy.max_retry_attempts` | `3` | Max retries per request |
| `CCH_RATE_LIMIT_ENABLED` | `rate_limit.enabled` | `true` | Enable rate limiting |
| `CCH_LOG_LEVEL` | `log.level` | `info` | Log level |
| `CCH_LOG_FORMAT` | `log.format` | `json` | Log format (`json`, `console`) |
| `CCH_AUTO_MIGRATE` | `auto_migrate` | `true` | Auto-run DB migrations on startup |

See `config.example.yaml` for the full reference.

## Architecture Overview

```
                                   +------------------+
                                   |   Web Dashboard  |
                                   |   (React/Vite)   |
                                   +--------+---------+
                                            |
                          +-----------------+------------------+
                          |            Go Server               |
                          |   (Gin + embedded static files)    |
                          +-----------------+------------------+
                          |                 |                  |
                   +------+------+  +-------+------+  +-------+------+
                   | Proxy       |  | Management   |  | Background   |
                   | Pipeline    |  | API          |  | Workers      |
                   | (v1/*)      |  | (api/*)      |  | (probe,sync) |
                   +------+------+  +--------------+  +--------------+
                          |
             +------------+------------+
             |            |            |
       +-----+----+ +----+----+ +-----+-----+
       | Provider  | | Provider| | Provider  |
       | (Claude)  | | (OpenAI)| | (Gemini)  |
       +----------+  +---------+ +-----------+

       PostgreSQL (data)          Redis (sessions, rate limits, cache)
```

**Proxy Pipeline** -- Each request flows through a configurable guard chain:

```
Request -> [auth -> sensitive -> client -> model -> version -> probe ->
            session -> warmup -> requestFilter -> rateLimit ->
            provider -> providerRequestFilter -> messageContext] ->
           Forwarder -> ResponseHandler -> Response
```

### Directory Structure

```
cmd/server/          Go entry point
internal/
  config/            Configuration loading (Viper)
  database/          Database connection and migration
  handler/           HTTP handlers (api/, v1/, middleware/)
  model/             Domain models
  pkg/               Shared utilities (errors, crypto, etc.)
  proxy/             Core proxy engine and format converters
  redis/             Redis client and helpers
  repository/        Data access layer (Bun ORM)
  service/           Business logic services
web/                 React + Vite frontend
deploy/              Docker & Kubernetes manifests
docs/                Documentation
```

## API Endpoints

### Proxy Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/messages` | Claude Messages API proxy |
| POST | `/v1/chat/completions` | OpenAI Chat Completions proxy |
| POST | `/v1/responses` | OpenAI Responses API proxy |
| GET | `/v1/models` | List available models |

### Management API

All management endpoints are under `/api/` and require admin authentication via `Authorization: Bearer <token>`.

| Group | Base Path | Description |
|-------|-----------|-------------|
| Overview | `/api/actions/overview/` | Dashboard overview data |
| Users | `/api/actions/users/` | User CRUD |
| Keys | `/api/actions/keys/` | API key management |
| Providers | `/api/actions/providers/` | Provider and endpoint management |
| Usage Logs | `/api/actions/usage-logs/` | Request log queries |
| Statistics | `/api/actions/statistics/` | Aggregated statistics |
| Error Rules | `/api/actions/error-rules/` | Error classification rules |
| Request Filters | `/api/actions/request-filters/` | Request filter rules |
| Sensitive Words | `/api/actions/sensitive-words/` | Content moderation |
| Notifications | `/api/actions/notifications/` | Notification settings and webhooks |
| Audit Logs | `/api/actions/audit-logs/` | Admin action audit trail |
| Settings | `/api/system-settings` | System configuration |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness check |

See [docs/api-reference.md](docs/api-reference.md) for detailed documentation.

## Deployment

- **Docker Compose** -- Recommended for single-server deployments. See [Quick Start](#quick-start-docker-compose).
- **Kubernetes** -- Manifests provided in `deploy/k8s/`. See [docs/deploy-guide.md](docs/deploy-guide.md).
- **Bare metal** -- Run the compiled binary directly with a `config.yaml` file.

## Migrating from the Node.js Version

If you are running the previous Next.js-based version, see [docs/migration-from-nodejs.md](docs/migration-from-nodejs.md) for a step-by-step guide. The database schema and API endpoints are fully compatible.

## License

[MIT](LICENSE)

---

<a id="chinese"></a>

## 中文简介

Claude Code Hub 是一个面向团队的 AI API 代理和管理平台，使用 Go 重写，以单一二进制文件提供完整服务（内嵌前端）。

主要功能：

- 多格式代理（Claude / OpenAI / Codex / Gemini 双向转换）
- 加权负载均衡与自动故障切换
- 熔断器、限流、会话粘性
- 请求过滤与敏感词检测
- 用量追踪与成本统计
- 实时监控仪表盘与大屏展示
- 通知推送（Slack / 钉钉 / 飞书 / Webhook）
- 5 语言国际化（中/繁中/英/日/俄）

快速开始：

```bash
git clone https://github.com/ding113/claude-code-hub.git
cd claude-code-hub
cp .env.example .env   # 编辑 .env 设置 ADMIN_TOKEN
docker compose up -d
# 访问 http://localhost:23000
```

详细文档请参阅 `docs/` 目录。
