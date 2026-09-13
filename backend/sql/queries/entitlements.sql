-- name: ListEntitlementCatalog :many
SELECT * FROM entitlements ORDER BY kind, key;

-- name: GetEntitlement :one
SELECT * FROM entitlements WHERE key = $1;

-- name: GetPlanByID :one
SELECT * FROM subscription_plans WHERE id = $1;

-- name: CreatePlan :one
INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
VALUES ($1, $2::plan_tier, $3::numeric, $4::jsonb)
RETURNING *;

-- name: UpdatePlan :one
UPDATE subscription_plans
SET name = $2, price_monthly = $3::numeric, features_json = $4::jsonb
WHERE id = $1
RETURNING *;

-- name: ListPlanEntitlements :many
SELECT * FROM plan_entitlements WHERE plan_id = $1 ORDER BY entitlement_key;

-- name: UpsertPlanEntitlement :one
INSERT INTO plan_entitlements (plan_id, entitlement_key, enabled, limit_value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (plan_id, entitlement_key)
DO UPDATE SET enabled = EXCLUDED.enabled, limit_value = EXCLUDED.limit_value, updated_at = NOW()
RETURNING *;

-- name: DeletePlanEntitlements :exec
DELETE FROM plan_entitlements WHERE plan_id = $1;

-- name: GetOrganizationPlanAssignment :one
SELECT * FROM organization_plan_assignments WHERE organization_id = $1;

-- name: UpsertOrganizationPlanAssignment :one
INSERT INTO organization_plan_assignments (organization_id, plan_id, status, assigned_by_platform_user_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (organization_id)
DO UPDATE SET plan_id = EXCLUDED.plan_id, status = EXCLUDED.status,
              assigned_by_platform_user_id = EXCLUDED.assigned_by_platform_user_id, updated_at = NOW()
RETURNING *;

-- name: ListOrganizationEntitlementOverrides :many
SELECT * FROM organization_entitlement_overrides WHERE organization_id = $1 ORDER BY entitlement_key;

-- name: UpsertOrganizationEntitlementOverride :one
INSERT INTO organization_entitlement_overrides
    (organization_id, entitlement_key, enabled, limit_value, reason, created_by_platform_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (organization_id, entitlement_key)
DO UPDATE SET enabled = EXCLUDED.enabled, limit_value = EXCLUDED.limit_value,
              reason = EXCLUDED.reason, created_by_platform_user_id = EXCLUDED.created_by_platform_user_id,
              updated_at = NOW()
RETURNING *;

-- name: DeleteOrganizationEntitlementOverride :exec
DELETE FROM organization_entitlement_overrides
WHERE organization_id = $1 AND entitlement_key = $2;
