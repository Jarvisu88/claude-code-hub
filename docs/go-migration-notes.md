# Go rewrite migration notes

This note is the working checklist for continuing the Go rewrite toward functional parity with the current Node app.
Keep `docs/REWRITE.md` as the broad plan; use this file for day-to-day migration decisions, sequencing, and cutover gates.

## Current baseline

- The Node app remains the behavior source of truth.
- `upstream/go-rewrite` is the rewrite history baseline.
- `go-rewrite-main` is the local integration branch for parity work.
- The current Go app already covers:
  - config, logging, database, Redis, validation, and error helpers
  - core models and repository/data-access layers
  - a running `/health` endpoint
- The current HTTP routes for `/v1/*` and `/api/actions/*` are still stubs.

## Branch strategy

| Branch | Purpose | Rule |
|---|---|---|
| `main` | Node app reference line | Treat as the functional parity baseline. |
| `upstream/go-rewrite` | Upstream rewrite history | Do not edit directly; use it as the rebase/merge source for rewrite work. |
| `go-rewrite-main` | Local integration branch | Keep all rewrite work landing here first. |
| `go-rewrite/<module>` | Short-lived work branches | One module or one tightly coupled slice per branch. |

Rules:

- Rebase feature branches onto `go-rewrite-main`.
- Merge forward only; do not backport unfinished Go changes into the Node line.
- Keep module branches small enough to review against the matching Node behavior.
- If Node-side changes land during the rewrite, record the delta in this note before the next cutover decision.

## Cutover strategy

1. **Shadow mode first**  
   Run the Go service side-by-side with the Node app and compare outputs before returning user traffic.
2. **Module-by-module enablement**  
   Turn on one completed module or endpoint group at a time behind a switch or routing rule.
3. **Canary next**  
   Move traffic in small steps only after the comparison checks stay clean.
4. **Full cutover last**  
   Switch primary traffic only after the Go path matches the Node path for the agreed checkpoints.
5. **Rollback stays available**  
   Keep the Node app ready until the final parity window is complete.

## Module order

Follow the existing rewrite phases, but keep the order strict:

| Order | Module group | Gate before moving on |
|---|---|---|
| 1 | Foundation: config, logger, database, Redis, validation, errors | Service starts and health checks pass. |
| 2 | Data access: models and repositories | CRUD paths match the Node data shape. |
| 3 | Core services: auth, cache, cost, rate limit, circuit breaker, session | Service logic is independently testable. |
| 4 | Proxy core: session, guards, provider selection, forwarder, SSE, converters | Request forwarding and streamed responses match. |
| 5 | HTTP layer: `/v1/messages`, `/v1/chat/completions`, `/v1/responses`, `/v1/models`, `/api/actions/*` | External API contract matches the Node app. |
| 6 | Auxiliary features: notifications, webhook rendering, sensitive-word filtering, request filtering | Non-core features can be enabled without breaking parity. |

Do not start a later module before the earlier module's contract checks are green.

## Compatibility constraints

- Keep response shapes, status codes, and error payloads compatible with the Node app.
- Preserve streaming behavior, including SSE framing and event ordering.
- Keep provider routing, fallback order, and rate-limit semantics stable.
- Keep Redis key names, TTL expectations, and DB schema migrations backward compatible.
- Preserve existing config keys and environment variable names unless a migration shim exists.
- Keep any new behavior default-off until the matching Node behavior is covered.
- Treat `/health` as a platform check only; it is not parity proof.

## Verification checkpoints

### Per module

- `go test ./...`
- `go build ./cmd/server`
- targeted request/response comparison against the Node app for the module under migration

### Parity checks

- compare JSON output for supported endpoints
- compare streamed output for proxy/SSE paths
- compare logs and error handling for known failure cases
- compare rate-limit and provider-selection behavior under load

### Cutover checks

- shadow traffic shows no unexplained response drift
- canary traffic stays stable through the agreed window
- rollback to Node is still possible until full cutover is signed off

### Suggested release gates

| Gate | Evidence |
|---|---|
| Dev ready | Build passes and the module's unit tests pass. |
| Shadow ready | Go and Node responses match for the covered paths. |
| Canary ready | Metrics and error rates stay within the agreed tolerance. |
| Full cutover ready | No known contract gaps remain for the migrated paths. |

## Working rule

If a change affects both the Go rewrite and the Node baseline, write the comparison rule down first, then implement the smallest Go change that preserves parity.
