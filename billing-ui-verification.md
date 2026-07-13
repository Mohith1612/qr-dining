# Billing UI — Browser Verification (Playwright)

**Date:** 2026-05-30 · **Branch:** `platform-governance-entitlements` · **Scope:** verify only, no redesign.
**Environment:** isolated throwaway stack — Postgres `:5544`, Redis `:6380`, app from `/tmp` (never the soak
DB/redis), Next.js dev `:3000`, seeded demo org (id 1) on the Free plan trial, `super_admin` operator.
Screenshots in `screenshots/billing/`.

## Result: PASS — every flow works end-to-end.

| Area | Action | Result |
|------|--------|--------|
| Subscription | view (trial) | ✅ shows status/plan/dates |
| | activate (+ expiry) | ✅ → active, sets started/expires/renewed |
| | suspend | ✅ → suspended (confirm dialog) |
| | resume (activate) | ✅ clears suspension |
| | renew (from active) | ✅ extends expiry |
| | cancel (+ reason) | ✅ → cancelled, reason persisted |
| Billing profile | view (seeded) | ✅ |
| | edit + save | ✅ contact + address added |
| | persist (reload) | ✅ survives full reload |
| Invoices | create draft | ✅ `INV-2026-000001`, ₹1499.00 |
| | issue | ✅ → issued, issue-date set |
| | mark paid | ✅ → paid (terminal; actions hidden) |
| | cancel (2nd invoice) | ✅ → cancelled |
| Manual payments | record (UPI + ref) | ✅ |
| | history | ✅ row shows amount/method/date/ref |
| | audit | ✅ `platform_audit_log` rows verified (see below) |

**Audit (DB-verified, authoritative):** `platform_audit_log` for org 1 recorded every mutation —
`platform.subscription.{activate×2,suspend,cancel}`, `platform.invoice.{create,issue,mark_paid,cancel}`,
`platform.payment.record`, `platform.billing_profile.update` (plus the read actions). All mutations audited.

## UX findings (no redesign performed)

1. **Invalid transitions are offered, then fail with a generic toast.** Action buttons (Renew, Change
   plan, Cancel) render regardless of the current status. Triggering an invalid transition — e.g. **Renew
   while suspended/cancelled** — calls the backend, returns **409**, and surfaces only as a generic
   *"Action failed."* toast; the dialog stays open. The backend is correct (state machine rejects it); the
   gap is purely UX. *Suggested (later, low-risk):* disable actions invalid for the current status, and/or
   map the 409 to a specific message ("Can't renew a suspended subscription — activate it first").
   **Severity: low.**

2. **Operator audit trail has no UI viewer.** All billing/governance mutations land in `platform_audit_log`
   (verified), but the only in-app audit explorer (`/platform/support/audit`) reads the **tenant-visible
   `audit_log`** (and is empty unless `AUDIT_LOG_V2_ENABLED`). Operators therefore cannot review their own
   billing/governance action history in the UI. *Candidate for the enforcement-observability / pilot-ops
   work — a read-only platform-audit viewer.* **Severity: low–medium (operational visibility).**

3. **"Activate / resume" is overloaded** (create + resume-from-suspended + reactivate-from-cancelled). Clear
   in practice; no change recommended.

## Positives
- Copy is accurate and sets correct expectations: page subtitle "*subscription status is recorded and
  audited, not enforced*"; suspend dialog "*recorded and audited but does not restrict the tenant yet*".
- Terminal states correctly hide actions (paid invoice has no buttons; suspended hides Suspend).
- Money (`₹1499.00`) and dates are cleanly formatted; persistence holds across a full page reload.
- No layout/console breakage during the flows (only the expected 409 network error).
