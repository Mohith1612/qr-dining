# R1 Activation Runbook — `AUDIT_LOG_V2_ENABLED`

Date: 2026-05-26
Wave: **R1** (first wave of the staged strict-flag rollout).
Scope: enables exactly ONE flag — `AUDIT_LOG_V2_ENABLED`. **Nothing else.** No auth,
websocket, guest, tenancy, or payment enforcement changes in this wave.

R1 is the lowest-risk wave: a pure write-path addition (one extra INSERT per audited
action). The read path is unaffected. This runbook is executable by an operator with no
prior context.

References: `production-enforcement-rollout.md` §Wave R1; `r1-soak-monitoring-guide.md`;
`production-alerting-baseline.md`.

---

## 0. What R1 does (and does not do)

- **Does:** turns on the audit-log V2 writer. Audited actions (staff auth, order place,
  payment initiate/settle/webhook, authz denials, session lifecycle, etc.) write an
  immutable row to `audit_log`.
- **Does NOT:** change any request's success/failure, reject any caller, or alter
  latency-sensitive read paths. No flag other than `AUDIT_LOG_V2_ENABLED` is touched.

Blast radius: write amplification + storage growth. No behavioral change to clients.

---

## 1. Pre-rollout checklist (ALL must be true before flipping)

Tick every box. If any is unknown, **stop** — R1 has not started.

- [ ] **Immutable trigger present.** `trg_audit_log_immutable` exists on `audit_log`
      (migration `000019_audit_log_v2`). Verify:
      `\d+ audit_log` shows trigger `trg_audit_log_immutable`; an `UPDATE`/`DELETE`
      attempt raises `audit_log rows are immutable`.
- [ ] **App healthy.** `/readyz` returns 200 with `{"postgres":"ok","redis":"ok"}` on all
      pods.
- [ ] **Metrics live.** Prometheus is scraping `/metrics`; `audit_write_failures_total`
      is queryable (currently flat-zero because the writer is gated off).
- [ ] **Alert wired & routed.** `AuditWriteFailures` (page) is loaded
      (`promtool check rules deploy/observability/prometheus-alerts.yml` → SUCCESS) and
      routed to the on-call pager. Confirm a test route delivers.
- [ ] **Dashboard reachable.** Grafana `qr-dining-ops` opens; the "Integrity" panels
      (audit write failures, rate-limiter unavailable) render.
- [ ] **Disk headroom.** At least 60 days of expected audit volume free on the Postgres
      data volume. Estimate per `r1-soak-monitoring-guide.md §2`.
- [ ] **Backups current.** Latest `backend/scripts/backup.sh` (or R2 backup) completed and
      restorable; `restore.sh` rehearsed.
- [ ] **Change ticket + two-person ack.** One engineer + one ops contact have
      acknowledged the flip start (per `production-enforcement-rollout.md §10`).
- [ ] **Low-traffic window.** Flip during the lowest-traffic local window for the
      predominant branch timezone.
- [ ] **No coupled change.** This deploy changes ONLY the env var. No migration, no code
      change in the same window (migrations ship in a previous release).

---

## 2. Exact flip procedure

1. Capture a **baseline** (5 min before flip), record values for post-flip comparison:
   - `sum(rate(http_requests_total[5m]))` and 5xx ratio
   - p99 on audited write routes:
     `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{path=~"/sessions/:id/orders|/sessions/:id/payments|/staff/auth"}[5m])))`
   - `audit_log` row count: `SELECT count(*) FROM audit_log;`
   - DB pool: `db_pool_acquired_conns`, `db_pool_idle_conns`
   - Postgres data-volume free space
2. Set the environment variable **only**: `AUDIT_LOG_V2_ENABLED=true`.
3. **Rolling restart** the app (one pod at a time; wait for `/readyz` 200 before the
   next). Do not restart all pods at once.
4. Confirm the flag is live: the boot log shows the audit writer enabled; a known audited
   action produces a new `audit_log` row.

> The flip is environment-variable-only. If you find yourself editing code or running a
> migration, **abort** — that violates the single-change rule.

---

## 3. Verification sequence (first 15 minutes)

Run in order; all must hold:

1. **Audit rows growing with traffic.** `SELECT count(*) FROM audit_log;` increases in
   line with request volume (not flat, not exploding disproportionately).
2. **Write failures ~zero.** `rate(audit_write_failures_total[5m]) == 0`. Any sustained
   non-zero is a problem (writer is non-fatal, so requests still succeed, but the audit
   trail has gaps — see rollback conditions).
3. **Latency within budget.** p99 on audited write routes within **+10 ms** of the
   recorded baseline. The extra INSERT is async/non-fatal; a larger regression means
   contention — investigate before continuing the soak.
4. **Readiness stable.** `/readyz` stays 200 on all pods; no pod restart loop.
5. **Immutability holds.** Spot-check: an attempted `UPDATE audit_log ...` still raises
   the immutable exception.

---

## 4. Immediate post-rollout checks + cadence

- **First 2 hours:** review §3 signals every **15 minutes**.
- **After 2 hours → end of soak:** review **hourly** (then per the soak guide's daily
  cadence for the remainder).
- Watch the Grafana Integrity row + the audited-route latency panel.
- Record `audit_log` row count + disk free at each check to build the growth curve
  (feeds the storage projection in the soak guide).

---

## 5. Rollback

**Rollback is cheap and safe — never hesitate to use it.**

- **Action:** set `AUDIT_LOG_V2_ENABLED=false`, rolling restart. **MTTR < 5 min.**
- **Data implications:** none. Existing `audit_log` rows remain (immutable); the writer
  simply stops emitting; readers degrade gracefully. No corruption risk.

### Roll back if ANY of these occur:
- `audit_write_failures_total` rate stays non-zero for >5 min (the `AuditWriteFailures`
  page fires) and is not an obvious transient — the audit trail is developing gaps.
- p99 on audited write routes regresses >10 ms sustained and is attributable to the flip.
- DB pool exhaustion (`db_pool_idle_conns == 0` with `acquired >= total`) tied to audit
  writes.
- Postgres disk growth far exceeds the projection (e.g. >2× expected rate) threatening
  headroom.
- Any pod restart loop or `/readyz` instability that correlates with the flip.

---

## 6. Escalation conditions

Escalate (page the rollout lead + DBA) rather than silently rolling back when:
- The immutable trigger is found missing/altered post-flip (integrity guarantee broken).
- Audit write failures correlate with a specific action/route class (possible bug, not
  load) — capture `audit_write_failures_total{action,error_class}` labels before rollback.
- Storage growth is anomalous in shape (spikes, not linear) — possible runaway emitter.

Two-person rule applies to both the flip and any rollback: one engineer + one ops contact
acknowledge.

---

## 7. Exit gate (to clear R1 and unlock R2 consideration)

Per `production-enforcement-rollout.md §Wave R1`:
- **Soak:** 48 h in staging, then **72 h in production** on 100% of pods.
- **Pass criteria:** zero immutable-trigger violations; `audit_write_failures_total`
  effectively zero; no latency regression beyond +10 ms; storage growth within
  projection.
- Only after the 72 h production soak clears does R2 (`TENANCY_ORGANIZATIONS_ENABLED`)
  enter its own pre-flip checklist. **Do not chain waves** — see
  `post-remediation-rollout-status.md`.
