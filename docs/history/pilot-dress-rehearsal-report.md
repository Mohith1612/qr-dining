# Pilot Readiness Report — Full Dress Rehearsal

> **Date:** 2026-05-30 · **Branch:** `platform-governance-entitlements` · **Type:** production-readiness
> review (validation, not construction). **Scope:** prove the system is operationally ready for the
> first real restaurant deployment.
>
> **Environment:** a fully isolated `pilot-validation` stack — Postgres `:15432` + Redis `:16379`
> (containers, project `pilot-validation`), two native backend instances `:8090`/`:8091`, frontend
> `:3001`. **The R1 soak stack (`qr-app-chaos`, `qr-dining-postgres-1`, `qr-dining-redis-1` on
> `:8080`) was never touched** — verified healthy before, during, and after (pre/post `/readyz` =
> `{postgres:ok,redis:ok}`, AUDIT_LOG_V2 preserved). All frontend traffic verified on `:8090` (zero
> requests to `:8080`).
>
> **Two passes:** Pass 1 = baseline (all 9 strict flags off, = current `main`). Pass 2 = restarted
> with `AUDIT_LOG_V2_ENABLED` + `TENANCY_ORGANIZATIONS_ENABLED` on for isolation/audit-sensitive
> checks. (Presence grace was relaxed for test practicality; the aggressive default is recorded as F-2.)
>
> Evidence: `screenshots/pilot/*.png` (8 shots) + `screenshots/pilot/FINDINGS.md` (detailed log).

---

## 1. What was exercised

Three restaurants onboarded through the **real operator workflow** (Alpha fully via the UI wizard;
Beta/Gamma via the identical control-plane endpoints the wizard calls):

| Tenant | Org/Branch | Tables | Plan | Subscription | Theme | Custom entitlements |
|---|---|---|---|---|---|---|
| **Cafe Alpha** | org 2 / Alpha Main | 10 | Free (Starter) | **active** | Vibrant | — |
| **Bistro Beta** | org 3 / Beta Downtown | 7 | Standard (Growth) | **trial** (→2026-06-29) | Dark Luxury | — |
| **Restaurant Gamma** | org 4 / Gamma Premium | 12 | Premium | **active** | Custom (modern-minimal + hex) | analytics.advanced, custom.theme |

Each got a distinct menu (built via the staff-admin path), real QR packages, and full guest
lifecycles. Multi-instance realtime, tenant isolation, operator/support tooling, and deployment
mechanics were validated directly.

---

## 2. PASS / FAIL

| Area | Verdict | Notes |
|---|---|---|
| **Onboarding** | ✅ PASS | 7-step wizard end-to-end (org→branch→plan→subscription→tables→QR→activate); all 3 tenants live. |
| **Billing** | ✅ PASS | Billing profile (GST/email/currency) set; subscription/billing/invoice endpoints present; per-org isolated. |
| **Subscriptions** | ✅ PASS | active/trial/active correctly persisted with plan + trial/expiry dates. |
| **Themes** | ✅ PASS | Per-tenant presets + Gamma custom tokens; applied on guest `<html>` (no localStorage bleed); custom gated by `custom.theme`. |
| **Support console** | ✅ PASS | Search + session/order/payment detail + bill snapshot + webhook history + lifecycle timeline; read-only; dual-audit (with R1 on). |
| **Analytics** | ✅ PASS | Org-filtered GMV (410/360/900); no-filter total 1670 = exact sum → no cross-tenant bleed. |
| **QR workflow** | ✅ PASS | Per-table QR grid, ZIP of PNGs, print-to-PDF sheet, correct T1/T5/T10 token resolution, bogus→404. Minor: F-3, F-4. |
| **Session lifecycle** | ✅ PASS | create/join/host-authority/shared-cart/close/reactivation all correct server-side. Client reconnect gap: F-1. |
| **Payments** | ✅ PASS | initiate→freeze→bill snapshot→staff settle→complete→session close; **no fake success**; alert-only escalation observed. |
| **Realtime** | ⚠️ PASS WITH CAVEAT | WS infra + payment/participant/cart/session-close events live (incl. cross-instance). **Guest order-status tracker does not advance live (F-6).** |
| **Multi-instance** | ✅ PASS | Bidirectional cross-instance propagation (order/cart/payment) via shared Redis pub/sub; tickets in shared Redis. |

---

## 3. Issues

### Critical (address before *public multi-tenant* pilot exposure)
- **F-8 — Guest endpoints fail open without credentials (gated by R6); snapshot leaks `session_token`.**
  With `AUTH_GUEST_CREDENTIALS_REQUIRED=false` (R6, current default), `GET /sessions/:id/snapshot`
  with **no token** returns full session state for any session UUID — including the `session_token`
  credential and participant PII. Present/invalid/cross-tenant tokens are correctly rejected (T-01
  holds). **Fix:** enable R6 before public exposure **and** strip `session_token` from the guest
  snapshot payload (the Support Console already omits it; the guest client never needs it). This is
  the staged rollout working as designed — but it must be ON for a public multi-tenant pilot.

### Medium (fix before scaling; small, well-scoped)
- **F-6 — Guest order-status realtime broken (WS payload mismatch).** Backend publishes
  `ORDER_{CONFIRMED,PREPARING,READY,SERVED,CANCELLED}` as `{order:{…}}` (order.go:517) but the guest
  handler reads `payload.id` (useWebSocket.ts:93-116) → no-op. The "track your order live from the
  kitchen" tracker never advances; only a reload corrects it. ORDER_PLACED + all PAYMENT/PARTICIPANT/
  CART/SESSION events work (top-level payloads). Staff dashboards poll (8–10s), unaffected.
  **Fix (tiny):** `(payload.order ?? payload)` on the client, or publish the order at top level.
- **F-1 — Guest reconnect dead-ends on an idled session.** `connection.ts openSocket()` sets status
  "failed" on any ws-ticket error with no snapshot-reactivation/retry. Reloading an
  `awaiting_reactivation` session → ws-ticket 409 → "Connection lost". Backend recovers fully
  (snapshot reactivates → ticket 201) — the client just doesn't route through it. **Fix (tiny):** on
  ticket failure, call snapshot (reactivates) then retry, via the existing `reconnect()` path.
- **F-2 — Presence/reactivation too aggressive for real dining.** Defaults (60s presence + 60s grace)
  pause a session ~1–2 min after a client stops pinging; mobile background-tab throttling will trip
  this for a guest who sets the phone down. Combined with F-1 this is a real friction risk. Tune
  grace/window for pilot.

### Nice-to-have (post-pilot)
- **F-3** — QR-resolve `branch_theme` reads only legacy `settings_json.theme` (returns default for
  all tenants); guest page re-resolves correctly via `/branches/:id/theme`, so only a brief flash.
- **F-4** — `buildQRUrl` local fallback rewrites `:8080→:3000` (brittle on non-default ports;
  production `baseDomain` path is fine).
- **F-5** — `scripts/seed.go` is stale vs schema (demo-session seed fails on `session_business_date`
  NOT NULL); platform-admin + plans + demo restaurant still seed.
- **F-7** — Support audit explorer is empty unless `AUDIT_LOG_V2` is on (run pilot with R1 on).
- **First-admin provisioning** — the platform super_admin can only be created via seed/DB; there is
  no bootstrap UI/CLI for the very first operator.

---

## 4. Final Assessment

**1. Is the system ready for Pilot Restaurant #1?**
**Qualified YES for a single, controlled pilot restaurant.** The entire operational loop —
onboard → QR → guest scan → shared-cart collaboration → host order → kitchen/waiter → cash payment →
staff settle → session close — works, as do multi-instance realtime, tenant isolation, billing/
subscriptions/entitlements, theming, and the read-only operator/support tooling. **Not yet ready for
public, self-serve multi-tenant exposure** until F-8 is closed.

**2. What blockers remain?**
- For **public multi-tenant**: **F-8** (enable R6 / strip `session_token`).
- For **guest experience quality** (any pilot): **F-6** (order tracker doesn't advance live) and
  **F-1** (reconnect dead-end after idle) — both small client fixes, both high-visibility to a guest.
- Operational: run the pilot with **AUDIT_LOG_V2 (R1) on** so the support audit trail works (F-7).

**3. What should happen next?**
1. Land the two tiny client fixes (F-6, F-1) and re-verify the guest order tracker + reload-after-idle.
2. Strip `session_token` from the guest snapshot response (defense-in-depth, independent of R6).
3. Decide R6 enablement timing; tune presence grace (F-2) for real dining.
4. Keep the pilot on the current flag posture **plus** R1 (audit) on; drive remaining waves per the
   existing staged, observable, reversible discipline.

**4. What should NOT be worked on before the first pilot?**
- No new platform-control-plane features; no support-console **mutations** (keep it observe-only).
- No websocket / session-lifecycle / payment rewrites — they are correct; F-6/F-1 are 1-line fixes.
- Do **not** flip all 9 strict flags at once; preserve the R-wave sequencing and soak gates.
- No speculative architecture (branch-level WS channels, hub sharding) — there is no evidence
  demanding it. Prove the core loop with a real restaurant first.

---

## 5. Notes for the operator

- Cleanup of this rehearsal: `docker compose -p pilot-validation down -v` (removes only pilot
  volumes) + stop the native `/tmp/pilot-app` processes and the `:3001` frontend. **Never** run
  compose against the `qr-dining` project.
- The pilot env used relaxed presence and (Pass 2) `AUDIT_LOG_V2`+`TENANCY_ORGANIZATIONS`; the soak's
  flags/data are independent and untouched.
- Reference IDs/credentials used: `screenshots/pilot/` + `/tmp/pilot-refs.txt` (super_admin
  `admin@pilot.test`; per-tenant staff `*-OWN`/PIN 1234).
