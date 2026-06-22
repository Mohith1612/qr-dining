# Staging and Burn-In Strategy

Date: 2026-05-22
Status: Authoritative pre-production validation plan. Every strict-enforcement flag rollout (`production-enforcement-rollout.md`) is gated by completion of the relevant burn-in here.

## 0. Why this exists

The platform has too many moving parts (Postgres, Redis, Pubsub, WebSockets, multiple HTTP surfaces, signed credentials, durable session events, idempotency, payment lifecycle) to be validated by ad-hoc smoke tests. This document defines a staging environment that is operationally similar to production and a burn-in regime that proves the system can survive realistic failure modes for sustained periods.

## 1. Staging Environment Parity

Staging MUST match production along these dimensions. Anything below "must match" is a known-divergence to record.

### Must match

- Postgres major version and key extensions (`uuid-ossp`, `citext`).
- Redis major version and replication topology.
- Application Go version and Docker base image.
- Frontend Next.js build artifact (no debug bundle).
- Migrations: staging is at the same migration version as production at the moment of the run.
- Feature flag defaults from `backend/internal/config/config.go`. Staging may have additional flags ON ahead of production to exercise the rollout sequence.
- Reverse proxy (nginx) config: TLS termination, security headers, rate limits.

### May differ

- Scale: staging runs at fewer pods. Burn-in scenarios specify required minimum scale.
- DNS: staging uses internal-only DNS.
- Payment providers: staging uses sandbox keys.
- R2 bucket: staging uses an isolated bucket.
- Email: staging uses sink mailbox.

### Must NOT differ

- Authentication shapes (no "dev backdoor" in any code path).
- Webhook signature verification (staging webhooks signed with sandbox secrets, but the verification path is identical).
- Audit log immutability.

## 2. Traffic Mirror

A subset of production traffic is mirrored to staging for the duration of each burn-in. Mirror requirements:

- Read endpoints are mirrored without response forwarding.
- Write endpoints are NOT mirrored (avoid double-writes against real branches).
- Synthetic write traffic is generated separately (Section 4).
- Mirror volume target: at least 25% of production read volume.

## 3. Synthetic Production-Like Load

For sustained tests we generate synthetic load using a load harness that simulates realistic guest and staff behavior. The harness:

- Logs in staff with `branch_code + staff_code + PIN` and refreshes tokens.
- Creates and joins guest sessions, places carts, places orders, initiates payments, completes via webhook for digital methods.
- Drives WS reconnects randomly.
- Drives multi-tab patterns (a single participant_id from N >= 2 connections).
- Drives idempotent retries (5% of mutations).

Synthetic volume target during burn-in: 2x expected production peak for short bursts (10 minutes), 0.5x baseline for sustained intervals (24h).

## 4. Burn-In Scenarios

### S1: Sustained Live Service (24h)

- Synthetic load at 0.5x baseline for 24 continuous hours.
- Pass criteria:
  - Zero unhandled errors in application logs.
  - p99 latency budget held (Section 9).
  - No `legacy_identity_usage_total` outside of intended legacy.
  - Daily reconciliation report clean.
  - No memory growth in app pods beyond 10% over the period.

### S2: Long-Lived Session Behavior (72h)

- Open 1000 guest sessions and keep them active for 72 hours via periodic activity.
- Pass criteria:
  - No session unexpectedly transitions to `abandoned` if there is regular activity.
  - WS reconnects across the period preserve `last_applied_sequence`.
  - Snapshot reads remain correct.

### S3: Reconnect Storm

- After 5 minutes of normal load, force-disconnect 80% of WS clients simultaneously.
- Pass criteria:
  - All clients reconnect within 60s.
  - WS ticket issuance does not exceed rate limit.
  - No event loss (every committed event eventually visible on at least one of each session's tabs).
  - No app pod OOM.

### S4: Redis Outage 60s

- Kill Redis pod for 60 seconds.
- Pass criteria:
  - Frontend degrades to polling.
  - No payment correctness regression (no double-settle, no lost webhook).
  - On recovery, WS resumes within 30 seconds.
  - No data corruption.

### S5: Redis Outage 5 min

- Kill Redis for 5 minutes.
- Pass criteria:
  - All of S4 plus:
  - Stale presence keys rebuild on next heartbeat.
  - No staff sessions invalidated incorrectly (Redis is cache; DB is source of truth).

### S6: Postgres Failover

- Promote replica to primary.
- Pass criteria:
  - App reconnects within 30 seconds.
  - No data loss for committed transactions.
  - Worker re-acquires locks correctly.

### S7: App Pod Restart Storm

- Restart all backend pods sequentially during sustained load.
- Pass criteria:
  - Zero failed payment finalizations.
  - WS clients reconnect to surviving pods, then back to fresh pods.
  - No `session_events` sequence gaps.

### S8: Concurrent Ordering Stress

- 200 concurrent orders against a single branch, half with idempotency replays.
- Pass criteria:
  - Exact count of distinct orders created.
  - Order numbers contiguous within branch-day.
  - No `unique` constraint violations.

### S9: Concurrent Promo Cap

- Promo cap 100. 200 concurrent attempts to redeem.
- Pass criteria:
  - Exactly 100 succeed; 100 receive `ERR_PROMO_CAP`.
  - `promos.current_redemptions` exactly 100 at end.

### S10: Concurrent Payment Race

- Two participants in the same session both press "Pay" within 100ms.
- Pass criteria:
  - Exactly one snapshot created.
  - Split-pay rules per branch policy honored.
  - No double settlement.

### S11: Webhook Replay Storm

- Provider replays 10,000 events over 10 minutes.
- Pass criteria:
  - Each unique `external_event_id` processed once.
  - All return 200 within rate limit headroom.
  - No regression on other payment paths.

### S12: Migration Dry-Run

- Apply a representative additive migration (e.g., adding a new column with backfill) against a staging DB seeded from a recent production snapshot.
- Pass criteria:
  - Migration completes within target window.
  - No lock contention beyond budget.
  - Application starts cleanly against new schema.
  - Down migration restores prior schema cleanly.

### S13: Cold Start Drill

- Stop all backend pods. Wait 5 minutes. Start all pods.
- Pass criteria:
  - First requests succeed within 30 seconds of pod readiness.
  - All workers re-acquire locks.
  - First WS connect within 60s.

### S14: Tenant Isolation Fuzz

- Synthetic load mixes operations across multiple orgs and branches with intentional malformed/malicious scope claims.
- Pass criteria:
  - Zero cross-org or cross-branch data leak.
  - Every attempt audited as `authz.denied{reason=...}`.

### S15: Long Reconnect Gap (Replay Window Boundary)

- Disconnect a client with many in-flight events on its session, exceed `event_replay_window` events while disconnected.
- Pass criteria:
  - On reconnect, server emits `SNAPSHOT_AUTHORITATIVE`.
  - Client refetches snapshot and resumes.

### S16: Frontend Crash Recovery

- Force-close a guest tab mid-order placement (after request, before response).
- Pass criteria:
  - Idempotent retry returns the original order.
  - No duplicate order.
  - Session state correctly reflects the order on next snapshot.

### S17: Pinball Browser History

- A guest visits the session via QR, then back-navigates, forward-navigates, navigates to bill, back, etc.
- Pass criteria:
  - No spurious mutations.
  - Stale tabs do not resubmit orders.
  - History-restored stale snapshots refresh on focus.

## 5. Burn-In Schedule

Per flag wave (see `production-enforcement-rollout.md`):

| Wave | Required burn-ins |
| --- | --- |
| R1 (Audit) | S1, S12 |
| R2 (Tenancy) | S1, S14 |
| R3 (Authz + Strict scope) | S1, S8, S14 |
| R4 (Staff code + DB sessions) | S1, S7, S13 |
| R5 (WS ticket) | S1, S3, S4, S15 |
| R6 (Guest credentials) | S1, S2, S14, S17 |
| R7 (Staff settlement) | S1, S10, S11, S16 |

Every wave also re-runs S1 to ensure prior gains are preserved.

## 6. Long Burn-In: 30-Day Soak

Before any single-shot production cutover (e.g., legacy code removal), a 30-day staging soak with synthetic load at 0.5x baseline must be clean. Clean means:

- Zero unhandled errors.
- No alert firing not tied to a known external dependency outage.
- Reconciliation report clean for 30 consecutive days.
- Memory growth bounded.

## 7. Chaos Engineering

Outside scheduled burn-ins, run targeted chaos:

- Random Redis kill, weekly, 30s duration.
- Random app pod kill, daily.
- Random network partition between app and Postgres, monthly, 30s.
- Random latency injection (100ms-500ms) on internal calls, weekly.

Chaos is paused during burn-in scenarios except when the scenario is itself chaos (S3, S4, S5, S6).

## 8. Deployment Verification Checklist

After every deploy to staging or production:

- [ ] `/healthz` returns 200 with build SHA matching the deploy.
- [ ] Migration version matches expected.
- [ ] Sample staff login from internal device succeeds.
- [ ] Sample guest session create/join/order succeeds.
- [ ] WS ticket issuance + WS upgrade succeeds.
- [ ] Sample webhook with known-good signature succeeds.
- [ ] Sample audit read by org owner returns events.
- [ ] Sample platform admin login succeeds (production only).
- [ ] Error rate dashboards at baseline within 5 minutes.

A deploy is not "complete" until this checklist is signed off.

## 9. Performance Budgets

Latency budgets (p99 at 0.5x baseline):

| Endpoint | Budget |
| --- | --- |
| GET `/sessions/:id/snapshot` | 250 ms |
| POST `/sessions` | 500 ms |
| POST `/sessions/:id/orders` | 700 ms |
| POST `/sessions/:id/payments` | 700 ms |
| POST `/staff/auth` | 400 ms |
| POST `/sessions/:id/ws-ticket` | 150 ms |
| GET `/branches/:id/menu` | 200 ms |
| Audit write inline (writer.Record) | 50 ms or async |

Throughput budgets at peak burst:

- 500 RPS sustained per branch.
- 50 concurrent WS connections per session (multi-tab safety).
- 200 concurrent active sessions per branch.

If any budget regresses by more than 10%, the deploy is rolled back.

## 10. Data Seeding

Staging DB is seeded with:

- 3 organizations.
- 9 branches across organizations.
- 20 staff per branch with diverse roles.
- 50 menu items, 10 categories, 30 modifiers per branch.
- 5 promos per branch.
- 1000 historical sessions across the previous 7 days for analytics realism.

Seed scripts in `backend/scripts/seed-staging/` (create if not present). Seed is run before each burn-in to a fresh DB; reuse is allowed for fast-iteration runs.

## 11. Reporting

Each burn-in produces:

- Start/end timestamps.
- Scenarios executed and pass/fail.
- Metrics deltas (latency, errors, memory, throughput).
- Any audit events of note.
- Sign-off by the engineer who ran it.

Reports stored in `docs/burn-in-reports/<date>-<wave>.md`. They are part of the production gate evidence.

## 12. Recovery After Burn-In Failure

When a burn-in scenario fails:

1. Capture full logs, metrics, and DB state.
2. File a ticket linked to the burn-in report.
3. Do not advance the corresponding rollout wave until the ticket is closed.
4. Re-run only the failing scenario plus S1 once fixed; full burn-in is not required unless the fix is invasive.

## 13. Pre-Production Cutover Gate

Before the production gate opens:

- All burn-ins per Section 5 have passed in staging.
- One 30-day soak (Section 6) has completed.
- Deployment verification (Section 8) has passed every deploy for 30 days.
- Performance budgets (Section 9) have held across the soak.
- Chaos engineering (Section 7) has been running without incident.
- The original 30-finding operational correctness audit has been re-walked against staging and is clean.

## 14. Documentation Linkage

- `production-enforcement-rollout.md` gates each wave on scenarios from here.
- `session-lifecycle-state-machine.md` defines invariants exercised in S2, S15, S17.
- `payment-finalization-invariants.md` defines invariants exercised in S8, S10, S11, S16.
- `realtime-reconciliation-invariants.md` defines invariants exercised in S3, S4, S5, S15.
- `operational-runbooks.md` is exercised in S4-S7, S11, S13.
- `playwright-behavioral-matrix.md` covers behavioral assertions for many of these scenarios at the UI level.

## 15. Not in Scope

- True load testing beyond peak burst (capacity planning is a separate exercise).
- Cross-region DR (current architecture is single region; multi-region is a post-Phase-9 program).
- Security penetration testing (separate engagement, see `security-hardening-checklist.md` Section 23).
