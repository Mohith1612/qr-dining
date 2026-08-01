# Phase 4 - Super Admin Trust Domain

Status: Backend implemented in commit `00ce53d` as part of the Phase 5 audit logging commit.

## Objective

Create a separate platform administration trust domain for provisioning, support, billing administration, and platform audit.

## Dependencies

- Phase 3 organization model.
- Phase 2 policy model.

## Core Principle

Super admin is not owner++.

Platform admin must not reuse branch staff identity, branch owner roles, or organization owner permissions. It must authenticate through separate platform middleware and operate through separate platform route namespaces.

## Target Platform Model

```text
platform_users
  id
  email
  display_name
  password_hash
  status
  mfa_required
  created_at
  updated_at

platform_user_roles
  platform_user_id
  role
  created_at

platform_sessions
  id
  platform_user_id
  token_hash
  device_name
  created_at
  last_seen_at
  expires_at
  revoked_at
```

Roles:

- `super_admin`
- `support_admin`
- `billing_admin`
- `read_only_auditor`

## Route Namespace

Add:

```text
/platform/auth
/platform/organizations
/platform/organizations/:org_id
/platform/organizations/:org_id/branches
/platform/branches/:branch_id
/platform/users
/platform/audit
/platform/support
```

All platform routes use `PlatformAuth`.

Rules:

- Staff tokens are rejected.
- Organization tokens are rejected.
- Platform tokens are rejected on regular staff/guest operational routes unless an explicit support session flow is used.

## Provisioning Flows

Organization provisioning:

1. Platform admin creates organization.
2. Platform admin creates compatibility restaurant/profile if needed.
3. Platform admin creates initial branch.
4. Platform admin assigns initial organization owner by email.
5. Owner accepts invite.
6. Owner/manager creates branch staff.
7. Every step is audited.

Branch creation:

1. Verify organization active.
2. Generate branch code.
3. Create branch with timezone, address, operational settings.
4. Create default order prefix.
5. Create default branch settings.
6. Optionally clone menu through audited copy flow.
7. Create initial tables only through authorized action.

## Break-Glass Support

Add support session model:

```text
platform_support_sessions
  id
  platform_user_id
  organization_id
  branch_id nullable
  reason text not null
  approved_by_platform_user_id nullable
  starts_at
  expires_at
  created_at
```

Rules:

- Default support is read-only.
- Write support actions require stronger role and reason.
- Every support read/write includes `support_session_id` in audit.
- Tenant-visible audit shows platform support access.

## Implemented Backend Changes

Implemented in commit `00ce53d`:

- Added `backend/migrations/000018_platform_trust_domain.up.sql` and `.down.sql`.
- Added `platform_users`, `platform_user_roles`, `platform_sessions`, `platform_support_sessions`, and `platform_audit_log`.
- Added SQLC platform queries and generated code in `backend/internal/db/sqlc/platform.sql.go`.
- Added `PlatformService` with password-hash login, opaque Redis-backed platform tokens, durable platform sessions, logout, role checks, and token validation.
- Added `PlatformAuth` middleware that stores platform session context separately from staff session context.
- Added `/platform/auth` and protected `/platform/*` routes for users, organizations, branches, support sessions, and platform audit.
- Added organization/branch provisioning from the platform trust domain:
  - organization plus compatibility restaurant/profile
  - branch linked to organization and restaurant
  - organization branch membership
  - optional initial tables
  - optional initial owner staff and staff-backed organization membership
- Added optional seed support through `PLATFORM_ADMIN_EMAIL`, `PLATFORM_ADMIN_PASSWORD`, and `PLATFORM_ADMIN_NAME`.
- Added platform repository integration tests and platform role tests.

Verification completed:

- `cd backend && make sqlc-generate`
- `cd backend && go test ./...`
- `cd backend && go vet ./...`

Known v1 limits:

- No frontend platform admin UI.
- No email invite/acceptance flow.
- `mfa_required` is represented but not enforced.
- No operational support impersonation; support sessions are auditable records for later support-access flows.

## Exit Criteria

- Platform admin cannot authenticate through staff or organization middleware.
- Organization owner cannot call platform routes.
- Provisioning creates organization, branch, and initial owner through audited flows.
- Support access is time-bound, reason-bound, and visible in audit.

## Test Requirements

- Staff token rejected by `/platform/*`.
- Organization owner token rejected by `/platform/*`.
- Platform token rejected by staff-only operational routes unless support flow applies.
- Platform provisioning emits audit records.
- Break-glass support expires and cannot be reused.
