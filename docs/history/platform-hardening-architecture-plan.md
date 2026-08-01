# QR Dining Platform Hardening Architecture Plan

Date: 2026-05-21

Status: Planning only. Do not treat this document as an implemented design.

Purpose: evolve QR Dining from a branch-scoped restaurant app into a governed multi-tenant hospitality platform while preserving the current branch-first operational UX.

This document is implementation-ready architecture planning. It is grounded in the current codebase, migrations, route structure, services, Redis usage, WebSocket model, and the operational correctness audit findings in `operational-correctness-audit.md`.

## 1. Current System Baseline

### 1.1 Existing Tenancy Shape

The current backend schema models:

- `restaurants` as the top-level tenant entity.
- `branches` owned by `restaurants`.
- `staff` attached directly to one branch.
- `sessions`, `tables`, `menu_*`, `orders`, `assistance_requests`, `payments`, `promos`, and `order_sequences` as branch/session operational data.

Relevant current files:

- `backend/migrations/000001_initial_schema.up.sql`
- `backend/migrations/000004_staff_is_active.up.sql`
- `backend/migrations/000006_branch_session_config.up.sql`
- `backend/migrations/000013_order_sequences.up.sql`
- `backend/migrations/000015_promos.up.sql`
- `backend/internal/server/server.go`
- `backend/internal/middleware/tenant.go`
- `backend/internal/middleware/branch_guard.go`

The product already behaves like a branch-first operational system. Kitchen, waiter, menu, table, session, QR, promo, analytics, and branch settings flows are routed around branch IDs and staff branch membership.

### 1.2 Existing Auth Shape

Staff auth currently:

- Accepts `branch_id + pin`.
- Scans staff for the branch and bcrypt-compares each PIN.
- Stores a random token in Redis under `staff:token:{token}`.
- Stores token membership under `staff:tokens:{staff_id}` for deactivation cleanup.
- Injects `services.StaffSession` into Gin context with `staff_id`, `branch_id`, and `role`.

Relevant current files:

- `backend/internal/services/staff.go`
- `backend/internal/handlers/staff.go`
- `backend/internal/middleware/staff_auth.go`
- `frontend/store/staff.ts`
- `frontend/app/(staff)/staff/login/page.tsx`

Guest auth currently:

- Does not have signed credentials.
- Uses `session_id` in URL/path and `participant_id` in request body, query string, or `X-Participant-ID`.
- Stores `session_id` and `participant_id` in browser `sessionStorage`.
- Connects WebSocket with `?session_id=...&participant_id=...`.

Relevant current files:

- `backend/internal/handlers/session.go`
- `backend/internal/handlers/cart.go`
- `backend/internal/handlers/order.go`
- `backend/internal/handlers/assistance.go`
- `backend/internal/handlers/ws.go`
- `frontend/lib/api/client.ts`
- `frontend/lib/ws/connection.ts`
- `frontend/app/(guest)/table/[token]/page.tsx`
- `frontend/app/(guest)/session/[id]/layout.tsx`

### 1.3 Existing RBAC Shape

Current authorization is scattered:

- Route middleware only validates staff token.
- Handlers often compare `staffSession.BranchID == branchID`.
- Handlers/services directly check roles like `owner`, `manager`, `waiter`, `kitchen`.
- Menu service uses `requireOwnerOrManager`.
- No centralized policy layer exists.
- Item-scoped mutations often trust client-supplied branch IDs or omit target ownership checks.

Relevant current files:

- `backend/internal/server/server.go`
- `backend/internal/handlers/staff.go`
- `backend/internal/handlers/menu_admin.go`
- `backend/internal/services/menu.go`
- `backend/internal/handlers/order.go`
- `backend/internal/handlers/assistance.go`
- `backend/internal/handlers/promo.go`
- `backend/internal/handlers/customer.go`

### 1.4 Existing Realtime Shape

Current realtime uses:

- Redis Pub/Sub channels named `session:{session_id}:events`.
- A Hub room map keyed only by session UUID.
- Presence Redis hashes named `presence:{session_id}`.
- WebSocket upgrade validates active session and participant membership but only from query params.
- WebSocket is mostly push-only. Client messages other than PING are ignored.
- Reconnect uses `GET /sessions/:id/snapshot`.

Relevant current files:

- `backend/internal/websocket/hub.go`
- `backend/internal/websocket/client.go`
- `backend/internal/handlers/ws.go`
- `backend/internal/events/events.go`
- `backend/internal/redis/pubsub.go`
- `backend/internal/redis/presence.go`
- `frontend/lib/ws/connection.ts`
- `frontend/lib/ws/reconciliation.ts`

### 1.5 Existing Audit and Payment Primitives

The codebase already contains early primitives:

- `event_log`, insert-only, branch/session scoped.
- `payment_webhook_events`, intended for webhook idempotency.
- Order idempotency via globally unique `orders.idempotency_key`.
- Branch daily order numbering via `order_sequences(branch_id, date)`.

These are useful foundations but are not production-complete.

Relevant current files:

- `backend/migrations/000002_event_log.up.sql`
- `backend/migrations/000003_webhook_events.up.sql`
- `backend/internal/repository/event_log.go`
- `backend/internal/services/payment.go`
- `backend/sql/queries/payments.sql`
- `backend/sql/queries/orders.sql`

## 2. Target Architecture

### 2.1 Product Direction

QR Dining should evolve to:

```text
Platform
`-- Organizations
    `-- Branches
        |-- Staff
        |-- Tables
        |-- Sessions
        |-- Orders
        |-- Payments
        |-- Menus
        |-- Promos
        `-- Operational events
```

Organizations are governance and ownership entities.

Branches remain operational isolation boundaries.

Platform users and super admins live in a separate trust domain from organization owners and branch staff.

### 2.2 Why Branch Remains the Operational Boundary

Branch must remain the operational boundary because the live restaurant workload is branch-local:

- Tables are physical branch resources.
- QR tokens identify branch tables.
- Sessions occur at a physical table in one branch.
- Orders route to the branch kitchen and waiter floor.
- Assistance requests map to branch staff.
- Realtime rooms are session/branch local.
- Menu availability, promos, order numbers, table occupancy, and payment settlement are branch local.
- Operational downtime or mistakes in one branch must not affect another branch.

Therefore:

- WebSocket channels must be branch/session scoped.
- Redis keys must be branch/session scoped.
- Staff operational tokens must be branch-bound for branch roles.
- Resource mutations must validate target branch ownership.
- Operational IDs should be branch-local and human-readable.

### 2.3 Why Organization Is the Governance Layer

Organization must become the ownership and accountability layer because multi-branch businesses need:

- Shared ownership across branches.
- Organization admins who can manage membership and settings.
- Central analytics across branches.
- Cross-branch reporting and exports.
- Shared billing/subscription ownership.
- Account-level audit trails.
- Branch provisioning and deprovisioning.
- Owner assignment workflows.
- Policy inheritance where appropriate.

Organization must not replace branch isolation. It grants governance over many branches, but branch operations still require explicit branch scope.

### 2.4 Target Trust Domains

There should be four explicit trust domains:

1. Platform domain
   - Platform employees/admins.
   - Separate auth, separate middleware, separate route namespace.
   - Can provision organizations, branches, plans, and emergency support.

2. Organization domain
   - Organization owners/admins/operators.
   - Can govern organization membership, branch access, billing, cross-branch analytics, and organization settings.

3. Branch staff domain
   - Staff operating within a branch.
   - Can perform live service workflows subject to branch role and policy.

4. Guest session domain
   - Signed participant credentials bound to a session, participant, branch, and token generation.
   - Can only operate on the active session and participant derived from the credential.

## 3. Target Data Model

This section describes the planned model. It is not a migration script.

### 3.1 Organization Tables

Add an organization layer above branches.

Planned tables:

```text
organizations
  id bigint primary key
  code text unique not null
  name text not null
  legal_name text
  status enum(active, suspended, archived)
  primary_contact_email text
  settings_json jsonb not null default '{}'
  created_at timestamptz not null
  updated_at timestamptz not null

organization_members
  id bigint primary key
  organization_id bigint not null references organizations(id)
  user_id bigint not null references users(id)
  role enum(owner, admin, finance, analyst)
  status enum(active, invited, suspended, removed)
  invited_by_user_id bigint references users(id)
  created_at timestamptz not null
  updated_at timestamptz not null
  unique(organization_id, user_id)

organization_branch_memberships
  id bigint primary key
  organization_id bigint not null references organizations(id)
  branch_id bigint not null references branches(id)
  created_at timestamptz not null
  unique(organization_id, branch_id)
```

Current `restaurants` can evolve in one of two ways:

- Preferred long-term: rename semantic use of `restaurants` to `organizations`, then introduce a separate `brands` or `restaurant_profiles` table later if needed.
- Lower-risk migration: add `organizations`, add `restaurants.organization_id`, then attach existing branches through existing restaurants.

Recommended path for this codebase:

- Keep `restaurants` temporarily as the compatibility tenant/profile table because many queries and settings read `restaurants.settings_json`.
- Add `organizations`.
- Add `restaurants.organization_id`.
- Backfill one organization per restaurant.
- Later decide whether `restaurants` becomes `brands` or collapses into organizations.

Rationale: this avoids a broad rename through `GetRestaurantByBranchID`, subscription queries, customer memory, theme/billing settings, and tenant slug logic while introducing organization governance safely.

### 3.2 Branch Model Evolution

Current `branches` fields:

- `restaurant_id`
- `name`
- `address`
- `timezone`
- `session_timeout_minutes`
- `order_prefix`

Planned additions:

```text
branches
  organization_id bigint references organizations(id)
  code text not null
  status enum(active, suspended, archived)
  operational_timezone text not null
  support_metadata_json jsonb not null default '{}'
  created_by_platform_user_id bigint
```

Branch code rules:

- Unique within organization.
- Human-readable.
- Stable across renames.
- Used in operational IDs, logs, exports, and support screens.
- Example: `BLR-INDIRANAGAR`, `DEL-CP`, `MUM-BKC`.

Existing `order_prefix` should remain branch-local but be validated for uniqueness within organization if used in support dashboards.

### 3.3 User and Staff Identity Split

Introduce a global identity table for organization users and platform users:

```text
users
  id bigint primary key
  email citext unique
  phone_e164 text unique
  display_name text not null
  status enum(active, invited, suspended, deleted)
  auth_provider text
  password_hash text nullable
  mfa_enabled boolean not null default false
  created_at timestamptz not null
  updated_at timestamptz not null
```

Keep branch `staff` as operational staff profiles, but redesign it:

```text
staff
  id bigint primary key
  organization_id bigint not null
  branch_id bigint not null
  user_id bigint references users(id)
  staff_code text not null
  display_name text not null
  role enum(branch_manager, waiter, kitchen, cashier, host)
  pin_hash text not null
  is_active boolean not null
  pin_version int not null default 1
  created_at timestamptz not null
  updated_at timestamptz not null
  unique(branch_id, staff_code)
```

Important change:

- PIN alone is never identity.
- Staff login must be `branch code + staff code + PIN` or staff selection by server-issued staff code plus PIN.
- Duplicate PINs can exist only because the staff code disambiguates identity.
- Inactive staff cannot authenticate.
- `pin_version` invalidates old branch staff sessions after rotation.

### 3.4 Platform Admin Model

Platform users must be separate from organization/staff users:

```text
platform_users
  id bigint primary key
  email citext unique not null
  display_name text not null
  status enum(active, suspended, deleted)
  mfa_required boolean not null default true
  created_at timestamptz not null
  updated_at timestamptz not null

platform_user_roles
  platform_user_id bigint references platform_users(id)
  role enum(super_admin, support_admin, billing_admin, read_only_auditor)
  created_at timestamptz not null
  primary key(platform_user_id, role)
```

Do not reuse branch `staff.role = owner` for platform power.

Super admin is not owner++.

Super admin can provision and inspect across tenants through explicit platform routes and audited break-glass flows. Organization owners cannot access platform internals.

### 3.5 Guest Participant Credential Model

Current `session_participants` can remain, but add credential lifecycle fields:

```text
session_participants
  credential_version int not null default 1
  last_credential_issued_at timestamptz
  revoked_at timestamptz
  device_fingerprint_hash text
```

Credential claims should include:

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

The backend must derive `participant_id`, `session_id`, `branch_id`, and host status from the verified credential, not request bodies or headers.

### 3.6 Audit Log Evolution

Current `event_log` is useful but too narrow.

Target:

```text
audit_log
  id bigserial primary key
  organization_id bigint
  branch_id bigint
  restaurant_id bigint nullable during migration
  session_id uuid nullable
  table_id bigint nullable
  resource_type text not null
  resource_id text
  action text not null
  actor_type enum(platform_user, organization_user, staff, guest, system, webhook)
  actor_id text
  actor_display text
  actor_scope_json jsonb not null default '{}'
  request_id text
  idempotency_key text
  ip inet
  user_agent text
  before_json jsonb
  after_json jsonb
  metadata_json jsonb not null default '{}'
  risk_level enum(low, medium, high, critical)
  created_at timestamptz not null default now()
```

Keep `event_log` temporarily for websocket/session operational history, but route all security/accountability events to `audit_log`.

Long term, either:

- Keep both: `event_log` for session operational timeline, `audit_log` for immutable accountability.
- Or unify behind a single append-only audit system with event categories.

Recommended for this codebase: keep both initially to avoid destabilizing WebSocket/session code.

## 4. Authorization Architecture

### 4.1 Central Policy Layer

Create a central authorization package:

```text
backend/internal/authz
  actor.go
  scope.go
  resource.go
  action.go
  policy.go
  middleware.go
  ownership.go
```

Core concepts:

```text
Actor
  type: platform_user | organization_user | staff | guest | system
  id
  organization_id optional
  branch_id optional
  roles []
  permissions []
  session_id optional
  participant_id optional
  token_version

Scope
  organization_id optional
  branch_id optional
  session_id optional
  participant_id optional

Resource
  type
  id
  organization_id
  branch_id
  session_id
  owner_participant_id optional

Action
  string permission name
```

Policy evaluation signature:

```text
Authorize(ctx, actor, action, resource) Decision
```

Decision should include:

- `allowed`
- `reason`
- `required_scope`
- `actor_scope`
- `resource_scope`
- `audit_hint`

Do not continue adding `if role == manager` in handlers. Handlers should authenticate, load resource or target scope, then call policy.

### 4.2 Action Naming

Use stable action names:

```text
organization.read
organization.update
organization.members.invite
organization.members.update_role
organization.members.remove
organization.analytics.read
organization.exports.create

branch.read
branch.update_settings
branch.create
branch.suspend
branch.analytics.read
branch.theme.update
branch.billing_settings.update

staff.create
staff.update_role
staff.deactivate
staff.rotate_pin.self
staff.rotate_pin.other

menu.read_public
menu.read_admin
menu.category.create
menu.category.update
menu.category.delete
menu.item.create
menu.item.update
menu.item.toggle_availability
menu.item.delete
menu.modifier.create
menu.modifier.delete

table.read
table.create
table.qr.rotate

session.create
session.read
session.snapshot.read
session.close.host
session.close.staff
session.abandon.system

cart.read.self
cart.item.add.self
cart.item.remove.self

order.create.self
order.read.session
order.status.update
order.cancel

assistance.create.self
assistance.ack
assistance.resolve

payment.request.self
payment.settle.staff
payment.refund
payment.webhook.process

promo.read
promo.create
promo.deactivate
promo.redeem

audit.read.branch
audit.read.organization
audit.read.platform
```

### 4.3 Role Permissions

#### Organization Owner

Scope: organization.

Can:

- Read/update organization settings.
- Invite/remove organization admins.
- Assign branch managers.
- View all branch analytics.
- Manage billing/subscription.
- Create exports.
- View organization audit logs.
- Access branches according to explicit branch assignments or owner override.

Cannot:

- Use platform admin routes.
- Bypass guest/session auth.
- Modify platform-level configuration.

#### Organization Admin

Scope: organization.

Can:

- Manage non-owner org members depending on delegated policy.
- View all branches and analytics.
- Manage branch settings if granted.
- Assign branch staff if granted.

Cannot:

- Remove organization owners.
- Access platform routes.
- Perform payment refunds unless explicitly granted.

#### Branch Manager

Scope: branch.

Can:

- Manage branch staff below manager level.
- Manage menu, tables, promos, branch settings, QR rotation.
- View active sessions, orders, assistance, customers, branch analytics.
- Settle cash/card payment requests.
- Resolve operational issues.

Cannot:

- Access other branches unless separately assigned.
- Access organization billing by default.
- Access platform routes.

#### Waiter

Scope: branch.

Can:

- View branch active sessions, table status, active orders.
- Acknowledge/resolve assistance.
- Mark served where policy allows.
- Request cash settlement confirmation if cashier role not separate.

Cannot:

- Edit menus, promos, branch settings, staff, QR tokens.
- Access other branches.

#### Kitchen

Scope: branch.

Can:

- View active kitchen orders.
- Transition orders through kitchen states: pending -> confirmed, confirmed -> preparing, preparing -> ready.

Cannot:

- Mark served unless branch policy allows.
- Edit menu/prices/promos/staff/settings.
- See customer PII.

#### Cashier

Scope: branch.

Can:

- View bills.
- Confirm cash/card/manual payments.
- Initiate refunds if branch policy allows or manager approval exists.

Cannot:

- Edit menus/staff/settings by default.

#### Guest

Scope: session and participant.

Can:

- Read their active session summary.
- Read branch public menu for their session branch.
- Add/remove own cart items.
- Place own orders for their session.
- Request assistance for their session/table.
- Request payment for session bill.
- Host can close session only if payment/settlement policy allows.

Cannot:

- Provide arbitrary participant ID.
- Place orders against another session or branch.
- Modify another participant cart.
- Use stale session credentials.

#### Platform Admin

Scope: platform.

Can:

- Provision organizations and branches.
- Assign initial organization owners.
- Suspend/restore organizations and branches.
- Manage plan entitlements.
- View platform audit.
- Use audited support impersonation/read-only diagnostic views.

Cannot:

- Operate as branch staff implicitly.
- Perform tenant operations without platform route, reason, and audit record.

### 4.4 Resource Ownership Validation Strategy

Every resource mutation must follow this order:

1. Authenticate actor.
2. Load target resource by ID.
3. Resolve resource scope:
   - organization_id
   - branch_id
   - session_id
   - participant_id where relevant
4. Compare actor scope to resource scope through policy.
5. Execute mutation with scoped SQL `WHERE`.
6. Write audit event.
7. Publish realtime event after commit.

No handler should authorize item-scoped mutation from client-supplied `branch_id`.

Examples:

- `PATCH /orders/:id/status` must load order, verify `order.branch_id == actor.branch_id`, verify action by role, then update with `WHERE id = $1 AND branch_id = $2`.
- `PATCH /menu/items/:id` must load item, verify item branch, verify destination category branch, then update with `WHERE id = $1 AND branch_id = $2`.
- `DELETE /menu/modifiers/:id` must join modifier -> item -> branch before authorization.
- `GET /customers/:id/history` must verify customer belongs to actor organization/restaurant before returning history.

### 4.5 Middleware Strategy

Introduce separate middleware:

```text
PlatformAuth
OrganizationAuth
StaffAuthV2
GuestSessionAuth
RequireAction(action, resourceResolver)
```

Route namespaces:

```text
/platform/...                      PlatformAuth only
/orgs/:org_id/...                  OrganizationAuth
/branches/:branch_id/...           StaffAuthV2 or OrganizationAuth depending route
/sessions/:session_id/...          GuestSessionAuth for guest routes
/webhooks/payments/:provider       WebhookAuth, not user auth
/public/...                        rate-limited public metadata only
```

During migration, existing routes can remain but should route through the new middleware internally.

## 5. Auth Hardening Plan

### 5.1 Staff Login Redesign

Current blocker:

- `branch_id + PIN` is ambiguous.
- Inactive staff are currently loaded by `ListStaffForBranch`.
- PIN collisions can authenticate the wrong staff.

Target staff login:

```text
POST /staff/auth
{
  "branch_code": "BLR-INDIRANAGAR",
  "staff_code": "KITCHEN01",
  "pin": "123456",
  "device_name": "Kitchen iPad 2"
}
```

Authentication steps:

1. Resolve branch by `branch_code`.
2. Resolve active staff by `(branch_id, staff_code)`.
3. Compare PIN hash.
4. Verify staff active, branch active, organization active.
5. Create session in durable DB table `staff_sessions`.
6. Return short-lived access token and refresh token or a server-backed opaque session token.
7. Store session metadata and token version.
8. Audit login success/failure.

Recommended token model:

- Access token TTL: 15 minutes.
- Refresh/session token TTL: one shift or 8-12 hours.
- Server-side revocation table for staff sessions.
- Redis cache is acceleration only, not source of truth.
- Token claims include `staff_id`, `organization_id`, `branch_id`, `role`, `staff_code`, `session_id`, `token_version`, `pin_version`, `branch_status`, `org_status`.

### 5.2 Staff PIN Rules

PIN remains useful for quick restaurant operations, but only with staff code.

Rules:

- PIN length 6 digits minimum preferred.
- Enforce rate limits per branch, staff code, IP, and device fingerprint.
- Lock staff auth for a cooldown after repeated failures.
- Require owner/manager reset with audit.
- Increment `pin_version` on rotation.
- Invalidate all active staff sessions for that staff on deactivation or PIN reset.

### 5.3 Organization User Auth

Organization owners/admins should authenticate separately from quick branch PIN flows.

Recommended:

- Email/password or magic link/OAuth.
- MFA for owners/admins.
- Sessions bound to organization membership.
- Branch access derived from organization role and branch assignments.

Organization user auth should not be optimized for speed like kitchen/waiter PIN login.

### 5.4 Platform Admin Auth Realm

Platform admin auth must be separate:

- Separate table: `platform_users`.
- Separate login endpoint: `/platform/auth`.
- Mandatory MFA.
- Short TTL.
- Stricter IP/device alerting.
- No reuse of staff token middleware.
- Every platform action audited with reason.

Platform admin routes must reject staff and organization tokens even if claims look powerful.

### 5.5 Guest Credential Redesign

Current blocker:

- Guest endpoints trust participant IDs from headers/body/query.

Target:

- `POST /sessions` returns:
  - session public summary
  - participant summary
  - signed guest access token
  - refresh/renew token if needed
- `POST /sessions/:id/join` returns the same for joined participant.
- Guest HTTP requests use `Authorization: Guest {token}` or `Bearer {token}` with distinct audience.
- Remove `X-Participant-ID`.
- Remove `placed_by_participant_id` from request body.
- Remove `participant_id` from assistance request body.
- WebSocket uses `Sec-WebSocket-Protocol` or Authorization-compatible token transport, not query participant ID.

Guest credential validation:

1. Verify signature and audience.
2. Verify expiration.
3. Verify `jti` not revoked if revocation is required.
4. Load session and participant.
5. Verify session active.
6. Verify participant belongs to session.
7. Verify credential version matches participant.
8. Inject `GuestActor`.

### 5.6 Session Token Lifecycle

Current schema has `sessions.session_token`, but guest operations do not use it as a credential.

Target:

- QR table token only allows discovery/start/join.
- Session credential allows session operations.
- Participant credential identifies actor.
- Host privilege is a claim derived from DB and credential version.

Lifecycle:

- Issue on create/join.
- Rotate when host transfers or participant is revoked.
- Expire at session timeout plus short grace period.
- Invalidate on session close/abandon.
- Reject after table QR token rotation if joining from old QR.

### 5.7 Replay Protection

Add replay controls:

- JWT `jti` for high-risk guest actions.
- Request idempotency records scoped by actor/session/action.
- Store request hash for order/payment mutations.
- Reject idempotency replay if principal, session, branch, or request hash differs.
- WebSocket connection nonce for reconnect handshakes.

## 6. Branch Isolation Hardening

### 6.1 Branch-Scoped SQL Standard

For any branch-owned resource, mutations should include branch scope in SQL.

Examples:

```text
UPDATE orders
SET status = $3
WHERE id = $1 AND branch_id = $2
RETURNING *

UPDATE menu_items
SET ...
WHERE id = $1 AND branch_id = $2
RETURNING *

DELETE FROM item_modifiers
USING menu_items
WHERE item_modifiers.id = $1
  AND item_modifiers.item_id = menu_items.id
  AND menu_items.branch_id = $2
```

The repository should expose scoped methods and avoid unscoped mutation helpers for branch-owned resources.

### 6.2 Critical Current Branch Isolation Fix Targets

Based on current code:

- `OrderHandler.UpdateStatus` must verify order branch against staff branch.
- `AssistanceHandler.Acknowledge/Resolve` must verify assistance request branch against staff branch.
- `MenuAdminHandler.UpdateItem/ToggleAvailability/ToggleFeatured/AddModifier/DeleteModifier` must verify target object branch, not request body/query branch.
- `PromoHandler.List/Create/Deactivate` must compare staff branch and role.
- `CustomerService.GetCustomerHistory` should apply `restaurant_id` or future organization scope in SQL, not only service parameter.
- `PaymentHandler.InitiatePayment` must verify the guest credential/session branch and server bill.
- `BillingHandler.GetBill` must require guest or staff authorization for the session.

### 6.3 Redis Namespacing

Current keys:

- `menu:{branch_id}`
- `presence:{session_id}`
- `session:{session_id}:events`
- `staff:token:{token}`
- `staff:tokens:{staff_id}`
- `worker:{name}:lock`

Target keys:

```text
org:{org_id}:branch:{branch_id}:menu:v{version}
org:{org_id}:branch:{branch_id}:session:{session_id}:presence
org:{org_id}:branch:{branch_id}:session:{session_id}:events
org:{org_id}:branch:{branch_id}:staff:{staff_id}:sessions
staff_session:{session_id}
platform_session:{session_id}
worker:{region}:{job}:lock
idempotency:{scope}:{actor}:{key}
```

Why:

- Reduces accidental cross-tenant key collisions.
- Improves debugging.
- Allows targeted cache invalidation.
- Makes Redis data understandable during incidents.

### 6.4 Upload Isolation

Current uploads:

- Menu item images use `menu/{branch_id}/{item_id}/{uuid}.{ext}`.
- Restaurant logo uses `restaurants/{restaurant_id}/logo.{ext}`.

Target:

```text
orgs/{org_code}/branches/{branch_code}/menu-items/{item_id}/{asset_id}.{ext}
orgs/{org_code}/brand/logo/{asset_id}.{ext}
orgs/{org_code}/branches/{branch_code}/exports/{export_id}.{ext}
```

Upload presign must:

- Verify actor policy.
- Include organization/branch scope in object key.
- Store asset metadata in DB before or after upload confirmation.
- Avoid overwriting stable logo path without versioning.
- Audit presign and asset attach events.

### 6.5 Promo Isolation

Promos remain branch operational resources by default.

Organization-level promo campaigns can be added later as templates:

```text
organization_promo_campaigns
branch_promos generated from campaign
```

Do not make branch promos silently cross-branch until redemption, cap, and reporting semantics are designed.

## 7. Super Admin Architecture

### 7.1 Separate Platform Namespace

Add routes under:

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

Use only `PlatformAuth` middleware.

Never let staff/organization tokens pass platform middleware.

### 7.2 Platform Roles

Recommended platform roles:

- `super_admin`: full platform control.
- `support_admin`: read diagnostics, limited safe support actions.
- `billing_admin`: subscription and billing operations only.
- `read_only_auditor`: audit and metadata only.

Require action-level policy even inside platform routes.

### 7.3 Provisioning Flows

Organization provisioning:

1. Platform admin creates organization.
2. Platform admin creates first restaurant/profile if compatibility table remains.
3. Platform admin creates initial branch.
4. Platform admin assigns organization owner by email.
5. Owner accepts invite and sets auth.
6. Owner/manager creates branch staff.
7. All steps audited.

Branch creation:

1. Verify organization active.
2. Generate branch code.
3. Create branch with timezone, address, operational settings.
4. Create default order prefix.
5. Create default branch settings.
6. Optionally clone menu from another branch through an audited copy flow.
7. Create initial tables only through organization/branch admin action.

Owner assignment:

- Platform may assign the initial owner.
- Later owner changes should require organization owner action or platform break-glass.
- Removing the final owner must be blocked unless platform super admin explicitly transfers ownership.

### 7.4 Break-Glass Support

Platform support access to tenant data must be explicit:

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

- Default support access is read-only.
- Write actions require elevated role and reason.
- Every read/write during break-glass includes `support_session_id` in audit.
- Tenant-visible audit should show platform support access.

## 8. Audit Logging Architecture

### 8.1 Audit Requirements

Audit must be immutable, queryable, and operationally useful.

Track:

- Login success/failure.
- Staff creation, role changes, PIN reset, deactivation.
- Organization membership invites, role changes, removals.
- Branch creation, suspension, settings changes.
- Promo create/deactivate/redeem.
- Payment request, settlement, webhook receipt, webhook verification failure, refund.
- Session create/join/close/abandon.
- QR token rotation.
- Menu category/item/modifier create/update/delete.
- Theme/logo changes.
- Billing/tax/service charge changes.
- Customer opt-in/delete/export.
- Analytics/export creation.
- Platform support access.

### 8.2 Audit Writer Design

Create a single audit writer service:

```text
Audit.Record(ctx, AuditEvent)
```

Audit event fields:

- Actor.
- Scope.
- Resource.
- Action.
- Before/after snapshots for admin/security changes.
- Request ID from `middleware.RequestID`.
- IP and user-agent.
- Idempotency key if present.
- Result: success/failure.
- Error code if failed.

Audit writes for committed mutations should happen in the same DB transaction where possible.

For login failures and rejected auth attempts, write audit separately with rate limiting to avoid log floods.

### 8.3 Immutable Storage

DB rules:

- No update/delete application paths.
- Restrict DB permissions so app role can insert/select but not update/delete audit rows.
- Optional monthly partitions by `created_at`.
- Optional archive to object storage.

Application rules:

- No `UpdateAuditLog`.
- No `DeleteAuditLog`.
- Redact secrets and raw PINs.
- Avoid storing full payment PAN or provider secrets.

### 8.4 Tenant Visibility

Branch managers:

- See branch audit for operational actions in their branch.

Organization owners/admins:

- See organization audit across branches.

Platform auditors:

- See platform audit.

Guests:

- Do not get audit log access.

## 9. Session and Realtime Hardening

### 9.1 Session Creation Correctness

Current blocker:

- `CreateSession` checks active session before transaction and lacks a partial unique active-session constraint.

Target:

- Enforce one active session per table in DB.
- Lock table row or rely on partial unique index.
- Create session, participant, table occupancy, and audit event in one transaction.
- Return signed guest credential.

Recommended DB invariant:

```text
unique active session per table:
  unique(table_id) where status = 'active'
```

Application behavior:

- If conflict occurs, return existing active session join path or `TABLE_OCCUPIED`.
- Do not create duplicate sessions under concurrent QR scans.

### 9.2 Guest Session HTTP Boundary

Guest routes should become:

```text
POST /sessions
POST /sessions/:id/join
GET /sessions/:id
DELETE /sessions/:id
GET /sessions/:id/cart
POST /sessions/:id/cart/items
DELETE /sessions/:id/cart/items/:item_id
POST /sessions/:id/orders
GET /sessions/:id/orders
POST /sessions/:id/assist
GET /sessions/:id/bill
POST /sessions/:id/payments
GET /sessions/:id/snapshot
```

All except create/join must require `GuestSessionAuth` or staff/organization policy where appropriate.

Rules:

- Derive session ID from route and credential, then compare.
- Derive participant ID from credential.
- Derive branch ID from session.
- Reject closed/abandoned session.
- Reject participant revoked from session.

### 9.3 WebSocket Auth Redesign

Current blocker:

- WebSocket accepts query `session_id` and `participant_id`.

Target:

Handshake:

- Client sends guest token through a supported channel:
  - `Sec-WebSocket-Protocol: qr-dining.guest.v1, token...`, or
  - short-lived `ws_ticket` created by authenticated HTTP endpoint.

Preferred for browser compatibility:

1. `POST /sessions/:id/ws-ticket` with guest token.
2. Backend returns one-time ticket with 30 second TTL and bound `session_id`, `participant_id`, `branch_id`, `jti`.
3. WebSocket connects to `/ws?ticket=...`.
4. Ticket is consumed once in Redis.

This avoids long-lived credentials in query strings.

### 9.4 Event Sequencing

Current WebSocket events are ephemeral and reconnect uses snapshot.

Target:

- Add per-session event sequence numbers.
- Envelope includes:

```text
event_id
sequence
organization_id
branch_id
session_id
event
payload
timestamp
```

Implementation options:

- Store sequence in DB session event stream table.
- Or use Redis `INCR` plus durable audit/event table for critical events.

Recommended:

- Use DB-backed `session_events` for operational events that clients may need to replay.
- Keep Redis Pub/Sub as delivery bus only.
- On reconnect, client provides `last_sequence`.
- Server can return missed events or authoritative snapshot.

### 9.5 Presence and Occupancy Correctness

Presence:

- Redis presence remains live view only.
- DB `last_seen_at` updates should be rate-limited to avoid write pressure.
- Presence key must include organization and branch.

Occupancy:

- Table status must be reconciled from active sessions.
- Stale session cleanup must abandon session and set table available in one transaction.
- Staff should see conflict state if table occupied without active session.
- Add reconciliation worker that detects:
  - occupied table with no active session
  - active session with available table
  - duplicate active sessions

### 9.6 Cart Ownership Correctness

Current carts are keyed by `(session_id, participant_id)` but participant ID is client supplied.

Target:

- Derive participant from guest actor.
- Verify cart belongs to participant and session.
- Shared cart mode should be explicit branch setting, not implicit nullable participant behavior.
- If shared cart is enabled, operations should still record actor participant.
- Cart item delete must verify cart ownership or shared cart permission.

## 10. Payment and Order Correctness

### 10.1 Order Creation Boundary

Current blocker:

- `PlaceOrder` accepts client branch and participant IDs.
- It does not verify all menu items belong to session branch.
- Idempotency is globally keyed and not request-bound.

Target:

- Request body excludes `branch_id` and `placed_by_participant_id`.
- Service derives:
  - session from route
  - branch from session
  - participant from guest actor
- Validate participant belongs to session.
- Validate menu items and modifiers belong to session branch.
- Validate item availability at order time.
- Store request hash for idempotency.

### 10.2 Idempotency Model

Add a general table:

```text
idempotency_keys
  id bigserial
  scope_type text not null
  scope_id text not null
  actor_type text not null
  actor_id text not null
  key text not null
  request_hash text not null
  response_resource_type text
  response_resource_id text
  status enum(in_progress, completed, failed)
  created_at timestamptz
  expires_at timestamptz
  unique(scope_type, scope_id, actor_type, actor_id, key)
```

For orders:

- Scope is `session:{session_id}`.
- Actor is `participant:{participant_id}`.
- Store normalized request hash.
- If same key and same hash: return original order.
- If same key and different hash: return 409 idempotency conflict.

For payments:

- Scope is `session:{session_id}`.
- Actor is guest/staff depending action.
- Same request hash requirement.

### 10.3 Order State Machine

Current order state machine is good foundation:

```text
pending -> confirmed -> preparing -> ready -> served
pending/confirmed -> cancelled
```

Hardening:

- Policy controls who can perform each transition.
- SQL update includes current status and branch:

```text
UPDATE orders
SET status = $new
WHERE id = $id
  AND branch_id = $branch_id
  AND status = $expected_current
RETURNING *
```

- This prevents stale clients from overwriting transitions.
- Audit includes old/new status and staff actor.

### 10.4 Branch Order Numbering

Current numbering:

- `order_sequences(branch_id, date, last_seq)`
- Date generated using branch timezone.
- `order_prefix + seq`.
- Starts at 1001.

Target:

- Keep branch-local numbering.
- Store `order_business_date` on orders.
- Enforce uniqueness:

```text
unique(branch_id, order_business_date, order_number)
```

- Decide format:

```text
{branch_order_prefix}-{YYYYMMDD}-{seq}
```

or keep display compact:

```text
OR1001
```

Recommended:

- Keep compact display for kitchen.
- Store separate searchable operational ID:

```text
order_number_display = "OR1001"
order_business_date = "2026-05-21"
order_operational_id = "BLR-INDIRANAGAR-20260521-OR1001"
```

Daily reset:

- Based on branch timezone.
- Sequence row locked by `INSERT ... ON CONFLICT DO UPDATE`.
- Works under concurrency.

### 10.5 Payment Lifecycle

Current blocker:

- Payment initiation creates pending payment.
- Webhook is unauthenticated.
- Cash/card/UPI UI can show success before backend completion.

Target payment states:

```text
requested
provider_pending
requires_staff_confirmation
completed
failed
cancelled
refunded
partially_refunded
```

Payment methods:

- `cash`: request created by guest, completed by staff/cashier.
- `card_manual`: request created by guest/staff, completed by staff/cashier after POS confirmation.
- `digital`: provider intent/order created, completed by verified webhook.
- `upi`: if provider-backed, treat as digital; if manual, treat as staff-confirmed.

Payment flow:

1. Compute authoritative bill server-side.
2. Lock session/orders or create bill snapshot.
3. Create payment request with amount snapshot.
4. Staff or provider completes payment.
5. On completion, close session in same transaction or transactional outbox.
6. Mark table available.
7. Audit payment completion.
8. Publish realtime event after commit.

### 10.6 Webhook Security

Webhook handler must:

- Verify provider signature using raw body.
- Validate timestamp tolerance.
- Store raw payload and headers.
- Use external provider IDs, not internal payment IDs from payload.
- Match provider payment/order ID to internal payment record.
- Verify amount, currency, branch/session linkage.
- Mark webhook processed after success/failure with correct webhook row ID.
- Return 2xx only for accepted duplicate or processed events; alert on invalid signatures.

### 10.7 Bill Snapshot and Settlement

Payment should not rely on recomputing a mutable bill after completion.

Add bill snapshot:

```text
bill_snapshots
  id
  session_id
  branch_id
  subtotal
  discount_amount
  tax_amount
  service_charge
  tip_amount
  total
  currency
  source_order_ids jsonb
  created_by_actor
  created_at
```

Payment references `bill_snapshot_id`.

Rules:

- If new orders are placed after a payment request, existing payment snapshot remains valid but may no longer settle the full session.
- Session auto-close only when paid amount covers all non-cancelled orders in latest settlement calculation.
- Staff can see unpaid delta.

### 10.8 Promo Redemption Safety

Current promo validation counts redemptions then inserts later.

Target:

- Promo validation at preview is informational.
- Final redemption occurs inside order/payment transaction.
- Lock promo row or use DB-enforced counters.
- Per-phone limits require verified/normalized customer phone or explicit guest-provided phone at final step.
- Cap checks must be atomic.
- Audit redemption and failure reasons.

## 11. Operational UX Governance

### 11.1 Human-Readable Identifiers

Operational screens should avoid exposing raw UUIDs except diagnostics.

Add:

- `organization.code`
- `branch.code`
- `staff.staff_code`
- `sessions.session_number` or `visit_number`
- `orders.order_operational_id`
- `payments.payment_reference`
- `refunds.refund_reference`
- `audit.event_reference`

Rules:

- UUID remains primary key.
- Human-readable IDs are unique within scope.
- Support/admin views can search by both.
- Guest and staff UI uses readable IDs.

### 11.2 Branch and Organization Codes

Organization code:

- Short, globally unique, uppercase slug.
- Example: `ACME-HOSPITALITY`.

Branch code:

- Unique within organization.
- Stable.
- Example: `BLR-INDIRANAGAR`.

Operational ID examples:

```text
Session: BLR-INDIRANAGAR-S-20260521-018
Order:   BLR-INDIRANAGAR-20260521-OR1042
Payment: BLR-INDIRANAGAR-PAY-20260521-0031
```

### 11.3 Observability

Metrics should be scope-aware without high-cardinality labels.

Good labels:

- route
- status_code
- action
- actor_type
- payment_provider
- worker_name
- event_type

Avoid labels:

- raw session_id
- participant_id
- order_id
- customer phone

Logs should include:

- request_id
- actor type/id
- organization_id
- branch_id
- session_id where relevant
- action
- decision allow/deny reason

### 11.4 Admin Visibility Boundaries

Branch staff:

- Default branch only.
- No cross-branch data unless policy grants multi-branch access.

Organization admins:

- Cross-branch dashboards and reports.
- Must choose branch scope before operational mutation.

Platform admins:

- Platform namespace only.
- Read/write support actions visible in audit.

## 12. Migration Strategy

### 12.1 Migration Principles

Do not big-bang rewrite auth and tenancy.

Use staged compatibility:

- Add nullable columns first.
- Backfill.
- Dual-read where needed.
- Dual-write where safe.
- Add constraints after backfill and code rollout.
- Keep existing API route compatibility briefly.
- Add policy layer before broad route rewrites.
- Feature-flag strict enforcement until clients are ready.

### 12.2 Suggested Feature Flags

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

Flags should only gate rollout behavior. They should not become permanent product configuration.

### 12.3 Phase-Compatible Route Evolution

Keep existing routes temporarily:

- `/staff/auth`
- `/branches/:id/...`
- `/sessions/:id/...`
- `/ws`

Add new behavior behind middleware:

- Existing `/staff/auth` can accept both old and new payloads during migration.
- Existing guest endpoints can accept legacy `X-Participant-ID` only when flag disabled.
- Emit deprecation metrics and audit warnings for legacy identity use.

Do not introduce frontend changes during this planning session. Future implementation should update frontend only after backend supports both old and new auth.

### 12.4 Schema Evolution Order

Recommended order:

1. Add organization tables and nullable foreign keys.
2. Backfill one organization per existing restaurant.
3. Add branch `organization_id` nullable, backfill from restaurant, then not null.
4. Add branch and organization codes.
5. Add user/platform identity tables.
6. Add staff code, pin_version, token/session version fields.
7. Add staff_sessions table.
8. Add guest credential version/revocation fields.
9. Add audit_log v2.
10. Add idempotency_keys table.
11. Add bill_snapshots and payment lifecycle fields.
12. Add session/order operational IDs.
13. Add unique active session per table constraint after dedupe.
14. Add scoped uniqueness constraints after backfill.

### 12.5 Backfill Strategy

Organizations:

- For each restaurant, create organization:
  - `name = restaurants.name`
  - `code = normalized restaurants.slug`
  - `status = active`
- Set `restaurants.organization_id`.
- Set each branch `organization_id`.

Branch codes:

- Generate from branch name/timezone/address or fallback:
  - `BR-{branch_id}`
- Allow admin cleanup later.

Staff codes:

- Generate deterministic readable codes:
  - owner: `OWNER{staff_id}`
  - manager: `MGR{staff_id}`
  - waiter: `WAITER{staff_id}`
  - kitchen: `KITCHEN{staff_id}`
- Require rotation/confirmation in admin UI later.

Operational IDs:

- New records get IDs immediately.
- Existing records can be backfilled best-effort from branch/order date/id.

### 12.6 Constraint Rollout

Before adding strict constraints:

- Detect duplicate active sessions per table.
- Detect menu item/category branch mismatches.
- Detect orders whose branch differs from session branch.
- Detect assistance table/session branch mismatches.
- Detect promo redemptions with inconsistent branch/order.
- Detect payments with no valid session.

Only after cleanup:

- Add foreign keys/check constraints.
- Add partial unique indexes.
- Convert nullable scope columns to not null.

### 12.7 Rollback Strategy

Every wave should be rollback-conscious:

- Additive migrations first.
- Avoid dropping columns until final cleanup.
- Keep old token validation temporarily.
- Keep old route payload compatibility behind feature flags.
- Make strict policy enforcement observable before mandatory.
- Use canary branch/organization rollout.

## 13. Implementation Waves

### Wave 0 - Baseline Re-Audit and Guardrails

Goal: freeze risky expansion and establish acceptance criteria.

Work:

- Confirm all current critical audit findings with tests or reproduction notes.
- Add a route/resource ownership matrix.
- Add feature flags.
- Add metrics for legacy participant header usage and legacy staff auth.
- Add integration test scaffolding for branch mismatch cases.

Exit criteria:

- Team agrees on policy names and target scopes.
- No production feature work depends on unsafe identity assumptions.

### Wave 1 - Identity Hardening

Dependencies: Wave 0.

Work:

- Add staff codes and active-staff-only lookup.
- Add staff session DB table and token versioning.
- Redesign staff auth to `branch_code + staff_code + PIN`.
- Add guest credential issuing for create/join.
- Add guest auth middleware in permissive mode.
- Add audit for login success/failure.

Exit criteria:

- New staff login is unambiguous.
- Guest credential can authenticate all session operations in shadow/permissive mode.
- Legacy participant ID usage is measurable.

### Wave 2 - Central RBAC and Ownership Validation

Dependencies: Wave 1.

Work:

- Add `authz` package.
- Define actor/resource/action model.
- Convert high-risk routes first:
  - orders status
  - assistance ack/resolve
  - menu item/category/modifier mutations
  - promo management
  - staff management
  - customer history/delete
- Add scoped repository methods.
- Add tests for cross-branch denial.

Exit criteria:

- No high-risk staff mutation authorizes from client-supplied branch ID.
- Policy denials are audited and observable.

### Wave 3 - Organization Model

Dependencies: Wave 2 can run partially in parallel after policy model is stable.

Work:

- Add organizations.
- Backfill from restaurants.
- Add organization membership.
- Add organization/branch code.
- Add organization admin auth.
- Add organization route namespace.
- Add cross-branch read-only analytics planning endpoints.

Exit criteria:

- Existing single-restaurant deployments map cleanly to one organization.
- Branch-first staff UX still works.
- Organization admins can govern multiple branches without becoming platform admins.

### Wave 4 - Super Admin Trust Domain

Dependencies: Wave 3.

Work:

- Add platform user tables.
- Add `/platform` route namespace.
- Add `PlatformAuth`.
- Add provisioning APIs.
- Add support/break-glass session model.
- Add platform audit events.

Exit criteria:

- Platform admin cannot authenticate through staff/org middleware.
- Organization owner cannot call platform routes.
- Provisioning creates organization, branch, and initial owner through audited flows.

### Wave 5 - Audit Logging V2

Dependencies: Wave 1 for actor model, Wave 2 for policy decisions.

Work:

- Add `audit_log` v2.
- Add audit writer.
- Wire high-risk actions.
- Preserve current `event_log` for session timeline.
- Add branch/org/platform audit read APIs with policy.

Exit criteria:

- Security-sensitive events are immutable and scoped.
- Audit reads enforce visibility boundaries.

### Wave 6 - Realtime and Session Hardening

Dependencies: Wave 1 guest credentials.

Work:

- Add active-session uniqueness invariant.
- Fix stale cleanup to release tables transactionally.
- Add WebSocket ticket auth.
- Add branch/org-scoped Redis keys.
- Add event sequence numbers.
- Add reconnect replay or snapshot-with-sequence.
- Add occupancy reconciliation worker.

Exit criteria:

- No duplicate active sessions per table.
- WebSocket cannot be joined with spoofed participant query params.
- Stale sessions do not strand occupied tables.

### Wave 7 - Payment and Order Correctness

Dependencies: Wave 1 guest auth, Wave 2 authz.

Work:

- Remove client branch/participant trust from order creation.
- Add scoped idempotency table and request hash.
- Add bill snapshots.
- Redesign payment lifecycle.
- Add staff settlement for cash/manual card/UPI.
- Add provider signature verification.
- Add webhook processing correctness.
- Add refund policy and audit.

Exit criteria:

- Payment completion means backend-settled payment.
- Webhooks cannot be forged.
- Orders cannot cross session/branch/menu boundaries.

### Wave 8 - Operational UX Cleanup

Dependencies: Waves 3, 6, 7.

Work:

- Add human-readable IDs in APIs.
- Reduce UUID exposure in staff/guest UI.
- Add branch/organization selectors for authorized org users.
- Add operational diagnostics screens.
- Add support search by order/session/payment reference.

Exit criteria:

- Staff can operate without raw UUIDs.
- Support can trace incidents across org/branch/session/payment using readable references.

### Wave 9 - Final Re-Audit and Production Gate

Dependencies: all waves.

Work:

- Re-run operational correctness audit.
- Run integration tests for auth, branch isolation, sessions, payments, promos, websockets.
- Run Playwright sweeps for migrated flows.
- Verify audit coverage.
- Verify metrics/logging.
- Verify rollback flags disabled or removed.

Exit criteria:

- No critical/high auth or tenancy findings remain.
- Production readiness sign-off includes security, operational, and migration checks.

## 14. Route-Level Migration Map

### 14.1 Staff Auth

Current:

```text
POST /staff/auth
body: branch_id + pin
```

Target:

```text
POST /staff/auth
body: branch_code + staff_code + pin
```

Compatibility:

- Accept legacy body only while `AUTH_STAFF_CODE_REQUIRED=false`.
- Audit legacy login.
- Remove legacy after frontend and staff onboarding are migrated.

### 14.2 Guest Session Routes

Current:

- `X-Participant-ID`.
- `placed_by_participant_id`.
- `participant_id`.

Target:

- Guest token required.
- Backend derives participant.

Compatibility:

- `GuestSessionAuth` attempts token first.
- If missing and legacy flag allows, use current participant ID with warning metric.
- Once migrated, reject legacy identifiers.

### 14.3 WebSocket

Current:

```text
GET /ws?session_id=...&participant_id=...
```

Target:

```text
POST /sessions/:id/ws-ticket
GET /ws?ticket=one_time_ticket
```

Compatibility:

- Run both briefly.
- Log legacy query auth.
- Enforce ticket auth after clients migrate.

### 14.4 Platform and Organization Routes

New:

```text
/platform/...
/orgs/:org_id/...
```

Existing staff routes remain branch-first.

Organization users should not use branch staff dashboard routes unless they have a branch staff assignment or an explicit org policy grants equivalent access.

## 15. Test Strategy

### 15.1 Integration Tests

Add tests for:

- Duplicate staff PINs do not authenticate without staff code.
- Inactive staff cannot login.
- Staff token invalid after deactivation and PIN rotation.
- Guest cannot spoof another participant.
- Guest cannot operate on another session.
- Staff branch A cannot mutate branch B order.
- Staff branch A cannot mutate branch B assistance request.
- Staff branch A cannot mutate branch B menu item by lying in body branch ID.
- Promo create/list/delete requires staff branch and manager policy.
- Duplicate active session creation is impossible under concurrency.
- Stale worker frees table.
- Order menu item branch mismatch rejected.
- Idempotency replay with different body rejected.
- Payment webhook invalid signature rejected.
- Cash payment requires staff settlement.

### 15.2 Policy Tests

Table-driven tests:

- Actor type.
- Role.
- Scope.
- Resource.
- Action.
- Expected decision.

Policy tests should be independent of Gin.

### 15.3 Realtime Tests

Tests:

- WebSocket ticket can be consumed once.
- Expired ticket rejected.
- Ticket for session A cannot join session B.
- Participant revoked cannot reconnect.
- Events include monotonic sequence.
- Reconnect with last sequence receives missed events or snapshot.

### 15.4 Migration Tests

Tests:

- Backfill creates one organization per existing restaurant.
- Branches receive organization_id and codes.
- Staff receive codes.
- Constraints can be added after cleanup.
- Legacy route compatibility works until flags flip.

## 16. Production Readiness Checklist

Before production:

- Staff identity is unambiguous.
- Guest operations require signed credentials.
- WebSocket auth is signed and replay-resistant.
- Branch-owned mutations verify loaded resource branch.
- Organization model is backfilled and enforced.
- Platform admin is separate trust domain.
- Audit log v2 captures security-sensitive events.
- Active sessions are unique per table.
- Stale sessions release tables.
- Order creation derives branch/participant server-side.
- Payments cannot be forged by webhook.
- Cash/manual payment requires staff settlement.
- Idempotency keys are scoped and request-bound.
- Redis keys are namespaced by organization/branch/session.
- Operational IDs exist for orders/payments/sessions.
- Cross-branch tests pass.
- Migration flags have clear removal plan.

## 17. Non-Goals for Initial Hardening

Do not attempt these during the first hardening waves:

- Full enterprise SSO.
- Franchise hierarchy above organization.
- Cross-branch shared inventory.
- Central kitchen routing.
- Multi-currency settlement.
- Complex promo campaigns across branches.
- Data warehouse architecture.
- Full event sourcing rewrite.

These can be added after core trust boundaries are correct.

## 18. Key Codebase Refactor Boundaries

When implementation begins, keep changes organized:

```text
backend/internal/authn        authentication token parsing/validation
backend/internal/authz        policy decisions and scope checks
backend/internal/audit        audit writer and models
backend/internal/tenancy      organization/branch resolution helpers
backend/internal/platform     platform admin services
backend/internal/services     business workflows only
backend/internal/repository   scoped data access
backend/internal/handlers     request parsing, authz calls, response mapping
```

Avoid putting policy decisions in:

- SQL query names alone.
- Frontend role checks.
- Ad hoc handler `if` statements.
- Redis token payloads without DB validation.

Frontend role checks can improve UX, but backend policy is authoritative.

## 19. Open Design Decisions

These need explicit product/security decisions before implementation:

1. Should organization users and branch staff share the same `users` identity table, or should quick-PIN staff remain independent unless linked?
2. Should owners be organization users only, branch staff only, or both with linked identities?
3. Should guests have refreshable credentials, or should they rejoin using QR/session link after expiry?
4. Which payment providers are actually in scope first, and what signature schemes must be supported?
5. Should staff settlement require cashier role or allow manager/waiter?
6. Should branch managers manage staff roles, or only organization admins/owners?
7. Should customer memory remain restaurant-level during migration or move directly to organization-level?
8. What support actions are allowed under platform break-glass?

Recommended defaults:

- Organization owners are `users` with org membership and may also have branch staff profiles only when they operate in-branch.
- Waiter/kitchen staff can remain branch staff with staff code/PIN and optional linked `user_id`.
- Customer memory moves from `restaurant_id` to `organization_id` after organization backfill, but branch access remains policy-scoped.
- Payment hardening starts with manual staff-settled cash/card plus one verified digital provider.

## 20. Summary Architecture Decision

The platform should evolve by adding governance above the current branch-first operational system, not by flattening branch boundaries into organization-wide access.

The decisive changes are:

- Introduce organizations as governance/accountability.
- Keep branches as operational isolation.
- Split platform admin into a separate trust domain.
- Replace scattered role checks with centralized policy evaluation.
- Replace PIN-only staff identity with staff code plus PIN and server-backed sessions.
- Replace client-spoofable guest identity with signed participant credentials.
- Bind all resources to organization/branch/session scopes and validate loaded ownership.
- Make audit logging immutable and actor-aware.
- Make realtime and payment flows signed, replay-resistant, and sequence-aware.

This should be delivered through staged, additive migrations with compatibility flags and cross-branch regression tests before strict enforcement.
