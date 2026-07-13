# Restaurant Go-Live Checklist (Pre-Launch Verification)

Run this with the restaurant **before the first real guest** scans a QR. It verifies the
operational loop end-to-end on the actual tenant. Most checks are done in the staff app
(`/staff/login`) and one guest device.

## Accounts & access
- [ ] Owner can sign in to the staff app with their **staff code + PIN**.
- [ ] At least one **kitchen** and one **waiter** staff account exist (create in Admin → Staff).
- [ ] Each role can reach its dashboard: Admin, Kitchen (`/staff/kitchen`), Waiter (`/staff/waiter`).

## Menu
- [ ] Menu categories + items created with correct **prices**.
- [ ] Item availability toggles work (toggle one off → it disappears for guests live).
- [ ] Modifiers/variants (if used) are correct and priced.

## Tables & QR
- [ ] Every physical table has its printed QR placed (from the onboarding QR package).
- [ ] Scanning a table QR opens the guest join screen for the **correct table**.
- [ ] Two guest devices can join the **same** table session and see one shared cart.

## Operational loop (one full walkthrough)
- [ ] Guest (host) adds items → **sends order** (only host can submit).
- [ ] Order appears on the **kitchen** display; advance pending → confirmed → preparing → ready.
- [ ] Order appears in the **waiter** ready-to-serve queue; mark **served**.
- [ ] Guest (host) initiates **payment**; waiter settles cash/UPI → guest sees real
      `PAYMENT_COMPLETED` (no fake success).
- [ ] Session closes cleanly; table frees up.

## Realtime resilience
- [ ] Kill a guest device's network briefly → it shows reconnecting → recovers via snapshot.
- [ ] Assistance request (call waiter) reaches the waiter queue and can be resolved.

## Branding & settings
- [ ] Theme is set (platform Themes or staff admin Appearance) and renders on the guest UI.
- [ ] Tax / service-charge rates correct on the bill breakdown.
- [ ] Branch timezone + order prefix correct (check an order's operational id).

## Billing posture
- [ ] Subscription is **trial** or **active** with the right plan (`/platform/organizations/:id/billing`).
- [ ] Billing profile (GST, billing email) is set for invoicing.

## Sign-off
- [ ] Owner trained on: kitchen flow, waiter serve+settle, assistance, reading the bill.
- [ ] Support contact + escalation path shared (see `support-runbooks-pilot.md`).
- [ ] Operator records go-live date + any known caveats.
