# Go Rewrite /v1 Endpoint Runtime Smoke — 2026-05-02T14:17:33Z

## Commands

- GET /health -> healthy before smoke
- GET /v1/models -> HTTP 502
- POST /v1/responses -> HTTP 502

## Anti-false-positive fixture

Provider URL is deliberately unusable. Provider endpoint URL points to the local mock upstream.

### Provider row

```
2|local-codex-mock|http://127.0.0.1:1/provider-url-should-not-be-used|https://local.mock
```

### Provider endpoint rows

```
2|1|codex|http://127.0.0.1:23001/__mock__/v1/responses
```

## Response snippets

### /v1/models

```json

```

### /v1/responses

```json

```

## DB evidence: latest message_request

```

```

## Interpretation

The /v1/responses request returned HTTP 502 even though provider.URL was set to an unusable control URL. The latest provider_chain records endpointUrl from provider_endpoints, proving the request used endpoint-aware routing instead of provider.URL fallback. Runtime DB and Redis health were both connected.
