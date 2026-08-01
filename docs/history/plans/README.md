# QR Dining Platform Hardening Plans

Date: 2026-05-21

Status: Living implementation plan. Phase documents now distinguish planned work from implemented backend changes.

This folder breaks `platform-hardening-architecture-plan.md` into phase-oriented implementation plans. The phases are ordered to reduce migration risk while moving QR Dining from a branch-scoped app into a governed multi-tenant hospitality platform.

## Phase Index

1. [Phase 0 - Baseline Re-Audit and Guardrails](./phase-0-baseline-reaudit-and-guardrails.md)
2. [Phase 1 - Identity Hardening](./phase-1-identity-hardening.md)
3. [Phase 2 - RBAC and Ownership Validation](./phase-2-rbac-and-ownership-validation.md)
4. [Phase 3 - Organization Model](./phase-3-organization-model.md)
5. [Phase 4 - Super Admin Trust Domain](./phase-4-super-admin-trust-domain.md)
6. [Phase 5 - Audit Logging V2](./phase-5-audit-logging-v2.md)
7. [Phase 6 - Realtime and Session Hardening](./phase-6-realtime-and-session-hardening.md)
8. [Phase 7 - Payment and Order Correctness](./phase-7-payment-and-order-correctness.md)
9. [Phase 8 - Operational UX Cleanup](./phase-8-operational-ux-cleanup.md)
10. [Phase 9 - Final Re-Audit and Production Gate](./phase-9-final-reaudit-and-production-gate.md)

## Implementation Status

- Phase 0 backend guardrails were implemented in commit `73a47ea`: rollout flag config, legacy identity metrics, backend guardrail scaffolding, and integration-test compatibility updates.
- Phase 1 backend identity hardening was implemented in commit `2367f16`: additive identity schema, staff-code login, durable staff sessions, guest token issuance/validation in permissive mode, and token invalidation checks.
- Phase 2 backend RBAC and ownership validation was implemented in commit `2b0d36e`: centralized staff authorization policy, loaded-resource ownership checks, scoped SQL/repository mutations, authz denial audit logging, and backend unit/integration verification.
- Phase 3 backend organization model was implemented in commit `f69efde`: additive organization schema/backfill, staff-backed organization memberships, real organization scope in tenant/authz/session flows, minimal `/orgs/:org_id` governance APIs, and organization-wide analytics.
- Phase 6 realtime/session hardening has a code implementation and completion follow-up pending code-only commit: active-session uniqueness, race-safe stale cleanup, reconciliation worker, sequenced event replay, scoped Redis keys, and WebSocket ticket auth.
- Plan files and audit docs remain intentionally separate from backend commits unless explicitly requested.

## Cross-Cutting Rules

- Keep branches as the operational isolation boundary.
- Add organizations as governance/accountability, not as a replacement for branch isolation.
- Keep platform super admin in a separate trust domain.
- Move authorization to centralized policy evaluation.
- Stop trusting client-supplied staff, participant, branch, and session identifiers.
- Prefer additive migrations, backfills, compatibility windows, and feature flags.
- Do not drop legacy fields or routes until strict enforcement is proven in tests and rollout metrics.

## Recommended Rollout Flags

```text
AUTH_GUEST_CREDENTIALS_REQUIRED
AUTH_STAFF_CODE_REQUIRED
AUTH_STAFF_SESSION_DB_REQUIRED
AUTHZ_CENTRAL_POLICY_ENFORCE
TENANCY_ORGANIZATIONS_ENABLED
AUDIT_LOG_V2_ENABLED
WS_TICKET_AUTH_REQUIRED
PAYMENT_STAFF_SETTLEMENT_REQUIRED
STRICT_BRANCH_SCOPED_MUTATIONS
```

## Production Gate

Production readiness requires Phase 9 sign-off. Earlier phases can be shipped incrementally only when their exit criteria are met and rollback behavior is understood.
