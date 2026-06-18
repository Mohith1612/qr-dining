-- name: GetPlatformUserByEmail :one
SELECT * FROM platform_users WHERE email = $1;

-- name: GetPlatformUserByID :one
SELECT * FROM platform_users WHERE id = $1;

-- name: ListPlatformUsers :many
SELECT * FROM platform_users ORDER BY created_at DESC, id DESC;

-- name: ListPlatformRolesForUser :many
SELECT role FROM platform_user_roles WHERE platform_user_id = $1 ORDER BY role ASC;

-- name: UpsertPlatformUser :one
INSERT INTO platform_users (email, display_name, password_hash, status, mfa_required)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    status = EXCLUDED.status,
    mfa_required = EXCLUDED.mfa_required,
    updated_at = NOW()
RETURNING *;

-- name: AddPlatformUserRole :exec
INSERT INTO platform_user_roles (platform_user_id, role)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: CreatePlatformSession :one
INSERT INTO platform_sessions (platform_user_id, token_hash, device_name, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetActivePlatformSessionByTokenHash :one
SELECT * FROM platform_sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: TouchPlatformSession :exec
UPDATE platform_sessions SET last_seen_at = NOW() WHERE id = $1;

-- name: RevokePlatformSession :exec
UPDATE platform_sessions SET revoked_at = NOW()
WHERE id = $1 AND platform_user_id = $2 AND revoked_at IS NULL;

-- name: CreatePlatformOrganization :one
INSERT INTO organizations (code, name, legal_name, primary_contact_email, settings_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPlatformOrganizations :many
SELECT * FROM organizations ORDER BY created_at DESC, id DESC;

-- name: CreatePlatformRestaurant :one
INSERT INTO restaurants (name, slug, settings_json, organization_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreatePlatformBranch :one
INSERT INTO branches (restaurant_id, organization_id, name, address, timezone, branch_code, order_prefix)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateOrganizationBranchMembership :exec
INSERT INTO organization_branch_memberships (organization_id, branch_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: CreateOrganizationMember :one
INSERT INTO organization_members (organization_id, staff_id, role, status)
VALUES ($1, $2, $3, 'active')
RETURNING *;

-- name: CreatePlatformSupportSession :one
INSERT INTO platform_support_sessions (
    platform_user_id, organization_id, branch_id, reason,
    approved_by_platform_user_id, starts_at, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPlatformSupportSessionByID :one
SELECT * FROM platform_support_sessions WHERE id = $1;

-- name: ListPlatformSupportSessions :many
SELECT * FROM platform_support_sessions
ORDER BY created_at DESC, id DESC
LIMIT 200;

-- name: InsertPlatformAuditLog :exec
INSERT INTO platform_audit_log (
    platform_user_id, action, target_type, target_id,
    organization_id, branch_id, support_session_id, request_id, payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListPlatformAuditLog :many
SELECT * FROM platform_audit_log
WHERE (sqlc.narg('platform_user_id')::BIGINT IS NULL OR platform_user_id = sqlc.narg('platform_user_id'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::TEXT IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('organization_id')::BIGINT IS NULL OR organization_id = sqlc.narg('organization_id'))
  AND (sqlc.narg('branch_id')::BIGINT IS NULL OR branch_id = sqlc.narg('branch_id'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at < sqlc.narg('to_time'))
ORDER BY created_at DESC, id DESC
LIMIT 200;
