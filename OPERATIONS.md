# OPERATIONS.md — QR-Dining Day-2 Operations

**Canonical operations guide.** Companions: `DEPLOYMENT.md` (bring-up), `RECOVERY.md` (restore/disaster). Deeper runbooks: `operational-runbooks.md`, `support-runbooks-pilot.md`, `recovery-procedures.md`.

---

## 1. Health & status

| Check | How | Healthy looks like |
|---|---|---|
| Liveness | `curl -fsS https://api.<domain>/health` | 200 `{"status":"ok"}` |
| Readiness | `curl -fsS https://api.<domain>/readyz` | 200 with `checks.postgres/redis: ok`; 503 = a datastore is down |
| Containers | `cd /opt/qr-dining && docker compose ps` | app/postgres/redis + observability all `healthy`/`running` |
| App logs | `docker compose logs -f app` | structured zerolog; no `audit write failure`, no worker panics |
| Metrics | SSH tunnel → http://localhost:9090 (Prometheus), :9093 (Alertmanager) | all targets UP |

The app self-heals aggressively: restart recovery ~12s (migrations idempotent), Redis outage degrades realtime but HTTP keeps serving, workers are lock-guarded and panic-isolated. `restart: unless-stopped` everywhere.

## 2. Deploying a new version

```bash
# CI pushed ghcr.io/mohith1612/qr-dining:<tag> (on main push / v* tag)
cd /opt/qr-dining
vi .env                    # IMAGE_TAG=<tag>   (pin tags; avoid latest in prod)
docker compose pull app
docker compose up -d app   # ~12s recovery; guests reconcile via snapshot
curl -fsS localhost... /readyz via proxy; watch logs for "migrations"
```

Rollback = set `IMAGE_TAG` back and repeat. Schema is additive-only by project rule, so rolling the binary back across a migration is safe by design.

## 3. Alerting

- 27 Prometheus rules; `severity=page` (11 — must wake a human) vs `ticket` (16 — next business day). Backup alerts: `BackupFailed`, `BackupTooOld` (>36h).
- **Delivery endpoint:** one line — the `&notify_url` anchor in `/opt/qr-dining/observability/alertmanager.yml`. Point it at the real webhook (Slack relay etc.), then `docker compose restart alertmanager`. Both routes (page: 1h repeat; default: 4h repeat) inherit it.
- Test-fire: `docker compose exec alertmanager amtool alert add TestPage severity=page --annotation=summary="drill"` and confirm receipt.
- **Rollout-gate alerts** (`LegacyAuthzBypassPresent`, `LegacyIdentityUsagePresent`, `PolicyShadowMismatchPresent`, …) must read **zero** before flag flips — they are the gate, not noise.

## 4. Rollout flags (the enforcement ladder)

All nine are explicit in `/opt/qr-dining/.env`. Current production posture:

| Wave | Flag(s) | State |
|---|---|---|
| R1 | `AUDIT_LOG_V2_ENABLED` | **true — live, soaked. Never disable.** |
| R2 | `TENANCY_ORGANIZATIONS_ENABLED` | false — needs org backfill + soak |
| R3 | `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` (pair) | false — 48h zero-mismatch shadow gate |
| R4 | `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` (pair) | false — staff-code decay window |
| R5 | `WS_TICKET_AUTH_REQUIRED` | false |
| R6 | `AUTH_GUEST_CREDENTIALS_REQUIRED` | false — closes finding F-8 |
| R7 | `PAYMENT_STAFF_SETTLEMENT_REQUIRED` | false — needs webhook-replay CI proof |

**Flip procedure:** one wave at a time, never chained. Confirm the wave's gate metrics/alerts are clean → edit the flag in `.env` → `docker compose up -d app` → watch the gate metrics. Rollback is the same edit reversed (<5 min MTTR). Ledger + gate details: `docs/master-system-context-v1.md` §2.5.

## 5. Backups (monitoring side)

- Nightly 02:30 systemd timer → R2. Verify cadence: `systemctl list-timers qr-dining-backup*`; last run: `journalctl -u qr-dining-backup -n 30`.
- Prometheus: `qr_dining_backup_last_run_status` (0=ok) and `qr_dining_backup_last_success_timestamp_seconds`; alerts fire on failure or >36h staleness — but only if `NODE_EXPORTER_TEXTFILE_DIR` in `/etc/qr-dining/backup.env` points at the `qr-dining_backup_textfile` volume mountpoint.
- Restore drills and full procedures: `RECOVERY.md`. Run a drill within week 1 of go-live and quarterly after.

## 6. Routine cadence

- **Daily (week 1, ~10 min):** Prometheus targets UP, no firing alerts, backup metric fresh, disk `df -h` + `docker system df`, skim app logs for audit/worker errors.
- **Weekly:** review payment-escalation events (`PAYMENT_SETTLEMENT_STALLED` — every one is a human follow-up), postgres volume growth curve (audit_log dominates), prune old images (`docker image prune`), spot-check a session lifecycle in the support console.
- **Before any flag flip:** the wave's gate alerts at zero for the required window.

## 7. Shared-VM etiquette

qr-dining shares the VM (proxy, invoice, job-queue-system, sketchiple). Its services carry `mem_limit`s (app 1g / pg 1g / redis 384m). Never edit `/opt/proxy` config except adding/updating the `qr-dining.conf` vhost, and always `nginx -t` before `nginx -s reload`. The vhost's request-time DNS means qr-dining being down can never block other projects' proxy reloads — preserve that property.

## 8. Standing rules

- **Never touch the dev-machine soak stack** (docker project `qr-dining` on the development host): no `down -v`, no restarts without `AUDIT_LOG_V2_ENABLED=true`, no volume wipes.
- Billing is **manual and outside the system** (the billing subsystem is shadow) — do not charge anyone through it.
- Payment escalation is **alert-only**; a human settles or cancels. Never "fix" a stalled payment by mutating state directly — use the staff settlement endpoint or the runbooked psql procedures.
- No time-windowed promos until the promo timezone bug is fixed.
- Additive migrations only; the audit_log schema/trigger, session transition table, bill snapshots, and the loyalty ledger are never touched casually.

## 9. Support surface

Read-only platform support console (search, session/order/payment inspection, audit explorer) + enforcement observability endpoints. Escalation beyond the console = psql via the runbooks in `support-runbooks-pilot.md` / `recovery-procedures.md`. Every support session is itself audited.
