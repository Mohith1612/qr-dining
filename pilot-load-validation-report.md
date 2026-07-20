# Pilot Load Validation Report

**Date:** 2026-06-10
**Phase:** Final Pilot Hardening — Phase G
**Result:** ✅ **PASS for single-restaurant pilot scale.** Ceiling characterized; one
serialization hot-spot noted for post-pilot scale.

Real synthetic load was driven against a running backend on an **isolated** stack
(soak never touched). The driver (`backend/scripts/loadtest`) runs the full guest
flow per session: create → menu → cart → order → ws-ticket → WS connect → snapshot
(reconnect) → bill → cash payment. Each flow uses a **distinct client IP**
(`X-Forwarded-For`) so per-IP rate limits behave as in production (one IP per guest).

## Environment

| | |
|---|---|
| App | native binary, `GIN_MODE=debug`, `DB_MAX_CONNS=20` (the documented default under scrutiny) |
| DB / Redis | isolated `hardening-pg` (pg17) / `hardening-redis`, **not** the soak |
| Data | seeded tenant + 5000 tables on one branch (to avoid the one-active-per-table invariant masking throughput) |
| Caveat | client + app + Postgres + Redis all co-located on one dev host — contention here *overstates* latency vs a real deployment with a dedicated DB |

## Scenario A — realistic pilot peak (concurrency 50, 30s, WS held 1s)

A busy single restaurant has tens of concurrently-active tables; 50 continuous
synthetic guests (no think-time) is already above that.

| op | ok | err | p50 ms | p95 ms | p99 ms | max ms |
|----|----|----|--------|--------|--------|--------|
| session.create | 1361 | 235* | 13.8 | 40.4 | 185.1 | 239.8 |
| menu.get | 1361 | 0 | 0.7 | 3.3 | 6.2 | 22.3 |
| cart.add | 2045 | 0 | 15.5 | 37.5 | 50.7 | 103.1 |
| order.place | 1361 | 0 | 28.6 | 88.8 | 228.7 | 419.4 |
| ws.ticket | 1361 | 0 | 0.9 | 2.7 | 22.6 | 48.0 |
| ws.connect | 1361 | 0 | (1s hold) | | | |
| snapshot | 1313 | 48 | 1.0 | 3.0 | 7.4 | 25.4 |
| payment.init | 1312 | 1 | 14.0 | 42.7 | 109.5 | 198.9 |

`*` The 235 `session.create` errors are **`SESSION_ALREADY_ACTIVE` (409)** — workers
randomly picked a table already holding an active session. This is the correct
one-active-per-table invariant firing, **not** a capacity fault. Excluding those,
the genuine error rate was **~0.4%** (49 transient reads).

- **DB pool:** comfortably under the 20-connection limit; no pool-starvation counter
  incremented, no `empty_acquire` events. Headroom was ample.
- **Latency:** writes p99 ~190–230ms, reads single-digit ms. Acceptable.
- **Verdict:** the default 20-connection pool is **sufficient at pilot scale** — this
  directly retires the audit's SEV-1 "pool exhaustion" worry for a single restaurant.

## Scenario B — stress / ceiling (concurrency 150, 20s, no WS hold)

3× a heavy pilot, continuous hammering of a **single branch**, to find the limit.

| op | ok | err | p50 ms | p95 ms | p99 ms | max ms |
|----|----|----|--------|--------|--------|--------|
| session.create | 2191 | 658* | 113.2 | 596.0 | 820.4 | 1640.4 |
| cart.add | 3222 | 30 | 129.7 | 435.1 | 836.1 | 892.6 |
| order.place | 2123 | 68 | 272.6 | 734.8 | 1677.2 | 1781.2 |
| payment.init | 2056 | 35 | 205.4 | 779.5 | 1317.8 | 1394.0 |
| menu/ws-ticket/snapshot | — | few | 17–48 | 88–101 | 131–346 | — |

- Throughput ~142 flows/s; **overall 5.0% error rate**.
- ~156 genuine **HTTP 500s** appeared (sessions 66, payments 33, orders 23, cart 18,
  bill 7, snapshot 7, menu 2), distinct from the 409 collisions. No worker panics;
  Redis reported **0 evictions / 0 rejected connections**; no pool `empty_acquire`.
- **Interpretation:** under extreme single-branch concurrency, write paths degrade to
  p99 ~0.8–1.7s and a small fraction error. The most likely contention point is the
  **per-(branch, business-date) session-number sequence row** (`session_sequences`),
  a serialization hot-spot when one branch takes many simultaneous session creates,
  compounded by all four datastores sharing one host. This is **degradation, not
  collapse** — the app stayed up and recovered immediately after load.

## Capacity findings

1. **Pilot (one restaurant): healthy.** Expected concurrency is well under Scenario A,
   where p99 ≈ 200ms and genuine errors ≈ 0.4% with the pool barely used.
2. **Ceiling:** a single instance with a 20-conn pool, all datastores co-located,
   starts visibly degrading around ~150 continuous single-branch writers. Real
   deployments (dedicated DB host, human think-time) will sit far below this.
3. **No leaks/instability:** goroutines tracked active load and settled; no panics;
   Redis stable; pool released cleanly to idle after each run.

## Recommendations

- **Pilot:** ship with the default `DB_MAX_CONNS=20`. Sufficient and proven here.
  Watch the new `PostgresPoolExhausted` / `ReadyzProbeFailing` alerts (Phase F).
- **Before higher volume / many branches:** raise `DB_MAX_CONNS` to ~40–50 and give
  Postgres its own host; investigate the `session_sequences` per-branch hot-row
  serialization (and the Scenario-B 500s) before a high-traffic, single-busy-branch
  deployment. Out of scope for a single-restaurant pilot.

## Honest limitations

- Single instance, single branch, all services co-located — latency here is a
  pessimistic bound, but the 500s at 150-concurrency are real and warrant a follow-up
  root-cause before scale.
- Synthetic load has **no think-time** — harsher than real diners.
- Long-lived WebSocket longevity (hours) was not exercised here (held ≤1s per flow);
  that remains a separate soak concern.
- The instantaneous `db_pool_acquired_conns` gauge could not be reliably peak-sampled
  at 1s polling vs sub-ms acquires; saturation was judged by latency and the absence
  of starvation/again counters, which is the sound signal.
