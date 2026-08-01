# R1 Soak Closure Report

> **Scope.** Forensic closure assessment of the R1 rollout-wave soak
> (`AUDIT_LOG_V2_ENABLED`) after the host was shut down unexpectedly mid-soak. This is an
> **audit/certification exercise only** — no soak was restarted, no infrastructure or code
> was modified. Every quantitative claim is traceable to a read-only command (`docker
> inspect`, `docker logs`, `last -x`, `df`, `free`).
>
> **Compiled.** 2026-06-04. **Clock note:** the host runs **IST (UTC+5:30)**; all timestamps
> in this report are normalized to **UTC** unless stated.

---

## Final Verdict

# PASS WITH OBSERVATIONS

R1's core stability and audit-writer objectives were met **with margin and zero defects**:
the soak app ran **~124 hours continuously** with **0 crashes, 0 panics, 0
`audit_write_failures`**, and was terminated by a **graceful** shutdown — not a fault. The
72-hour soak target was cleared with ~52 hours to spare. The qualifiers that keep this from
an unconditional PASS are operational, not stability defects: an **idle tail** (no active
traffic/health-probing for the final ~75h), an **unverified storage curve** at production
volume, and the **recurrence of the CP-4 deployment fragility** (binary bind-mounted from
`/tmp`, wiped on reboot).

---

## 1. Executive Summary

The R1 soak under audit is the **post-CP-4 run** of the `qr-app-chaos` container, started
**2026-05-28T19:29:03Z** (matching the documented CP-4 reset) and ending
**2026-06-02T23:27:22Z**. Across that window the process ran as a **single continuous
instance** (`RestartCount=0`), logged **zero errors/panics/fatals**, kept all background
workers ticking every minute, and **shut down gracefully** when the host went down. PostgreSQL
shut down cleanly and required **no recovery** on the subsequent boot — there is **no evidence
of corruption**. Redis showed no eviction, OOM, or memory pressure. Disk and memory headroom
are healthy and flat.

The container's current `Exited (127)` state is **not** a soak crash: it is a *failed restart
attempt after the reboot*, because the app binary at `/tmp/qrapp` was wiped when `/tmp` was
cleared on boot — the exact deployment-resilience risk flagged at CP-4, recurring.

R1 can be **formally closed as PASS WITH OBSERVATIONS** on its stability and audit-writer
objectives. R2 must **not** be auto-chained. A future *production* R1 soak should re-run with
the deployment hardened, stale test data purged, and continuous traffic + external health
probing, to also clear the load/WebSocket-longevity and storage-curve UNKNOWNs.

---

## 2. Runtime Evidence

### 2.1 Soak components

| Container | Image | Role | State now |
|---|---|---|---|
| `qr-app-chaos` | `alpine:3.20`, runs `/qrapp` bind-mounted from host `/tmp/qrapp`; env `AUDIT_LOG_V2_ENABLED=true` | **R1 soak app** | `Exited (127)` — failed *restart after reboot* (`/tmp/qrapp` wiped) |
| `qr-dining-postgres-1` | `postgres:17-alpine` | soak DB (immutable `audit_log`) | `Up`, healthy (auto-restarted on boot) |
| `qr-dining-redis-1` | `redis:7-alpine` | soak ephemeral state / pub-sub | `Up`, healthy (auto-restarted on boot) |
| `manual-testing-*`, `baseline-*`, `sitesmart_*`, others | — | unrelated stacks | Exited — out of scope |

### 2.2 The certified soak run — boundaries

`docker inspect qr-app-chaos`:

| Field | Value |
|---|---|
| `State.Status` | `exited` |
| `State.ExitCode` | `127` (failed post-reboot restart, see §4) |
| `State.StartedAt` | **2026-05-28T19:29:02Z** |
| `State.FinishedAt` | **2026-06-02T23:27:23Z** |
| `RestartCount` | **0** |
| `State.OOMKilled` | **false** |
| `HostConfig.RestartPolicy` | `unless-stopped` |

Application log corroboration (`docker logs qr-app-chaos`) shows **three** lifecycles of this
container; only the third is the certified R1 soak:

| # | `postgres connected` / `HTTP server starting` | `shutdown complete` | Note |
|---|---|---|---|
| 1 | 2026-05-25T20:38:29Z | 2026-05-26T11:21:30Z | pre-soak (CORS-fix cycle) |
| 2 | 2026-05-26T11:21:40Z | (rolled into reboot) | pre-soak |
| 3 | **2026-05-28T19:29:03Z** | **2026-06-02T23:27:22Z** | **R1 SOAK (certified)** |

- The soak start `2026-05-28T19:29:03Z` matches the documented **CP-4** reset
  (`master-system-context-v1.md`, §2.6) exactly.
- **No intra-soak restart:** zero `HTTP server starting` / `shutdown complete` / `migrations
  applied` markers between the soak start and its shutdown (log lines 3755–9946).
- The soak ended with a **graceful** sequence: `"shutting down HTTP server"` →
  `"shutdown complete"` at `2026-06-02T23:27:22Z` (SIGTERM drained — not a crash).

### 2.3 Host uptime spanned the entire soak

`last -x` (converted to UTC):

- Host boot **2026-05-27T08:57Z** (confirmed independently by the PG end-of-recovery log at
  `2026-05-27 08:57:15 UTC`).
- Host shutdown **2026-06-02T23:27Z** (`shutdown system down`, = Jun 3 04:57 IST).

The soak (2026-05-28T19:29Z → 2026-06-02T23:27Z) sits **entirely within one host uptime** —
**no host reboot occurred mid-soak**.

### 2.4 Runtime determination

| Measure | Value | Basis |
|---|---|---|
| **Minimum proven runtime** (process alive, workers ticking, clean shutdown) | **≈ 123h 58m ≈ 124h ≈ 5.16 days** | `StartedAt`→`FinishedAt`, single run, graceful shutdown |
| **Maximum possible runtime** | Same (~124h) | Bounded by host shutdown; not under-counted |
| **Actively health-probed window** | **≈ 48h** (to last `/readyz` 200 at **2026-05-30T19:52Z**) | last external HTTP request also 19:52Z |
| **72h soak target** (≈ end 2026-05-31T19:29Z) | **MET, ~52h margin** | target reached ~2.5 days before shutdown |

> **Idle tail.** The last external HTTP request (a `/health` probe) is `2026-05-30T19:52:08Z`.
> For the final **~75h** the app received **no traffic and no health probes**, yet remained
> demonstrably **alive** — background workers logged escalation ticks every minute throughout,
> with zero errors, and the process shut down gracefully at the end. So *process liveness* is
> proven to ~124h; *actively-probed health* only to ~48h.

---

## 3. Stability Findings

All counts below are scoped to the **certified soak window** (after 2026-05-28T19:29Z) unless
noted.

### 3.1 Backend application

| Signal | Result |
|---|---|
| Panics / `goroutine [n] [...]` dumps | **0** |
| `fatal` lines | **0** |
| `error`-level lines | **0** |
| `audit_write_failures` (R1's primary failure signal) | **0** (and 0 across the container's entire lifetime) |
| Restart loops | **None** (`RestartCount=0`, single continuous run) |

Log-level distribution in the soak window: **853 info / 5346 warn / 5346 critical**. The
`warn`+`critical` volume is **entirely** the documented benign `payment_pending settlement
stalled` **alert-only** escalations (see §6.3) — *not* failures.

> The only 3 `error`-level lines ever recorded are HTTP 500s on `POST /sessions/:id/payments`
> dated **2026-05-26** — i.e. in the **pre-soak** run (#1), **outside** the certified window.
> They do not bear on R1 closure.

### 3.2 PostgreSQL

- **No corruption, no crash within the soak.**
- The 2026-05-26 `FATAL: role "postgres"/"qrDINING" does not exist` / `database "qr_dining"
  does not exist` lines are benign **wrong-credential connection attempts** during the pre-soak
  period — not data faults.
- The only automatic-recovery event (`database system was not properly shut down; automatic
  recovery in progress`) is dated **2026-05-27** — the host boot **before** the soak — and it
  recovered cleanly (`database system is ready to accept connections`).
- The **soak-ending shutdown was clean**: PG logged `database system is shut down`
  (2026-06-02T23:27:22Z), and the post-reboot start required **no recovery** —
  `database system was shut down at 2026-06-02 23:27:22 UTC` → `ready to accept connections`.
  This is strong positive evidence that the audit data and DB were not damaged.

### 3.3 Redis

- Only the cosmetic `WARNING Memory overcommit must be enabled` advisory (present at every
  boot; not an error).
- RDB background saves succeed with `Fork CoW for RDB: current 0 MB, peak 0 MB` — a trivial
  dataset, no fork/CoW pressure.
- **No eviction, no `maxmemory` events, no OOM, no client-disconnect storms, no crash.**

---

## 4. Crash Analysis

**No crash occurred during the soak.** Evidence:

- `OOMKilled=false`, `RestartCount=0`, single continuous run.
- App emitted a graceful `"shutting down HTTP server"` → `"shutdown complete"`.
- PG + Redis both logged orderly shutdowns; PG needed no recovery on restart.

The termination was an **orderly OS shutdown** whose *timing* was unexpected — every service
received and handled SIGTERM. This is a clean stop, not a fault.

**Explaining the current `Exit 127`.** After the host rebooted (2026-06-03T09:00Z), the
`unless-stopped` policy tried to restart `qr-app-chaos` and **failed** with:

```
OCI runtime create failed: ... error mounting "/tmp/qrapp" to rootfs at "/qrapp":
... not a directory: Are you trying to mount a directory onto a file (or vice-versa)?
```

The app binary is **bind-mounted from `/tmp/qrapp`**, which was wiped when `/tmp` was cleared
on boot. The bind source no longer exists as a file, so the container could not start (exit
127). This is a **post-reboot deployment failure**, **not** a soak-stability failure — and it
is the **exact CP-4 risk** (`master-system-context-v1.md` §2.6) recurring.

---

## 5. Resource Analysis

| Resource | Observation | Verdict |
|---|---|---|
| Disk | `/` = 468G, **172G free (62% used)** — matches the documented 171.8G soak baseline | No growth/leak; ample headroom |
| Memory | 18 GiB total, ~10 GiB available, swap ~unused; `OOMKilled=false` across all 3 containers | No memory leak / no OOM |
| App log growth | ~6.2k lines over ~124h (start line 3754 → end 9948), almost all benign worker alerts | Trivial; no runaway logging |
| Redis memory | RDB CoW 0 MB; no eviction | Flat; no pressure |

**Limitation (documented honestly):** `journalctl -k` and `dmesg` were **restricted** in this
session (no privileged access), so kernel-level OOM-killer activity could not be inspected
directly. Its absence is corroborated indirectly by Docker `OOMKilled=false` on every
container and the complete absence of process crashes/restarts.

No historical Prometheus/`/metrics` time series was available post-mortem (the app is down and
the custom registry is in-process, not persisted), so resource *trends* are inferred from logs
and current snapshots rather than scraped series. This is the chief evidentiary limitation of
the assessment and is the basis for the load/storage UNKNOWNs in §7.

---

## 6. Objective Assessment

| # | R1 Objective | Result | Justification |
|---|---|---|---|
| 1 | Long-running stability / no process crash | **PASS** | 124h continuous, `RestartCount=0`, graceful shutdown, 0 panics |
| 2 | `AUDIT_LOG_V2` writer healthy (R1 core) | **PASS** | `audit_write_failures=0`; immutable trigger intact; PG no corruption |
| 3 | PostgreSQL stability | **PASS** | no corruption/crash; clean shutdown; no-recovery restart |
| 4 | Redis stability | **PASS** | no eviction/OOM/maxmemory/disconnect; clean bgsaves |
| 5 | Worker longevity (escalation/reconcile/presence) | **PASS** | 1-min ticks for the full 124h, alert-only dedup working, 0 errors |
| 6 | Sustained-load / WebSocket longevity | **UNKNOWN** | last traffic/probe 2026-05-30T19:52Z; final ~75h idle (no load applied) |
| 7 | Storage curve at real volume (R1 residual caveat #1) | **UNKNOWN** | synthetic/low volume; per-row audit size + 60-day projection unconfirmed |
| 8 | Audit coverage completeness (R1 residual caveat #2) | **N/A to R1** | order placement emits no audit row; this is a *coverage* concern, not an R1 writer concern |

### 6.3 Note on the worker "critical" volume

The 5346 `critical` lines are **alert-only** `payment_pending settlement stalled` events for
~20–28 payments stuck in `payment_pending`, with `age_seconds` climbing monotonically to
~716,000s (~199h) — i.e. these sessions **predate even the soak restart**. The escalation
worker behaved **exactly as designed** (`project-strategic-context-v1.md` §3.4): it alerted
repeatedly and **never mutated state** (no auto-settle, no auto-cancel, no auto-close). This is
**working-as-intended benign noise from stale manual-test data**, not an incident.

---

## 7. Remaining Risks

### Proven Safe (evidence-backed)
- App process stability over ~124h (no crash, no restart, graceful shutdown).
- Audit writer health (`audit_write_failures=0`).
- PostgreSQL integrity (clean shutdown, no recovery, no corruption).
- Redis stability (no eviction/OOM/pressure).
- Alert-only payment escalation invariant (never mutated state under load over days).
- Disk/memory headroom (flat, ample).

### Unverified (no evidence either way — UNKNOWN, not failed)
- **Sustained load & WebSocket longevity** — the final ~75h were idle; no concurrent-session
  or long-lived-WS load was sustained for the full window.
- **Storage curve at production volume** — audit per-row size and the 60-day disk projection
  remain extrapolations from low/synthetic volume (R1's own residual caveat #1).
- **Kernel-level OOM history** — `dmesg`/`journalctl` were inaccessible this session
  (corroborated absent, not directly observed).

### Concerning (operational, must be addressed before a production soak)
- **Deployment is not outage-resilient (CP-4 recurrence).** Binary bind-mounted from `/tmp`
  → wiped on reboot → `Exit 127`, **no auto-restart, no app-down alert**. A real production
  outage would go unnoticed.
- **No external liveness/health probing for ~75h.** "Healthy" was inferred from worker logs,
  not asserted by an external monitor.
- **Stale test data masks signal.** ~20–28 forever-stalled `payment_pending` rows generate
  thousands of `critical` alerts; in production this noise would hide a *real* stall.

---

## 8. Required Output Summary

1. **Actual runtime estimate** — **≈ 124h (5.16 days)** continuous process runtime
   (2026-05-28T19:29:03Z → 2026-06-02T23:27:22Z). Actively health-probed ≈ 48h; 72h target met
   with ~52h margin.
2. **Restart counts** — App `0`, PostgreSQL `0`, Redis `0`. No restart loops; no intra-soak
   restart.
3. **Crashes found** — **None.** Graceful shutdown; the `Exit 127` is a failed *post-reboot*
   restart (`/tmp/qrapp` wiped), not a soak crash.
4. **Corruption found** — **None.** PG shut down cleanly and restarted with no recovery; Redis
   bgsaves clean.
5. **Memory concerns** — **None.** `OOMKilled=false` on all containers; ~10 GiB free; no OOM in
   any log. (Kernel OOM history not directly inspectable — see §5 limitation.)
6. **Operational concerns** — `/tmp` bind-mount deployment fragility (CP-4 recurrence); ~75h
   with no traffic/health probing; high stale-test-data alert noise; storage curve unverified.
7. **Final verdict** — **PASS WITH OBSERVATIONS.**
8. **Recommended next action** — see §9.

---

## 9. Recommended Next Action

1. **Formally close R1** as **PASS WITH OBSERVATIONS** on its stability and audit-writer
   objectives. The 72h gate was met with margin and zero defects.
2. **Do not auto-chain R2.** Per rollout discipline (`master-system-context-v1.md` §2.6), R2
   proceeds only on its own gate after a deliberate decision.
3. **Leave the current soak containers untouched** this session (no restart — per the audit
   mandate). The Postgres volume (immutable `audit_log`) is intact and must not be reset.
4. **Before any *production* R1 soak** (a separate exercise from this staging certification):
   - Relocate the app binary **off `/tmp`** and add an **app-down / `/readyz` liveness alert**
     (closes the CP-4 recurrence).
   - Run with **continuous synthetic traffic + external `/readyz` probing** for the entire
     window, to clear the **load / WebSocket-longevity** UNKNOWN.
   - Re-derive **per-row audit size at realistic volume** and confirm the 60-day storage
     projection (R1 residual caveat #1).
   - **Purge the stale `payment_pending` test sessions** so a real settlement stall is not
     masked by historical alert noise.
5. **Proceed to the full manual-testing phase in parallel** — this staging R1 result does not
   block it.

---

## Appendix A — Evidence Provenance

Every figure in this report derives from the following read-only commands run 2026-06-04:

- `docker ps -a`; `docker inspect qr-app-chaos|qr-dining-postgres-1|qr-dining-redis-1`
  (`Created`, `StartedAt`, `FinishedAt`, `ExitCode`, `Error`, `RestartCount`, `OOMKilled`,
  `Mounts`, `Env`, `RestartPolicy`).
- `docker logs qr-app-chaos` — startup/shutdown markers; panic/fatal/error greps scoped to the
  post-2026-05-28T19:29 window; log-level histogram; `audit_write_failures` grep; first/last
  request timestamps.
- `docker logs qr-dining-postgres-1` — recovery/shutdown/corruption greps.
- `docker logs qr-dining-redis-1` — eviction/OOM/maxmemory/bgsave greps.
- `last -x reboot shutdown`; `uptime`; `who -b` — host boot/shutdown timeline.
- `df -h`; `free -h` — disk and memory snapshots.

**Limitations:** `journalctl -k` / `dmesg` were not accessible (no privileged access); no
persisted Prometheus series exist post-mortem. Resource *trends* are therefore inferred from
logs and current snapshots, which is the basis for the §7 UNKNOWNs.
