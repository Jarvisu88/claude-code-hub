# API Reference

Claude Code Hub exposes two categories of endpoints: **Proxy endpoints** for forwarding AI requests to upstream providers, and **Management endpoints** for administering the platform.

## Authentication

### Proxy authentication

Proxy endpoints (`/v1/*`) authenticate via API keys. Include the key in the `Authorization` header:

```
Authorization: Bearer sk-your-api-key
```

Or use the `x-api-key` header:

```
x-api-key: sk-your-api-key
```

### Management authentication

Management endpoints (`/api/*`) require the admin token. First, obtain a session token via the login endpoint, then include it in subsequent requests:

```
Authorization: Bearer <session-token>
```

#### Login

```
POST /api/auth/login
Content-Type: application/json

{
  "token": "your-admin-token"
}
```

Response:

```json
{
  "token": "session-jwt-token",
  "expiresAt": "2026-05-23T12:00:00Z"
}
```

## Proxy Endpoints

### POST /v1/messages

Claude Messages API proxy. Accepts the same request format as the Anthropic Messages API.

```bash
curl -X POST http://localhost:23000/v1/messages \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Supports both streaming (`"stream": true`) and non-streaming responses.

### POST /v1/chat/completions

OpenAI Chat Completions API proxy. Accepts the standard OpenAI request format.

```bash
curl -X POST http://localhost:23000/v1/chat/completions \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

When the upstream provider is Claude, requests are automatically converted between OpenAI and Claude formats.

### POST /v1/responses

OpenAI Responses API proxy.

```bash
curl -X POST http://localhost:23000/v1/responses \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "input": "Hello"
  }'
```

### GET /v1/models

List available models across all active providers.

```bash
curl http://localhost:23000/v1/models \
  -H "Authorization: Bearer sk-your-key"
```

Response follows the OpenAI models list format.

## Management API

All management endpoints require admin authentication. They are grouped by resource.

### Overview

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/overview/getOverviewData` | Dashboard overview (stats, charts, activity) |

### Dashboard Real-time

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/actions/dashboard-realtime/getDashboardRealtimeData` | Real-time metrics for the big screen dashboard |

Returns: metrics (today's requests, cost, sessions, error rate, requests/min), activity stream, provider rankings, model distribution, hourly trend data.

### Users

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/users/list` | List users (paginated) |
| POST | `/api/actions/users/create` | Create a user |
| PUT | `/api/actions/users/:id` | Update a user |
| PATCH | `/api/actions/users/:id` | Toggle user enabled/disabled |
| DELETE | `/api/actions/users/:id` | Delete a user |

Query parameters for list: `page`, `pageSize`, `search`.

### API Keys

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/keys/list` | List API keys (paginated) |
| POST | `/api/actions/keys/create` | Create an API key |
| PUT | `/api/actions/keys/:id` | Update an API key |
| PATCH | `/api/actions/keys/:id` | Toggle key enabled/disabled |
| DELETE | `/api/actions/keys/:id` | Delete an API key |

### Providers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/providers/list` | List providers (paginated) |
| POST | `/api/actions/providers/create` | Create a provider |
| PUT | `/api/actions/providers/:id` | Update a provider |
| PATCH | `/api/actions/providers/:id` | Toggle provider enabled/disabled |
| DELETE | `/api/actions/providers/:id` | Delete a provider |
| POST | `/api/actions/providers/:id/test` | Test provider connection |

### Provider Groups

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/providers/getProviderGroups` | List provider groups |
| POST | `/api/actions/providers/createProviderGroup` | Create a group |
| PUT | `/api/actions/providers/updateProviderGroup/:id` | Update a group |
| DELETE | `/api/actions/providers/deleteProviderGroup/:id` | Delete a group |

### Provider Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/providers/getProviderEndpoints` | List endpoints (paginated) |
| POST | `/api/actions/providers/probeEndpoint/:id` | Probe a single endpoint |
| GET | `/api/actions/providers/getProbeLog/:id` | Get probe logs for an endpoint |

### Usage Logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/usage-logs/getUsageLogs` | Query usage logs (paginated, filterable) |

Query parameters: `page`, `pageSize`, `startDate`, `endDate`, `userId`, `model`, `status`.

### Statistics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/statistics/getStatistics` | Aggregated statistics |

Query parameters: `range` (`24h`, `7d`, `30d`).

### Leaderboard

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/leaderboard` | User leaderboard |

Query parameters: `period` (`today`, `7d`, `30d`).

### Model Prices

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/prices` | List model prices (paginated) |
| PUT | `/api/prices/:id` | Update a model price |
| POST | `/api/prices/sync` | Sync prices from LiteLLM |

### Error Rules

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/error-rules/list` | List error rules (paginated) |
| POST | `/api/actions/error-rules/create` | Create an error rule |
| PUT | `/api/actions/error-rules/:id` | Update an error rule |
| PATCH | `/api/actions/error-rules/:id` | Toggle rule enabled/disabled |
| DELETE | `/api/actions/error-rules/:id` | Delete an error rule |

### Request Filters

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/request-filters/list` | List request filters (paginated) |
| POST | `/api/actions/request-filters/create` | Create a filter |
| PUT | `/api/actions/request-filters/:id` | Update a filter |
| PATCH | `/api/actions/request-filters/:id` | Toggle filter enabled/disabled |
| DELETE | `/api/actions/request-filters/:id` | Delete a filter |

### Sensitive Words

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/sensitive-words/list` | List sensitive words (paginated) |
| POST | `/api/actions/sensitive-words/create` | Create a sensitive word |
| PUT | `/api/actions/sensitive-words/:id` | Update a sensitive word |
| PATCH | `/api/actions/sensitive-words/:id` | Toggle word enabled/disabled |
| DELETE | `/api/actions/sensitive-words/:id` | Delete a sensitive word |
| POST | `/api/actions/sensitive-words/refreshCache` | Refresh the in-memory cache |

### Notifications

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/notifications/getSettings` | Get notification settings |
| PUT | `/api/actions/notifications/updateSettings` | Update notification settings |

### Webhook Targets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/webhook-targets/list` | List webhook targets |
| POST | `/api/actions/webhook-targets/create` | Create a webhook target |
| PUT | `/api/actions/webhook-targets/:id` | Update a webhook target |
| DELETE | `/api/actions/webhook-targets/:id` | Delete a webhook target |
| POST | `/api/actions/webhook-targets/:id/test` | Test a webhook |

### Notification Bindings

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/notification-bindings/list` | List notification bindings |
| POST | `/api/actions/notification-bindings/create` | Create a binding |
| DELETE | `/api/actions/notification-bindings/:id` | Delete a binding |

### Audit Logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/actions/audit-logs/list` | Query audit logs (paginated) |

Query parameters: `page`, `pageSize`, `action`, `startDate`, `endDate`.

### System Settings

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/system-settings` | Get current system settings |
| PUT | `/api/system-settings` | Update system settings |

## Health Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Returns 200 if the service is running |

The health endpoint does not require authentication and is suitable for load balancer and container health checks.

## Error Responses

All endpoints return errors in a consistent format:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "statusCode": 400
}
```

Common HTTP status codes:

| Code | Meaning |
|------|---------|
| 400 | Bad request (validation error) |
| 401 | Unauthorized (missing or invalid token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Resource not found |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 502 | Upstream provider error |
| 503 | Service unavailable (circuit breaker open) |
