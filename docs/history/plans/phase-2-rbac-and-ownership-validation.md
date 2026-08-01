# Phase 2 - RBAC and Ownership Validation

Status: Backend implementation completed in commit `2b0d36e` (`Implement phase 2 RBAC ownership validation`).

## Objective

Replace scattered handler role checks with centralized policy evaluation and enforce loaded resource ownership before mutations.

## Implementation Summary

Implemented backend changes:

- Added centralized policy evaluation in `backend/internal/authz` with actor, scope, resource, action, decision, policy, and ownership helpers.
- Wired an `authz.Authorizer` into order, assistance, menu admin, promo, staff, and customer handlers.
- Converted priority staff mutations to load the target resource first, derive real branch/restaurant/session scope, authorize against that loaded scope, and then execute scoped mutations.
- Added scoped repository/service mutations for order status updates, assistance ack/resolve, menu item/category/modifier changes, promo management, staff PIN/deactivation, and customer history/deletion.
- Stopped item-scoped menu mutations from relying on client-supplied body/query `branch_id`; legacy fields remain accepted for API compatibility but are ignored for authorization.
- Added `AUTHZ_DENIED` audit logging through `event_log` when a branch/session context is available, plus structured denial logs.
- Added `authz` unit coverage and updated existing service integration tests for scoped mutation signatures.

Verification completed:

```text
GOCACHE=/tmp/qr-dining-go-build go test ./...
GOCACHE=/tmp/qr-dining-go-build go test -tags=integration ./...
```

Implementation note: `sqlc` was not available in the local PATH during implementation, so query files were updated as source of truth and new scoped repository methods were implemented against the existing repository DB handle. A future `sqlc generate` pass should regenerate typed query methods from the updated SQL files.

## Dependencies

- Phase 1 actor model for staff and guest identities.
- Phase 0 resource ownership matrix.
- Feature flags:
  - `AUTHZ_CENTRAL_POLICY_ENFORCE`
  - `STRICT_BRANCH_SCOPED_MUTATIONS`

## Current Codebase Context

High-risk areas converted in the backend implementation:

- `backend/internal/handlers/order.go`
- `backend/internal/handlers/assistance.go`
- `backend/internal/handlers/menu_admin.go`
- `backend/internal/services/menu.go`
- `backend/internal/handlers/promo.go`
- `backend/internal/handlers/staff.go`
- `backend/internal/handlers/customer.go`
- `backend/sql/queries/orders.sql`
- `backend/sql/queries/menu.sql`
- `backend/sql/queries/assistance.sql`

Converted routes now use centralized policy calls and loaded-resource scopes instead of trusting handler-local role/branch comparisons. Some non-priority routes may still contain direct checks and should be addressed by later phases or follow-up hardening passes.

## Target Architecture

Created:

```text
backend/internal/authz
  actor.go
  scope.go
  resource.go
  action.go
  policy.go
  ownership.go
```

Policy signature:

```text
Authorize(actor, action, resource) Decision
```

Decision fields:

- `allowed`
- `reason`
- `required_scope`
- `actor_scope`
- `resource_scope`
- `audit_hint`

## Required Actions

Use stable action names:

```text
organization.read
organization.update
branch.read
branch.update_settings
staff.create
staff.update_role
staff.deactivate
menu.item.update
menu.item.toggle_availability
order.status.update
assistance.ack
assistance.resolve
payment.settle.staff
promo.create
promo.deactivate
audit.read.branch
```

## Ownership Validation Rule

Every mutation must:

1. Authenticate actor.
2. Load target resource by ID.
3. Resolve target organization/branch/session scope.
4. Authorize actor against loaded target scope.
5. Execute scoped SQL mutation.
6. Audit the result.
7. Publish realtime event after commit.

Never authorize item-scoped mutation from client-supplied `branch_id`.

## Priority Conversion Targets

Converted in backend commit `2b0d36e`:

- `PATCH /orders/:id/status`
- `PATCH /assist/:id/ack`
- `PATCH /assist/:id/resolve`
- `PATCH /menu/items/:id`
- `PATCH /menu/items/:id/availability`
- `PATCH /menu/items/:id/featured`
- `POST /menu/items/:id/modifiers`
- `DELETE /menu/modifiers/:id`
- `GET/POST/DELETE /branches/:id/promos`
- `PATCH /staff/:id/pin`
- `PATCH /staff/:id/deactivate`
- `GET /customers/:id/history`
- `DELETE /customers/:id`

## Scoped SQL Standard

Examples:

```text
UPDATE orders
SET status = $3
WHERE id = $1 AND branch_id = $2
RETURNING *
```

```text
UPDATE menu_items
SET ...
WHERE id = $1 AND branch_id = $2
RETURNING *
```

```text
DELETE FROM item_modifiers
USING menu_items
WHERE item_modifiers.id = $1
  AND item_modifiers.item_id = menu_items.id
  AND menu_items.branch_id = $2
```

## Exit Criteria

- High-risk staff mutations no longer trust body/query branch IDs for converted routes.
- Cross-branch mutation attempts are denied by loaded-resource policy checks and scoped mutations.
- Policy denials are logged and auditable through `AUTHZ_DENIED`.
- Handler role checks are reduced to policy calls for converted routes.

## Test Requirements

- Staff from branch A cannot update branch B order status.
- Staff from branch A cannot acknowledge branch B assistance.
- Staff from branch A cannot update branch B menu item by lying in request body.
- Promo management requires matching staff branch and authorized role.
- Customer history cannot leak across tenant scope.
