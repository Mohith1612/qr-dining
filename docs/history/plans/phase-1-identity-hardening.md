# Phase 1 - Identity Hardening

Status: Backend identity hardening implemented in commit `2367f16` (`Add phase 1 identity hardening`). Frontend migration and login audit event expansion remain follow-up work.

## Objective

Replace ambiguous staff identity and client-spoofable guest identity with explicit, signed, revocable identity boundaries.

## Dependencies

- Phase 0 route/resource ownership matrix.
- Feature flags available:
  - `AUTH_STAFF_CODE_REQUIRED`
  - `AUTH_STAFF_SESSION_DB_REQUIRED`
  - `AUTH_GUEST_CREDENTIALS_REQUIRED`

## Current Codebase Context

Staff auth:

- `backend/internal/services/staff.go`
- `backend/internal/handlers/staff.go`
- `backend/internal/middleware/staff_auth.go`
- `frontend/store/staff.ts`

Guest identity:

- `backend/internal/handlers/session.go`
- `backend/internal/handlers/cart.go`
- `backend/internal/handlers/order.go`
- `backend/internal/handlers/assistance.go`
- `backend/internal/handlers/ws.go`
- `frontend/lib/api/client.ts`
- `frontend/lib/ws/connection.ts`

Before Phase 1, staff login accepted `branch_id + pin`, and guest routes trusted `X-Participant-ID`, body `participant_id`, body `placed_by_participant_id`, and websocket query params. After Phase 1, the backend supports staff-code login and signed guest credentials while preserving legacy compatibility unless strict flags are enabled.

## Target Staff Auth

Staff login should become:

```text
POST /staff/auth
{
  "branch_code": "BLR-INDIRANAGAR",
  "staff_code": "KITCHEN01",
  "pin": "123456",
  "device_name": "Kitchen iPad 2"
}
```

Server behavior:

1. Resolve active branch by `branch_code`. Implemented for branch lookup.
2. Resolve active staff by `(branch_id, staff_code)`. Implemented.
3. Compare PIN hash for that exact staff member. Implemented.
4. Verify staff, branch, and organization status. Staff active state is enforced; organization status is deferred to Phase 3 organization modeling.
5. Create durable staff session. Implemented with `staff_sessions`.
6. Return short-lived access token and refresh/session token. Implemented as a session token/access token compatibility response; separate refresh-token lifecycle is deferred.
7. Include `staff_id`, `branch_id`, `role`, `staff_code`, `session_id`, `token_version`, and `pin_version` in server-side session state. Implemented. `organization_id` is deferred until Phase 3 introduces organizations.
8. Audit login success/failure. Deferred to Phase 5 audit logging expansion.

## Target Guest Auth

Create/join session should return:

- session summary
- participant summary
- signed guest access token
- optional renew token

Guest token claims:

```text
sub: participant:{participant_id}
sid: session_id
bid: branch_id
tid: table_id
org: organization_id
role: guest | host
participant_id: number
credential_version: number
iat: timestamp
exp: timestamp
jti: unique token id
aud: qr-dining-guest
```

All guest routes must derive participant/session/branch from the verified credential.

Implemented backend behavior: create/join now return `guest_access_token`; guest routes accept and validate the signed credential in permissive mode. Legacy identifiers are still accepted while `AUTH_GUEST_CREDENTIALS_REQUIRED=false`. Strict mode rejects missing/invalid guest credentials.

## Migration Strategy

1. Add staff codes and credential version fields additively. Implemented in migration `000016_identity_hardening`.
2. Backfill staff codes. Implemented with deterministic existing-row defaults in the migration.
3. Add durable staff session table. Implemented as `staff_sessions`.
4. Support both old and new staff auth while `AUTH_STAFF_CODE_REQUIRED=false`. Implemented.
5. Issue guest credentials on session create/join. Implemented.
6. Add guest auth middleware in permissive mode. Implemented as shared guest credential validation helpers wired into guest handlers.
7. Emit metrics when legacy participant identifiers are used. Implemented in Phase 0 and preserved in Phase 1.
8. Flip strict guest credential enforcement after frontend migration. Not yet done; controlled by `AUTH_GUEST_CREDENTIALS_REQUIRED`.

## Implemented Backend Changes

- Added additive migration `000016_identity_hardening`:
  - `branches.branch_code`
  - `staff.staff_code`
  - `staff.token_version`
  - `staff.pin_version`
  - `session_participants.credential_version`
  - `staff_sessions`
- Regenerated SQLC models and queries for branch-code lookup, staff-code lookup, staff session creation/validation, participant credential version reads, and staff credential version updates.
- Updated staff authentication:
  - new `branch_code + staff_code + pin + device_name` path;
  - legacy `branch_id + pin` path remains available when `AUTH_STAFF_CODE_REQUIRED=false`;
  - legacy PIN matching now filters inactive staff and rejects ambiguous duplicate PIN matches instead of selecting the first match.
- Added durable staff session validation:
  - stores hashed session token in `staff_sessions`;
  - checks staff active state, `token_version`, and `pin_version` during token validation;
  - revokes durable sessions on deactivation;
  - invalidates sessions by incrementing versions on PIN rotation/deactivation.
- Added signed guest credentials in `backend/internal/auth`:
  - issued on session create/join;
  - validated against session ID, participant ID, audience, expiry, and participant `credential_version`;
  - covered by unit tests for round-trip, expiry, and tamper rejection.
- Wired permissive guest credential validation into guest session, cart, order, assistance, payment, bill, customer opt-in, promo validation, snapshot, and websocket paths.
- Updated seed/test fixtures for required `branch_code` and `staff_code`.

## Remaining Follow-Up

- Frontend must migrate staff login to send `branch_code`, `staff_code`, `pin`, and optional `device_name`.
- Frontend guest API and websocket clients must persist and send `guest_access_token`; only then should `AUTH_GUEST_CREDENTIALS_REQUIRED=true` be enabled.
- Separate refresh-token or renewal-token lifecycle is not implemented yet.
- Login success/failure audit events are deferred to Phase 5 audit logging.
- Organization status checks are deferred to Phase 3 organization modeling.

## Deliverables

- Staff code design and backfill plan. Implemented.
- Durable staff session model. Implemented.
- Guest credential issuer and validator design. Implemented.
- Session/participant credential lifecycle rules. Partially implemented through participant `credential_version` and token expiry; participant revocation workflows remain future work.
- Token invalidation rules for staff deactivation and PIN rotation. Implemented. Session close and participant revocation credential lifecycle remains future work.
- Login audit events. Not implemented; deferred to Phase 5.

## Exit Criteria

- Staff login is unambiguous when using `branch_code + staff_code + pin`. Implemented.
- Inactive staff cannot authenticate through new staff-code login or legacy active-staff PIN scan. Implemented.
- Duplicate PINs do not choose the wrong staff member; legacy duplicate matches are rejected. Implemented.
- Guest token can authenticate session operations in permissive mode. Implemented for backend guest handlers.
- Legacy guest identity usage is measurable. Implemented in Phase 0 metrics.

## Test Requirements

- Duplicate staff PINs require staff code. Covered by legacy-auth ambiguity rejection behavior; deeper integration coverage should be added as Phase 2/CI expands.
- Inactive staff login is rejected. Implemented by active staff lookup.
- Staff token invalid after deactivation. Implemented by token version/session revocation checks.
- Staff token invalid after PIN rotation. Implemented by token and PIN version checks.
- Guest cannot spoof another participant. Implemented in credential-vs-legacy participant comparison.
- Guest cannot operate on another session. Implemented in guest credential session check.
- Guest credential expires and is rejected. Covered by `backend/internal/auth` unit tests.
