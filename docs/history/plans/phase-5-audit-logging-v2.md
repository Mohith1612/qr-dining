# Phase 5 - Audit Logging V2

## Objective

Create immutable, actor-aware audit logging for security-sensitive and operationally accountable events.

## Dependencies

- Phase 1 actor model.
- Phase 2 policy decisions.
- Feature flag:
  - `AUDIT_LOG_V2_ENABLED`

## Current Codebase Context

Current audit primitive:

- `backend/migrations/000002_event_log.up.sql`
- `backend/internal/repository/event_log.go`
- `backend/sql/queries/event_log.sql`
- `backend/internal/handlers/event_log.go`

Current `event_log` is insert-only and branch/session scoped, but it lacks organization scope, rich actor context, before/after state, request metadata, and security risk classification.

## Target Audit Model

```text
audit_log
  id bigserial primary key
  organization_id bigint
  branch_id bigint
  restaurant_id bigint nullable
  session_id uuid nullable
  table_id bigint nullable
  resource_type text not null
  resource_id text not null default ''
  action text not null                          -- resource.verb convention (e.g. staff.login, menu.item.update)
  result enum(success, failure, denied)         -- explicit outcome; queryable/filterable
  actor_type enum(platform_user, organization_user, staff, guest, system, webhook)
  actor_id text not null default ''
  actor_display text not null default ''
  actor_scope_json jsonb not null default '{}'
  request_id text not null default ''
  correlation_id text not null default ''       -- groups multi-request logical operations
  idempotency_key text not null default ''
  ip text not null default ''                   -- stored as text; no INET type
  user_agent text not null default ''
  source enum(web, mobile, pwa, api, webhook, system)  -- inferred from request headers
  before_json jsonb
  after_json jsonb
  metadata_json jsonb not null default '{}'
  risk_level enum(low, medium, high, critical)
  row_hash text                                 -- reserved for future tamper-evident chaining
  previous_hash text                            -- reserved for future tamper-evident chaining
  created_at timestamptz not null default now()
```

Keep current `event_log` for session operational timeline during migration. Route security/accountability events to `audit_log`.

No foreign keys on `audit_log` — rows must survive org/branch deletion.

Retention note: expect ~10K rows/day at scale. Partition by RANGE(created_at) monthly when table exceeds ~10M rows.

## Required Audit Events

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

## Audit Writer

Create:

```text
backend/internal/audit
  event.go       -- AuditEvent struct; ActorType, RiskLevel, ResultType, SourceType; action constants
  writer.go      -- Writer.Record; feature-flag gated; failure increments AuditWriteFailuresTotal metric
  redaction.go   -- Recursive Redact(before, after) for PINs, tokens, secrets, payment fields
  middleware.go  -- AuditRequestContext (IP, UA, RequestID, CorrelationID, Source); Middleware() and FromContext()
```

Use:

```text
auditWriter.Record(ctx, AuditEvent)
```

Rules:

- Write audit rows after primary mutation commits (fire-and-forget, non-fatal).
- Write denied auth/policy attempts with result=denied.
- Redact raw PINs, tokens, secrets, and payment-sensitive fields recursively.
- Include request_id, correlation_id, ip, user_agent, source, actor, scope, resource, action, and result.
- Audit write failures increment `audit_write_failures_total` Prometheus counter and log structured error.
- Action names follow `resource.verb` convention — no SCREAMING_SNAKE, no mixed styles.

## Visibility Boundaries

Branch managers:

- Branch audit for their branch.

Organization owners/admins:

- Organization audit across branches.

Platform auditors:

- Platform audit and support activity.

Guests:

- No audit log access.

## Exit Criteria

- Security-sensitive actions are represented in immutable audit log.
- Audit reads enforce branch/organization/platform visibility boundaries.
- Existing session event timeline remains functional.

## Implementation Status

Implemented in commit `00ce53d`, with follow-up correctness fixes completed in commits `ff5032b` and `711f818`:

- [x] Migration 000019 applied (schema, enums, trigger, indexes)
- [x] sqlc generated (`audit_log.sql.go`, updated `models.go`, `querier.go`)
- [x] `backend/internal/audit/` package added
- [x] `AuditWriteFailuresTotal` metric added to observability
- [x] Repository audit_log methods added
- [x] Server wiring complete
- [x] Handler event wiring added for several high-value actions
- [x] Visibility endpoints added (`GET /branches/:id/audit`, `GET /orgs/:org_id/audit`, platform audit endpoint work started)
- [x] Tests compile and pass
- [x] Follow-up correctness fixes below completed: audit read authorization, platform Audit V2 read path and self-auditing, organization `source` filtering, recursive array redaction, and focused regression tests.

## Review Findings and Required Fixes

### 1. Audit read authorization is too broad

Status: Fixed in commit `ff5032b`.

Original implementation:

- `GET /branches/:id/audit` allows any staff member whose `StaffSession.BranchID` matches the path branch.
- `GET /orgs/:org_id/audit` allows any staff member whose `StaffSession.OrganizationID` matches the path organization.

Why this is inconsistent:

- The visibility boundary in this plan says branch audit is for branch managers.
- Organization audit is for organization owners/admins.
- Waiter/kitchen staff should not be able to read security/accountability logs just because they belong to the same branch or organization.

Fix completed:

- Branch audit reads now require same branch plus centralized authz role `owner` or `manager`.
- Organization audit reads now require active organization membership with role `owner` or `admin`; they no longer trust only `StaffSession.OrganizationID`.
- Authz tests prove waiter/kitchen staff are denied branch audit reads and cross-branch owners are denied.

### 2. Platform audit endpoint still reads the Phase 4 audit table

Status: Fixed in commit `ff5032b`.

Original implementation:

- `/platform/audit` still reads `platform_audit_log` through `ListPlatformAuditLog`.
- Phase 5 added the unified `audit_log` and `ListAuditLogPlatform`, but the platform route is not using it.

Why this is inconsistent:

- Platform auditors should see Phase 5 audit records, including platform support access and cross-tenant security/accountability events.
- Reading only `platform_audit_log` misses most Audit V2 rows.

Fix completed:

- `/platform/audit` now reads `audit_log` through `ListAuditLogPlatform`.
- `/platform/audit` no longer writes its successful audit-read event to `platform_audit_log`; it writes an Audit V2 `audit.read` event instead.
- `platform_audit_log` remains only as a legacy/platform-operation timeline for older platform operations.
- Platform audit responses now expose Audit V2 fields: actor, scope, action, result, source, risk level, request/correlation IDs, before/after, metadata, resource, and tenant scope.
- Handler/repository tests cover Audit V2 platform filters and JSON response shape.

### 2a. Platform audit reads are not self-audited into Audit V2

Status: Fixed in commit `711f818`.

Original implementation:

- `/platform/audit` read from Audit V2 but recorded successful audit reads only through `logPlatformAudit`, which writes to legacy `platform_audit_log`.

Fix completed:

- Successful `/platform/audit` reads now call `audit.Writer.Record` with `ResourceAuditLog`, `ActionAuditRead`, `ActorTypePlatformUser`, platform actor ID/display, `ResultSuccess`, and `RiskLow`.
- The legacy platform audit write was removed for this endpoint.
- Handler tests cover the Audit V2 event fields used for platform audit read self-auditing.

### 3. Organization audit query is missing source filtering

Status: Fixed in commit `ff5032b`.

Original implementation:

- Branch audit and platform audit list queries support `source`.
- Organization audit list query does not filter by `source`.

Why this is inconsistent:

- The test requirements say `source` must be filterable in all three list queries.

Fix completed:

- Added `source` filter to `ListAuditLogForOrganization`.
- Wired `source` query param in `GetOrgAuditLog`.
- Regenerated sqlc.
- Added repository test coverage for organization audit source filtering.

### 4. Redaction does not recurse into arrays

Status: Fixed in commit `ff5032b`.

Original implementation:

- `Redact` recursively walks JSON objects.
- It does not recurse into arrays containing objects.

Why this is inconsistent:

- The plan requires recursive redaction for PINs, tokens, secrets, and payment-sensitive fields.
- Sensitive payloads are often arrays of objects, such as payment attempts, webhook events, selected modifiers, or nested request bodies.

Fix completed:

- Redaction now recursively traverses:
  - objects
  - arrays
  - arrays containing nested objects/arrays
- Scalar values remain unchanged unless their object key is sensitive.
- Tests cover arrays of objects containing `token`, `password`, `card_number`, `cvv`, and nested secrets.

### 5. Phase 5 completion status should remain open until fixes land

Status: Fixed in commit `ff5032b`.

Current verification after follow-up fixes:

- `cd backend && go tool sqlc generate` completed.
- `cd backend && GOCACHE=/tmp/go-build go test ./internal/audit ./internal/authz ./internal/handlers ./internal/repository` passes.
- `cd backend && GOCACHE=/tmp/go-build go test ./...` passes.
- `cd backend && GOCACHE=/tmp/go-build go vet ./...` passes.
- After commit `711f818`, `cd backend && GOCACHE=/tmp/go-build go test ./internal/handlers`, `go test ./...`, and `go vet ./...` pass.

Interpretation:

- Phase 5 Audit Logging V2 is complete for the backend scope described in this plan.
- Remaining future work is operational, such as retention partitioning after table volume warrants it.

## Test Requirements

- Audit rows cannot be updated/deleted by application code paths (DB trigger).
- Staff deactivation writes actor/scope/resource/action/result=success.
- Policy denial writes result=denied without leaking secrets.
- Login failure writes result=failure.
- Platform support access writes result=success with risk_level=critical.
- Organization admin cannot read another organization's audit log.
- `correlation_id` propagates across related events.
- Audit read itself writes an `audit.read` event (self-auditing).
- Redaction handles nested JSON structures.
- `source` field is filterable in all three list queries.
