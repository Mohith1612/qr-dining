# Phase 9 - Final Re-Audit and Production Gate

## Objective

Verify that QR Dining is production-ready after identity, RBAC, tenancy, super admin, audit, realtime, session, payment, and operational UX hardening.

## Dependencies

- Phases 0 through 8 complete.
- Strict enforcement flags ready to be enabled or already enabled.

## Required Re-Audit Areas

- Staff auth ambiguity removed.
- Inactive staff cannot authenticate.
- Guest identity cannot be spoofed.
- Branch isolation is enforced on loaded resources.
- Organization governance does not bypass branch operations.
- Platform admin is a separate trust domain.
- Audit logging captures security-sensitive events.
- WebSocket auth is signed and replay-resistant.
- Session creation cannot create duplicate active sessions.
- Stale cleanup releases tables.
- Order creation derives branch and participant server-side.
- Payment settlement is authoritative.
- Webhooks are signature-verified.
- Promo redemption caps are concurrency-safe.
- Redis keys are namespaced.
- Operational IDs are present in staff/support workflows.

## Test Suite Gate

Required passing tests:

- Auth integration tests.
- Branch isolation integration tests.
- Guest credential tests.
- Policy table tests.
- Session lifecycle and worker tests.
- WebSocket ticket and reconnect tests.
- Payment webhook and settlement tests.
- Promo concurrency tests.
- Migration/backfill tests.
- Playwright sweeps for migrated user flows.

## Rollout Gate

Before production:

- Legacy staff auth disabled or explicitly time-boxed.
- Legacy guest participant identifiers disabled.
- WebSocket query auth disabled.
- Central policy enforcement enabled.
- Strict branch-scoped mutations enabled.
- Audit log V2 enabled.
- Payment staff settlement enabled.
- Organization model enabled.

## Operational Readiness

Verify:

- Dashboards and logs expose request IDs.
- Support can search by readable operational references.
- Audit reads are scoped.
- Platform support access is visible.
- Rollback strategy is documented.
- Data cleanup scripts for pre-constraint inconsistencies have been run.
- Alerts exist for auth failures, webhook failures, worker failures, and branch isolation denials.

## Exit Criteria

- No critical or high auth/tenancy findings remain.
- Production readiness sign-off includes security, operational, and migration checks.
- Any remaining medium/low findings have owners and dates.
