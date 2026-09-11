# RC Manual Certification — Scope, Limits & Attention Plan

**Date:** 2026-08-04
**Branch under certification:** `feature/signoz-observability` @ `ecfc3a6`
(strict superset of `feature/certification-fixes-ui-redesign` @ `664d412`, the RC lineage)
**Schema:** v38 · **Routes:** 172 · **OpenAPI:** 2.2.0 / 171 operations
**Gate:** the last human gate before freezing the RC, merging to `main`, tagging `v1.0.0-rc.1`, and starting the soak.

> **Currency note (2026-08-22).** The *scope, limits and attention plan* below still stand — that is why this
> document is kept. Its header facts have moved: the branch tip is now `9865a48` (not `ecfc3a6`) and the
> schema is **v39** (not v38). For the current verdict and deployed-candidate provenance, read
> [`manual-certification-preflight-2026-08-22.md`](manual-certification-preflight-2026-08-22.md) —
> certification is presently **BLOCKED**.

Companions: `docs/manual-testing/manual-testing-user-guide.html` (teaching manual) ·
`docs/manual-testing/manual-testing-checklist.html` (197-item run sheet) ·
`docs/manual-testing/testing-dashboard.html` (live environment + credentials).

---

## 1. Why this gate exists

The standing SEV-0 is unchanged: **the Release Candidate has never been soaked.** The 124-hour R1 soak
that passed on 2026-06-04 ran the pre-redesign June binary. Everything since is unproven under sustained
load:

- ~18 certification bug fixes (Phase 0, C1–C20)
- migrations 000036 (promo-at-payment), 000037 (single-select modifiers), 000038 (serene default)
- the entire two-track redesign — **Serene** (guest) and **Harmony** (ops)
- the OpenTelemetry instrumentation on this branch (dark by default)

A soak is expensive in wall-clock time. This manual certification decides whether the RC is worth
soaking at all. Its output is a **verdict**, not a list of notes.

---

## 2. What will be tested

197 checklist items across 12 sections. Coverage by area:

| Section | Items | What it proves |
|---|---:|---|
| 0 · Environment sanity | 7 | The stack is real; the soak is untouched; docs match the database |
| 1 · Guest | 30 | QR entry, multi-guest shared cart, host model, ordering, bill, payment, reactivation |
| 2 · Kitchen | 10 | Order state machine, poll latency, legibility, branch scoping |
| 3 · Waiter | 15 | Serve, assistance, **settlement** (the only real payment path for pilot), lockout, revocation |
| 4 · Manager & Owner | 30 | All 12 admin tabs: menu, tables, staff RBAC, stats, promos, collateral, appearance, settings |
| 5 · Platform Admin | 30 | Onboarding wizard, org/branch lifecycle, plans, entitlements, flags, themes, collateral, RBAC |
| 6 · Support & Billing | 11 | Read-only forensics, credential-leak regression, shadow billing lifecycle |
| 7 · Flags & entitlements | 12 | The `entitlement AND flag` truth table for both gated features; loyalty ledger invariants |
| 8 · Cross-tenant isolation | 8 | The three trust domains; guest, staff and platform boundary enforcement |
| 9 · Failure & recovery | 25 | Redis/Postgres/backend/collector outages, concurrency, rate limits, webhook safety, audit immutability |
| 10 · Observability | 10 | Trace stitching, the audit→trace→log pivot, metric movement, zero-failure counters |
| 11 · Regression sweep | 9 | Re-prove the happy path after all the configuration you changed |

Severity distribution: **27 SEV-0 · 78 SEV-1 · 76 SEV-2 · 16 SEV-3.**

### Every role is covered
Guest · Participant vs Host · Waiter · Kitchen · Manager · Organization Owner ·
Platform Super Admin · Support Admin · Billing Admin · Read-only Auditor.

### Every feature flag is covered
The 9 env strict-rollout flags (`AUDIT_LOG_V2_ENABLED`, `TENANCY_ORGANIZATIONS_ENABLED`,
`AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`, `AUTH_STAFF_CODE_REQUIRED` +
`AUTH_STAFF_SESSION_DB_REQUIRED`, `WS_TICKET_AUTH_REQUIRED`, `AUTH_GUEST_CREDENTIALS_REQUIRED`,
`PAYMENT_STAFF_SETTLEMENT_REQUIRED`) plus `OTEL_ENABLED`, and the 2 database-driven platform product
flags (`staff_performance_analytics`, `loyalty`). Each is documented individually in the user guide with
purpose, default, rollout status, code paths, on/off behaviour, pilot safety, dependencies, caveats,
metrics, and a manual verification procedure.

> **Flags that must NOT be flipped during this run:** R2 (`TENANCY_ORGANIZATIONS_ENABLED`),
> R3 (the authz pair) and R7 (`PAYMENT_STAFF_SETTLEMENT_REQUIRED`). They are deferred behind
> metric gates. Testing them means testing the *off* path, and confirming the shadow/inert
> behaviour is what actually ships.

### Every entitlement is covered
All 13 catalog entries, the plan matrix (verified live against the seeded database), the resolution
order, and 7 worked examples including the two cases most people get wrong (entitlement-alone and
flag-alone must both leave the feature **off**).

---

## 3. What cannot be tested manually — record these as N/A, not as passes

| Area | Why it is out of scope here | Where it must actually be proven |
|---|---|---|
| **Sustained load, memory and storage curves** | Manual testing is minutes; leaks and growth surface over days | **The RC soak.** This is the SEV-0 and nothing here substitutes for it |
| **Split-origin production edge** | Everything is localhost — no Cloudflare Worker, no nginx, no TLS, no real domain | Production rehearsal: CORS, CSP `connect-src`, WSS proxying, a printed QR scanned from a phone on cellular |
| **Real payment gateway** | None is integrated; the webhook endpoint is generic by design | Post-pilot, behind the existing webhook abstraction. Pilot payments are cash / card-manual / UPI-staff-confirmed |
| **Real alert delivery** | Alertmanager's receiver is still a placeholder webhook sink | SEV-1 before day 1: wire one real app-down/`/readyz` page to a phone |
| **Image upload to R2** | R2 is unconfigured; endpoints return 503 **by design** | A production environment with R2 credentials |
| **Platform MFA end-to-end** | `MFA_ENCRYPTION_KEY` is unset, so enrolment fails closed | Set the key and re-test, or defer to the production checklist |
| **Backup / restore** | Not wired to the manual-testing stack | Already certified byte-faithful against the real production R2 bucket on 2026-07-18 |
| **Multi-instance WS at scale** | Two instances prove Redis fan-out routing, not hub sharding | Deferred by decision; revisit around 50 restaurants |
| **R2 / R3 / R7 enforcement behaviour** | Those flags are off and must stay off | Their own rollout waves, each with a metric gate |
| **Printed-QR durability** | Print quality, lamination and restaurant lighting are physical variables | A physical rehearsal with the real printer and the real tables |
| **Concurrency beyond ~50** | Load-tested separately: p99 ~200ms at concurrency 50; a known `session_sequences` write hot-spot produces ~5% 5xx at ~150 | Already measured; far beyond pilot scale |

**Two additional caveats specific to this environment:**

1. **Tracing is ON here** (`OTEL_ENABLED=true`, sample ratio 1.0) so that section 10 is testable.
   Production default is `false`, and the VM deploy is blocked pending an explicit Phase 0 sizing GO.
   Do not treat "tracing worked in certification" as authorisation to enable it in production.
2. **The environment carries pre-existing state.** Saffron Bandra's three tables are occupied by
   `payment_pending` sessions from earlier smoke runs. That is correct behaviour (a table awaiting
   settlement must never be auto-abandoned) but it blocks fresh session creation on that branch.
   Either use the other branches or run `./scripts/manual-testing-up.sh --reset` — which regenerates
   every QR token, so refresh the dashboard afterwards.

---

## 4. Recommended testing duration

| Phase | Effort | Notes |
|---|---|---|
| 0 · Environment sanity | 0.2 h | Do not skip; a half-up stack invalidates everything after it |
| 1 · Guest | 1.0 h | Needs **two devices**. The only role that creates data |
| 2 · Kitchen | 0.5 h | Includes a 20-minute soak of the KDS tab (run it in the background) |
| 3 · Waiter | 0.75 h | Includes concurrent-settlement and lockout tests |
| 4 · Manager & Owner | 1.5 h | The largest surface: 12 tabs |
| 5 · Platform | 1.5 h | Includes a full end-to-end onboarding of a 4th tenant |
| 6 · Support & Billing | 0.5 h | |
| 7 · Flags & entitlements | 1.0 h | Truth tables are slow but high-value |
| 8 · Cross-tenant | 0.75 h | Mostly curl; fast once you have tokens captured |
| 9 · Failure scenarios | 1.5 h | Includes two ~15-minute waits (escalation, session expiry) — start them early and interleave |
| 10 · Observability | 0.5 h | |
| 11 · Regression sweep | 0.75 h | |
| **Total** | **~10.5 h** | |

**Realistic scheduling:**

- **Two focused days** (~5 h/day) for one person. Do not attempt it in one sitting — fatigue after
  hour six produces generous passes, which is worse than not testing.
- **One day with two people** — one on a phone as the guest, one on a laptop as staff. This is the
  better shape: realtime latency and the host model are only properly felt with two humans in the
  same room, and it roughly halves the wall-clock.
- Add **0.5–1 h** for the physical print-and-scan test (PLT-21) if the printer is available.
- Add contingency for triage: expect to spend ~30 min per genuine SEV-0/SEV-1 finding writing it up
  properly. Budget 2 h of slack.

**Do not compress by skipping section 11.** The regression sweep historically catches more real defects
than any other section, because by then you have changed themes, flags, plans, menus and timeouts.

---

## 5. Expected issues — what previous rounds and the code suggest you will find

These are predictions, not known defects. Finding none of them would itself be informative.

### Likely (I would be surprised if none of these appear)

1. **Cosmetic/layout defects in the redesign at mobile viewport.** Serene and Harmony have never been
   through a hostile mobile pass on real hardware. Expect clipped modals, tight tap targets, or
   overflow on the item-detail dialog and the collateral studio.
2. **Empty/disabled-state polish on gated surfaces.** The Performance and Loyalty tabs default off;
   whether they present as *cleanly unavailable* versus *broken* is exactly the sort of thing that
   reads as a bug to a restaurant owner.
3. **Kitchen poll latency feeling slow.** 10s is by design, but measure it. If the observed worst case
   is meaningfully above 10s, that is a real finding rather than a design consequence.
4. **Analytics empty-range rendering.** Charts on ranges with no data are a classic NaN/blank source.
5. **Collateral print fidelity.** Margins, bleed and QR safe-area on at least one of the five formats.
6. **Divergence between the two collateral surfaces** (platform studio vs admin tab) — flagged as
   known accidental debt in the handbook, never systematically diffed.

### Possible (worth specific attention)

7. **Joinability vs the one-active-session-per-table constraint** (GST-30). This was certification
   finding #11; the two must stay aligned and the simultaneous-scan race is the way to expose it.
8. **Host-transfer edge cases** — transfer while the old host is mid-submit, or after the host's
   browser has closed entirely (FAI-25). The "host walked out" recovery path is the one I would most
   expect to be under-specified.
9. **Rate-limit UX.** Six different per-key caps exist. At least one probably surfaces a raw 429
   rather than friendly copy.
10. **Promo timezone edges.** A timezone bug was found and fixed during certification; the boundary
    conditions around window start/end in Asia/Kolkata deserve re-probing.
11. **Cache-invalidation latency on menu edits.** The menu is Redis-cached; whether a price change
    invalidates immediately or waits out the TTL should be observed and recorded, not assumed.

### Unlikely but catastrophic — check anyway

12. **Bill snapshot mutability** (GST-23, ADM-21). If the frozen bill ever moves, the entire
    payment-correctness model is void. This is the single highest-value test in the document.
13. **Payment escalation mutating state** (FAI-10). Alert-only is a design commitment. Any state
    change here is SEV-0.
14. **Guest credential leakage in support payloads** (SUP-03). Finding F-8 was exactly this; it is
    fixed, and this is its regression test.
15. **Cross-tenant leakage** (section 8). Commercially the most damaging failure class in the product.
16. **Unplanned enforcement** (PLT-06, PLT-07, PLT-13). Governance is supposed to be inert. If
    suspending an org actually blocks traffic, an accidental click could take a paying restaurant
    offline. Confirming it does *nothing* is a real test with a real failure mode.

---

## 6. Things to pay special attention to

**The five that decide the verdict:**

1. **Bill snapshot immutability.** Freeze a bill, then change prices and deactivate the promo behind
   it. Nothing may move.
2. **Payment escalation is alert-only.** Leave a payment unsettled past 15 minutes and confirm the
   alerts fire while payment and session status stay exactly where they were.
3. **The three trust domains never mix.** Staff token → platform = 401. Guest A token → session B =
   refused. Saffron staff → Copper Pot branch = refused. Verify at least these three by curl, not
   through the UI.
4. **Audit immutability.** `UPDATE` and `DELETE` against `audit_log` must both raise the trigger.
5. **The regression sweep passes.** Everything above is worthless if your own configuration changes
   broke the guest flow and you did not re-check.

**Behaviours that look like bugs but are deliberate — do not file these:**

- Staff and kitchen poll REST every 10 s; there is no staff WebSocket.
- Org and branch suspension, subscription status, invoices and entitlement *limits* are all recorded
  and audited but **not enforced**.
- R2 image uploads return 503 (R2 unconfigured).
- `audit_log.row_hash` is always NULL (hash-chain deferred).
- `db_query_duration_seconds`, `db_errors_total` and `redis_ops_total` are declared but never
  incremented — OTel spans replace them.
- The PresenceExpiry worker is a stub that only debug-logs.
- Managers cannot create staff (owner-only) — certified as intended RBAC.
- Promos apply at payment initiation, not at order placement (migration 000036).
- There is no first-super-admin bootstrap UI; seeding is a one-shot CLI in the production image.
- No WebSocket, worker or outbound-HTTP spans exist in tracing.

**Discipline for the run:**

- **Mark N/A honestly.** Eleven areas genuinely cannot be certified here. An honest N/A is worth more
  to the release decision than a generous pass, and a false pass on a SEV-0 is how a pilot fails in
  front of a customer.
- **Record actual measured numbers** where the checklist asks for them (kitchen poll latency, feature
  gate propagation, restart recovery time). "Felt fine" is not evidence.
- **Never touch the soak stack** (compose project `qr-dining`). ENV-05 and REG-08 bracket the entire
  run for exactly this reason.
- **Write the verdict** (REG-09). A checklist without a decision is just notes.

---

## 7. Exit criteria

| Verdict | Condition |
|---|---|
| **GO** — freeze the RC, merge, tag, soak | Zero SEV-0 failures. SEV-1 failures either zero or each with an agreed fix landing before the soak build is cut. |
| **CONDITIONAL GO** | Zero SEV-0. Open SEV-1s exist but are documented, owned, and provably not exercised by the soak workload. State explicitly what the soak does *not* cover as a result. |
| **NO-GO** | Any SEV-0 failure, or a cluster of SEV-1s in one subsystem suggesting the redesign destabilised it. Fix, then re-run at minimum sections 1, 3, 7 and 11. |

After a GO, the release sequence is unchanged from the handbook's Part 15:
merge `--no-ff` → `main`, tag `v1.0.0-rc.1`, build the soak binary **from the tag** and **off `/tmp`**,
run the soak with R1 and R4–R6 enabled plus continuous traffic and external probing, and wire one real
app-down alert before Restaurant #1.

---

## 8. Artefacts this run should produce

1. `rc-certification-results.md` — exported from the checklist (failures ranked by severity, N/A list,
   unanswered list).
2. `rc-certification-raw.json` — the raw run state, for the record.
3. Screenshots of every failure, filed alongside `release-certification/screenshots/`.
4. The written verdict, appended to this document or filed as
   `release-certification/rc-certification-verdict.md`.
