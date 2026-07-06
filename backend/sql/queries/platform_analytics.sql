-- Platform (cross-tenant) analytics. On-demand aggregation; optional org filter via
-- sqlc.narg('organization_id') (NULL = platform-wide). UTC day bucketing for cross-tenant
-- consistency. Operator-facing; not entitlement-gated.

-- name: PlatformSessionsPerDay :many
SELECT DATE(s.created_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND s.created_at >= sqlc.arg('from_time')::timestamptz AND s.created_at < sqlc.arg('to_time')::timestamptz
GROUP BY day
ORDER BY day;

-- name: PlatformOrdersPerDay :many
SELECT DATE(o.created_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM orders o
JOIN branches b ON b.id = o.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND o.created_at >= sqlc.arg('from_time')::timestamptz AND o.created_at < sqlc.arg('to_time')::timestamptz
  AND o.status <> 'cancelled'
GROUP BY day
ORDER BY day;

-- name: PlatformPaymentsPerDay :many
SELECT DATE(p.completed_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM payments p
JOIN branches b ON b.id = p.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND p.completed_at >= sqlc.arg('from_time')::timestamptz AND p.completed_at < sqlc.arg('to_time')::timestamptz
  AND p.status IN ('completed', 'partially_refunded')
GROUP BY day
ORDER BY day;

-- name: PlatformParticipantJoinsPerDay :many
SELECT DATE(sp.joined_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM session_participants sp
JOIN sessions s ON s.id = sp.session_id
JOIN branches b ON b.id = s.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND sp.joined_at >= sqlc.arg('from_time')::timestamptz AND sp.joined_at < sqlc.arg('to_time')::timestamptz
GROUP BY day
ORDER BY day;

-- name: PlatformActiveBranches :one
SELECT COUNT(DISTINCT b.id)::BIGINT AS count
FROM branches b
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND EXISTS (
    SELECT 1 FROM sessions s
    WHERE s.branch_id = b.id
      AND s.created_at >= sqlc.arg('from_time')::timestamptz AND s.created_at < sqlc.arg('to_time')::timestamptz
  );

-- name: PlatformActiveDiners :one
SELECT COUNT(*)::BIGINT AS count
FROM session_participants sp
JOIN sessions s ON s.id = sp.session_id
JOIN branches b ON b.id = s.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND sp.revoked_at IS NULL
  AND s.status IN ('active', 'payment_pending', 'awaiting_reactivation');

-- name: PlatformGMV :one
SELECT COALESCE(SUM(p.amount), 0)::TEXT AS gmv
FROM payments p
JOIN branches b ON b.id = p.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND p.completed_at >= sqlc.arg('from_time')::timestamptz AND p.completed_at < sqlc.arg('to_time')::timestamptz
  AND p.status IN ('completed', 'partially_refunded');

-- name: PlatformRevenuePerDay :many
SELECT DATE(p.completed_at AT TIME ZONE 'UTC') AS day,
       COUNT(*)::BIGINT AS count,
       COALESCE(SUM(p.amount), 0)::TEXT AS revenue
FROM payments p
JOIN branches b ON b.id = p.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND p.completed_at >= sqlc.arg('from_time')::timestamptz AND p.completed_at < sqlc.arg('to_time')::timestamptz
  AND p.status IN ('completed', 'partially_refunded')
GROUP BY day
ORDER BY day;

-- name: PlatformRevenueByBranch :many
SELECT b.id AS branch_id, b.name AS branch_name, b.branch_code,
       COUNT(*)::BIGINT AS count,
       COALESCE(SUM(p.amount), 0)::TEXT AS revenue
FROM payments p
JOIN branches b ON b.id = p.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND p.completed_at >= sqlc.arg('from_time')::timestamptz AND p.completed_at < sqlc.arg('to_time')::timestamptz
  AND p.status IN ('completed', 'partially_refunded')
GROUP BY b.id, b.name, b.branch_code
ORDER BY SUM(p.amount) DESC NULLS LAST
LIMIT 100;

-- name: PlatformRevenueByOrg :many
SELECT o.id AS organization_id, o.code AS organization_code,
       COUNT(*)::BIGINT AS count,
       COALESCE(SUM(p.amount), 0)::TEXT AS revenue
FROM payments p
JOIN branches b ON b.id = p.branch_id
JOIN organizations o ON o.id = b.organization_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND p.completed_at >= sqlc.arg('from_time')::timestamptz AND p.completed_at < sqlc.arg('to_time')::timestamptz
  AND p.status IN ('completed', 'partially_refunded')
GROUP BY o.id, o.code
ORDER BY SUM(p.amount) DESC NULLS LAST
LIMIT 100;

-- name: PlatformAuthzDenialsPerDay :many
SELECT DATE(created_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM audit_log
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR organization_id = sqlc.narg('organization_id'))
  AND result = 'denied'
  AND created_at >= sqlc.arg('from_time')::timestamptz AND created_at < sqlc.arg('to_time')::timestamptz
GROUP BY day
ORDER BY day;

-- name: PlatformWebhookFailuresPerDay :many
SELECT DATE(pwe.created_at AT TIME ZONE 'UTC') AS day, COUNT(*)::BIGINT AS count
FROM payment_webhook_events pwe
LEFT JOIN payments p ON p.id = pwe.payment_id
LEFT JOIN branches b ON b.id = p.branch_id
WHERE (sqlc.narg('organization_id')::bigint IS NULL OR b.organization_id = sqlc.narg('organization_id'))
  AND pwe.created_at >= sqlc.arg('from_time')::timestamptz AND pwe.created_at < sqlc.arg('to_time')::timestamptz
  AND ((pwe.error_message IS NOT NULL AND pwe.error_message <> '') OR pwe.processed = FALSE)
GROUP BY day
ORDER BY day;
