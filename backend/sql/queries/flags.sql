-- name: ListFeatureFlags :many
SELECT * FROM platform_feature_flags ORDER BY key;

-- name: GetFeatureFlag :one
SELECT * FROM platform_feature_flags WHERE key = $1;

-- name: CreateFeatureFlag :one
INSERT INTO platform_feature_flags (key, name, description, default_enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateFeatureFlag :one
UPDATE platform_feature_flags
SET name = $2, description = $3, default_enabled = $4, updated_at = NOW()
WHERE key = $1
RETURNING *;

-- name: ListGlobalFlagOverrides :many
SELECT * FROM platform_flag_global_overrides ORDER BY flag_key;

-- name: UpsertGlobalFlagOverride :one
INSERT INTO platform_flag_global_overrides (flag_key, enabled, updated_by_platform_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (flag_key)
DO UPDATE SET enabled = EXCLUDED.enabled,
              updated_by_platform_user_id = EXCLUDED.updated_by_platform_user_id,
              updated_at = NOW()
RETURNING *;

-- name: DeleteGlobalFlagOverride :exec
DELETE FROM platform_flag_global_overrides WHERE flag_key = $1;

-- name: ListOrganizationFlagOverrides :many
SELECT * FROM platform_flag_organization_overrides WHERE organization_id = $1 ORDER BY flag_key;

-- name: UpsertOrganizationFlagOverride :one
INSERT INTO platform_flag_organization_overrides (organization_id, flag_key, enabled, reason, updated_by_platform_user_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id, flag_key)
DO UPDATE SET enabled = EXCLUDED.enabled, reason = EXCLUDED.reason,
              updated_by_platform_user_id = EXCLUDED.updated_by_platform_user_id, updated_at = NOW()
RETURNING *;

-- name: DeleteOrganizationFlagOverride :exec
DELETE FROM platform_flag_organization_overrides WHERE organization_id = $1 AND flag_key = $2;

-- name: ListBranchFlagOverrides :many
SELECT * FROM platform_flag_branch_overrides WHERE branch_id = $1 ORDER BY flag_key;

-- name: UpsertBranchFlagOverride :one
INSERT INTO platform_flag_branch_overrides (branch_id, flag_key, enabled, reason, updated_by_platform_user_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (branch_id, flag_key)
DO UPDATE SET enabled = EXCLUDED.enabled, reason = EXCLUDED.reason,
              updated_by_platform_user_id = EXCLUDED.updated_by_platform_user_id, updated_at = NOW()
RETURNING *;

-- name: DeleteBranchFlagOverride :exec
DELETE FROM platform_flag_branch_overrides WHERE branch_id = $1 AND flag_key = $2;
