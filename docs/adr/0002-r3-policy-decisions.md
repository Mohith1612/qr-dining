# ADR 0002 — R3 central-authz policy semantics

**Status:** Accepted · **Date:** 2026-05-29 · **Outcome:** all three decisions ratified existing behaviour, so no production behaviour change was required

> **Read as a record, not as current state.** R3 (`AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`) is still **off**, gated on 48 hours of zero shadow mismatches — see [../OPERATIONS.md §4](../OPERATIONS.md#4-rollout-flags-the-enforcement-ladder).


> **Purpose.** This document closes the only *hard* blocker on rollout wave **R3**
> (`master-system-context-v1.md` §11): the three policy-semantics questions that must be
> answered in writing before central-policy enforcement goes strict. It records the
> finalized decisions, their rationale, the enforcement/rollout implications, and the
> shadow-validation findings that gate the cutover.
>
> **Status.** Authored 2026-05-29. Every claim below was verified directly against source
> (`internal/authz/policy.go`, `internal/handlers/authz.go`, `guest_auth.go`, `audit_log.go`,
> `internal/observability/metrics.go`, the route table in `internal/server/server.go`).
>
> **Outcome.** All three decisions **ratify existing behavior** — the system already
> implements safe, defensible defaults. No production behavior change is required for R3
> policy semantics. The remaining R3 prerequisites are operational (shadow soak), not code.

---

## 1. Scope & rollout framing

R3 flips a **paired** flag set (both flip together, both reversible via `flag=false` +
restart, MTTR <5 min):

- **`AUTHZ_CENTRAL_POLICY_ENFORCE`** toggles the central authorizer
  (`NewEnforcingAuthorizer`, `internal/authz/policy.go:34`) via the `requireAuthorized`
  helper (`internal/handlers/authz.go:47`):
  - **Shadow (current default):** a policy denial is logged (`LogAuthzDenied`), written to
    the audit trail with `ResultSuccess`, counted, and **allowed** to proceed.
  - **Strict:** a policy denial is counted and **blocked** with `403 FORBIDDEN`.
- **`STRICT_BRANCH_SCOPED_MUTATIONS`** toggles `enforceBranchScopeFromBody`
  (`internal/handlers/helpers.go:49-61`): a client-supplied body `branch_id` that mismatches
  the server-derived branch is **logged** (`legacy_identity_usage_total`) and accepted in
  shadow, and **rejected with 400** in strict. This is orthogonal to the central policy: it
  closes the legacy body-`branch_id` override path, while the authz flag closes role/scope
  denials. They are sequenced as one wave.

**Exit gate (operational):** a shadow week with `policy_shadow_mismatch_total` == 0 for 48h
before flipping — read with the coverage caveat in §5.

---

## 2. Decision A — Org-owner authority = **GOVERNANCE-ONLY** (ratified)

**Decision.** An organization owner is a **governance** actor, not a cross-branch
**operational** actor. They may not create/modify a branch's operational resources (menu,
staff, tables, promos, order status, payment settlement) on a branch where they hold no
branch staff row. There is **no emergency override**.

**Why this is already true (double-enforced).**
- Staff identity is **branch-scoped**: a `staff` row belongs to one branch, and a staff
  session carries that single `BranchID` (`services/staff.go`). The authz actor's scope is
  always anchored to `sess.BranchID` (`handlers/authz.go:25 staffActorForRequest`) — never
  org-wide.
- **Handler layer:** branch-param mutations check `sess.BranchID != :id` → 403 *before* any
  policy evaluation (e.g. `handlers/staff.go`, `handlers/menu_admin.go`).
- **Policy layer:** `requiresSameBranch` (`policy.go:75`) + `Scope.SameBranch`
  (`authz/scope.go`) deny by numeric branch-ID equality; there is no org-ownership scope
  override. Confirmed cross-branch denial is locked by tests
  (`policy_test.go`: order/menu/payment/staff/audit cross-branch cases, plus the new
  `ActionStaffCreate` / `ActionBranchUpdateSettings` cases).
- **Governance surface is read-only and org-scoped:** `/orgs/:org_id/*`
  (`server.go`) is gated by `organization_members` membership
  (`handlers/organization.go authorizeOrg`) and covers org metadata, branch listing, org
  analytics, and org audit — never branch operational mutations.

**Rationale.** Preserves the branch-first multi-tenant isolation invariant; keeps a single
accountable operational actor per branch; avoids a privilege path that would let one
compromised org-owner credential mutate every branch.

**Rollout implication.** The R3 flip is **safe** for this decision: no cross-branch
operational path exists in shadow *or* strict; strict mode only hardens already-correct
behavior. Shadow surfaces would-be denials as `role_not_allowed`, `branch_mismatch`, or
`org_mismatch`.

---

## 3. Decision B — Revoked guest credential semantics (ratified)

**Decision.** A revoked guest credential is **hard-blocked on every new action and every
new connection**. Revocation occurs at session close; teardown of any already-open
WebSocket socket is driven by the `SESSION_CLOSED` broadcast. No forced mid-connection
eviction is added.

**Behavior matrix (verified).**

| Surface | Guard | Revoked result |
|---|---|---|
| Cart / order / assist / payment / billing / promo / customer / session get·join·reactivate | `guestParticipantID` revoked check (`handlers/guest_auth.go:73`) | **401** |
| Snapshot (`GET /sessions/:id/snapshot`) | via `requireGuestSession` → same guard | **401** |
| WebSocket connect (token path) | `guestParticipantID` (`handlers/ws.go:59`) | **401** |
| WebSocket connect (ticket path) | revoked check (`handlers/ws.go:126`) | **401** |
| WebSocket **already open** | none | **not force-evicted** (see residual) |

**Revocation triggers.** Only `CloseSession` →
`RevokeAllParticipants(reason="session_closed")` + credential-version bump
(`services/session.go`). Reactivation does **not** revoke (credentials stay valid across
`awaiting_reactivation`); a revoked host is skipped by `ensureHostBaseline`, which promotes
the oldest non-revoked participant. The specific §11 sub-question — *can a revoked credential
create an assistance request?* — is **No** (blocked at `guest_auth.go:73`).

**Accepted residual (documented, benign).** An open socket is not force-evicted the instant
its credential is revoked. This is acceptable because revocation only happens at session
close, which broadcasts `SESSION_CLOSED` (clients disconnect on receipt), the session is
terminal (no further mutations are possible — every HTTP surface is blocked), and only that
session's now-defunct room events could flow. Forced eviction would be pure defense-in-depth
with marginal value and is intentionally **not** implemented in this phase.

**Rollout implication.** Independent of the R3 flags (guest revocation is always enforced,
not flag-gated). No cutover risk.

---

## 4. Decision C — Org-level audit aggregation (ratified)

**Decision.** Organization owners/admins may read the **full cross-branch audit trail for
their organization**, strictly org-isolated, with **no time limit** on historical or
closed-session visibility.

**Read paths & scope (verified).**

| Endpoint | Guard | Scope |
|---|---|---|
| `GET /branches/:id/audit` | `sess.BranchID == :id` + `requireAuthorized(ActionAuditReadBranch)` (owner/manager) | single branch (`audit_log.sql` `WHERE branch_id = …`) |
| `GET /orgs/:org_id/audit` | `organization_members` role owner\|admin (`handlers/audit_log.go:85`) | **cross-branch, org-scoped** (`WHERE organization_id = …`, optional `branch_id`) |
| `GET /platform/audit` | platform auditor role (separate trust domain) | optionally unscoped (all orgs) |

**Isolation.** Org and branch reads filter by `organization_id` / `branch_id`; no guarded
non-platform path returns rows from another organization. Platform support search is
cross-org but restricted to platform auditors/support admins.

**Historical visibility.** Audit rows for closed/terminal sessions remain readable
indefinitely — this is **intended** for dispute resolution and compliance, and is distinct
from the 60-minute terminal-read window that applies only to the live session *snapshot*
endpoint.

**Rationale.** Org owners need a complete operational/compliance view of their own estate;
the immutable audit log is the system of record. Bounding retention or hiding closed
sessions would degrade dispute handling without an isolation benefit (org scoping already
prevents cross-tenant exposure).

**Rollout implication.** Independent of the R3 flags. No cutover risk.

---

## 5. Shadow-pipeline validation findings

**Instrumentation is correct and sufficient** (verified in `handlers/authz.go` +
`observability/metrics.go`):

- `policy_shadow_mismatch_total{route,reason}` — incremented on a would-be denial **while
  enforcement is off**: the pre-flip "what would break" signal, pinned to the gin route.
- `authz_denied_total{reason}` — incremented on an **enforced** denial (strict mode).
- `legacy_authz_bypass_total{action,actor_role}` — companion shadow counter (which
  action/role was allowed through).
- `reason` is bounded: `unsupported_actor_type | branch_mismatch | org_mismatch |
  role_not_allowed | other`. Every denial also writes a durable audit row
  (`ResultSuccess` in shadow, `ResultDenied` in strict).

**Coverage caveat (must inform the exit gate).** Most mutating staff routes flow through
`requireAuthorized` and therefore participate in shadow accounting (order status, menu
admin, promo, assist, payment settle, staff PIN/deactivate, branch audit read, customer,
org read/update). **Three endpoints enforce *inline, always-strict* checks and bypass the
central policy:**

- `CreateStaff` (`handlers/staff.go`) — inline `sess.BranchID` + owner-only.
- `UpdateBranch` (`handlers/branches.go`) — inline `sess.BranchID` + owner/manager.
- `GetOrgAuditLog` (`handlers/audit_log.go:85`) — inline `organization_members` owner/admin.

Because these are already strict regardless of the flag, **flipping R3 is a no-op for them
and they never emit `policy_shadow_mismatch_total`.** Operators MUST interpret the "48h zero
mismatch" exit gate as covering the **flag-gated** surface only; these three are already at
the strict end-state and carry no cutover risk. (They are intentionally inline rather than
centralized so they cannot become shadow-permissive during the rollout window — routing them
through `requireAuthorized` in shadow mode would *weaken* them. Centralizing them behind an
always-strict path is a possible future consistency cleanup, not an R3 requirement.)

**Conclusion.** No metric or enforcement-pipeline changes are required for R3.

---

## 6. Rollout readiness & remaining blockers

- **Policy-semantics writing blocker (§11): CLOSED** by this document (Decisions A/B/C).
- **Operational prerequisites (not code):** run the shadow week and confirm
  `policy_shadow_mismatch_total` == 0 for 48h (read with the §5 coverage caveat); flip the
  pair (`AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS`) **together**; keep
  the flip reversible; do not chain ahead of R1/R2 soak.
- **Accepted residual:** revoked-credential live-WS non-eviction (Decision B) — documented,
  benign, not fixed.
- **Regression locks added:** `internal/authz/policy_test.go` now asserts the ratified
  governance boundary for staff creation, branch-settings updates, and organization scope.

*End of ADR 0002 (originally `r3-policy-decisions-v1.md`).*
