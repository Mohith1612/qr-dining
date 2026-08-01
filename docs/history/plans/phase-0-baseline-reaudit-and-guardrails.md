# Phase 0 - Baseline Re-Audit and Guardrails

Status: Backend guardrails implemented in commit `73a47ea` (`Add phase 0 backend guardrails`). Documentation artifacts remain living references for later phases.

## Objective

Freeze risky expansion, convert the existing audit findings into actionable engineering guardrails, and prepare the codebase for staged auth, RBAC, tenancy, and operational hardening.

## Current Codebase Context

Relevant areas:

- `operational-correctness-audit.md`
- `backend/internal/server/server.go`
- `backend/internal/middleware/staff_auth.go`
- `backend/internal/middleware/tenant.go`
- `backend/internal/middleware/branch_guard.go`
- `backend/internal/handlers/*`
- `backend/internal/services/*`
- `backend/sql/queries/*`
- `backend/internal/websocket/*`
- `backend/internal/redis/*`

The audit found deployment blockers in staff auth, guest identity, branch isolation, websocket auth, payment lifecycle, stale session cleanup, and ownership validation.

## Scope

This phase is mostly analysis, test scaffolding, and rollout safety. It should not redesign frontend UX or introduce major schema behavior yet.

## Deliverables

- Route and resource ownership matrix. Implemented as `plans/phase-0-guardrail-matrix.md`.
- Current auth boundary map:
  - staff token routes
  - public guest routes
  - public tenant/menu routes
  - webhook routes
  - websocket upgrade route
- Current branch ownership matrix for:
  - orders
  - assistance
  - menu categories/items/modifiers
  - promos
  - sessions
  - payments
  - customers
  - uploads
- Feature flag definitions for staged enforcement. Implemented in `backend/internal/config/config.go` as disabled-by-default `FeatureFlags`.
- Metrics plan for legacy identity usage:
  - `X-Participant-ID`
  - body `participant_id`
  - body `placed_by_participant_id`
  - legacy `branch_id + PIN` staff auth
  - websocket query auth
- Legacy identity usage metrics. Implemented as `legacy_identity_usage_total{mechanism,endpoint_class}` in `backend/internal/observability/metrics.go` and wired through legacy identity observation points.
- Integration test scaffolding for cross-branch denial cases. Implemented as pending guardrail tests behind `RUN_PHASE0_GUARDRAIL_TESTS=true`.

## Implemented Backend Changes

- Added rollout flag parsing for:
  - `AUTH_GUEST_CREDENTIALS_REQUIRED`
  - `AUTH_STAFF_CODE_REQUIRED`
  - `AUTH_STAFF_SESSION_DB_REQUIRED`
  - `AUTHZ_CENTRAL_POLICY_ENFORCE`
  - `TENANCY_ORGANIZATIONS_ENABLED`
  - `AUDIT_LOG_V2_ENABLED`
  - `WS_TICKET_AUTH_REQUIRED`
  - `PAYMENT_STAFF_SETTLEMENT_REQUIRED`
  - `STRICT_BRANCH_SCOPED_MUTATIONS`
- Added legacy identity metrics for:
  - `X-Participant-ID`
  - body `participant_id`
  - body `placed_by_participant_id`
  - legacy `branch_id + PIN` staff auth
  - websocket query `participant_id`
- Instrumented observation-only counters in cart, session close, order placement, assistance request, staff auth, and websocket upgrade paths.
- Repaired integration-test scaffolding around the current service constructor signatures.
- Added pending guardrail tests for duplicate/inactive staff PIN risks, cross-branch mutation risks, spoofed order/session branch mismatch, unauthenticated webhook acceptance, stale session table release, and duplicate active session races.

## Not Implemented In Phase 0

- No schema hardening, new credentials, RBAC engine, webhook signature verification, or frontend migration was introduced in this phase.
- Strict enforcement remains delegated to Phase 1, Phase 2, Phase 6, and Phase 7 as mapped in the guardrail matrix.

## Required Checks

- Confirm duplicate PIN behavior in `StaffService.Authenticate`.
- Confirm inactive staff login risk in `ListStaffForBranch`.
- Confirm guest endpoints that trust participant identifiers.
- Confirm staff mutations that load target resources without branch comparison.
- Confirm webhook processing lacks provider signature verification.
- Confirm stale worker does not release occupied tables.
- Confirm duplicate active session race risk.

## Exit Criteria

- Critical audit findings are mapped to owning phase documents. Complete.
- Feature flags are agreed, named, and parsed by backend config. Complete.
- No new feature work should depend on legacy unsafe identity assumptions. Guardrail metrics and pending tests are in place.
- Phase 1 and Phase 2 can begin without re-litigating baseline risks. Complete.

## Risks

- Skipping this phase causes Phase 1 and Phase 2 to patch symptoms instead of trust boundaries.
- Missing route ownership cases will leave cross-branch bypasses after RBAC rollout.
