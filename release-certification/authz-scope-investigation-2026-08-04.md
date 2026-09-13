# Authorization Scope Bypass — Independent Investigation & Fix

**Date:** 2026-08-04
**Scope:** §2 of the removed report retained at
`docs-before-rebuild:release-certification/independent-audit-2026-08-04.md`
("BLOCKER — Cross-organization and cross-branch write bypass")
**Base commit:** `feature/signoz-observability` @ `ecfc3a6`
**Verdict:** **VALID — FIXED**

> This is retained historical evidence because
> `backend/internal/handlers/authz_scope_integration_test.go:38` cites it. Its
> pre-fix source locations and live-run results describe commit `ecfc3a6`, not
> the current worktree; use `docs/SECURITY.md` for current behavior.

**Isolation:** all live testing ran against a throwaway Postgres (`:15499`) and Redis
(`:16399`) created for this investigation, with the backend on `:18499`. The protected
soak (`qr-app-soak`, `qr-dining-postgres-1`) and the manual-testing stack
(`:8090`/`:8095`/`:25432`/`:26379`) were **untouched**. One incidental confirmation of
that: an early `pkill -x qrapp` matched the soak container's `/qrapp` and was refused with
`Operation not permitted`; the harness was then rewritten to target only a recorded PID
file so no system-wide pattern match could reach it.

---

## 1. Executive summary

The audit's finding is **valid**. Cross-organization and cross-branch reads, writes and
deletes all succeed under the shipped default configuration. I reproduced every claim and
found **25 leaking operations** across 15 endpoints, including two the audit did not
report.

The audit's *stated root cause is incomplete*, and the incompleteness matters, because it
invalidates the audit's proposed fix:

- The audit attributes every leak to `AUTHZ_CENTRAL_POLICY_ENFORCE=false` and recommends
  flipping the flag.
- **`GET /sessions/{id}/events` has no authorization check of any kind** — it never reads
  the staff session and never calls the authorizer. I ran the matrix with the flag on:
  25 of 27 rows closed, and that route still returned **200** with another organization's
  private event payload, in both the cross-org and cross-branch case.
- `PATCH /staff/{id}/deactivate` gates on role but not on branch, so a foreign
  organization's owner could deactivate any staff member anywhere and invalidate their
  tokens. The audit's table did not list this.

So the answer to "does flipping the flag fix it" is **no** — proven empirically, not
argued.

The fix makes tenant isolation independent of the rollout flag entirely, in ~146 lines
across 9 files. No flag defaults changed, no architecture changed, the R3 rollout wave is
intact, and audit logging is strictly better than before.

---

## 2. Was the finding valid?

| Audit claim | Verdict | Note |
|---|---|---|
| Cross-organization writes possible | **Confirmed** | Menu items/categories/modifiers renamed, 86'd, featured, and **deleted** across orgs |
| Cross-branch writes possible | **Confirmed** | Sibling branch of the same org — the case that bites a single-tenant pilot |
| Foreign staff token modifies another org's resources | **Confirmed** | Saffron/Bandra owner mutated Copper Pot objects end to end |
| Foreign session's events readable | **Confirmed** | And **not** caused by the flag — see §4.2 |
| Root cause is the shadow-mode flag | **Partially correct** | True for 23 of 25 leaks; two routes are not flag-gated at all |
| Some handlers rely solely on the middleware | **Confirmed** | 15 handlers had no independent actor-scope check |
| Policy engine itself is correct | **Confirmed** | Computes every denial accurately and fails closed on zero scope |
| Role separation is intact | **Confirmed** | Waiter correctly 403'd on menu admin, at the service layer, flag-independently |
| Branch-param routes are safe | **Confirmed, different mechanism** | See §4.3 — not the middleware the audit credits |

Two corrections to the audit's supporting detail:

1. **`enforceBranchScopeFromBody` is irrelevant here**, as the audit says — but so is
   `STRICT_BRANCH_SCOPED_MUTATIONS`, its flag. The audit's fix recommends flipping both;
   the second one changes nothing about this vulnerability.
2. **`PATCH /payments/{id}/settle` was never exploitable.** The audit did not claim it was,
   but it is adjacent to routes that were. `SettlePaymentByStaff`
   (`internal/services/payment.go`) re-checks `payment.BranchID != branchID` against the
   *actor's* branch and fails closed with `ErrPaymentNotFound`. It returned 404, not a
   successful settlement.

---

## 3. Evidence

### 3.1 Static — the bypass in code

`backend/internal/handlers/authz.go:47-99` (pre-fix). `requireAuthorized` computes the
decision, records it, and then:

```go
enforced := authorizer.Enforce()
...
if !enforced {
    // metrics ...
    return true          // <-- request proceeds
}
```

`Enforce()` returns `AUTHZ_CENTRAL_POLICY_ENFORCE`, which defaults to `false`
(`backend/internal/config/config.go:258`) and ships `false`
(`deploy/vm/.env.production.example:52`).

The policy engine is correct — `backend/internal/authz/policy.go:56-69` — and `Scope.SameBranch` /
`SameOrganization` fail closed on a zero scope (`backend/internal/authz/scope.go:11-17`).

The affected handlers pass the **resource's own** branch into the service call, e.g.
`h.svc.ToggleAvailability(ctx, itemID, target.BranchID, ...)`, so the SQL scope is a
tautology (`WHERE id=$1 AND branch_id=$2` where `$2` came from the row itself) and adds no
protection.

### 3.2 Live — reproduced attack matrix

Three tenants seeded: **Saffron House** (org 1) with branches Bandra (1) and Indiranagar
(2), and **Copper Pot Kitchen** (org 2) with Koramangala (3). Actor tokens are all
Saffron/Bandra staff.

Baseline set in SQL, mutated only through the API:

```
BEFORE  menu_items.3      is_available=t  is_featured=f   name='B1 Dish'      (org 2)
        menu_categories.3 name='B1 Starters'                                  (org 2)
        staff.7           is_active=t                                         (org 2)

PATCH /menu/items/3/availability  -> 204     DELETE /menu/items/3       -> 204
PATCH /menu/items/3/featured      -> 204     DELETE /menu/categories/3  -> 204
PATCH /menu/categories/3          -> 200     PATCH  /staff/7/deactivate -> 204

AFTER   menu_items.3      DELETED
        menu_categories.3 DELETED
        staff.7           is_active=f
```

The event-log read returned another organization's private payload:

```json
[{"session_id":"5c0ce883-...","branch_id":3,"event_type":"session.created",
  "payload":{"secret":"copper-pot-private"}}, ...]
```

— including, with some irony, the `AUTHZ_DENIED` row the system had just written *about the
attacker*.

**Pre-fix totals:** 25 of 27 attack rows leaked with the flag off; 2 of 27 still leaked
with the flag on.

---

## 4. Root cause

### 4.1 Primary — shadow mode allows the request (configuration + implementation)

`requireAuthorized` returning `true` on a denial is intentional shadow-mode behaviour for
the R3 rollout. The design error is that it applies the same shadow treatment to two very
different classes of denial:

- **Role policy** — "a waiter may not edit the menu". Plausibly wrong in an unmigrated
  deployment; genuinely benefits from a shadow week before enforcement.
- **Tenant scope** — "this actor's branch does not own this resource". *Never* legitimate
  traffic. There is nothing to learn from letting it through.

Collapsing both into one flag means the multi-tenancy boundary ships off by default.

### 4.2 Secondary — `GET /sessions/{id}/events` had no authorization at all

`backend/internal/handlers/event_log.go:22-36` (pre-fix) parsed the session UUID and returned the
event log. It never called `middleware.GetStaffSession`, never called `requireAuthorized`.
Any valid staff token from any organization read any session's timeline.

This is the finding that breaks the audit's recommendation, and it is why "flip the flag
and re-run the matrix" would have produced a green-looking run with a hole still in it.

### 4.3 Contributing — the safety of branch-param routes is not where the audit says

The audit credits `/branches/{id}/...` routes with "a second, independent per-handler
ownership check". That is right, but it also implies `BranchTenantGuard` is doing work.
It is not: `backend/internal/middleware/branch_guard.go:21-25` returns early and does nothing
when no tenant is resolved from the request, which is the case whenever `BASE_DOMAIN` is
unset — including every environment in this repo today. The 403s come entirely from
per-handler `sess.BranchID != branchID` comparisons. Item-scoped routes simply never got
that line.

### Answers to the seven questions

| # | Question | Answer |
|---|---|---|
| 1 | Exact root cause | Shadow-mode `requireAuthorized` returns `true` for tenant-scope denials; **plus** one route with no authz at all and one with no branch check |
| 2 | Configuration, implementation, or both | **Both.** The flag default is configuration; `GetSessionEvents` and `DeactivateStaff` are implementation defects that no configuration could close |
| 3 | Endpoints affected | 15 — see the matrix in §5 |
| 4 | Roles affected | All staff roles. Reach is bounded by `roleAllowed()`: owner/manager for menu admin, owner for staff deactivation, waiter+ for order/assistance. Role separation itself was never broken |
| 5 | Vulnerability class | Cross-branch **and** cross-organization; **read, write, and delete**. Not cross-tenant in the platform sense — `platformAPI` is a separate trust domain and was never affected |
| 6 | Feature flags involved | `AUTHZ_CENTRAL_POLICY_ENFORCE` is the gate. `STRICT_BRANCH_SCOPED_MUTATIONS` is **not** — `enforceBranchScopeFromBody` compares a client-supplied body value against the resource, never against the actor |
| 7 | Does the flag alone fix it? | **No — proven.** With `=true`, 25 of 27 rows closed and `GET /sessions/{id}/events` still returned 200 cross-org and cross-branch |
| 8 | Should handlers validate actor scope? | **Yes.** Implemented — see §6.2 |

---

## 5. Security matrix

Actor: Saffron House / Bandra (branch 1, org 1). Targets: branch 2 = sibling branch, same
org; branch 3 = Copper Pot, different org. "Before" = `ecfc3a6`, flag off (shipped
default). "Flag on" = `ecfc3a6` with `AUTHZ_CENTRAL_POLICY_ENFORCE=true`. "After" = fixed
build, **identical at both flag settings**.

### A. Cross-organization

| Endpoint | Method | Actor role | Expected | Before | Flag on | After |
|---|---|---|---|---|---|---|
| `/menu/items/{id}` | PATCH | owner | Denied | **200 LEAK** | 403 | **403** |
| `/menu/items/{id}/availability` | PATCH | owner | Denied | **204 LEAK** | 403 | **403** |
| `/menu/items/{id}/featured` | PATCH | owner | Denied | **204 LEAK** | 403 | **403** |
| `/menu/items/{id}` | DELETE | owner | Denied | **204 LEAK** | 403 | **403** |
| `/menu/categories/{id}` | PATCH | owner | Denied | **200 LEAK** | 403 | **403** |
| `/menu/categories/{id}` | DELETE | owner | Denied | **204 LEAK** | 403 | **403** |
| `/menu/items/{id}/modifiers` | POST | owner | Denied | **201 LEAK** | 403 | **403** |
| `/menu/modifiers/{id}` | PATCH | owner | Denied | **200 LEAK** | 403 | **403** |
| `/menu/modifiers/{id}` | DELETE | owner | Denied | **204 LEAK** | 403 | **403** |
| `/orders/{id}/status` | PATCH | waiter | Denied | **200 LEAK** | 403 | **403** |
| `/assist/{id}/ack` | PATCH | waiter | Denied | **200 LEAK** | 403 | **403** |
| `/assist/{id}/resolve` | PATCH | waiter | Denied | **200 LEAK** | 403 | **403** |
| `/staff/{id}/deactivate` | PATCH | owner | Denied | **204 LEAK** | 403 | **403** |
| `/sessions/{id}/events` | GET | owner | Denied | **200 LEAK** | **200 LEAK** | **403** |
| `/payments/{id}/settle` | PATCH | waiter | Denied | 404 (denied) | 403 | **403** |

### B. Cross-branch, same organization

| Endpoint | Method | Actor role | Expected | Before | Flag on | After |
|---|---|---|---|---|---|---|
| `/menu/items/{id}` | PATCH | manager | Denied | **200 LEAK** | 403 | **403** |
| `/menu/items/{id}/availability` | PATCH | manager | Denied | **204 LEAK** | 403 | **403** |
| `/menu/items/{id}/featured` | PATCH | manager | Denied | **204 LEAK** | 403 | **403** |
| `/menu/items/{id}` | DELETE | manager | Denied | **204 LEAK** | 403 | **403** |
| `/menu/categories/{id}` | PATCH | manager | Denied | **200 LEAK** | 403 | **403** |
| `/menu/categories/{id}` | DELETE | manager | Denied | **204 LEAK** | 403 | **403** |
| `/menu/modifiers/{id}` | PATCH | manager | Denied | **200 LEAK** | 403 | **403** |
| `/orders/{id}/status` | PATCH | waiter | Denied | **200 LEAK** | 403 | **403** |
| `/assist/{id}/ack` | PATCH | waiter | Denied | **200 LEAK** | 403 | **403** |
| `/staff/{id}/deactivate` | PATCH | owner | Denied | **204 LEAK** | 403 | **403** |
| `/sessions/{id}/events` | GET | owner | Denied | **200 LEAK** | **200 LEAK** | **403** |
| `/payments/{id}/settle` | PATCH | waiter | Denied | 404 (denied) | 403 | **403** |

### C. Controls — must not change

| Check | Expected | Before | After |
|---|---|---|---|
| Waiter edits own-branch menu (role separation) | Denied | 403 | **403** |
| `/branches/3/menu/full` (foreign branch param) | Denied | 403 | **403** |
| `/branches/3/staff` | Denied | 403 | **403** |
| `/branches/3/audit` | Denied | 403 | **403** |
| `/branches/3/tables` | Denied | 403 | **403** |
| `/branches/3/orders/active` | Denied | 403 | **403** |

### D. Legitimate same-branch operations — must keep working

| Operation | Expected | After (flag off) | After (flag on) |
|---|---|---|---|
| Owner updates own menu item | Allowed | **200** | **200** |
| Owner toggles own availability | Allowed | **204** | **204** |
| Manager toggles own featured | Allowed | **204** | **204** |
| Manager updates own category | Allowed | **200** | **200** |
| Owner adds own modifier | Allowed | **201** | **201** |
| Waiter reads own branch menu | Allowed | **200** | **200** |
| Owner reads own branch staff roster | Allowed | **200** | **200** |
| Owner reads own branch audit log | Allowed | **200** | **200** |
| Indiranagar owner edits Indiranagar item | Allowed | **200** | **200** |
| Indiranagar owner reads own session events | Allowed | **200** | **200** |
| Copper Pot owner edits Copper Pot item | Allowed | **200** | **200** |
| Copper Pot owner reads own session events | Allowed | **200** | **200** |
| Copper Pot owner deactivates own waiter | Allowed | **204** | **204** |

### E. Platform operator — capabilities must be retained

`platformAPI` sits behind `middleware.PlatformAuth` in a separate trust domain and was not
touched by this change. Verified with a real platform super-admin token:

| Endpoint | Expected | After |
|---|---|---|
| `/platform/sessions/{id}` (org B session) | Allowed | **200** |
| `/platform/orders/{id}` (org B order) | Allowed | **200** |
| `/platform/organizations` | Allowed | **200** |
| `/platform/organizations/2/entitlements` | Allowed | **200** |
| `/platform/analytics/usage` (cross-tenant) | Allowed | **200** |
| `/platform/analytics/health` (cross-tenant) | Allowed | **200** |
| `/platform/branches/3/flags` | Allowed | **200** |

**Totals — fixed build:** 33/33 attack rows denied and 13/13 legitimate operations
allowed, at **both** flag settings. Post-run DB inspection confirmed every targeted row
still at its seeded baseline: nothing renamed, nothing deleted, no staff deactivated, no
order or assistance status changed.

---

## 6. Code changes

146 lines across 9 files. No feature flag defaults changed, no routes changed, no
signatures changed, no refactoring.

### 6.1 Central: tenant scope violations are non-bypassable

**`internal/authz/policy.go`** (+7) — new `Decision.ScopeViolation` field, set `true` in
the two existing scope branches. The decision logic itself is untouched.

**`internal/handlers/authz.go`** (+5 in `requireAuthorized`):

```go
enforced := authorizer.Enforce() || decision.ScopeViolation
```

*Why this and not a flag flip:* it closes all 23 flag-gated leaks in one place rather than
23 places, and it keeps the R3 rollout gate pointed at what it was actually built to
measure. Role-policy denials still shadow, still increment `legacy_authz_bypass_total` and
`policy_shadow_mismatch_total`, and still write an audit row with `enforced: false`. The
operator's pre-flip signal is unchanged in meaning; it simply no longer conflates
"a working flow will break" with "someone is reaching into another tenant".

*Why it is safe:* a staff session is pinned to exactly one branch, and every call site
supplies a resource scope derived from the resource itself. `Scope.SameBranch` /
`SameOrganization` fail closed on zero, so an unresolved scope denies rather than matching
another zero — pinned by a new unit test.

### 6.2 Defense in depth: explicit actor-scope checks

**`internal/handlers/authz.go`** (+15) — new `requireActorBranch(c, sess, resourceBranchID)`
helper, mirroring the idiom already at `tables.go:160` and `staff.go:336`.

Called in 15 handlers (one line each), **after** `requireAuthorized`:

- `menu_admin.go` (+27) — `UpdateItem`, `ToggleAvailability`, `ToggleFeatured`,
  `DeleteMenuItem`, `UpdateMenuCategory`, `DeleteMenuCategory`, `AddItemModifier`,
  `UpdateItemModifier`, `DeleteItemModifier`
- `order.go` (+3) — `UpdateStatus`
- `payment.go` (+3) — `Settle`
- `assistance.go` (+6) — `Acknowledge`, `Resolve` (against `session.BranchID`)
- `staff.go` (+6) — `RotatePIN`, `DeactivateStaff`

*Why after and not before:* placing the guard first would short-circuit
`requireAuthorized`, so no `AUTHZ_DENIED` audit row and no metrics would be written for a
cross-tenant attempt — a net **loss** of audit signal on exactly the events most worth
recording. Placed second, the central path still audits and denies, and the explicit check
is the backstop for the case where it wrongly allows. The plan called for placing it after
the resource fetch; this ordering was chosen instead to satisfy the "preserve audit
logging" constraint.

`customer.go` is organization-scoped rather than branch-scoped and stays on the central
guarantee (`requiresSameOrganization` covers `ActionCustomerHistory` / `ActionCustomerDelete`).

One behaviour change worth naming: `PATCH /payments/{id}/settle` on a foreign payment now
returns **403 instead of 404**. It was never exploitable, but it is now consistent with
every neighbouring route, all of which already returned 403 — so no new information is
disclosed.

### 6.3 `GET /sessions/{id}/events` — the route with no authorization

**`internal/handlers/event_log.go`** (+16) — `GetSessionEvents` now reads the staff
session, loads the session via `h.repos.GetSessionByID`, and returns 403 when
`session.BranchID != sess.BranchID`. This is the exact shape of its sibling
`GetBranchRecentEvents` in the same file. No constructor or wiring change, so no
`server.go` churn.

This one is flag-independent by construction, which is the whole point: it was never
flag-gated, so no configuration change could have closed it.

### 6.4 Feature flag defaults — deliberately unchanged

`AUTHZ_CENTRAL_POLICY_ENFORCE` and `STRICT_BRANCH_SCOPED_MUTATIONS` remain `false` in
`config.go` and `deploy/vm/.env.production.example`. Tenant isolation no longer depends on
them. The R3 rollout wave keeps its shadow week for the role-policy question it was
designed to answer.

---

## 7. Regression protection

### New — `internal/handlers/authz_scope_integration_test.go`

Two independent tenants (two `testutil.SeedFixtures` calls, which generate a fresh slug
each time), plus a third branch under tenant A's org for the sibling-branch case.

- **`TestCrossTenantWritesAreDeniedRegardlessOfEnforcementFlag`** — 10 handlers × 2 scopes
  (cross-org, cross-branch) × 2 flag settings = **40 cases**, all asserting 403. The flag
  dimension is the point: it is what stops someone reading "the flag is off" and
  reintroducing shadow-mode scope handling.
- **`TestSameBranchOperationsStillSucceed`** — positive control at both flag settings, and
  it asserts the returned event timeline is non-empty so the control cannot pass vacuously.
- **`TestCrossTenantWritesLeaveResourcesUntouched`** — snapshots a foreign menu item,
  fires four mutating requests at it with the flag off, and asserts the DB row is
  byte-identical afterwards. This distinguishes a real refusal from a 403 returned after
  the write already landed.

**Verified non-vacuous:** with the fix stashed, the suite fails 20 subtests immediately at
`enforce=false`, and `session_events_read` fails at `enforce=true` as well. Restored, all
pass.

### Updated — `internal/authz/policy_test.go`

- `TestScopeViolationIsSetOnlyForTenantScopeDenials` — asserts `ScopeViolation` is true for
  branch and org mismatch, **false** for role denials, and false for allowed decisions.
  Mislabelling in either direction is a security bug, so both directions are pinned.
- `TestScopeViolationFailsClosedOnZeroScope` — guards the zero-value semantics of
  `Scope.SameBranch` / `SameOrganization`.

### Updated — `docs-before-rebuild:docs/manual-testing/audit-added-checks-2026-08-04.md`

§8 rows 8.9–8.15 now carry a FIXED status note, and the verification instruction changed
from "re-run with the flag on; every row must become 403" to "run **twice**, flag off and
flag on; every row must be 403 in **both**". Check 10.12 was corrected: since scope
denials are now enforced, `legacy_authz_bypass_total` tracks role-policy shadow bypasses
only, and scope denials land in `authz_denied_total{reason="branch_mismatch"|"org_mismatch"}`.

### Test results

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test -count=1 -p 1 -tags integration ./...` | **12/12 packages ok, 0 skips** |
| `go test -count=1 -p 1 -race -tags integration ./...` | **12/12 packages ok** |

Matches the audit's own baseline, with the new tests added.

### Audit trail and metrics — verified preserved

With `AUDIT_LOG_V2_ENABLED=true` and the enforcement flag **off**, after the attack matrix:

```
 result  | count |                   reason                    | enforced |  role
---------+-------+---------------------------------------------+----------+---------
 denied  |    11 | actor branch does not match resource branch | true     | owner
 denied  |     7 | actor branch does not match resource branch | true     | manager
 denied  |     7 | actor branch does not match resource branch | true     | waiter
 success |     1 | role is not allowed for action              | false    | waiter
```

Scope violations are recorded as **denied / enforced: true** even with the flag off; the
role denial still records as **success / enforced: false**, preserving the R3 signal.
Metrics agree: `authz_denied_total{reason="branch_mismatch"} 25`,
`legacy_authz_bypass_total{...,actor_role="waiter"} 1`,
`policy_shadow_mismatch_total{reason="role_not_allowed",...} 1`.

---

## 8. Risk assessment

**Pre-fix severity: SEV-0** for any deployment with more than one branch or more than one
tenant in a database.

- **Confidentiality** — another organization's full session event timeline, readable with
  any staff token. Unauthenticated attackers are not in scope; a compromised or malicious
  waiter account at any tenant is.
- **Integrity** — menu items and categories renamed, re-priced, 86'd, and **deleted**
  across organizations. Order status and assistance requests driven to arbitrary states.
- **Availability** — staff accounts deactivated across organizations, invalidating their
  tokens. In a live service that is a shift-stopping event at a restaurant the attacker
  does not own.
- **Detection** — the system *did* record every one of these as `AUTHZ_DENIED`, so a
  post-incident investigation would have had the evidence. Nobody was reading it: no
  alert consumes `legacy_authz_bypass_total`, and it was recorded with
  `result: success`.

**Exploitability:** trivial. A valid staff token and an integer ID. IDs are sequential and
small, so enumeration is not a barrier.

**The audit's scoping caveat is correct and worth repeating:** if Restaurant #1 is a
single branch alone in its database, this was not exploitable and did not block. Any
second branch — which the pilot restaurant may well add — or any second tenant on the same
instance made it an immediate SEV-0.

**Post-fix residual risk: low.**

- Tenant isolation is now enforced at two independent layers, neither flag-dependent.
- Remaining exposure is the ordinary one: a new item-scoped route added later that forgets
  both layers. The regression suite covers today's surface but cannot cover a route that
  does not exist yet. The durable mitigation is that `requireAuthorized` is now
  fail-closed on scope by default, so a new handler that merely calls it inherits the
  protection.
- `RotatePIN` remains reachable cross-branch by an owner *who already knows the target's
  current PIN* — the service requires it. Now closed by `requireActorBranch` regardless.

**This finding no longer blocks Restaurant #1.** The other blockers in the independent
audit — no deployable artifact, no environment, CI never run, RC unmerged and unsoaked —
are untouched by this work and still stand.

---

## 9. Final conclusion

# VALID — FIXED

The audit was right that this was real and right that it was SEV-0. It was wrong about the
fix: flipping `AUTHZ_CENTRAL_POLICY_ENFORCE` would have closed 25 of 27 attack rows and
left `GET /sessions/{id}/events` serving another organization's private data, while
producing a matrix that looked clean if only the endpoints in §2's table were re-tested.

Tenant isolation is now independent of the rollout flag. Verified live at both flag
settings: 33/33 attacks denied, 13/13 legitimate operations allowed, zero mutation of
targeted rows, platform operator capabilities retained, audit trail and R3 rollout signal
preserved, full integration suite green with and without `-race`.

---

## Appendix — artifacts

- `/tmp/qrd-authz/matrix.sh` — attack matrix (33 rows), reproducible
- `/tmp/qrd-authz/legit.sh` — legitimate-operation control (13 rows)
- `/tmp/qrd-authz/matrix-BEFORE-flagoff.txt` — pre-fix, shipped default: 27 deviations
- `/tmp/qrd-authz/matrix-BEFORE-flagon.txt` — pre-fix, flag on: 2 deviations (the proof for question 7)
- `/tmp/qrd-authz/matrix-AFTER-flagoff.txt`, `matrix-AFTER-flagon.txt` — post-fix: 0 deviations
- `backend/internal/handlers/authz_scope_integration_test.go` — permanent regression guard
