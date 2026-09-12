# E2E vacuity census

**66 executing trusted-green specs out of 133 declarations, against the 115 we have been quoting.**

The structural census is 73 KEEP, 53 QUARANTINE, and 7 UNREACHABLE. Seven of the
73 KEEP declarations are silently disabled by a parent `test.describe.skip`, leaving
66 executing KEEP declarations. The three Playwright projects multiply those 66 into
198 executing trusted-green cases. `T-03` remains in that count only under the explicit
retention decision below: it exercises T1 on the implemented path, but it also has a
known vacuous early-return path and must be repaired in phase 2b.

## Reproducible denominator

Spec files on disk:

```sh
find e2e -type f -name '*.spec.ts' | wc -l
# 109
```

Declaration forms after quarantine:

```sh
printf 'test('; rg --glob '*.spec.ts' -c '^\s*test\s*\(' e2e | awk -F: '{s+=$2} END{print s+0}'
printf 'test.fixme('; rg --glob '*.spec.ts' -c '^\s*test\.fixme\s*\(' e2e | awk -F: '{s+=$2} END{print s+0}'
printf 'test.skip('; rg --glob '*.spec.ts' -c '^\s*test\.skip\s*\(' e2e | awk -F: '{s+=$2} END{print s+0}'
# test(73
# test.fixme(60
# test.skip(0
```

Before this change all 133 were raw `test(` declarations. The declaration total is
unchanged; 60 are now explicitly labelled `fixme`.

Declarations per project and multiplier:

```sh
for p in desktop mobile tablet; do
  printf '%s: ' "$p"
  (cd e2e && npx playwright test --list --project="$p") | tail -n 1
done
(cd e2e && npx playwright test --list) | tail -n 1
# desktop: Total: 133 tests in 109 files
# mobile: Total: 133 tests in 109 files
# tablet: Total: 133 tests in 109 files
# Total: 399 tests in 109 files
```

Each declaration is collected once by each of three projects: multiplier **3**.
Structural KEEP breaks down as **20 mapped to an Invariance.md identifier and 53 mapped
to `none`**. Among the 66 executing KEEP declarations, 20 are mapped and 46 are `none`.

Primary quarantine signature counts are non-overlapping:

| signature | declarations |
|---|---:|
| sig-1 — swallowed failure | 5 |
| sig-2 — conditional/no-assertion path | 13 |
| sig-3 — disjunctive widening | 15 |
| sig-4 — nonexistent response field | 1 |
| sig-5 — claim without attempt | 8 |
| sig-6 — unreachable fixture state | 7 |
| sig-7 — UI/device/reconnect claim without page interaction | 11 |
| sig-8 — asserts only seeded fixture values | 0 |
| **total QUARANTINE + UNREACHABLE** | **60** |

## Protected-reference audit and retained caveat

The protection list was used as calibration, not as an exemption. One protected spec
has a genuine vacuity:

| spec | retained verdict | evidence | action |
|---|---|---|---|
| `e2e/tenancy/T-03-org-suspension.spec.ts` | KEEP with caveat | `if (suspendRes.status === 404) { return }`; if the suspension route disappears or regresses to 404, the test reports green without attempting session creation. | Do not quarantine because its implemented path is the suite's only T1 coverage. Repair the early-return/status widening in phase 2b. |

No other protected reference has a genuine signature hit. In particular, L-09's
`.catch(() => {})` guards only the race between two wait mechanisms, not the action
under test, and its final disjunction names two concrete acceptable UI outcomes. The
other protected references use strict outcomes or success-only alternatives and remain
KEEP without caveat.

## Existing describe-level skips

There are 14 declarations under 11 `test.describe.skip` blocks. Seven are structurally
healthy KEEP declarations and seven are independently vacuous. None is re-enabled here.

`git log` on the current branch contains no commit adding these skips; relative to HEAD
they are uncommitted working-tree changes. With `--all`, their first observable addition
is T3 checkpoint `130fa78ee981c59aafed1f0b406060ece0b567bd`, dated
2026-09-11 22:36:13 +05:30. That checkpoint is not an ancestor of HEAD. The inline
comments added with the skips give the reason: merchant/provider settlement is
unimplemented or out of scope under P5.

| file | skipped describe block | declarations | census result |
|---|---|---:|---|
| `e2e/adversarial/X-04-webhook-signature-forgery.spec.ts` | `X-04: Webhook signature forgery rejected` | 3 | 3 KEEP |
| `e2e/payment/P-03-webhook-replay.spec.ts` | `P-03: Webhook replay idempotency` | 1 | 1 QUARANTINE |
| `e2e/payment/P-04-webhook-bad-signature.spec.ts` | `P-04: Webhook bad signature rejected` | 1 | 1 KEEP |
| `e2e/payment/P-13-webhook-amount-mismatch.spec.ts` | `P-13: Webhook amount mismatch flagged` | 1 | 1 QUARANTINE |
| `e2e/webhook/W-01-valid-webhook-settles-session.spec.ts` | `W-01: Valid webhook settles the session` | 1 | 1 QUARANTINE |
| `e2e/webhook/W-02-webhook-idempotency.spec.ts` | `W-02: Webhook idempotency — duplicate delivery handled` | 1 | 1 QUARANTINE |
| `e2e/webhook/W-03-bad-signature-rejected.spec.ts` | `W-03: Bad webhook signature rejected` | 2 | 2 KEEP |
| `e2e/webhook/W-04-stale-timestamp-rejected.spec.ts` | `W-04: Stale webhook timestamp rejected` | 1 | 1 KEEP |
| `e2e/webhook/W-05-unknown-event-type-accepted.spec.ts` | `W-05: Unknown webhook event type accepted gracefully` | 1 | 1 QUARANTINE |
| `e2e/webhook/W-06-webhook-for-unknown-session.spec.ts` | `W-06: Webhook referencing unknown session handled safely` | 1 | 1 QUARANTINE |
| `e2e/webhook/W-07-payment-failed-webhook.spec.ts` | `W-07: payment.failed webhook reverts session to active or payment_pending` | 1 | 1 QUARANTINE |

## Per-declaration census

Every file is represented; files with multiple declarations have one row per declaration
so a vacuous declaration does not hide or quarantine a healthy sibling.

| spec | declarations | verdict | signature | evidence | invariant |
|---|---|---|---|---|---|
| `e2e/adversarial/X-01-forged-guest-token.spec.ts` | 1 — tampered signature | KEEP | — | `expect(res.status).toBe(401)`; accepting the forged token as 2xx makes this fail. | none |
| `e2e/adversarial/X-01-forged-guest-token.spec.ts` | 1 — wrong session ID | KEEP | — | `expect([401, 403]).toContain(res.status)`; allowing the session-A token to read session B makes this fail. | none |
| `e2e/adversarial/X-02-cross-session-mutation.spec.ts` | 1 — session-A token on session B | KEEP | — | `expect([401, 403]).toContain(res.status)`; accepting the cross-session cart access makes this fail. | none |
| `e2e/adversarial/X-03-staff-token-on-guest-routes.spec.ts` | 1 — staff token on guest route | KEEP | — | `expect([401, 403]).toContain(cartRes.status)`; accepting a staff token on the guest cart route makes this fail. | none |
| `e2e/adversarial/X-04-webhook-signature-forgery.spec.ts` | 1 — invalid signature | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting an invalid signature makes this fail when enabled. | none |
| `e2e/adversarial/X-04-webhook-signature-forgery.spec.ts` | 1 — missing timestamp | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting a timestamp-free webhook makes this fail when enabled. | none |
| `e2e/adversarial/X-04-webhook-signature-forgery.spec.ts` | 1 — stale timestamp | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting a stale signed webhook makes this fail when enabled. | none |
| `e2e/adversarial/X-05-brute-force-lockout.spec.ts` | 1 — ten wrong PINs | KEEP | — | `expect(lockedResponse.status).toBe(423)`; failing to lock the eleventh attempt makes this fail. | none |
| `e2e/adversarial/X-06-cross-org-data-leak.spec.ts` | 1 — org-A staff reads org B | KEEP | — | `expect([403, 404]).toContain(res.status)`; returning org-B sessions to org-A staff makes this fail. | none |
| `e2e/adversarial/X-07-replay-attack-old-ws-ticket.spec.ts` | 1 — reused WS ticket | KEEP | — | `expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("rejected")`; allowing the replayed upgrade makes this fail. | none |
| `e2e/adversarial/X-08-payment-replay-attack.spec.ts` | 1 — payment replay | QUARANTINE | sig-3 | `expect([200, 201, 409]).toContain(pay2.status)`; a backend that rejects every replay with 409 still passes without proving deduplication. | P2 |
| `e2e/audit/A-01-session-lifecycle-audited.spec.ts` | 1 — create and close audit | QUARANTINE | sig-1 | `fetchAudit("session", sessionId).catch(() => [] as any[])`; an unavailable audit endpoint becomes an empty array and passes. | none |
| `e2e/audit/A-02-payment-audited.spec.ts` | 1 — payment audit | QUARANTINE | sig-1 | `fetchAudit("session", sessionId).catch(() => [] as any[])`; an audit endpoint that errors for every payment becomes an empty array and passes. | none |
| `e2e/audit/A-03-staff-auth-audited.spec.ts` | 1 — successful login audit | QUARANTINE | sig-1 | `fetchAudit("staff", String(staffCtx.staffId)).catch(() => [] as any[])`; missing successful-login audit data passes. | none |
| `e2e/audit/A-03-staff-auth-audited.spec.ts` | 1 — failed login audit | QUARANTINE | sig-1 | `fetchAudit("staff", String(staff.id)).catch(() => [] as any[])`; missing failed-login audit data passes. | none |
| `e2e/audit/A-04-waiter-cannot-read-audit.spec.ts` | 1 — waiter audit read | KEEP | — | `expect(res.status).toBe(403)`; allowing the waiter read as 200 makes this fail. | none |
| `e2e/audit/A-05-audit-entries-immutable.spec.ts` | 1 — DELETE audit entry | QUARANTINE | sig-2 | `if (entries.length === 0) { return }`; a system producing no audit rows never attempts DELETE and passes. | none |
| `e2e/audit/A-05-audit-entries-immutable.spec.ts` | 1 — PATCH audit entry | QUARANTINE | sig-2 | `if (entries.length === 0) { return }`; a system producing no audit rows never attempts PATCH and passes. | none |
| `e2e/audit/A-06-cross-org-audit-isolation.spec.ts` | 1 — cross-org audit read | KEEP | — | `expect(res.status).toBe(403)`; returning org-B audit data to org-A ownership makes this fail. | none |
| `e2e/audit/A-07-guest-action-audited.spec.ts` | 1 — guest order audit | QUARANTINE | sig-1 | `fetchAudit("session", sessionId).catch(() => [] as any[])`; an audit endpoint that omits every guest order still passes. | none |
| `e2e/frontend/F-01-qr-rescan-joins-existing.spec.ts` | 1 — QR rescan | QUARANTINE | sig-7 | ``const joinRes = await fetch(`${API_URL}/sessions`, {``; a broken browser QR-rescan flow still passes because no page performs it. | none |
| `e2e/frontend/F-02-frozen-ui-payment-pending.spec.ts` | 1 — frozen payment UI | QUARANTINE | sig-7 | ``const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {``; enabled UI controls still pass because no page is inspected. | C4 |
| `e2e/frontend/F-03-guest-token-in-session-storage.spec.ts` | 1 — storage after join | QUARANTINE | sig-5 | ``await page.goto(`${BASE_URL}/`)``; a join flow that writes the guest token to localStorage passes because no join occurs. | none |
| `e2e/frontend/F-04-staff-token-not-in-local-storage.spec.ts` | 1 — storage after login | QUARANTINE | sig-5 | ``await page.goto(`${BASE_URL}/staff/login`)``; a login flow that writes staff-auth to localStorage passes because no login occurs. | none |
| `e2e/frontend/F-05-reconnecting-banner-on-disconnect.spec.ts` | 1 — awaiting-reactivation snapshot | UNREACHABLE | sig-6 | `expect(["active", "awaiting_reactivation"]).toContain(data.session.status)`; the fresh active fixture passes when awaiting_reactivation is never produced. | none |
| `e2e/frontend/F-06-staff-login-new-format.spec.ts` | 1 — new auth form | QUARANTINE | sig-7 | ``const res = await fetch(`${API_URL}/staff/auth`, {``; a stale form can send the old payload while this direct backend call passes. | none |
| `e2e/frontend/F-06-staff-login-new-format.spec.ts` | 1 — old auth form | QUARANTINE | sig-7 | ``const res = await fetch(`${API_URL}/staff/auth`, {``; a stale browser form can keep using branch_id while this direct call passes. | none |
| `e2e/guest/G-01-fresh-session-and-order.spec.ts` | 1 — fresh guest flow | KEEP | — | `expect(createEvent).toBeDefined()`; removing the session-create audit event makes this fail. | C2 |
| `e2e/guest/G-02-cannot-spoof-participant.spec.ts` | 1 — host spoof attempt | KEEP | — | `expect(result.order.placed_by_participant_id).toBe(hostParticipantId)`; trusting the spoofed body participant makes this fail. | none |
| `e2e/guest/G-02-cannot-spoof-participant.spec.ts` | 1 — non-host spoof attempt | KEEP | — | `expect(orderRes.status).toBe(403)`; allowing the non-host order makes this fail. | H6 |
| `e2e/guest/G-03-revocation.spec.ts` | 1 — revoked token | KEEP | — | `expect(cartRes.status).toBe(401)`; accepting the revoked token makes this fail. | none |
| `e2e/guest/G-04-stale-token-replay.spec.ts` | 1 — closed-session replay | KEEP | — | `expect([401, 403, 409, 410]).toContain(cartRes.status)`; accepting the closed-session mutation as 2xx makes this fail. | S1 |
| `e2e/guest/G-05-token-expiry-refresh.spec.ts` | 1 — expiry refresh | UNREACHABLE | sig-6 | `const snap = await fetchSnapshot(sessionId, guestToken)` immediately follows creation; refresh can be absent for near-expiry tokens and the active fixture still passes. | none |
| `e2e/guest/G-06-two-tabs-same-participant.spec.ts` | 1 — two-tab cart sync | QUARANTINE | sig-7 | ``const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {``; broken browser tab propagation passes because both sides are HTTP calls. | C1 |
| `e2e/guest/G-07-incognito-second-guest.spec.ts` | 1 — shared cart | KEEP | — | `expect(guestBCartData.items[0].quantity).toBe(2)`; restoring per-participant carts makes the second guest's check fail. | C1 |
| `e2e/guest/G-08-creds-cleared-on-session-close.spec.ts` | 1 — credentials cleared | KEEP | — | `expect(tabToken).toBeNull()`; retaining the closed session's bearer in sessionStorage makes this fail. | none |
| `e2e/multidevice/M-01-dual-tab-sync.spec.ts` | 1 — dual-tab order visibility | QUARANTINE | sig-7 | `const snap = await fetchSnapshot(sessionId, guestToken)`; broken tab synchronization passes because no tab or page is created. | none |
| `e2e/multidevice/M-02-concurrent-cart-mutations.spec.ts` | 1 — concurrent cart writes | QUARANTINE | sig-3 | `expect([200, 201, 409]).toContain(resB.status)`; rejecting one valid shared-cart writer with 409 still passes. | C1 |
| `e2e/multidevice/M-03-revocation-propagation.spec.ts` | 1 — revoked token on two routes | KEEP | — | `expect(snapRes.status).toBe(401)` and the cart equivalent; accepting the revoked credential on either route makes this fail. | none |
| `e2e/multidevice/M-04-second-device-joins.spec.ts` | 1 — second-device join | QUARANTINE | sig-7 | `const snap = await fetchSnapshot(sessionId, newToken)`; broken second-device UI/reconnect behavior passes because no device page is exercised. | none |
| `e2e/multidevice/M-05-session-cap-enforced.spec.ts` | 1 — participant cap | QUARANTINE | sig-2 | `expect(typeof capHit).toBe("boolean")`; a server with no participant cap leaves `capHit` false and passes. | none |
| `e2e/multidevice/M-06-host-close-notifies-guests.spec.ts` | 1 — guest observes close | KEEP | — | `expect(["closed", "abandoned", "expired"]).toContain(snap.session.status)`; returning an active session after host close makes this fail. | S1 |
| `e2e/multidevice/M-07-ws-ticket-per-connection.spec.ts` | 1 — per-device WS tickets | QUARANTINE | sig-7 | `const [r1, r2] = await Promise.all([fetchTicket(), fetchTicket()])`; shareable tickets at actual device upgrades pass because no upgrade is attempted. | none |
| `e2e/operational-ids/OPID-01-order-operational-id.spec.ts` | 1 — order operational ID | QUARANTINE | sig-2 | `if (opId !== undefined) {`; omitting both guessed operational-ID fields executes no assertion and passes. | none |
| `e2e/operational-ids/OPID-02-payment-reference.spec.ts` | 1 — payment reference | QUARANTINE | sig-2 | `if (returnedRef !== undefined) {`; omitting the payment reference executes no equality assertion and passes. | none |
| `e2e/operational-ids/OPID-03-operational-id-sequential.spec.ts` | 1 — sequential IDs | QUARANTINE | sig-2 | `if (!r1.ok \|\| !r2.ok) return`; a server rejecting either valid order performs no sequencing assertion and passes. | none |
| `e2e/operational-ids/OPID-04-operational-id-kitchen-display.spec.ts` | 1 — kitchen display ID | QUARANTINE | sig-3 | `expect(opId !== undefined \|\| opId === undefined).toBe(true)`; a kitchen response with no operational ID satisfies the tautology. | none |
| `e2e/order/O-01-order-placement.spec.ts` | 1 — order placement | KEEP | — | `expect(result.order.order_operational_id).toMatch(/^\S+$/)`; omitting the documented operational ID makes this fail. | C2 |
| `e2e/order/O-02-unavailable-item-rejected.spec.ts` | 1 — unavailable item | KEEP | — | `expect(unavailableRes.status).toBe(204)` plus a failure-only order status set; accepting the unavailable item as 201 makes this fail. | none |
| `e2e/order/O-03-cross-branch-item-rejected.spec.ts` | 1 — cross-branch item | KEEP | — | `expect([400, 403, 404, 422]).toContain(res.status)`; accepting the foreign-branch item as 201 makes this fail. | none |
| `e2e/order/O-04-idempotency.spec.ts` | 1 — same key, same body | QUARANTINE | sig-4 | `expect(o1.id ?? o1.order_id).toEqual(o2.id ?? o2.order_id)`; the API returns nested `order.id`, so two missing fields compare undefined to undefined. | none |
| `e2e/order/O-04-idempotency.spec.ts` | 1 — same key, different body | KEEP | — | `expect([409]).toContain(conflictRes.status)`; accepting the conflicting body as 2xx makes this fail. | none |
| `e2e/order/O-05-empty-order-rejected.spec.ts` | 1 — empty items | KEEP | — | `expect([400, 422]).toContain(res.status)`; accepting an empty order as 201 makes this fail. | none |
| `e2e/order/O-05-empty-order-rejected.spec.ts` | 1 — zero quantity | KEEP | — | `expect([400, 422]).toContain(res.status)`; accepting zero quantity as 201 makes this fail. | none |
| `e2e/order/O-06-order-on-closed-session.spec.ts` | 1 — closed-session order | KEEP | — | `expect([400, 401, 403, 409, 410, 422]).toContain(res.status)`; accepting a new order as 201 after closure makes this fail. | S1 |
| `e2e/order/O-07-concurrent-orders.spec.ts` | 1 — host-only ordering | KEEP | — | `expect(nonHostOrder.status).toBe(403)` and `expect(hostOrder.status).toBe(201)`; either inverted authority outcome fails. | H6 |
| `e2e/order/O-08-order-operational-id-unique.spec.ts` | 1 — unique operational IDs | QUARANTINE | sig-2 | `if (opId1 != null && opId2 != null) {`; omitting both guessed fields executes no uniqueness assertion and passes. | none |
| `e2e/payment/P-01-cash-settlement.spec.ts` | 1 — cash settlement | QUARANTINE | sig-3 | `expect([200, 204, 404]).toContain(settleRes.status)`; a missing settlement endpoint returns 404 and the test never asserts closure. | P5 |
| `e2e/payment/P-02-concurrent-settlement.spec.ts` | 1 — concurrent settlement | QUARANTINE | sig-2 | `if (!paymentId) return`; an incompatible or missing payment ID avoids both settlement attempts and passes. | P4 |
| `e2e/payment/P-03-webhook-replay.spec.ts` | 1 — triple webhook replay | QUARANTINE | sig-3 (`describe.skip`) | `expect(statuses.every((s) => [200, 204, 400, 404, 409].includes(s))).toBe(true)`; three failures can pass without one processed event. | none |
| `e2e/payment/P-04-webhook-bad-signature.spec.ts` | 1 — tampered webhook body | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting the tampered signed body makes this fail when enabled. | none |
| `e2e/payment/P-05-cart-frozen-during-payment.spec.ts` | 1 — frozen cart | KEEP | — | `expect(err.code).toBe("PAYMENT_IN_PROGRESS")`; accepting the cart mutation or returning the wrong conflict reason makes this fail. | C4 |
| `e2e/payment/P-06-stuck-payment.spec.ts` | 1 — payment-pending state | KEEP | — | `expect(snap.session.status).toBe("payment_pending")`; leaving the session active after initiation makes this fail. | C4 |
| `e2e/payment/P-07-reconnect-mid-payment.spec.ts` | 1 — reconnect mid-payment | QUARANTINE | sig-7 | `const snap = await fetchSnapshot(sessionId, guestToken)`; browser reconnect that duplicates payment passes because only HTTP calls are made. | P2 |
| `e2e/payment/P-08-split-payment.spec.ts` | 1 — full-bill payment reuse | KEEP | — | `expect(partialPayment.status).toBe(422)` and `expect(second.id).toBe(first.id)`; partial acceptance or a second outstanding payment fails. | P2, P3 |
| `e2e/payment/P-09-overpayment-rejection.spec.ts` | 1 — overpayment | KEEP | — | `expect([400, 409, 422]).toContain(payRes.status)`; accepting an overpayment as 2xx makes this fail. | P3 |
| `e2e/payment/P-10-promo-race.spec.ts` | 1 — concurrent promo | QUARANTINE | sig-3 | `expect([200, 201, 400, 404, 409, 422]).toContain(s)`; two missing-endpoint 404 responses pass without testing a promo race. | none |
| `e2e/payment/P-11-bill-snapshot-immutable.spec.ts` | 1 — immutable bill snapshot | QUARANTINE | sig-5 | `expect(totalAfter).toEqual(totalBefore)` follows no post-payment mutation; a mutable snapshot passes because nothing tries to change it. | P1 |
| `e2e/payment/P-12-payment-pending-blocks-order.spec.ts` | 1 — pending blocks order | KEEP | — | `expect([409, 422]).toContain(orderRes.status)`; accepting a new order during payment_pending as 201 makes this fail. | C4 |
| `e2e/payment/P-13-webhook-amount-mismatch.spec.ts` | 1 — amount mismatch | QUARANTINE | sig-3 (`describe.skip`) | `expect([200, 400, 409, 422]).toContain(res.status)`; silently accepting the mismatched amount as 200 can still pass. | P4 |
| `e2e/payment/P-14-ws-during-payment-pending.spec.ts` | 1 — WS during payment | KEEP | — | `expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("opened")`; refusing the actual upgrade during payment_pending makes this fail. | C4 |
| `e2e/platform/PT-01-support-session-read.spec.ts` | 1 — platform cross-org read | KEEP | — | `expect(data.session.id).toBe(sessionId)`; denying the platform read or returning another session makes this fail. | none |
| `e2e/platform/PT-02-force-close-any-session.spec.ts` | 1 — platform force close | QUARANTINE | sig-5 | `await forceCloseSession(sessionId, guestToken)`; removing platform force-close authority still passes because the guest host closes the session. | S2 |
| `e2e/platform/PT-03-mfa-required-for-platform.spec.ts` | 1 — MFA status | QUARANTINE | sig-3 | `expect([200, 401, 404]).toContain(res.status)`; an absent or unauthorized MFA status endpoint still passes. | none |
| `e2e/platform/PT-03-mfa-required-for-platform.spec.ts` | 1 — MFA enroll | QUARANTINE | sig-3 | `expect([200, 201, 400, 401, 404, 409, 503]).toContain(res.status)`; almost any broken enrollment response passes. | none |
| `e2e/platform/PT-04-org-provisioning.spec.ts` | 1 — organization provisioning | KEEP | — | `expect(branchResult.branch.organization_id).toBe(orgResult.organization.id)`; provisioning a branch under the wrong organization makes this fail. | none |
| `e2e/platform/PT-05-awaiting-reactivation-worker.spec.ts` | 1 — platform pause | QUARANTINE | sig-3 | `expect([200, 204, 404]).toContain(pauseRes.status)`; a missing pause route returns 404 and avoids all state checks. | none |
| `e2e/realtime/R-01-ws-ticket-required.spec.ts` | 1 — ticket authentication | KEEP | — | `expect(validRes.status).toBe(201)` and strict 401 checks; refusing valid credentials or accepting missing credentials fails. | none |
| `e2e/realtime/R-01-ws-ticket-required.spec.ts` | 1 — single-use ticket | KEEP | — | `expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("rejected")`; accepting a second real upgrade makes this fail. | none |
| `e2e/realtime/R-02-snapshot-authority.spec.ts` | 1 — committed snapshot state | KEEP | — | `expect(snap.participants.some((p) => p.display_name === "P2")).toBe(true)`; omitting the committed join from the snapshot makes this fail. | none |
| `e2e/realtime/R-02-snapshot-authority.spec.ts` | 1 — missed_events field | QUARANTINE | sig-2 | `expect(Array.isArray(snap.missed_events ?? [])).toBe(true)`; omitting missed_events substitutes an empty array and passes. | none |
| `e2e/realtime/R-03-ws-ticket-rate-limit.spec.ts` | 1 — ticket rate limit | KEEP | — | `expect(statuses.filter((status) => status === 429)).toHaveLength(3)`; removing or changing the rate limit makes this fail. | none |
| `e2e/realtime/R-04-event-sequence-monotonic.spec.ts` | 1 — monotonic sequence | QUARANTINE | sig-2 | `if (events.length >= 2) {`; returning no replay events runs no ordering assertion and passes. | none |
| `e2e/realtime/R-05-reconnect-replays-missed-events.spec.ts` | 1 — missed-event replay | QUARANTINE | sig-2 | `if (events.length === 0) return`; a replay implementation returning no events immediately passes. | none |
| `e2e/realtime/R-06-snapshot-authoritative-on-large-gap.spec.ts` | 1 — large replay gap | UNREACHABLE | sig-6 | `for (let i = 0; i < 3; i++)` creates only three events; removing large-gap fallback behavior does not affect this fixture. | none |
| `e2e/realtime/R-07-multi-device-event-propagation.spec.ts` | 1 — multi-device propagation | QUARANTINE | sig-7 | `const snapB = await fetchSnapshot(sessionId, tokenB)`; broken live delivery to device B passes because no second page or socket is used. | none |
| `e2e/realtime/R-08-presence-expiry.spec.ts` | 1 — presence expiry | UNREACHABLE | sig-6 | `const snap = await fetchSnapshot(sessionId, guestToken)` immediately follows session creation; presence can remain forever and the fixture passes. | H4 |
| `e2e/realtime/R-09-duplicate-event-dedup.spec.ts` | 1 — duplicate order event | KEEP | — | `expect(order1.order.id).toBe(order2.order.id)`; creating a second order for the same key makes this fail. | none |
| `e2e/screenshots/sweep.spec.ts` | 1 — guest screenshot flow | KEEP | — | `await page.screenshot({ path: path.join(dir, "05-payment.png"), fullPage: true })`; navigation/render failure rejects the screenshot flow. This is artifact coverage, not pilot invariant coverage. | none |
| `e2e/screenshots/sweep.spec.ts` | 1 — staff screenshots | KEEP | — | `await page.getByLabel("PIN").fill(staff.pin)`; removing the login control makes the flow fail. This is artifact coverage only. | none |
| `e2e/screenshots/sweep.spec.ts` | 1 — ended-session screenshot | KEEP | — | `expect(closeRes.status).toBe(200)`; failure to stage the ended state makes this fail. The screenshot itself has no visual assertion. | none |
| `e2e/screenshots/sweep.spec.ts` | 1 — payment-pending screenshot | KEEP | — | ``await page.goto(`${BASE_URL}/session/${sessionId}/payment`)``; a navigation/render exception fails, but the screenshot has no visual assertion. | none |
| `e2e/screenshots/sweep.spec.ts` | 1 — platform login screenshot | KEEP | — | `await page.screenshot({ path: path.join(dir, "01-platform-login.png"), fullPage: true })`; capture failure rejects the flow, but there is no visual assertion. | none |
| `e2e/session/L-01-one-session-per-table.spec.ts` | 1 — concurrent session creation | KEEP | — | `expect(conflicts.length).toBe(4)`; allowing more than one active session or rejecting all five makes this fail. | none |
| `e2e/session/L-02-stale-session-worker.spec.ts` | 1 — stale worker | UNREACHABLE | sig-6 | `expect(snap.session.status).toBe("active")` is the only state check; a worker that never terminates stale sessions still passes. | S1 |
| `e2e/session/L-03-reconnect-awaiting-reactivation.spec.ts` | 1 — active-session ticket | UNREACHABLE | sig-6 | `const created = await createSession(table.id, "ReconnUser")`; only active state is constructed, so awaiting-reactivation reconnect behavior can be absent and pass. | none |
| `e2e/session/L-03-reconnect-awaiting-reactivation.spec.ts` | 1 — active snapshot | UNREACHABLE | sig-6 | `expect(snap.session.status).toBe("active")`; only active state is asserted, so the suite-title state is never reached. | none |
| `e2e/session/L-04-reconnect-after-abandoned.spec.ts` | 1 — reconnect after abandonment | QUARANTINE | sig-7 | ``const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {``; broken browser reconnect passes because only HTTP endpoints are called. | S1 |
| `e2e/session/L-05-host-close-idempotent.spec.ts` | 1 — close revokes credential | KEEP | — | `expect(close2.status).toBe(401)`; leaving the credential usable after close makes the replay fail this assertion. | S1 |
| `e2e/session/L-06-non-host-cannot-close.spec.ts` | 1 — non-host close | KEEP | — | `expect(closeRes.status).toBe(403)`; allowing the non-host close makes this fail. | H6 |
| `e2e/session/L-07-force-close-by-staff.spec.ts` | 1 — staff force close | KEEP | — | `expect(closeRes.status).toBe(200)` and `expect(closeAudit).toBeTruthy()`; missing closure or audit makes this fail. | S2 |
| `e2e/session/L-08-closed-not-resurrectable.spec.ts` | 1 — closed session mutations | KEEP | — | `expect(newSessionRes.status).toBe(201)` plus failure-only mutation checks; retaining table occupancy or accepting stale mutations fails. | S1 |
| `e2e/session/L-09-stale-browser-revisit.spec.ts` | 1 — stale browser revisit | KEEP | — | `expect(isHome \|\| hasEndedText).toBe(true)`; leaving the stale browser on the active session UI makes both concrete outcomes false. | S1 |
| `e2e/session/L-10-back-forward-no-mutations.spec.ts` | 1 — browser navigation duplication | QUARANTINE | sig-5 | `expect(snap.session.status).toBe("active")`; duplicate-order creation does not change session status, so the claimed regression passes. | none |
| `e2e/session/L-11-reactivation-clears-banner.spec.ts` | 1 — reactivation clears banner | KEEP | — | `await expect(pausedBanner).toBeHidden({ timeout: 12_000 })`; retaining the overlay after active snapshot reconciliation makes this fail. | none |
| `e2e/staff/S-01-staff-login.spec.ts` | 1 — valid login | KEEP | — | `expect(data.branch_id).toBe(branch.id)`; wrong tenant scope or failed valid authentication makes this fail. | none |
| `e2e/staff/S-01-staff-login.spec.ts` | 1 — wrong PIN | KEEP | — | `expect([401, 403]).toContain(res.status)`; accepting the wrong PIN as 200 makes this fail. | none |
| `e2e/staff/S-01-staff-login.spec.ts` | 1 — legacy login format | KEEP | — | `expect([400, 401, 422]).toContain(res.status)`; accepting branch_id plus PIN as 200 makes this fail. | none |
| `e2e/staff/S-02-inactive-staff-rejected.spec.ts` | 1 — inactive staff | KEEP | — | `expect([401, 403]).toContain(loginRes.status)`; authenticating deactivated staff as 200 makes this fail. | none |
| `e2e/staff/S-03-duplicate-staff-code-rejected.spec.ts` | 1 — duplicate staff code | KEEP | — | `expect([409, 422]).toContain(second.status)`; creating the duplicate as 201 makes this fail. | none |
| `e2e/staff/S-04-pin-rotation.spec.ts` | 1 — PIN rotation | KEEP | — | `expect(newLoginRes.status).toBe(200)` plus old-PIN rejection; failing either half of rotation makes this fail. | none |
| `e2e/staff/S-05-lockout-after-failures.spec.ts` | 1 — repeated-PIN lockout | QUARANTINE | sig-3 | `expect([200, 401, 423, 429]).toContain(correctRes.status)`; successful authentication while supposedly locked is explicitly accepted. | none |
| `e2e/staff/S-06-role-separation.spec.ts` | 1 — waiter creates staff | KEEP | — | `expect([401, 403]).toContain(res.status)`; allowing staff creation as 201 makes this fail. | none |
| `e2e/staff/S-06-role-separation.spec.ts` | 1 — waiter edits menu | KEEP | — | `expect([401, 403]).toContain(res.status)`; allowing the waiter mutation as 204 makes this fail. | none |
| `e2e/staff/S-06-role-separation.spec.ts` | 1 — waiter resets owner PIN | KEEP | — | `expect(res.status).toBe(403)`; allowing the takeover mutation makes this fail. | none |
| `e2e/staff/S-06-role-separation.spec.ts` | 1 — waiter reads roster | KEEP | — | `expect(waiterRes.status).toBe(403)` and owner 200; fail-open roster access makes this fail. | none |
| `e2e/staff/S-07-cross-branch-mutation-rejected.spec.ts` | 1 — cross-branch close | QUARANTINE | sig-5 | ``fetch(`${API_URL}/staff/sessions/${sessionBId}/close`, {`` targets an unregistered route; 404 passes without testing cross-branch authorization. | none |
| `e2e/staff/S-08-owner-platform-limits.spec.ts` | 1 — owner on platform route | KEEP | — | `expect([401, 403]).toContain(res.status)`; allowing organization creation as 201 makes this fail. | none |
| `e2e/tenancy/T-01-cross-org-session-access.spec.ts` | 1 — cross-org session read | KEEP | — | `expect([401, 403, 404]).toContain(res.status)`; returning org-B session data to org-A staff as 200 makes this fail. | none |
| `e2e/tenancy/T-02-cross-branch-menu-isolation.spec.ts` | 1 — cross-branch menu item | KEEP | — | `expect([400, 403, 404, 422]).toContain(res.status)`; accepting branch-B's item into branch-A's order as 201 makes this fail. | none |
| `e2e/tenancy/T-03-org-suspension.spec.ts` | 1 — suspended-org session creation | KEEP | sig-2/sig-3 caveat | `if (suspendRes.status === 404) { return }`; loss of the suspension route passes without attempting creation. Retained for its implemented-path T1 assertion; repair in phase 2b. | T1 |
| `e2e/tenancy/T-04-table-belongs-to-branch.spec.ts` | 1 — table reassignment | QUARANTINE | sig-5 | `body: JSON.stringify({ identifier: "T-STOLEN" })`; no branch reassignment is attempted, so missing ownership enforcement passes. | none |
| `e2e/tenancy/T-04-table-belongs-to-branch.spec.ts` | 1 — QR branch context | QUARANTINE | sig-2 | `if (res.ok) {`; a missing QR-resolution endpoint takes the accepted 404 branch and never checks branch_id. | none |
| `e2e/tenancy/T-05-platform-admin-cross-org-read.spec.ts` | 1 — platform versus org staff | QUARANTINE | sig-5 | `const orgB = await seedOrg("t05b")`; no org-A staff identity or denied read is created, so broken tenant isolation passes. | none |
| `e2e/tenancy/T-06-guest-token-scoped-to-session.spec.ts` | 1 — guest token session scope | KEEP | — | `expect([401, 403, 404]).toContain(cartRes.status)`; accepting the session-A credential on session B as 2xx makes this fail. | none |
| `e2e/webhook/W-01-valid-webhook-settles-session.spec.ts` | 1 — completed webhook | QUARANTINE | sig-2 (`describe.skip`) | `if (snapRes.status === 200) {`; a failed snapshot performs no state assertion, and payment_pending is accepted even when settlement did nothing. | P4 |
| `e2e/webhook/W-02-webhook-idempotency.spec.ts` | 1 — duplicate webhook | QUARANTINE | sig-3 (`describe.skip`) | `expect([200, 202, 409]).toContain(r2.status)`; status alone passes if two 200 deliveries both settle independently. | P4 |
| `e2e/webhook/W-03-bad-signature-rejected.spec.ts` | 1 — unsigned webhook | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting an unsigned webhook makes this fail when enabled. | none |
| `e2e/webhook/W-03-bad-signature-rejected.spec.ts` | 1 — wrong webhook secret | KEEP | — (`describe.skip`) | `expect(res.status).toBe(401)`; accepting a signature from the wrong secret makes this fail when enabled. | none |
| `e2e/webhook/W-04-stale-timestamp-rejected.spec.ts` | 1 — stale webhook timestamp | KEEP | — (`describe.skip`) | `expect([400, 401, 422]).toContain(res.status)`; accepting the ten-minute-old webhook as 2xx makes this fail when enabled. | none |
| `e2e/webhook/W-05-unknown-event-type-accepted.spec.ts` | 1 — unknown webhook event | QUARANTINE | sig-3 (`describe.skip`) | `expect([200, 202, 400]).toContain(res.status)`; rejecting the event as 400 passes despite the claim that it is accepted. | none |
| `e2e/webhook/W-06-webhook-for-unknown-session.spec.ts` | 1 — unknown-session webhook | QUARANTINE | sig-3 (`describe.skip`) | `expect([200, 202, 404, 422]).toContain(res.status)`; success and failure are both accepted without proving a no-op. | none |
| `e2e/webhook/W-07-payment-failed-webhook.spec.ts` | 1 — failed-payment webhook | QUARANTINE | sig-3 (`describe.skip`) | `expect([200, 202, 404]).toContain(failRes.status)`; a missing handler returns 404 and the conditional snapshot assertion still permits no reset. | P4 |

## Repair notes for phase 2b

- `T-03` must require the suspension call to succeed before asserting that subsequent
  new-session creation is rejected. It is deliberately not marked `fixme` in this task.
- The seven healthy declarations hidden under `test.describe.skip` need a separate
  product-scope decision. Re-enabling them is not part of this quarantine commit.
- Files marked sig-4 must read the documented response field directly; fallback chains
  must not permit `undefined === undefined`.
- UNREACHABLE declarations require fixture control over time, worker execution, or
  staged lifecycle state. Changing only an expected status would preserve the vacuity.
