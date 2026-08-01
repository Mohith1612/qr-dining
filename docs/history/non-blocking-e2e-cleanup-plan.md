# Non-Blocking E2E Cleanup Plan

Date: 2026-05-26
Source: `e2e-failure-analysis.md` (43 failures). The 5 P0 blockers are fixed and
committed; the remaining **38 are NOT rollout blockers**. This document **classifies,
prioritizes, and sequences** them. **Do not fix them broadly now** — none block R1 or any
wave; they are test-suite hygiene to restore the E2E suite as a trustworthy CI gate.

> Already committed in this session (`api.ts` + spec drift, commit `92bbfb8`): the
> `seedOrg` rewrite to the current platform API, staff-token menu seeding, and
> `forceCloseSession → DELETE /sessions/:id`. That likely clears part of Category 3 and
> the G-03/M-03 logic items. **Counts below are pre-cleanup estimates from the analysis;
> reconcile against a fresh CI run after the env/bootstrap step.**

---

## Classification & priority

### P1 — Env / bootstrap cleanup (do FIRST — highest leverage, unblocks the most)
Fixing config/bootstrap clears ~9 failures at once and is prerequisite to running any
`seedOrg` spec in CI.

- **Platform-admin bootstrap (meta-enabler).** `PlatformAuth.ValidateToken` requires a
  Redis-backed session, so `seedOrg`/`loginStaff` need a seeded platform user + a Redis
  `platform_token:e2e-admin-secret` entry. Without it, **no `seedOrg` spec runs in CI**.
  Add a Playwright `globalSetup` (or a make target) that provisions this. *This is the
  single highest-value cleanup item.*
- **`PAYMENT_WEBHOOK_SECRET_STRIPE`** missing in `backend/.env` → 7 webhook/payment tests
  401 (W-01, W-02, W-05, W-06, W-07, P-03, P-13). Add `=test-webhook-secret`.
- **`MFA_ENCRYPTION_KEY`** missing → PT-03 MFA-enroll 500 (1). Add a 32-byte test key.
  (PT-03 is env-only — the handler is correct when the key is set; NOT a code bug.)

### P2 — Helper cleanup (small, broad effect)
- **`fetchAudit` shape:** backend returns `{audit:[...]}`; helper must `return data.audit
  ?? data` → fixes P-01, A-05 (3). One-line `helpers/api.ts` change.
- Re-verify `seedOrg`/`forceCloseSession` (already edited) against the live backend in CI.

### P3 — Spec cleanup (test-only status/shape/logic; no backend change)
Safe, mechanical edits once P1/P2 land:
- Accept `201` for ws-ticket/join: R-01 (×2), X-07, R-03, L-03, M-04 (6).
- Lockout/idempotent-close status sets: X-05 add `423`; L-05 add `401` (2).
- Platform-token → staff-token in specs that still pass `adminToken`: S-02, S-03, S-04,
  A-06, OPID-04, S-08, O-02 (≤7 — several may already pass via the new `seedOrg` owner
  token; confirm in CI).
- Response-shape: O-01, O-07 (`order.Order.id`), PT-04 (org `{code,name,restaurant_slug}`)
  (3).
- Spec logic: S-06 (seed a waiter, not the owner) (1); G-03/M-03 (verify the committed
  `forceCloseSession` change resolved them).

### P4 — Frontend-infra cleanup (needs the Next.js app running)
- G-01, L-09, L-10, `screenshots/sweep#staff` (4) require the frontend server. Run them in
  a job that boots the frontend (Playwright `webServer`), or tag them
  `@requires-frontend` and exclude from the backend-only run.

### P5 — Post-rollout / wave-aligned (defer intentionally)
- **S-01 test 3** (legacy `{branch_id, pin}` accepted): only meaningful once
  `AUTH_STAFF_CODE_REQUIRED` is rolling out (**R4**). Revisit it *with* R4, not before —
  changing it earlier would assert behavior the platform deliberately still allows.
- Any spec asserting strict-mode rejection should be revisited as part of the wave that
  enables its flag (R3–R7), not pre-emptively.

---

## Recommended sequence

1. **P1 env/bootstrap** — unblocks ~9 + makes the suite runnable in CI. *(Prereq for
   trusting any later count.)*
2. **Re-run the full suite in CI**, get an accurate post-bootstrap failure list (the
   committed `api.ts`/spec edits + P1 will have cleared a chunk; don't fix from stale
   counts).
3. **P2 helper** (`fetchAudit`).
4. **P3 spec edits** — batch by category; one PR per category for clean review.
5. **P4 frontend-infra** — separate job/tag; lowest urgency for a backend-gated rollout.
6. **P5** — fold into R3–R7 wave prep as each flag is enabled.

---

## Guardrails

- This is **test hygiene**, not product work. No backend behavior changes (the only
  legitimate backend-status items were the 5 P0s, already fixed).
- Do **not** weaken a spec to make it pass if the backend behavior is the intended one —
  fix the spec's expectation, not the product (e.g. 201 vs 200, 423 vs 429 are correct
  backend statuses; the specs are wrong).
- Restoring the suite as a **green CI gate** should precede relying on it to gate later
  waves — but it does **not** block R1.
