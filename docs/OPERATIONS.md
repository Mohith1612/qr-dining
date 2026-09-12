# Operations

Day-2 operation of a running QR Dining deployment.

Companions: [DEPLOYMENT.md](DEPLOYMENT.md) (bring-up) · [RUNBOOKS.md](RUNBOOKS.md) (incidents) · [RECOVERY.md](RECOVERY.md) (backup/restore) · [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) (when to stop the pilot) · [SECURITY.md](SECURITY.md).

---

## 1. Health and status

| Check | How | Healthy looks like |
|---|---|---|
| Liveness | `curl -fsS https://api.<domain>/health` | 200 `{"status":"ok"}` |
| Readiness | `curl -fsS https://api.<domain>/readyz` | 200 with `checks.postgres/redis: ok`; **503 = a datastore is down** |
| Containers | `cd /opt/qr-dining && docker compose ps` | app/postgres/redis + observability all `healthy`/`running` |
| App logs | `docker compose logs -f app` | structured zerolog; no `audit write failure`, no worker panics |
| Metrics | SSH tunnel → `localhost:9090` (Prometheus), `:9093` (Alertmanager) | all targets UP |
| Traces | SSH tunnel → `localhost:3301` (SigNoz), when `OTEL_ENABLED=true` | spans arriving for recent requests |

`/readyz` is the authoritative datastore-outage signal. The Redis pub/sub gauge stays at 1 through a transient blip, so it is unreliable on its own — this was a real finding, not a theoretical one.

The app self-heals aggressively: restart recovery is ~12s (migrations are idempotent), a Redis outage degrades realtime while HTTP keeps serving, and workers are lock-guarded and panic-isolated. Everything runs `restart: unless-stopped`.

## 2. Deploying a new version

```bash
cd /opt/qr-dining
vi .env                    # IMAGE_TAG=<tag>   pin tags; never `latest` in production
docker compose pull app
docker compose up -d app   # ~12s recovery; guests reconcile via the snapshot endpoint
curl -fsS https://api.<domain>/readyz
docker compose logs app | grep -i migrat
```

Rollback is the same procedure with the previous `IMAGE_TAG`.

> **Binary rollback is the only rollback.** Additive-only is a *project rule for new migrations*,
> not a property the existing schema has. Rolling the **schema** back is unsafe across migrations
> 16–24 and at 36, 38 and 39, and migration 22 cannot even be re-applied to a populated
> `audit_log` — it aborts and leaves `schema_migrations` dirty, which stops the app from starting.
> Verified 2026-09-12; details and the last-resort recovery in
> [RUNBOOKS.md §8](RUNBOOKS.md#8-migration-rollback).
>
> Rolling the *binary* back across a migration is safe only when the release you are leaving added
> no migration, or added a strictly additive one the old binary ignores. **Know which before you
> need it**, not during an incident.

**During the pilot: no deploys during service hours.** Deploy before opening, with a verified
backup taken first, and run one synthetic guest journey before the doors open. Every recovery path
below a binary rollback costs ≥1 hour with one developer, which is not a mid-service option
([PILOT-ABORT-CRITERIA.md §5](PILOT-ABORT-CRITERIA.md)).

Deploying an unmerged branch (what the beta runs) is [DEPLOYMENT.md §3b](DEPLOYMENT.md#3b-deploying-an-unmerged-branch-build-on-the-vm).

## 3. Alerting

### What exists

| File | Role |
|---|---|
| `deploy/observability/prometheus.yml` | scrapes app `/metrics`, the `/readyz` blackbox probe and node-exporter; loads rules; forwards to Alertmanager |
| `deploy/observability/prometheus-alerts.yml` | **31 alert rules**, `promtool`-clean (27 baseline + 4 backing the pilot abort criteria) |
| `deploy/observability/alertmanager.yml` | routing + receivers |
| `deploy/observability/blackbox.yml` | `/readyz` HTTP probe module |
| `deploy/observability/grafana-dashboard.json` | importable dashboard (Grafana is not deployed) |
| `backend/scripts/nightly-backup.sh` | emits `qr_dining_backup_*` textfile metrics |

The app emits **43 metric collectors** covering HTTP, WebSocket, workers, escalations, cache, DB pool, audit failures, rate limits and tenant resolution.

### Severities

`severity=page` (14 rules — must wake a human) versus `ticket` (17 — next business day). An inhibit rule mutes `ticket` alerts while the same alertname is already paging.

The `qr-dining-pilot-abort` group exists so that each criterion in [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) has an observable signal: `CrossTenantScopeDenial`, `AppUnavailableAbortThreshold`, `AuditWriteFailureAny`, `BackupStaleEarlyWarning`. Two criteria — a guest charged incorrectly, and a single table unable to order or pay — **have no possible signal today** and are human-detected; that is stated there rather than papered over with an aspirational rule.

### The four that must always work

| Condition | Alert | Signal |
|---|---|---|
| App down | `AppTargetDown` | `up{job="qr-dining"} == 0` |
| Database unavailable | `ReadyzProbeFailing` (+ `PostgresPoolExhausted`, `PostgresQueryErrors`) | `/readyz` 503, body names `postgres: unhealthy` |
| Redis unavailable | `ReadyzProbeFailing` (+ `RedisPubSubDisconnected`) | `/readyz` 503, body names `redis: unhealthy` |
| Backup failure | `BackupFailed`, `BackupTooOld` | `qr_dining_backup_last_run_status != 0`, or no success in >36h |

### Delivery

One line changes it: the `&notify_url` anchor in `/opt/qr-dining/observability/alertmanager.yml`. Point it at a real webhook, then `docker compose restart alertmanager`. Both routes inherit it — `page` repeats hourly, `default` every 4 hours. Webhook is the default because one URL can target Slack, Teams, PagerDuty, Opsgenie or a relay; native receiver blocks are included commented out.

Test-fire:

```bash
docker compose exec alertmanager amtool alert add TestPage severity=page \
  --annotation=summary="drill"
```

The route and receiver have been verified end to end against a webhook sink. **Before go-live, set a real receiver and send one test alert through it** to confirm the credential and channel — this is an open SEV-1, see [RELEASE.md](RELEASE.md#5-open-before-v10).

This is single-Prometheus / single-Alertmanager with no HA. Correct for a pilot; revisit for production scale.

### Rollout-gate alerts

`LegacyAuthzBypassPresent`, `LegacyIdentityUsagePresent`, `PolicyShadowMismatchPresent` and friends must read **zero** before a flag flip. They are the gate, not noise.

## 4. Rollout flags (the enforcement ladder)

All nine are explicit in `/opt/qr-dining/.env`. Never rely on application defaults for a production record.

| Wave | Flag(s) | State |
|---|---|---|
| R1 | `AUDIT_LOG_V2_ENABLED` | **true — live and soaked. Never disable.** |
| R2 | `TENANCY_ORGANIZATIONS_ENABLED` | false — needs org backfill + soak |
| R3 | `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` (pair) | false — 48h zero-mismatch shadow gate |
| R4 | `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` (pair) | **true — launch baseline** |
| R5 | `WS_TICKET_AUTH_REQUIRED` | **true — launch baseline** |
| R6 | `AUTH_GUEST_CREDENTIALS_REQUIRED` | **true — launch baseline, `GUEST_TOKEN_TTL=12h`** |
| R7 | `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | false — needs webhook-replay CI proof |

R4–R6 were activated together before traffic because there is no legacy client population — the only frontend already speaks all three protocols.

**Change procedure:** confirm the wave's gate metrics read zero for the required window → edit `.env` → `docker compose up -d app` → watch the gate metrics. A rollback to a legacy auth path is time-bounded and supervised, never a steady-state posture. Rollback detail: [RUNBOOKS.md §6](RUNBOOKS.md#6-feature-flag-rollback).

## 5. Backups (monitoring side)

- Nightly 02:30. Verify cadence with `systemctl list-timers qr-dining-backup*`; last run with `journalctl -u qr-dining-backup -n 30`. The rootless containerized path (what the beta runs) uses an `appuser` crontab instead — see [../deploy/backup/README.md](../deploy/backup/README.md).
- Prometheus: `qr_dining_backup_last_run_status` (0 = ok) and `qr_dining_backup_last_success_timestamp_seconds`. Alerts fire on failure or >36h staleness — **but only if** `NODE_EXPORTER_TEXTFILE_DIR` points at the `qr-dining_backup_textfile` volume mountpoint. If that wiring is missing the alerts are silently useless.
- **`qr_dining_backup_last_run_status` can report success while the backup is broken.** `nightly-backup.sh` exits at its `DATABASE_URL` guard *before* the `ERR` trap is installed, so a missing or unreadable env file leaves the metric showing the last good run; `BackupFailed` never fires and only `BackupTooOld` catches it, 36h later. The same blind spot covers "cron never fired at all" — plausible on the rootless path, where a user crontab does not survive a VM rebuild. **Read `qr_dining_backup_last_success_timestamp_seconds` directly each morning rather than trusting the absence of a page.** Finding R-3, [history/restore-verification-report-2026-09-12.md](history/restore-verification-report-2026-09-12.md).
- Run a restore drill within week 1 of go-live and quarterly after. Procedures: [RECOVERY.md](RECOVERY.md).

## 6. Routine cadence

**Daily (week 1, ~10 min).** Prometheus targets UP; no firing alerts; **read** `qr_dining_backup_last_success_timestamp_seconds` (do not infer freshness from the absence of an alert — see §5); `SELECT version, dirty FROM schema_migrations` reads the expected version with `dirty = false`; an `UPDATE` against `audit_log` still fails; `df -h` and `docker system df`; skim app logs for audit and worker errors.

**Weekly.** Review payment-escalation events — every `PAYMENT_SETTLEMENT_STALLED` is a human follow-up. Check the Postgres volume growth curve (`audit_log` dominates). `docker image prune`. Spot-check one session lifecycle in the support console.

**Before any flag flip.** The wave's gate alerts at zero for the required window.

**Quarterly.** Restore drill ([RECOVERY.md §3a](RECOVERY.md#3a-drill--side-restore-non-destructive--also-the-quarterly-drill)), cold restart drill and network partition drill ([RUNBOOKS.md §15–16](RUNBOOKS.md)).

## 7. Payment escalation

`payment_pending` has a bound, and crossing it raises `PAYMENT_SETTLEMENT_STALLED` at warn/critical thresholds (defaults 1m/5m/15m).

**It is alert-only, by design.** The system never auto-settles, auto-cancels or auto-refunds. A human resolves every stalled payment through the staff settlement endpoint. Never "fix" one by mutating state directly.

## 8. Onboarding a restaurant

The platform UI orchestrates this end-to-end at `/platform/onboarding` (super-admin only). Each wizard step maps to a backend action that writes a `platform_audit_log` row.

**Before the call:** confirm the plan tier (free/standard/premium) and trial-vs-paid; collect restaurant name, desired slug, legal name, owner contact email; collect billing name, GST number, billing email.

**In the wizard:** create the organization and its primary branch → create tables → create staff accounts (at minimum one owner, one kitchen, one waiter) → build the menu with categories, items, prices and availability → set theme/branding → generate QR collateral.

**Go-live verification, with the restaurant, before the first real guest scans anything:**

- [ ] Owner signs in with **branch code + staff code + PIN**.
- [ ] Each role reaches its dashboard: Admin, Kitchen (`/staff/kitchen`), Waiter (`/staff/waiter`).
- [ ] Menu categories and items exist with correct prices; toggling availability off makes the item disappear for guests live.
- [ ] A guest device scans a real table QR and reaches the branded join screen.
- [ ] Shared cart syncs live between two guest devices in one session.
- [ ] An order reaches the kitchen display and moves through to served.
- [ ] Assistance request reaches the waiter view.
- [ ] Bill totals are correct; a cash settlement closes the session and frees the table.
- [ ] Staff know that **billing is manual and outside the system**.

## 9. Support surface

Diagnose through the **read-only Support Console** (`/platform/support`) — never open a psql shell against production. It searches sessions, orders, payments, tables and participants, and shows sanitized detail plus the lifecycle timeline and audit trail. Every support read is itself audited; deep reads emit a tenant-visible row.

RBAC: support reads need `support_admin` or `read_only_auditor`. Billing surfaces additionally need `billing_admin`.

Escalation beyond the console: [RUNBOOKS.md](RUNBOOKS.md).

## 10. Shared-VM etiquette

qr-dining shares the VM with other projects (proxy, invoice, job-queue-system, sketchiple). Its services carry `mem_limit`s (app 1g / postgres 1g / redis 384m).

Never edit `/opt/proxy` config except to add or update the `qr-dining.conf` vhost, and always `nginx -t` before `nginx -s reload`. The vhost uses **request-time DNS** (`resolver 127.0.0.11` plus a variable upstream) deliberately: it means qr-dining being down can never block another project's proxy reload. Preserve that property.

## 11. Standing rules

- **Never touch the development-host soak stack** (compose project `qr-dining`): no `down -v`, no restart without `AUDIT_LOG_V2_ENABLED=true`, no volume wipes.
- **Billing is manual and outside the system.** The billing subsystem is shadow — do not charge anyone through it.
- **Payment escalation is alert-only.** A human settles or cancels.
- **No time-windowed promos** until the promo timezone behaviour is confirmed against the branch-local clock.
- **Additive migrations only** — for *new* migrations. The existing schema is **not** rollback-safe (16–24, 36, 38, 39), so never plan around a schema rollback: [RUNBOOKS.md §8](RUNBOOKS.md#8-migration-rollback). The `audit_log` schema and trigger, the session transition table, bill snapshots and the loyalty ledger are never touched casually.
- **No deploys during service hours** while the pilot is running.
- **Never mutate `audit_log`.** It is trigger-protected; an attempt should fail, and that failure is a verification step, not a bug.
