# Pilot Support Runbooks

Diagnose-first runbooks for the most common pilot issues. **Diagnose via the read-only
Support Console (`/platform/support`) — never open a psql shell against production.** The
console searches sessions/orders/payments/tables/participants and shows sanitized detail +
the lifecycle timeline + audit. Every support read is itself audited (and deep reads emit a
tenant-visible audit row).

> RBAC: support reads require `support_admin` or `read_only_auditor`. Billing issues also need
> a `billing_admin` for the billing surface. Operator mutations remain deliberate and audited.

---

## 1. Payment issue ("guest paid but it's not clearing" / stuck payment)
**Symptoms:** guest UI stuck "awaiting confirmation"; waiter doesn't see it settled.
1. Support → search the **session id / table / order**; open the **payment detail**.
2. Read `payment_status`: `requires_staff_confirmation` (cash/manual → waiter must settle),
   `provider_pending` (awaiting webhook), `completed`, `failed`.
3. Check the **webhook history** on the payment detail for provider callbacks + failures.
4. **Resolution:** cash/manual → have the **waiter settle** in the waiter app (the system never
   auto-settles — fabricating revenue is forbidden). Provider race → wait for/replay the webhook.
5. Confirm the bill snapshot matches what the guest was charged (immutable at initiation).
> Escalation worker only **alerts** on stalled payments; it never auto-cancels/settles. A human
> is the settlement authority (`payment-escalation-lifecycle.md`).

## 2. Stuck session ("table shows occupied but nobody's there" / can't start new session)
1. Support → search the **table**; open the session detail; read `session_status`
   (`active`, `payment_pending`, `awaiting_reactivation`, `abandoned`, `expired`, `closed`).
2. A non-terminal **payment** blocks abandonment — resolve the payment first (runbook 1).
3. The reconciliation worker repairs table↔session drift on its tick; if a table is stranded,
   confirm the session is terminal, then it frees on the next reconcile.
4. Terminal sessions stay **readable for 60 min** for disputes — that's expected, not a bug.

## 3. Reconnect issue ("connection lost" / guest UI not updating)
1. Confirm it's the guest device, not the tenant: other devices on the same session updating?
2. The guest client reconnects with backoff and **reconciles from a snapshot** — ask the guest
   to wait ~30s or reload; state is backend-authoritative and will re-sync.
3. Check `/readyz` on the app (DB+Redis). Redis blips reset presence/tickets but lose no business
   data; clients re-snapshot.
4. Staff dashboards are lighter realtime (partial polling) — a manual refresh is expected.

## 4. QR issue ("QR doesn't open the menu" / wrong table)
1. Support → search the **qr token / table identifier**; confirm the table + branch.
2. Verify the printed QR encodes the correct guest URL (tenant subdomain + `/table/<token>`).
   Regenerate the QR package from onboarding if a sheet was misprinted.
3. If a token was rotated (staff Admin → table QR refresh), **reprint** — old prints are dead.
4. Confirm the branch/restaurant is **active** (suspended status is currently inert, but verify).

## 5. Subscription issue ("is this tenant paid / expiring?")
1. Billing (`/platform/organizations/:id/billing`) → read status + dates.
2. Cross-tenant view: **Observability** (`/platform/observability`) lists suspended / expired /
   trial-ending subscriptions and over-limit tenants in one place.
3. Manual payment received? Record it on the billing page (method, amount, reference) and, if
   invoiced, **mark the invoice paid**. Renew/extend-trial/activate as agreed — all audited.
4. **Remember:** subscription status is **not enforced** yet — an expired sub does **not** block
   the restaurant. Don't promise enforcement behavior that isn't live.

## General
- Find anything by id in Support search (session, order, payment, table, participant, org/branch).
- Sanitized reads only: guest credentials (`session_token`, `device_fingerprint`) are never shown.
- If you can't resolve from the console, escalate per `recovery-procedures.md` — do **not** mutate
  the DB directly.
