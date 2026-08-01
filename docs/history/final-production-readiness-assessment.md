# Final Production Readiness Assessment

Date: 2026-05-25 (post Phase E)
Stance: adversarial. Question: is the platform truly ready to **begin** staged strict-flag
rollout, and what still stands between here and production launch?

## Position

**GO to begin staged rollout at Wave R1.** Phase D gave a conditional GO blocked on
(a) missing rollout metrics and (b) no `payment_pending` bound. Phase E closed both. The
instrumentation prerequisites for every wave R1–R7 now exist, emit at authoritative paths
with bounded labels, and are wired to alerts and a dashboard.

This is **not** a GO to flip everything, nor a GO for production launch. Per-wave soak and
operational steps stand, and one hard blocker remains for R3.

## What changed since Phase D

- Six rollout signals implemented and verified (two confirmed live, all share identical
  wiring): `authz_denied_total`, `policy_shadow_mismatch_total`,
  `tenant_resolution_failures_total`, `guest_token_validation_failed_total`,
  `ws_ticket_consume_failed_total`, `payment_pending_escalations_total`.
- `payment_pending` now has a bounded, alert-only escalation lifecycle (no unsafe
  auto-mutation) — closes the last major operational correctness gap.
- `/readyz` reports honest status; blackbox probe config ships so Redis outages page
  correctly (not masked by the go-redis gauge, Phase D F-1).
- Webhook idempotency now has a metric-asserting integration test (CI-runnable).

## Remaining blockers (must precede the named wave)

- **R3 (hard):** answer the three open policy questions in
  `production-enforcement-rollout.md` §11 in writing. Until then R3 must not flip. This is
  a decision task, not engineering.
- **R5:** size the per-IP ws-ticket cap (60/min) against measured Cloudflare/NAT egress
  concentration before flip (Phase D chaos §4). A mass post-deploy reconnect behind one
  egress IP could brush the cap; per-session reconnects are unaffected.
- **R7:** staff settlement UI deployed to every branch device; run the webhook idempotency
  integration test to green in CI (see caveat below).

## Must precede production LAUNCH (independent of flag order)

- Run the integration suite in CI against a dedicated test DB. The webhook test is
  committed and compiles; it could not be executed green in the live-stack sandbox because
  the shared harness (`testutil.OpenTestDB` + golang-migrate) deadlocks `pgxpool.Close()`
  on teardown — a harness/env quirk affecting the whole suite, not this test. Worth a small
  follow-up to make `db.RunMigrations` release its borrowed stdlib connection so the suite
  tears down cleanly locally too.
- Enforce, as deploy gates (not warnings): `Secure=true` on the staff cookie and a
  non-empty `CORS_ALLOWED_ORIGINS` in release (Phase D cookie/CSRF review).
- Stand up Prometheus + the alert rules + Grafana dashboard + the `/readyz` blackbox probe
  in the real environment; set every alert threshold during the staging soak, not after the
  prod flip.

## Can safely wait until post-launch

- Tuning escalation thresholds beyond the conservative defaults (5m/15m).
- Frontend adoption of the `PAYMENT_SETTLEMENT_STALLED` event (backend emits it; staff UI
  can consume later).
- `/readyz` JSON cosmetic polish beyond the status-field fix.
- Legacy code-path removal (already 30-day gated by the rollout doc).
- `SameSite=Strict` / CSRF-token hardening (only if a cookie-auth mutating GET is ever
  added; none exist today).

## Residual risks accepted for rollout start

- **Redis event-durability gap:** events published while the pub/sub subscriber is impaired
  are not queued; clients recover via snapshot reconciliation on reconnect (Postgres
  authoritative). Acceptable; documented for on-call ("missed live events, recovered on
  reconnect", not data loss).
- **Escalation is alert-only:** a stalled payment will keep a table occupied until a human
  resolves it. This is intentional — auto-mutating money state is more dangerous than an
  occupied table. The critical page is the forcing function.

## Bottom line

The platform is operationally instrumented to see what a strict flip would break before it
breaks it — the whole point of Phase E. Start the sequence at R1, gate each subsequent wave
on its soak/operational checklist in `final-rollout-gates-status.md`, and resolve the R3
policy questions before R3. No engineering blocker remains to *beginning* the rollout.
