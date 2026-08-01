# R1 Live Rollout Status — Operational Ledger

Wave: **R1** — `AUDIT_LOG_V2_ENABLED`. This is the running operational record for R1.
Executed per `r1-activation-runbook.md`; monitored per `r1-soak-monitoring-guide.md`.

**Current state: R1 ENABLED — soak RESTARTED 2026-05-28T19:29:03Z after a ~34h app
outage (bind-mounted binary wiped); 72h clock RESET; healthy; no rollback.** See CP-4.

> Environment note: executed on the single-instance local **staging** stack (host-built
> Go binary in an `alpine` container against `qr-dining-postgres-1` / `qr-dining-redis-1`).
> "Rolling restart" was a single-container restart — the multi-pod one-at-a-time sequence
> in the runbook applies to real production, which remains an operator action. All metric
> names, the flag, and the immutable trigger are the production ones.

---

## 1. Timeline (UTC, 2026-05-25)

| Time | Event |
|------|-------|
| 19:34:42 | Pre-flip validation (read-only) — all green (see §2) |
| 19:35:54 | App started in **baseline** state (`AUDIT_LOG_V2_ENABLED` unset = false); `/readyz` 200 |
| 19:36:~ | Baseline proof: audited actions drove **0** audit rows (writer gated off) |
| **19:36:34** | **FLIP**: app restarted with `AUDIT_LOG_V2_ENABLED=true` (only change); `/readyz` 200 |
| 19:36:~ | Post-flip verification passed (see §4) |
| **19:37:42** | **R1 soak START** |

---

## 2. Pre-flip validation

| Check | Result |
|-------|--------|
| Alert rules load (`promtool`) | ✅ 24 rules, incl. `AuditWriteFailures` (page) |
| Immutable trigger present | ✅ `trg_audit_log_immutable` on `audit_log` |
| `audit_log` baseline rows | ✅ 0 (clean) |
| Disk headroom (pg volume) | ✅ 171.8 GB free, 61% used (>60-day headroom) |
| Datastores healthy | ✅ pg + redis `Up (healthy)`; `/readyz` 200 |
| No other strict flags set | ✅ only `AUDIT_LOG_V2_ENABLED=true` (verified via `docker inspect` env) |
| No coupled code/migration | ✅ env-var-only change; binary unchanged across flip |
| Dashboards | ⚠️ committed JSON only; no live Grafana in sandbox — validated by metric presence on `/metrics` instead |

## 3. Baseline (flag OFF)

| Signal | Value |
|--------|-------|
| `/readyz` | 200 `{postgres:ok, redis:ok}` |
| session create latency | 30.4 ms (201) |
| order place latency | 23.4 ms (201) |
| `audit_log` rows after 2 audited actions | **0** (proves auditing is flag-gated) |
| `audit_write_failures_total` | absent (= 0) |

## 4. Post-flip values (flag ON)

| Signal | Value | vs baseline |
|--------|-------|-------------|
| `/readyz` | 200 (stable) | unchanged |
| session create latency | 25.8 ms (201) | within budget (≤ +10 ms) |
| order place latency | 23.3 ms (201) | unchanged |
| payment initiate latency | 23.5 ms (201) | within budget |
| `audit_log` rows | grew 0 → 2 with traffic | ✅ auditing ON |
| audited actions written | `session.create` (success/guest), `payment.initiate` | ✅ |
| `audit_write_failures_total` | 0 (no failure series) | ✅ |
| DB pool | `idle=2`, `total=3` (after 30s poll) | stable, idle > 0 |
| Immutability — UPDATE | ❌ rejected: "audit_log rows are immutable: UPDATE is not permitted" | ✅ enforced |
| Immutability — DELETE | ❌ rejected: "...DELETE is not permitted" | ✅ enforced |

## 5. Storage baseline

- `audit_log`: 2 rows, `pg_total_relation_size` = **104 kB** (dominated by fixed
  table+index page overhead at this row count — per-row payload is far smaller; the
  53 kB/row figure is an artifact of n=2 and must be re-derived once rows accumulate).
- **Re-measure during soak** at a higher row count for a real per-row average, then
  project 60-day growth against the 171.8 GB headroom (per soak guide §2).

## 6. Anomalies / observations

| # | Observation | Severity | Action |
|---|-------------|----------|--------|
| O-1 | **Order placement did not emit an audit row** (only `session.create` and `payment.initiate` did). `order.go` has no order-place audit action. | Low — **not** an R1 defect | R1 only turns auditing ON; the audited-action set is whatever handlers emit. The runbook listed "order place" as audited — that's a doc/coverage mismatch. Review audit *coverage* separately (post-rollout); **not** a rollback trigger. |
| O-2 | `db_pool_*` read 0 immediately post-start (poller is 30 s); populated to idle=2/total=3 on next scrape. | Info | Expected; no action. |
| O-3 | Single-instance staging; "rolling restart" simulated. | Info (env) | Real prod uses the multi-pod runbook sequence. |

**No metric spikes, latency drift, alert noise, audit gaps, DB pressure, reconnect
anomalies, or storage surprises observed.**

## 7. Rollback decision

**None.** No rollback condition met (write failures 0, latency within +10 ms, pool
stable, readiness stable, immutability enforced, disk ample). Rollback remains armed and
cheap: `AUDIT_LOG_V2_ENABLED=false` + restart, MTTR <5 min, zero data risk.

## 8. Soak state & cadence

- **Soak start:** 2026-05-25T19:37:42Z. Target: 48 h staging → 72 h production at 100%.
- **Cadence:** 15-min for first 2 h → hourly to 24 h → twice-daily thereafter
  (`r1-soak-monitoring-guide.md §4`).
- **Watch (priority):** `audit_write_failures_total` (page-backed), `audit_log` row growth
  vs disk free, audited-route p99, `db_pool_*`.
- **Rollback triggers / pause-escalate triggers:** per soak guide §5 — unchanged.

## 9. Operational confidence

**Healthy. Continue the soak unchanged.** R1 behaved exactly as designed: auditing is
strictly flag-gated (0 rows off, rows on), the writer is non-fatal with zero failures,
latency is flat, the DB immutability guarantee is enforced live, and rollback is trivial.

**What still warrants attention:**
- Audit **coverage** (O-1): confirm which actions are meant to be audited; if order
  placement should be, that's a separate (non-R1) coverage fix.
- **Storage curve:** the per-row size must be re-derived at scale and the 60-day
  projection confirmed — the quiet long-tail risk of R1.
- **Discipline:** do **not** chain R2. R1 must clear its full soak first
  (`post-remediation-rollout-status.md`).

---

## 10. Soak checkpoints

### CP-1 — 2026-05-25T19:38:37Z (T+~1 min)
- `/readyz` 200; app Up; `db_pool` idle=2/total=3; `audit_write_failures_total` = 0.
- `audit_log` grew 2 → **23**. Breakdown:
  `payment.settlement.stalled`/system = 19, `session.abandon`/system = 2,
  `payment.initiate`/guest = 1, `session.create`/guest = 1.
- **Explanation (benign):** with the writer now ON, the already-running background workers
  persist their audit events. 20 `payment_pending` sessions accumulated from prior
  phase/remediation testing; the Phase E escalation worker audited them once as stalled,
  and the stale-session cleaner abandoned two — a **one-time backlog burst**, not steady
  rate.
- **Dedup verified:** `payment.settlement.stalled` count was **19 at t0 and still 19 after
  a full worker tick (+70 s)** → the per-session+level Redis dedup marker is working; the
  escalation worker is **not** re-auditing the same sessions each minute. No audit-volume
  runaway. This is the key reassurance for the storage curve.

| # | Observation | Severity | Action |
|---|-------------|----------|--------|
| O-4 | Initial audit volume included a one-time **backlog burst** (19 settlement-stalled + 2 abandon) from accumulated test sessions. | Low / benign | Expected when enabling the writer with workers running. Real prod has no such test backlog. Steady-state worker audit volume is bounded (dedup confirmed). No action. |

Steady-state confirmation: stalled-audit count flat across one tick; no failures; readiness
and pool stable. Soak continues unchanged.

### CP-2 — 2026-05-25T20:36Z (T+~1 h)
- **Backend restart gap (documented):** during this window the backend (`qr-app-chaos`)
  was found stopped and was restarted to fix a frontend **CORS** issue — re-run **with
  `AUDIT_LOG_V2_ENABLED=true`** (soak preserved) plus `CORS_ALLOWED_ORIGINS` for
  `http://localhost:3000` and `--restart unless-stopped`. Brief auditing gap during the
  down period; `audit_log` rows (immutable) were never lost. Soak resumed, not reset.
- **Health:** `/readyz` 200; flag on; `audit_write_failures_total` = **0**; **active write
  confirmed** (count 51 → 52 on a session.create); DB pool idle=2/total=3; immutability
  still enforced (UPDATE rejected); no backend errors in logs.
- `audit_log` = 52 rows, `pg_total_relation_size` ≈ 136 kB.
- **Observation O-5 (benign):** `payment.settlement.stalled` grew 19 → 41 across exactly
  **20 distinct** `payment_pending` sessions (leftover test data, aged 1–4.5 h). This is
  the escalation worker correctly periodically re-alerting (warn+critical; dedup marker
  TTL ~30 min), **not** a dedup bug or write failure. It inflates audit volume only
  because of accumulated test sessions — real prod wouldn't carry chronically-stalled
  sessions. **Optional soak hygiene:** resolve/cancel these 20 stale test sessions to keep
  the storage projection realistic (soak-safe — does not touch `audit_log` or the flag).

**Verdict: soak HEALTHY, continue unchanged.** No rollback condition met.

### CP-3 — 2026-05-25T20:43Z (T+~1h05m)
- **Second backend restart gap (documented):** redeployed `qr-app-chaos` to ship a CORS
  fix — `Access-Control-Allow-Credentials: true` for credentialed requests (commit
  `94591b7`), needed for the browser frontend (`credentials: "include"`). Re-run **with
  `AUDIT_LOG_V2_ENABLED=true`** + `CORS_ALLOWED_ORIGINS` + `--restart unless-stopped`.
  Brief auditing gap during the rebuild/restart; `audit_log` rows never lost. Soak
  resumed, not reset.
- **Health:** `/readyz` 200; flag on; `audit_write_failures_total` = **0**; `audit_log`
  = 76 rows (continued growth from escalation re-alerts on the 20 stale test sessions per
  O-5; benign). No backend errors.
- **Cumulative integrity note:** R1 has now taken **two short restart gaps** (CP-2 CORS
  origins, CP-3 CORS credentials), both intentional fixes, both restarted with the flag.
  For a *real* production soak these would count as minor interruptions — acceptable for a
  reversible write-path flag, but the 72 h production clock should run without such churn.
  In this local staging context they do not invalidate the exercise.

**Verdict: soak HEALTHY, continue unchanged.** No rollback condition met.

### CP-4 — 2026-05-28T19:29:03Z (SOAK INTERRUPTED → RESTARTED, 72h clock RESET)

- **Outage:** soak app `qr-app-chaos` exited **127** at 2026-05-27T08:57:12Z and stayed
  down **~34 h**. Root cause: the app runs `/qrapp` bind-mounted from host `/tmp/qrapp`;
  `/tmp` was cleaned and the binary removed, after which Docker recreated `/tmp/qrapp` as an
  empty root-owned **directory**, so `exec /qrapp` failed (127) and `restart: unless-stopped`
  could not self-heal. `qr-dining-postgres-1` / `qr-dining-redis-1` stayed up throughout; the
  immutable `audit_log` lives in the pg volume and was never reset.
- **Continuous runtime before death:** flip 2026-05-25T19:37Z → death 2026-05-27T08:57Z
  ≈ **37 h** (short of the 72 h gate), then a ~34 h gap. The original soak **did not** meet
  its exit gate — this is a reset, not a completion.
- **Recovery (operator-approved — reset the clock):** rebuilt the server binary statically
  (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/qrapp ./cmd/server`) from commit
  `075bbf8` (current HEAD; includes the PT-03 503 fix — behaviourally a fresh soak artifact),
  removed the stale dir via a throwaway root container, then restarted `qr-app-chaos` **with
  `AUDIT_LOG_V2_ENABLED=true` preserved** (verified in container env).
- **Post-restart health:** `/readyz` 200 `{postgres:ok, redis:ok}`; `/health` 200; clean
  boot (`postgres connected` → `migrations applied` [idempotent] → `redis connected` →
  `websocket hub started` → HTTP on :8080); `audit_write_failures_total` = 0;
  `policy_shadow_mismatch_total` = 0; `db_pool_total_conns` = 3. Pre-existing benign warn:
  `GUEST_TOKEN_SECRET` dev default in release mode (staging only).
- **Soak DB:** direct soak-DB reads intentionally **not** performed (do-not-touch guardrail);
  integrity inferred from continuous pg volume + clean app reconnect + zero write failures.
- **R3:** shadow-soak monitoring remains **deferred** until R1 clears (rollout sequencing —
  do not chain). Shadow baseline at restart: `policy_shadow_mismatch_total` = 0.

**Verdict: soak RESTARTED, healthy, 72h clock reset. New R1 soak start = 2026-05-28T19:29:03Z.
No rollback condition met.**

> **Soak-fragility note for operators:** the bind-mount-from-`/tmp` deployment is not
> outage-resilient — `/tmp` cleanup silently breaks the app and `unless-stopped` cannot
> recover (missing-binary → 127 loop). For a credible continuous soak, relocate the binary
> off `/tmp` (or build it into the image) and add an external liveness check that pages on
> app-down, so a 34 h silent outage cannot recur.

_Last updated: 2026-05-28T19:29:03Z (CP-4 — soak restarted, clock reset). Append subsequent
soak checkpoints above this line as the soak progresses._
