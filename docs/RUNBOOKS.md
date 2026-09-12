# Incident Runbooks

Diagnose-first playbooks for a running deployment. Every metric name below was verified to exist in `backend/internal/observability/metrics.go`.

Companions: [OPERATIONS.md](OPERATIONS.md) (day-2) · [RECOVERY.md](RECOVERY.md) (data loss) · [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) (when to stop) · [SECURITY.md](SECURITY.md).

> **Topology reality check.** This is a **single-host Docker Compose** deployment: one app container, one Postgres, one Redis, behind a shared nginx. There are no pods, no replicas, no shards and no failover target. "Restart the container" is the only containment lever for a given service. Any runbook that tells you to drain a pod or fail over to a replica is describing a system that does not exist.

---

## 0. Conventions

- **Sev1** — customer-visible total outage, money-handling impacted, or data-loss risk.
- **Sev2** — degraded service or partial outage.
- **Sev3** — latent issue, service unaffected.

Each runbook: detection → triage → containment → recovery → verification.

**Diagnose through the read-only Support Console (`/platform/support`) first.** Never open a psql shell against production unless a runbook here explicitly says to.

**During the pilot, every runbook here has a time limit.** These playbooks assume you can keep working the problem. In a live service you cannot: [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) §3 caps in-service debugging at 30 minutes and tells you when to put the restaurant on paper instead. Read that clock first, then come back here.

## 1. Auth outage (staff or guest)

**Detection.** `guest_token_validation_failed_total` or `authz_denied_total` rate well above baseline; `staff.login.failed` audit events spiking; `db_errors_total` rising; `audit_write_failures_total` non-zero.

**Triage.**
1. `curl /readyz` — if Postgres is unhealthy this is a database incident (§7), not an auth incident.
2. Check `GUEST_TOKEN_SECRET` and `MFA_ENCRYPTION_KEY` in `/opt/qr-dining/.env`. A bad rotation is the most common cause. (There is no `STAFF_TOKEN_SECRET` — staff sessions are database-backed, not secret-signed.)
3. Check `audit_write_failures_total`. `AUDIT_LOG_V2_ENABLED=true` means audit writes accompany auth; if the audit writer is failing, auth symptoms follow.
4. Check `rate_limiter_unavailable_total` — sensitive routes fail **closed**, so a Redis outage presents as auth denial (§2).

**Containment.** If the regression correlates with a deploy, roll `IMAGE_TAG` back ([OPERATIONS.md §2](OPERATIONS.md#2-deploying-a-new-version)). Flipping `AUTH_GUEST_CREDENTIALS_REQUIRED` or the R4 pair back to permissive is a **Sev1 decision requiring explicit approval** — it is time-bounded and supervised, never a resting state.

**Recovery.** Restore the previous image, or re-apply the correct secrets and `docker compose up -d app`.

**Verification.** A test staff login from an internal device succeeds. A test guest QR scan → join → cart action succeeds. `guest_token_validation_failed_total` returns to baseline.

## 2. Redis outage

**Detection.** `redis_pubsub_connected == 0`; `redis_pubsub_errors_total` / `redis_pubsub_reconnects_total` climbing; `rate_limiter_unavailable_total` non-zero; `cache_misses_total` at 100%; `/readyz` 503 naming `redis: unhealthy`; WebSocket clients dropping en masse.

**Triage.** Confirm from inside the network: `docker compose exec redis redis-cli ping`. Distinguish full outage from latency.

**Containment.** Mostly automatic and intended:
- Menu cache misses fall through to Postgres — higher DB load, no data loss.
- Realtime stops; clients fall back to snapshot reconciliation.
- Staff and platform sessions are Redis-backed, so **active logins are lost** and users must re-authenticate.
- Rate limiting **fails closed on sensitive routes** (staff auth, platform auth, payments, webhooks, WS tickets) and fails open elsewhere. Verify that asymmetry is holding rather than "fixing" the denials.

**Recovery.** `docker compose restart redis`. Publishing resumes with no manual intervention. Presence rebuilds on the next heartbeat cycle.

**Verification.** `redis_pubsub_connected == 1`; `/readyz` 200; a status change on one device appears on another within ~2s.

**Post-incident.** If the outage exceeded 5 minutes, confirm nothing treated Redis as a source of truth. There should be nothing — document it if you find something.

## 3. WebSocket degradation

**Detection.** Connections succeed but "live updates aren't arriving." `ws_connections_active` unexpectedly low or flat; `ws_client_evictions_total` or `ws_inbound_dropped_total` climbing; `ws_reconnects_total` spiking; `event_publish_duration_seconds` p95 elevated; `ws_ticket_consume_failed_total` non-zero.

**Triage.** Trace one session end to end: DB insert → publish → hub broadcast → client receive. Check `redis_pubsub_connected` — if it is 0, this is §2.

**Containment.** `ws_ticket_consume_failed_total` climbing usually means ticket issuance is rate-limited (reconnect storm) — that endpoint must stay available or clients cannot reconnect at all. Frontend snapshot reconciliation on focus keeps users approximately correct meanwhile.

**Recovery.** `docker compose restart app`. Clients reconnect and reconcile via `GET /sessions/:id/snapshot`.

**Verification.** Sample session: a status change on one device appears on another within 2 seconds. `ws_client_evictions_total` flat.

**Post-incident.** Check for reports of "I have to refresh to see updates" — that indicates clients stuck on stale sequence numbers, which the reconciliation contract is supposed to prevent ([reference/realtime-reconciliation-invariants.md](reference/realtime-reconciliation-invariants.md)).

## 4. Stuck payments

**Detection.** `payment_pending_escalations_total` climbing; `PAYMENT_SETTLEMENT_STALLED` alerts; staff reporting "the guest paid but it isn't clearing."

**Triage.** Find the session in the Support Console. Read the lifecycle timeline: was the payment initiated? Did a webhook arrive? Is the session in `payment_pending`?

**Containment.** A session in `payment_pending` has its cart and order placement frozen by design — that is the invariant working, not a bug.

**Recovery.** **A human settles or cancels, through the staff settlement endpoint.** Never mutate payment or session state directly. Escalation is alert-only precisely so that a machine never decides money.

**Verification.** Session leaves `payment_pending`; the table returns to `available`; the bill snapshot is unchanged.

**Post-incident.** Every stalled payment is a human follow-up in the weekly review. If the cause was a missing webhook, see §5.

## 5. Webhook outage or replay storm

**Detection.** Payments initiating but never completing; `idempotency_replays_total` spiking; webhook route rate limiting.

**Triage.** Webhook receipt is idempotent and its rate limiter **fails closed**. A burst of replays is therefore contained by design — confirm that before treating it as an incident.

**Containment.** For a genuine replay storm, the per-provider rate limit is the containment. Do not raise it to "let them through."

**Recovery.** Once the provider recovers, replays are absorbed idempotently. Payments that never received a terminal webhook are settled by a human (§4).

> The pilot is cash/UPI-manual with simulated webhooks. Exact-replay proof in CI is deferred and gates rollout wave R7.

## 6. Feature flag rollback

**When.** A flipped enforcement flag causes denials for legitimate traffic.

**Steps.**
1. Confirm the symptom maps to the flag — check the wave's gate metric (`legacy_authz_bypass_total`, `legacy_identity_usage_total`, `policy_shadow_mismatch_total`, `tenant_resolution_failures_total`).
2. Edit the flag in `/opt/qr-dining/.env`.
3. `docker compose up -d app`.
4. Watch the gate metric and error rate return to baseline.

**Constraints.**
- **`AUDIT_LOG_V2_ENABLED` is never rolled back.** R1 is live and soaked; disabling it breaks the audit guarantee.
- A rollback to a legacy auth path is time-bounded and supervised. Re-flip once the cause is fixed.
- Flags are reversible in under five minutes. If a rollback is taking longer, you are fixing the wrong thing.

## 7. Database incident

**Detection.** `/readyz` 503 naming `postgres: unhealthy`; `db_errors_total` climbing; `db_pool_acquired_conns` pinned at `db_pool_total_conns`.

**Triage.** `docker compose ps postgres`; `docker compose logs postgres`. Distinguish *unavailable* (restart it) from *corrupt or badly mutated* (restore — [RECOVERY.md §3b](RECOVERY.md#3b-production-restore-destructive--real-incident-only)).

**Containment.** Pool exhaustion without a Postgres fault usually means a slow query or a leak: `docker compose restart app` reclaims the pool and buys time.

**Recovery.** Restart, or restore. Data loss on restore is bounded by the last nightly backup (RPO ≤24h).

**Verification.** `/readyz` 200; row counts sane; **an `UPDATE` against `audit_log` must fail** — that proves the append-only trigger survived.

## 8. Migration rollback

> **There is no schema rollback. Do not attempt one.** Roll the *binary* back
> ([OPERATIONS.md §2](OPERATIONS.md#2-deploying-a-new-version)); never the *schema*.

**Why CI's green migration job does not mean what it looks like.** CI runs `up → down -all → up`
against an **empty** database. That proves the DDL parses. It proves nothing about a migration
meeting data, and three of the worst defects below pass CI and always will.

**Established by a full audit of every up and down migration, and verified empirically on
2026-09-12** ([../docs/history/restore-verification-report-2026-09-12.md](history/restore-verification-report-2026-09-12.md) Part 2):

| Migration | What its `down` actually does |
|---|---|
| **16–24** | Unsafe as a band. |
| **21** | Drops `bill_snapshots` and `idempotency_keys` plus eight `payments` columns including `bill_snapshot_id` and `settled_at` — **money data** — then fails re-adding a unique constraint that migration 21 itself made unsatisfiable. |
| **22** | Destroys every receipt number, visit number and audit reference. |
| **20** | Leaves `sessions` with **no** one-session-per-table index at all (below 20, only the non-unique `idx_sessions_table_id` from `000001:148` remains). Two parties can then order into one table's bill. |
| **36** | **Deletes every payment-time promo redemption** (`DELETE FROM promo_redemptions WHERE order_id IS NULL`). Verified: the guest keeps the discount — it is in the immutable bill snapshot — but the record enforcing `uses_per_phone` is gone, and rolling forward does not restore it. |
| **38, 39** | Unsafe. 39's down is a documented no-op: the original values are unrecoverable by design. |

**And the one that wedges the system:**

**Migration 22's `up` cannot be re-applied to a populated `audit_log`.** `000022:109` runs
`UPDATE audit_log` against the `BEFORE UPDATE ... RAISE EXCEPTION` immutability trigger from
`000019:82`. It passes on an empty table and aborts with a single row present:

```
ERROR: audit_log rows are immutable: UPDATE is not permitted (SQLSTATE P0001)
```

golang-migrate then leaves `schema_migrations` at **`22, dirty=true`**, and:

- the app will not start — `RunMigrations` calls the same `m.Up()`, which returns
  `Dirty database version 22. Fix and force version.`
- `cmd/migrate` has no `force` subcommand, so there is no in-tree tool to unwedge it;
- clearing `dirty` by hand is not enough — the next `up` hits the trigger and re-wedges.

Migration 40's `up` has the same wedge shape for a different reason: `000040:19` raises if any
session has more than one non-terminal payment, which a pre-40 dump can legitimately contain.

### Last resort: unwedging a dirty migration at 22

**This drops the audit tamper-evidence guarantee for the duration. It is the procedure of last
resort, it is [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) A6, and its 60-minute clock is
already running because the app is down.** Take a backup first. Record in the incident report the
exact window during which the trigger was absent.

```sql
-- 1. Confirm the wedge.
SELECT version, dirty FROM schema_migrations;      -- expect: 22, true

-- 2. Step the recorded version back to the last clean one.
UPDATE schema_migrations SET version = 21, dirty = false;

-- 3. Remove the trigger that migration 22 collides with. audit_log is now WRITABLE.
DROP TRIGGER trg_audit_log_immutable ON audit_log;
```
```bash
# 4. Replay forward.
docker compose up -d app        # or: DATABASE_URL=… migrate up
```
```sql
-- 5. Re-create the trigger IMMEDIATELY. Do not open the restaurant before this.
CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

-- 6. Prove it bites again. This MUST fail:
UPDATE audit_log SET action = 'x' WHERE id = (SELECT min(id) FROM audit_log);
```

Verified end to end on 2026-09-12; steps 2–4 reach version 40 clean, and step 6 errors as
required.

**Also know:** 22's backfill regenerates `session_number` / `visit_number` / `payment_reference`
with `ROW_NUMBER()`. On an unchanged row population it reproduces the same values exactly. If any
row has been added or removed since, it **reassigns** them — one purged session in a six-session
partition reassigned four of the five remaining visit numbers under test. Receipts already given
to guests, and support tickets quoting a session number, then point at a different visit.

**Prevention, which is far cheaper than any of the above:** no deploys during service hours, and
a verified backup before every deploy ([PILOT-ABORT-CRITERIA.md §5](PILOT-ABORT-CRITERIA.md)).

## 9. Stale sessions / stranded tables

**Detection.** "Table shows occupied but nobody's there"; a new session cannot start on a table.

**Triage.** Support Console → look up the table → check the session state and the last participant activity.

**Containment.** The stale session cleaner and the session/table reconciler run on their intervals and are self-healing — check `background_worker_runs_total` and `background_worker_panics_total` first. If the workers are running, wait one interval before intervening.

**Recovery.** If a worker has panicked, `docker compose restart app`. Workers are Redis-`NX`-lock guarded and panic-isolated, so one failure should not stop the others.

**Verification.** Table returns to `available`; a new session can start on it.

## 10. Support escalation

Everything read-only goes through the Support Console. Escalate beyond it only when the console genuinely cannot answer the question, and:

- Reads before writes, always.
- Never mutate `audit_log`.
- Never reset the soak or production database.
- Test every destructive step against an isolated database first.
- Preserve the strict-rollout flags across restarts.
- Prefer the governed surfaces — Support Console, billing, platform lifecycle — over psql for anything they cover.

## 11. Common support requests

| Symptom | First move |
|---|---|
| "Guest paid but it's not clearing" | §4. Support Console → session timeline → human settlement |
| "Table occupied but empty" | §9. Check worker health before intervening |
| "Connection lost / UI not updating" | §3. Check `redis_pubsub_connected` first |
| "QR doesn't open the menu / wrong table" | Verify the table's QR token in Support Console. Tokens regenerate on a stack reset |
| "Is this tenant paid / expiring?" | Billing surface, needs `billing_admin`. **Billing is shadow — it records, it does not charge** |

## 12. Tenant disable (abuse / non-payment / emergency)

Use the platform lifecycle surface, not the database. Organization and branch status flips are audited. Note that lifecycle enforcement is currently **resolve-only** — a status flip is recorded and visible but does not by itself cut off access. Do not assume it does.

## 13. Break-glass

If the platform control plane itself is unavailable and a Sev1 requires action:

1. Record why break-glass is being used, before doing it.
2. Prefer `docker compose exec` against the app over psql against the database.
3. Any psql session is read-only unless the incident commander approves a write, in writing.
4. Every break-glass action is reconstructed into the incident report, since it bypasses the audit surface.

## 14. Disaster recovery

Data loss, volume loss, or whole-host loss: [RECOVERY.md](RECOVERY.md). Bad deploy is **not** a restore case — roll the image tag.

A restore is safe **only into the schema version the dump was taken at** (§8). And if containment is not working, the question stops being "how do I fix this" and becomes "should this pilot still be running": [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md).

## 15. Cold restart drill (quarterly)

```bash
cd /opt/qr-dining
docker compose down          # NOT -v. Never -v.
docker compose up -d
```

Confirm: migrations apply idempotently; `/readyz` 200 within ~12s; Prometheus targets return UP; one full guest journey succeeds. Record the actual recovery time.

## 16. Network partition drill (quarterly)

Use the chaos harness rather than improvising: `./scripts/chaos/run-all.sh` covers Redis flap, backend restart, nginx reload and webhook replay, capturing metrics and logs to `scripts/chaos/results/`. See [../scripts/chaos/README.md](../scripts/chaos/README.md).

## 17. Post-incident

Write a report with the timeline, blast radius (how many sessions and tenants), recovery steps and root cause. File a ticket for every process gap. If a runbook here was wrong or missing, fix it in the same week — a runbook that misleads on-call is worse than no runbook.
