# Go Rewrite Rollout / Rollback Checklist — 2026-04-26

## Purpose

This document is the operator-facing execution checklist for attempting a controlled cutover from the Node runtime path to the Go rewrite path.

It complements:
- `reports/go-rewrite-cutover-readiness-20260426.md`
- `reports/go-rewrite-cutover-smoke-checklist-20260426.md`

This file is intentionally procedural. It is not a design note.

---

## Rollout Strategy

Preferred rollout shape:
1. **Pre-cutover proof** in staging-like environment
2. **Small-scope production exposure** if available
3. **Tight observation window**
4. **Immediate rollback on first unexplained regression**

Avoid “big-bang” replacement unless the environment offers no safer routing option.

---

## Preconditions Before Rollout

All of the following should be true before any real cutover attempt:

- [ ] `go test ./...` is green
- [ ] `go build ./cmd/server` is green
- [ ] `go vet ./...` is green
- [ ] `reports/go-rewrite-cutover-smoke-checklist-20260426.md` has been reviewed
- [ ] staging or equivalent environment has reachable PostgreSQL + Redis
- [ ] provider secrets and routing config are present
- [ ] `ADMIN_TOKEN` configured
- [ ] `ENABLE_SECURE_COOKIES` set correctly for environment
- [ ] `SESSION_TOKEN_MODE` chosen intentionally (`legacy` / `dual` / `opaque`)
- [ ] operator knows how traffic is routed back to Node if rollback is needed
- [ ] logs / metrics / DB access available during rollout window

---

## Minimum Environment Validation

Before any traffic switch:

- [ ] Go service starts cleanly
- [ ] DB ping succeeds
- [ ] Redis ping succeeds
- [ ] `/api/health`, `/api/health/live`, `/api/health/ready` behave correctly
- [ ] `/api/version` returns expected branch/version metadata

Recommended commands:

```bash
go test ./...
go build ./cmd/server
go vet ./...
./server
```

---

## Recommended Rollout Order

### Phase 1 — Direct/Admin/Auth verification under real env

Validate these first because they are easier to reason about than proxy traffic:

- [ ] `/api/auth/login`
- [ ] `/api/auth/logout`
- [ ] `/api/system-settings`
- [ ] `/api/version`
- [ ] `/api/prices`
- [ ] `/api/availability*`
- [ ] `/api/ip-geo/:ip`

Acceptance:
- no auth-cookie regressions
- no admin-token regressions
- no malformed response shapes
- no unexplained 5xx

### Phase 2 — `/v1/models` and non-streaming proxy

Validate low-risk proxy surfaces before long-lived streaming:

- [ ] `/v1/models`
- [ ] `/v1/messages/count_tokens`
- [ ] simple `/v1/chat/completions`
- [ ] simple `/v1/responses` non-streaming

Acceptance:
- correct upstream routing
- expected model scoping
- request logs written
- session continuity still sane

### Phase 3 — streaming + resilience behavior

- [ ] `/v1/messages` streaming
- [ ] `/v1/responses` SSE
- [ ] warmup intercept path
- [ ] quota rejections
- [ ] fallback / retry behavior
- [ ] minimal breaker behavior

Acceptance:
- SSE not truncated
- `prompt_cache_key` continuity preserved
- fallback paths observable in providerChain
- no unexplained stuck sessions

---

## What To Watch During Rollout

### Application logs

Look for spikes in:
- provider transport failures
- request-body parse failures
- session extraction failures
- DB write failures for `message_request`
- Redis lookup failures on session/quota paths

### Database verification

Spot-check `message_request` rows for:
- `session_id`
- `request_sequence`
- `provider_chain`
- `blocked_by`
- `blocked_reason`
- `status_code`
- token usage fields

### Redis verification

Spot-check keys for:
- auth opaque sessions (`cch:session:*`)
- session manager keys (`session:*`)
- live-chain keys (`cch:live-chain:*`)

### Functional warning signs

Rollback should be considered immediately if you see:
- login succeeds but follow-up direct APIs 401 unexpectedly
- SSE streams stall or truncate
- valid proxy traffic starts returning 5xx at materially higher rate than Node baseline
- quota checks reject clearly healthy traffic unexpectedly
- provider fallback loops become unstable
- repeated circuit open without expected recovery

---

## Rollout Checklist

### T-30 min
- [ ] confirm branch / commit deployed intentionally
- [ ] verify env vars and provider config
- [ ] verify DB/Redis reachability
- [ ] verify smoke owner and rollback owner are available

### T-10 min
- [ ] run direct/admin/auth smoke
- [ ] run non-streaming `/v1` smoke
- [ ] capture baseline success/error rates from current system

### T-0
- [ ] shift first traffic slice to Go path
- [ ] observe logs continuously
- [ ] verify first successful direct request set
- [ ] verify first successful `/v1/messages` / `/v1/responses`

### T+5 min
- [ ] inspect `message_request` samples
- [ ] inspect Redis session / continuity keys
- [ ] inspect fallback / retry events if triggered

### T+15 min
- [ ] compare observed 4xx/5xx rates to expected baseline
- [ ] verify no auth/session regressions
- [ ] decide continue / pause / rollback

### T+30 min
- [ ] if stable, continue wider rollout
- [ ] if not stable, rollback immediately

---

## Rollback Triggers

Rollback is recommended if any of the following happen and root cause is not immediately trivial:

- [ ] auth flow regression affecting operators or users
- [ ] materially elevated proxy 5xx rate
- [ ] SSE reliability regression
- [ ] persistent bad quota decisions
- [ ] repeated fallback/circuit instability
- [ ] message_request write corruption or missing critical telemetry

Do not wait for “maybe it settles down” if the symptom is user-visible and unexplained.

---

## Rollback Procedure

1. **Stop widening traffic**
   - [ ] freeze rollout immediately

2. **Route traffic back to Node path**
   - [ ] revert traffic switch / route flag / deployment target

3. **Verify rollback worked**
   - [ ] direct/admin/auth endpoints normal on Node
   - [ ] `/v1` traffic normal on Node
   - [ ] 5xx rate returns to baseline

4. **Preserve evidence before cleanup**
   - [ ] copy relevant app logs
   - [ ] note suspect provider/session/request ids
   - [ ] capture representative DB rows / Redis keys

5. **Open follow-up investigation artifact**
   - [ ] create a short incident note with:
     - time of rollback
     - visible symptom
     - suspected subsystem
     - whether issue was auth / session / quota / fallback / breaker / SSE

---

## Post-Rollout Success Criteria

A rollout attempt can be considered successful only if:

- [ ] no unexplained auth/session regressions
- [ ] no material increase in proxy 5xx
- [ ] SSE behavior acceptable
- [ ] fallback / retry / breaker behavior explainable
- [ ] request logging intact
- [ ] operator confidence high enough to keep traffic on Go path

---

## Current Known Residual Risks

These are still expected at the current maturity level and should be monitored closely:

1. breaker remains intentionally smaller than Node full implementation
2. retry/fallback classification is still not exhaustive
3. quota behavior is strong but not yet lease-aware
4. cross-process/state-distribution proof is still lighter than final desired state

---

## Suggested Companion Artifacts

Keep these nearby during rollout:
- `reports/go-rewrite-cutover-readiness-20260426.md`
- `reports/go-rewrite-cutover-smoke-checklist-20260426.md`
- latest deployment commit / tag note
- latest environment diff note
