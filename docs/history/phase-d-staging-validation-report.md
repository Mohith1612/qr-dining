# Phase D — Staging Validation Report

Date: 2026-05-25
Scope: operational validation, not feature work. Validates that the platform's
deployment topology, datastores, realtime layer, and edge behave correctly under
staging-style conditions before any strict-flag rollout.

This report records **observed behavior** from a live local staging stack, not
intentions. Companion docs: `chaos-test-results.md`, `production-alerting-baseline.md`,
`strict-rollout-plan-final.md`, `final-operational-readiness-audit.md`.

## 1. Staging environment used

The base `docker-compose.yml` plus a new `docker-compose.staging.yml` override
(adds local host port publishing for Postgres :5432, Redis :6379, app :8080, and
an nginx edge on :18080). Because the build sandbox had no network for
`go mod download`, the app was run as a **host-built static binary**
(`CGO_ENABLED=0 go build ./cmd/server`) inside an `alpine` container attached to
both the internal `backend` network (DNS to pg/redis) and the external
`proxy_network` (publishes :8080). This is documented in `scripts/chaos/README.md`.

Versions exercised: Postgres 17-alpine, Redis 7-alpine, Go 1.26 build, nginx 1.27.

## 2. Parity checklist (vs `staging-burnin-strategy.md`)

| Item | State | Notes |
|------|-------|-------|
| PG version parity (17) | OK | compose pins `postgres:17-alpine` |
| Redis version parity (7) | OK | `redis:7-alpine`, `--appendonly yes`, 256mb, allkeys-lru |
| Migrations applied on boot | OK | "migrations applied" logged on every start; idempotent across restarts (chaos backend-restart) |
| Redis persistence | OK (AOF) | RDB not explicitly configured; AOF sufficient for a non-authoritative cache/pubsub role |
| Healthchecks wired | OK | `/health` (liveness) and `/readyz` (pg+redis composite) |
| Restart policy | OK | `unless-stopped` on all services |
| nginx websocket config | NEW | `deploy/nginx/qr-dining.conf` authored + validated live (HTTP+WS survive reload) |
| R2 backup scripts | PRESENT (not exercised) | `backend/scripts/backup_r2.sh`, `backup.sh`, `restore.sh` exist; not run this phase |
| Env parity | PARTIAL | single `.env`; no committed staging/prod env templates beyond `.env.example` |
| Traffic mirror ≥25% prod | NOT APPLICABLE locally | burn-in soak rule applies in real staging only |

## 3. Topology findings

1. **No nginx config existed in-repo.** Authored `deploy/nginx/qr-dining.conf`:
   websocket-aware (`Upgrade`/`Connection` map, `proxy_http_version 1.1`,
   `proxy_buffering off`, `proxy_read_timeout 3600s` > the 54s server ping),
   `limit_req` on the HTTP API surface but **not** on `/ws`, and `/metrics`
   restricted to private CIDRs. Validated: `nginx -s reload` mid-traffic preserved
   both HTTP (200) and WebSocket upgrades.

2. **Cloudflare assumption documented in-config:** idle WS timeout (~100s) is
   covered by the app's 54s server ping (`pingPeriod` in `client.go`). Do not lower
   `proxy_read_timeout` below the ping interval.

3. **`/readyz` is the correct outage signal.** On a sustained Redis stop, `/readyz`
   returns HTTP 503 with `redis:unhealthy` and the app process stays up (Redis is
   non-authoritative). On recovery it returns 200. See finding F-1 below.

4. **App is stateless.** Restarting the backend mid-operation re-runs migrations
   idempotently and re-establishes pg+redis with no data loss; all durable state
   lives in Postgres.

## 4. Findings

- **F-1 (info): `redis_pubsub_connected` gauge does not reflect transient outages.**
  go-redis's `Channel()` reconnects internally and never closes the receive channel
  for a brief drop, so the gauge stays `1` even while Redis is down. The authoritative
  liveness signal during an outage is `/readyz` (which pings Redis directly). The
  alert baseline keys the Redis-down alert primarily off `/readyz`/probe failure, and
  treats `redis_pubsub_connected==0` as a secondary signal (it fires on subscriber
  teardown / process issues, not transient flaps).

- **F-2 (low): `/readyz` JSON `status` field is misleading.** During a partial outage
  the HTTP status is correctly 503 but the JSON body still reads `"status":"ready"`
  while `checks.redis` is `"unhealthy"`. Orchestrators key off the HTTP code (correct),
  but the body is confusing for humans. Cosmetic; recommend aligning the field to the
  worst check. Not a rollout blocker.

- **F-3 (info): reactivation pipeline confirmed live.** A session left without WS
  presence transitioned `active → awaiting_reactivation` on its own during testing —
  the lifecycle worker is functioning as specified in `session-lifecycle-state-machine.md`.

## 5. Verdict

The staging topology is sound for burn-in: datastores persist, migrations are
idempotent, the backend is stateless, the new nginx edge handles websockets and
reloads cleanly, and Redis is demonstrably non-authoritative. Remaining items
(R2 backup exercise, real ≥25% traffic mirror, staging/prod env templates) are
operational, not code blockers.
