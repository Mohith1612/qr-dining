# Phase 3 - Organization Model

Status: Backend implemented in commit `f69efde`.

## Objective

Introduce organizations as the governance/accountability layer above branches while preserving branch-first operations.

## Dependencies

- Phase 2 policy model stable enough to represent organization and branch scopes.
- Feature flag:
  - `TENANCY_ORGANIZATIONS_ENABLED`

## Current Codebase Context

Current top-level tenant entity is `restaurants`.

Relevant files:

- `backend/migrations/000001_initial_schema.up.sql`
- `backend/migrations/000017_organization_model.up.sql`
- `backend/sql/queries/restaurants.sql`
- `backend/sql/queries/organizations.sql`
- `backend/sql/queries/analytics.sql`
- `backend/internal/middleware/tenant.go`
- `backend/internal/middleware/branch_guard.go`
- `backend/internal/handlers/tenant.go`
- `backend/internal/handlers/branches.go`
- `backend/internal/handlers/organization.go`
- `backend/internal/handlers/billing.go`
- `backend/internal/handlers/customer.go`
- `backend/sql/queries/subscriptions.sql`

Many systems still read `restaurants.settings_json` for compatibility behavior such as theme, billing, and customer memory. Organization governance now has its own `organizations.settings_json`.

## Target Model

Add:

```text
organizations
  id
  code
  name
  legal_name
  status
  primary_contact_email
  settings_json
  created_at
  updated_at

organization_members
  id
  organization_id
  staff_id
  role
  status
  invited_by_staff_id
  created_at
  updated_at

organization_branch_memberships
  id
  organization_id
  branch_id
  created_at
```

Recommended migration path:

- Keep `restaurants` temporarily as compatibility tenant/profile table.
- Add `organizations`.
- Add `restaurants.organization_id`.
- Add `branches.organization_id`.
- Backfill one organization per existing restaurant.
- Backfill staff-backed organization owner/admin memberships from existing active owner/manager staff.
- Later decide whether `restaurants` becomes `brands`, `restaurant_profiles`, or collapses into organizations.

## Branch Model Changes

Add:

```text
branches.organization_id
branches.branch_code
branches.status
branches.support_metadata_json
```

Branch code rules:

- Unique within organization.
- Stable across branch renames.
- Human-readable.
- Used in operational IDs, support, audit, and exports.

## Organization Permissions

Organization owner:

- Manage organization settings.
- Invite/remove organization admins.
- Assign branch managers.
- View cross-branch analytics.
- Manage billing/subscription.
- View organization audit logs.

Organization admin:

- View branches and analytics.
- Manage branches/settings if delegated.
- Assign branch staff if delegated.

Organization user must not become platform admin and must not bypass branch operational isolation.

## Migration Strategy

1. Add nullable organization tables and foreign keys.
2. Backfill organizations from restaurants.
3. Add organization codes.
4. Add branch organization IDs and branch codes.
5. Dual-read organization scope from branch/restaurant compatibility model.
6. Add organization membership.
7. Add organization routes under `/orgs/:org_id`.
8. Move cross-branch analytics and exports behind organization policy.
9. Convert nullable organization IDs to required after backfill validation.

## Implemented Backend Changes

Implemented in commit `f69efde`:

- Added `backend/migrations/000017_organization_model.up.sql` and `.down.sql`.
- Added `organizations`, `organization_members`, and `organization_branch_memberships`.
- Added `restaurants.organization_id`, `branches.organization_id`, `branches.status`, and `branches.support_metadata_json`.
- Backfilled one organization per existing restaurant and attached existing restaurants/branches to that organization.
- Reused existing `branches.branch_code` from Phase 1 as the stable branch code; no duplicate `branches.code` column was added.
- Added unique constraints for organization code, one restaurant per organization, branch code within organization, and one organization membership per staff/org pair.
- Backfilled organization memberships for active owner/manager staff as staff-backed organization owner/admin records.
- Added SQLC organization queries, repository wrappers, and organization-scoped analytics queries.
- Updated tenant middleware to resolve both restaurant ID and organization ID when `TENANCY_ORGANIZATIONS_ENABLED` is enabled.
- Updated branch tenant guard to validate branch organization ownership under the organization flag while preserving restaurant-based compatibility behavior when the flag is disabled.
- Updated staff sessions and guest token issuance to use real `organization_id` instead of the previous restaurant-ID compatibility shortcut.
- Updated centralized authz scopes so organization and customer policy checks compare real organization IDs.
- Added staff-authenticated organization routes:
  - `GET /orgs/:org_id`
  - `PATCH /orgs/:org_id`
  - `GET /orgs/:org_id/branches`
  - `GET /orgs/:org_id/analytics/top-items`
  - `GET /orgs/:org_id/analytics/busy-hours`
  - `GET /orgs/:org_id/analytics/order-volume`
- Added minimal frontend API typing/context updates for new `organization_id` response fields; no organization management UI was added.

Verification completed:

- `cd backend && make sqlc-generate`
- `cd backend && go test ./...`
- `cd backend && go vet ./...`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run lint` passed with warnings only.

## Exit Criteria

- Existing single-restaurant deployments map cleanly to one organization.
- Branch-first staff flows still work.
- Organization admins can govern multiple branches without platform access.
- Branch isolation remains enforced for operational mutations.

## Test Requirements

- Every existing branch resolves to exactly one organization.
- Existing tenant slug behavior still resolves correctly during compatibility window.
- Organization owner can read cross-branch analytics.
- Organization owner cannot call platform routes.
- Branch staff cannot access another branch through organization linkage unless explicitly authorized.
