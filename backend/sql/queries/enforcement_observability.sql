-- Enforcement observability: read-only aggregates that surface where enforcement
-- WOULD bite if it were turned on. Observe-only — nothing here gates any path.

-- name: ListSubscriptionsForObservability :many
SELECT os.organization_id, o.code AS org_code, o.name AS org_name, os.status,
       sp.tier AS plan_tier, os.trial_ends_at, os.expires_at
FROM organization_subscriptions os
JOIN organizations o ON o.id = os.organization_id
JOIN subscription_plans sp ON sp.id = os.plan_id
ORDER BY o.name;

-- name: ListOrgResourceCounts :many
SELECT o.id AS organization_id, o.code, o.name,
  (SELECT COUNT(*) FROM branches b WHERE b.organization_id = o.id)::bigint AS branch_count,
  (SELECT COUNT(*) FROM staff s JOIN branches b ON b.id = s.branch_id
     WHERE b.organization_id = o.id AND s.is_active)::bigint AS staff_count,
  COALESCE((
    SELECT MAX(c) FROM (
      SELECT COUNT(*) AS c FROM tables t JOIN branches b ON b.id = t.branch_id
      WHERE b.organization_id = o.id GROUP BY t.branch_id
    ) per_branch
  ), 0)::bigint AS max_tables_per_branch
FROM organizations o
ORDER BY o.name;

-- name: ListFlagOverrideCounts :many
SELECT flag_key, 'global'::text AS scope, COUNT(*)::bigint AS n FROM platform_flag_global_overrides GROUP BY flag_key
UNION ALL
SELECT flag_key, 'organization'::text AS scope, COUNT(*)::bigint AS n FROM platform_flag_organization_overrides GROUP BY flag_key
UNION ALL
SELECT flag_key, 'branch'::text AS scope, COUNT(*)::bigint AS n FROM platform_flag_branch_overrides GROUP BY flag_key;

-- name: ListFlagCatalogKeys :many
SELECT key, name, default_enabled FROM platform_feature_flags ORDER BY key;
