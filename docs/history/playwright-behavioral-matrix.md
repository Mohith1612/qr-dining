# Playwright Behavioral Test Matrix

Date: 2026-05-22
Status: Specification only. Tests are not implemented in this session. The matrix is intentionally written so that a lower-cost model or contractor can execute test authoring directly against it.

## 0. How to use this document

Each row in each matrix is a single test. Implement one Playwright test per row. Tests must be deterministic (no time-dependent flakes), self-cleaning (each test seeds and tears down its own data), and parallelizable (no shared state).

Naming convention: `e2e/<area>/<id>-<short-name>.spec.ts`. Example: `e2e/guest/G-04-stale-token-replay.spec.ts`.

Assertion style: every test asserts both a UI outcome (what the user sees) and a backend outcome (audit event, DB state, or API response from a verification endpoint).

Helpers required (build once, reuse):

- `seedOrg(spec)`: returns `{ org, branch, table, staff, menu }`.
- `loginStaff({ branchCode, staffCode, pin })`: returns a Playwright `BrowserContext` authenticated as staff.
- `joinSessionAsGuest({ tableToken, displayName })`: returns `{ context, sessionId, participantId, guestToken }`.
- `fetchAudit({ resourceType, resourceId })`: backend admin call to fetch the audit trail for verification.
- `fetchSnapshot({ sessionId, asGuestToken })`: backend call.
- `forceCloseSession({ sessionId, asAdmin })`: helper.
- `placeWebhook({ provider, event })`: signs and posts a webhook to the local server.

## 1. Identity and Credential Tests (matrix area: `guest`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| G-01 | Fresh guest session and order | Scan QR, set display name, add cart item, place order. | UI shows order confirmation. Audit has `session.create`, `cart.item.add`, `order.create.self`. Order row references session and participant from guest credential. |
| G-02 | Guest cannot spoof another participant | Capture host's participant_id from snapshot, attempt order placement with host's participant_id in body. | When `AUTH_GUEST_CREDENTIALS_REQUIRED=true`, request 401. When false, server logs `legacy_identity_usage_total` and order is attributed to guest token's participant, not the body value. |
| G-03 | Guest credential after revocation | Host revokes participant. Revoked participant attempts cart action. | UI shows "session ended" toast. API returns 401 `credential_revoked`. WS closed. |
| G-04 | Stale token replay after session closed | Save guest token, close session, replay token on a fresh tab. | API returns 410. UI clears local state and routes to QR rescan. |
| G-05 | Guest token expiry refresh | Set short TTL, wait for expiry, attempt mutation. | UI prompts refresh. Snapshot read re-issues a fresh token while session is active. |
| G-06 | Two tabs same participant | Open session in two tabs, mutate cart in tab A, verify tab B. | Tab B reflects A's cart within 2 seconds via `CART_UPDATED`. |
| G-07 | Incognito second guest | Host joins, second guest joins via incognito tab. | Two distinct participants. Each sees their own cart. Host can see both in snapshot. |

## 2. Staff Auth Tests (matrix area: `staff`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| S-01 | Staff login with staff_code | Login with `branch_code + staff_code + PIN`. | UI redirects to dashboard. Audit `staff.login.success`. `staff_sessions` row exists. |
| S-02 | Inactive staff cannot login | Deactivate staff via admin, attempt login. | API 401. Audit `staff.login.failed{reason=inactive}`. |
| S-03 | Duplicate PIN with different staff_code | Two staff in same branch share PIN. Login with the wrong staff_code. | Wrong staff_code rejects. Correct one accepts the identity claimed. |
| S-04 | PIN rotation invalidates active session | Login on device A. Manager rotates PIN. Use device A's session. | Device A's next mutation returns 401. Forced re-login. |
| S-05 | Lockout after N failures | Submit wrong PIN N times. | After N, API returns 429 with `Retry-After`. Audit `auth.lockout`. |
| S-06 | Staff token rejected by platform middleware | Use staff token against `/platform/*`. | API 401 regardless of staff role. |
| S-07 | Staff cannot mutate another branch's order | Branch A staff PATCH branch B order status. | API 403. Audit `authz.denied{reason=branch_mismatch}`. |
| S-08 | Staff org owner has only platform-permitted actions | An owner-role staff attempts a platform-only action. | API 403. |

## 3. Tenancy Isolation Tests (matrix area: `tenancy`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| T-01 | Org A staff cannot read Org B branch resources | Cross-org request. | 403 + audit. |
| T-02 | Org owner can read all branches in own org | Org owner pulls cross-branch analytics. | 200 + scoped data. |
| T-03 | Org owner cannot read other org's analytics | Same as T-02 but other org. | 403 + audit. |
| T-04 | Customer history scoped to org/restaurant | Pull history for customer assigned to another org. | 404 or 403; no data leak. |
| T-05 | Branch suspension closes sessions | Admin suspends branch with active sessions. | Sessions transition to `abandoned{reason=branch_suspended}`. Credentials revoked. UI on affected devices: "session ended". |
| T-06 | Branch under one org cannot be moved to another via direct API | Attempt to swap `organization_id`. | API rejects (no endpoint), or 403 if route exists. |

## 4. Session Lifecycle Tests (matrix area: `session`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| L-01 | One session per table under concurrency | 5 concurrent session creates against the same table. | Exactly 1 success; 4 receive 409 `TABLE_OCCUPIED`. |
| L-02 | Stale session worker abandons + releases table | Create session, let it go quiet beyond `quiet_grace_seconds`. | Worker transitions to `awaiting_reactivation` then `abandoned`. Table `available`. Credentials revoked. |
| L-03 | Reconnect during awaiting_reactivation | Trigger awaiting_reactivation, reconnect within window. | Session back to `active`. Snapshot consistent. |
| L-04 | Reconnect after abandoned | Same as L-03 but after window. | Snapshot returns 410. UI rescans QR. |
| L-05 | Host close idempotent | Host calls DELETE twice rapidly. | First closes, second is no-op 200. |
| L-06 | Non-host cannot close | Non-host attempts DELETE. | 403. |
| L-07 | Force close by staff | Staff `force_close`. | Session `closed{reason=staff_force_close}`. Audit recorded. |
| L-08 | Closed session not resurrectable | Replay closed session URL, attempt any mutation. | All 410. |
| L-09 | Stale browser revisit | Close session, restore the same browser tab next day. | UI: "session has ended" + scan CTA. Local state cleared. |
| L-10 | Browser back/forward without spurious mutations | Add to cart, navigate away, navigate back. | No duplicate orders. Cart matches server. |

## 5. Order and Cart Tests (matrix area: `order`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| O-01 | Order placed with valid items | Add items, place order. | Order created. Status `pending`. Events propagate to staff dashboard. |
| O-02 | Cross-branch menu items rejected | Send order with menu_item_id from another branch. | 400 `branch_mismatch`. |
| O-03 | Order idempotency same body | Same `Idempotency-Key`, identical body, 5 retries. | One order, five 201s with the same body. |
| O-04 | Order idempotency different body | Same key, different items. | 409 `IDEMPOTENCY_CONFLICT`. |
| O-05 | Order cancellation by staff | Staff cancels pending order. | Status `cancelled`. Audit `order.cancel`. UI updates both staff and guest. |
| O-06 | Status transition expected-status | Two staff devices both try `preparing -> ready` at once. | One succeeds, one no-op. No event flap. |
| O-07 | Order during payment_pending | Place order while session is `payment_pending`. | 409 `PAYMENT_IN_PROGRESS`. |
| O-08 | Order count concurrency | 50 concurrent orders, idempotency on 5%. | Exact count, contiguous order numbers within branch-day. |

## 6. Payment Tests (matrix area: `payment`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| P-01 | Cash settlement happy path | Guest requests payment, staff settles. | Payment `completed`. Session `closed`. Table `available`. Audit chain present. |
| P-02 | Concurrent settlement attempts | Two staff devices settle the same payment at once. | One succeeds, one no-op. |
| P-03 | Webhook replay | Send the same signed webhook 10 times. | One status transition, nine `already_processed`. |
| P-04 | Webhook bad signature | Tamper body. | 401, audit `payment.webhook.signature_failed`. |
| P-05 | Cart mutation during payment_pending | Add item during pending. | 409 `PAYMENT_IN_PROGRESS`. |
| P-06 | Timeout during payment_pending stuck | Drive session to stuck threshold without progress. | Alert audit `payment.stuck`. Staff `cancel_pending_payments` returns session to active. |
| P-07 | Reconnect mid-payment | Disconnect guest after payment request, reconnect. | Snapshot reflects pending payment. No duplicate payment. |
| P-08 | Split payment cap | Two guests pay half each. | Both complete. Session closes when residual zero. |
| P-09 | Over-payment rejection | Third attempt after residual zero. | 409 `OVERPAYMENT`. |
| P-10 | Refund flow | Issue refund, replay refund. | First succeeds, second rejected by policy or 409. |
| P-11 | Promo race cap | 200 concurrent attempts at 100-cap promo. | Exactly 100 succeed. |
| P-12 | Per-phone promo without phone | Apply per-phone-capped promo without providing phone. | `PROMO_REQUIRES_PHONE`. |
| P-13 | Webhook arriving after staff cash settlement | Settle cash, then webhook arrives for the digital attempt. | Digital payment `failed{reason=snapshot_already_settled}`. Audit divergence. No money double-credit. |

## 7. Realtime and Reconnect Tests (matrix area: `realtime`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| R-01 | WS ticket happy path | Issue ticket, connect, receive event. | Live updates flowing. Audit shows ticket issuance and consume. |
| R-02 | WS ticket replay rejected | Reuse consumed ticket. | Connect rejected. |
| R-03 | WS expired ticket | Wait past 30s. | Rejected. |
| R-04 | Sequence dedup | Inject artificial duplicate event_id at frontend. | One application; second discarded. |
| R-05 | Replay on reconnect | Disconnect, miss N events, reconnect with `since`. | Server replays N. Final state matches. |
| R-06 | Snapshot authoritative on big gap | Miss more than `event_replay_window`. | Server emits `SNAPSHOT_AUTHORITATIVE`. Client refetches and resumes. |
| R-07 | Out-of-order event discarded | Inject lower-sequence event after applying higher. | Discarded. No UI flicker. |
| R-08 | Reconnect storm | 50 concurrent reconnects to same session. | All succeed within 30 seconds. No event loss. |
| R-09 | Cross-branch event filter | Force-publish an event with wrong branch_id (test hook). | Subscriber filter drops it. UI does not show. |

## 8. Multi-Device and Multi-Tab Tests (matrix area: `multi`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| M-01 | Single participant, two tabs, same browser | Mutate in A, observe in B. | B reflects within 2s via WS event. |
| M-02 | Single participant, two devices | Mutate in phone, observe in laptop. | Laptop reflects within 2s. |
| M-03 | Two participants, concurrent orders | Both place orders in parallel. | Both succeed. Audit shows two participant ids. |
| M-04 | Concurrent payment attempts (multi) | Two devices, two participants press pay. | One snapshot. Either split-pay or `PAYMENT_IN_PROGRESS`. |
| M-05 | Duplicate tab after browser restart | Browser restores tab; the session is now closed. | "Session ended" CTA; local state cleared. |
| M-06 | Waiter dashboard concurrent ack | Two staff click "Acknowledge" on the same assistance request. | One success, one no-op. UI updates both. |
| M-07 | Admin and waiter concurrent mutation | Manager edits menu item while a waiter is reading; waiter retries on stale. | Manager update succeeds. Waiter sees new value on next refresh. |

## 9. Webhook and Provider Behavior (matrix area: `webhook`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| W-01 | Provider event new | Fresh event with valid signature. | Processed, audit, payment updated. |
| W-02 | Provider event duplicate | Same `external_event_id` again. | 200, no transition. |
| W-03 | Provider event old timestamp | Stale timestamp beyond tolerance. | 401. |
| W-04 | Provider event amount mismatch | Amount differs from snapshot total. | Audit `amount_mismatch`. No transition. |
| W-05 | Provider event unknown payment id | Provider payment id we do not know. | Audit `unknown_payment_reference`. No transition. |
| W-06 | Two providers' secrets coexist | Rotate secret while keeping old valid. | Both old and new signed events accepted during rotation window. |
| W-07 | Webhook replay storm | 10k events in 10 min mostly duplicates. | All complete within rate limit. No regression. |

## 10. Audit Visibility Tests (matrix area: `audit`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| A-01 | Org owner reads org audit | Owner GET `/orgs/:org/audit`. | 200 with rows. Self-audit `audit.read.organization` written. |
| A-02 | Branch manager reads branch audit | Manager GET `/branches/:branch/audit`. | 200 + scoped rows. |
| A-03 | Branch manager cannot read another branch audit | Cross-branch GET. | 403 + audit. |
| A-04 | Guest cannot read any audit | Guest token tries. | 401. |
| A-05 | Platform admin reads platform audit | Platform admin GET `/platform/audit`. | 200. |
| A-06 | Audit row immutability | Attempt UPDATE/DELETE via DB role used by app. | Rejected at DB level. |
| A-07 | Sensitive fields redacted | Force a `staff.pin.reset` event; inspect metadata. | No raw PIN visible. |

## 11. Platform Support and Break-Glass (matrix area: `platform`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| PT-01 | Platform admin creates org and assigns owner | Provisioning flow. | Org row, owner invited, audit chain. |
| PT-02 | Platform admin support session | Create, approve, perform a read. | Reads carry `support_session_id`. Org owner sees the read in org audit. |
| PT-03 | Support session expiry | Wait past expires_at, attempt read. | 401. |
| PT-04 | Support session write requires elevated role | Read role attempts write. | 403. |
| PT-05 | Platform admin cannot use staff middleware | Use platform token against staff routes. | 401. |

## 12. Operational ID Surface (matrix area: `ops-id`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| OPID-01 | Order operational id present in API | Place order, observe response. | Body contains `order_operational_id` formatted `<branch_code>-<YYYYMMDD>-<seq>`. |
| OPID-02 | Payment reference unique per branch-day | Generate 1000 payments in one branch-day. | All unique, contiguous, no gaps. |
| OPID-03 | Session number stable across reconnect | Note `session_number`, reconnect. | Same value. |
| OPID-04 | Support search by reference | Backend lookup by `order_operational_id` and `payment_reference`. | Returns the resource. |

## 13. Frontend Behavior Tests (matrix area: `frontend`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| F-01 | QR rescan path after stale session | Visit stale URL. | "Session ended" + scan CTA. |
| F-02 | Cart frozen UI during payment_pending | Enter payment_pending. | Add-to-cart button disabled. |
| F-03 | Status badges reflect order state | Walk an order through states. | Each state shown distinctly. |
| F-04 | Snapshot refresh on tab focus | Focus a backgrounded tab. | Snapshot fetch fires. |
| F-05 | Local cart cleared on credential revocation | Trigger revocation. | Local cart cleared. |
| F-06 | Staff token cookie scoped | After staff token migrated to cookie. | Cookie has `HttpOnly; Secure; SameSite=Lax`; not present on guest path. |

## 14. Negative / Adversarial Tests (matrix area: `adversary`)

| ID | Scenario | Steps | Assertions |
| --- | --- | --- | --- |
| X-01 | Forge guest token (bad signature) | Submit forged token. | 401. |
| X-02 | Forge JWT-like header but no signature | Use unsigned token. | 401. |
| X-03 | Send 10k requests with random session ids | Probe. | All 404/410. No 5xx. |
| X-04 | Send absurd body sizes | 50 MB JSON body. | 413 at gateway. |
| X-05 | Inject SQL fragments in display_name | Place orders / join sessions. | Stored verbatim, no execution. Postgres parameterized queries hold. |
| X-06 | XSS in display name | Place orders. | Frontend escapes. CSP would block any script anyway. |
| X-07 | CSRF against staff cookie (after cookie migration) | Cross-origin form post. | CSRF token absent -> 403. |
| X-08 | Replay snapshot URL after participant revoke | After revocation, refetch snapshot. | 401. |

## 15. Burn-In Linked Tests

These Playwright tests are also run during burn-in scenarios in `staging-burnin-strategy.md`:

| Burn-in scenario | Tests |
| --- | --- |
| S3 (reconnect storm) | R-01, R-05, R-08 |
| S4/S5 (Redis outage) | R-08, R-05, F-04 |
| S7 (pod restart storm) | L-05, O-06, R-05 |
| S8 (concurrent ordering) | O-03, O-04, O-08 |
| S9 (promo cap) | P-11 |
| S10 (concurrent payment) | P-02, P-04, P-08 |
| S11 (webhook replay) | W-02, W-07 |
| S14 (tenant fuzz) | T-01..T-06, X-03 |
| S15 (replay window) | R-06 |
| S16 (frontend crash) | O-03, P-07 |
| S17 (history pinball) | L-09, L-10 |

## 16. Coverage Goals

- Every state transition in `session-lifecycle-state-machine.md` has at least one test.
- Every invariant in `payment-finalization-invariants.md` has at least one test, with at least one positive and one negative case where applicable.
- Every delivery rule in `realtime-reconciliation-invariants.md` has at least one test.
- Every TODO/CONFIRM item in `security-hardening-checklist.md` has a verification test.

## 17. CI Integration

- Run `frontend/ ` matrix on every PR for the affected areas.
- Run full matrix nightly against staging.
- Run negative/adversarial matrix weekly with extended timeouts.
- A failing test blocks the merge that introduced it. A flake retry of N=2 is allowed, but a flake budget per area is tracked.

## 18. Out of Scope

- Visual regression testing (separate program if pursued).
- Performance testing (lives in `staging-burnin-strategy.md`).
- True browser-matrix testing on legacy Safari/IE (define list with product; not a security gate).

## 19. Author Guidance for the Implementer

- Use `request.fulfill` or a controlled backend mock only where the matrix explicitly asks ("test hook" wording in R-09). Otherwise drive the real backend in staging.
- Every test that mutates real state must clean up — close the session at the end, deactivate created staff, remove created promos.
- Use Playwright fixtures to reuse seeded data across tests in the same file.
- Capture screenshots and traces only on failure to keep artifact storage manageable.
- Run the matrix with `WORKERS=1` initially to debug ordering; tune to `WORKERS=auto` once stable.

## 20. Deliverables (when this matrix is implemented)

- A `frontend/playwright/` (or `e2e/`) directory with one spec file per row.
- A `playwright.config.ts` that supports staging and local dev.
- A CI workflow that publishes a per-area summary.
- A nightly aggregate report linked from the operations dashboard.
