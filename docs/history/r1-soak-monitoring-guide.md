# R1 Soak Monitoring Guide — `AUDIT_LOG_V2_ENABLED`

Date: 2026-05-26
Companion to `r1-activation-runbook.md`. What to watch during the R1 soak, what "normal"
looks like, and exactly what triggers a pause/escalate vs a rollback.

Soak duration (per `production-enforcement-rollout.md §Wave R1`): **48 h staging → 72 h
production** at 100% pods. R1 is write-path only; the dominant risks are *audit write
failures* and *storage growth*, not request behavior.

---

## 1. Metrics that matter (in priority order)

| Signal | Query / source | Healthy | Why |
|--------|----------------|---------|-----|
| Audit write failures | `rate(audit_write_failures_total[5m])` | **0** | Non-zero = audit trail gaps (compliance/forensics risk). Writer is non-fatal so requests still succeed — this is silent unless watched. |
| Audit row growth | `SELECT count(*) FROM audit_log;` over time | Linear with request volume | Detects both "not writing" (flat) and "runaway emitter" (super-linear). |
| Audited-route p99 | `http_request_duration_seconds` on `/staff/auth`, `/sessions/:id/orders`, `/sessions/:id/payments` | ≤ baseline + 10 ms | The extra INSERT must not bloat latency. |
| DB pool | `db_pool_acquired_conns`, `db_pool_idle_conns`, `db_pool_total_conns` | idle > 0 | Audit writes consume connections; exhaustion would back up requests. |
| Postgres disk free | node/volume metric | Tracks the projection (§2) | Storage is the slow-burn risk. |
| Liveness/readiness | `up{job="qr-dining"}`, `/readyz` probe (`ReadyzProbeFailing`) | up=1, 200 | Catch pod instability tied to the flip. |
| 5xx ratio | `sum(rate(http_requests_total{status=~"5.*"}[5m]))/sum(rate(http_requests_total[5m]))` | < baseline + 0.5% | R1 should not change this at all; any rise is suspicious. |

Dashboard: Grafana `qr-dining-ops` — Integrity row (audit failures), Datastores row
(pool), Availability row (5xx, up).

---

## 2. Expected audit throughput & storage growth

- **Throughput:** ≈ **one extra INSERT per audited action**. Audited actions are the
  security-relevant mutations (staff login success/fail, order place, payment
  initiate/settle/webhook, authz denial, session create/close/abandon, staff/menu admin
  changes), NOT every read. So audit-row rate ≈ rate of those mutations, well below total
  request rate.
- **Storage estimation (do this before flip, refine during soak):**
  1. Measure audited-action rate during a normal peak hour (count of the above actions).
  2. Average `audit_log` row size — sample post-flip:
     `SELECT pg_total_relation_size('audit_log') / GREATEST(count(*),1) FROM audit_log;`
  3. Daily growth ≈ audited-actions/day × avg row size. Project 60 days; confirm headroom.
- **What "surprising" looks like (investigate):**
  - Row growth materially faster than the audited-action rate → an over-eager emitter or
    a hot loop writing audits.
  - Average row size climbing → oversized metadata/before-after blobs (redaction should
    bound these).
  - Disk growth non-linear (spikes) rather than steady.

---

## 3. Alert behavior & false-positive expectations

- **`AuditWriteFailures` (page):** the primary R1 alert. Expected behavior during a clean
  soak = **silent**. A brief one-off blip during the rolling restart window may occur as
  pods cycle — treat a *single* transient as noise, but any **sustained** (>5 min) firing
  is real → follow rollback conditions. Capture `{action,error_class}` labels first.
- **`PostgresPoolExhausted` / `PostgresQueryErrors`:** not expected to change from R1; if
  they fire concurrently with audit activity, suspect audit-write contention.
- **`ReadyzProbeFailing` / `AppTargetDown`:** unrelated to R1 logic but watch during the
  rolling restart — expected to flap briefly per pod as it restarts, then clear.
- **Rollout-gate alerts** (`LegacyAuthzBypassPresent`, `LegacyIdentityUsagePresent`,
  etc.): **ignore for R1** — they gate later waves, not R1. Their firing/non-firing is
  informational here (it's the legacy-decay signal R3/R5/R6 depend on).

Tune any noisy thresholds during the **staging** soak, never after the production flip.

---

## 4. Review cadence

- **T0 → T+2h:** every 15 min (per the runbook verification cadence).
- **T+2h → T+24h:** hourly.
- **T+24h → end of 72 h prod soak:** at least **twice daily** (start & end of each
  operator shift), plus alert-driven. Record `audit_log` count + disk free each review to
  maintain the growth curve.
- **Hand-off note** at each shift change: current row count, disk free, any alert blips,
  latency delta vs baseline.

---

## 5. Decision matrix — pause/escalate vs rollback

**Roll back immediately** (set flag false, rolling restart, MTTR <5 min) if:
- Sustained `audit_write_failures_total` > 0 not explained by the restart window.
- Audited-route p99 regression > 10 ms attributable to the flip.
- DB pool exhaustion tied to audit writes.
- Disk growth > ~2× projection threatening 60-day headroom.
- `/readyz` instability / pod restart loop correlated with the flip.

**Pause the soak & escalate** (do not necessarily roll back; page rollout lead + DBA) if:
- Immutable trigger found missing/altered.
- Failures cluster on a specific `{action}` (possible bug, capture labels).
- Storage growth is anomalous in *shape* (spikes) even if within headroom.
- Any signal you don't understand — pausing a soak is cheap; an unexplained anomaly in
  the integrity layer is not.

**Extend the soak by 24 h** (per `production-enforcement-rollout.md §8`) on any
alert-firing or anomaly tied to the flag; if paused twice for the same reason, re-do the
shadow/baseline before retrying.

---

## 6. Exit signal

R1 clears when the 72 h production soak completes with: zero immutable violations,
effectively-zero write failures, no latency regression beyond +10 ms, and storage growth
on the projected linear curve. Then — and only then — R2 is *considered* (its own
checklist), per `post-remediation-rollout-status.md`.
