-- name: GetTopOrderedItems :many
-- Returns the most ordered menu items for a branch within a time window.
SELECT
    mi.id                          AS menu_item_id,
    mi.name                        AS menu_item_name,
    SUM(oi.quantity)::BIGINT       AS total_quantity
FROM order_items oi
JOIN menu_items mi ON oi.menu_item_id = mi.id
JOIN orders o      ON oi.order_id     = o.id
WHERE o.branch_id  = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY mi.id, mi.name
ORDER BY total_quantity DESC
LIMIT 10;

-- name: GetBusyHours :many
-- Returns order count per hour-of-day (0–23) in the branch's configured timezone.
SELECT
    EXTRACT(HOUR FROM o.created_at AT TIME ZONE b.timezone)::SMALLINT AS hour,
    COUNT(*)::BIGINT                                                    AS order_count
FROM orders o
JOIN branches b ON o.branch_id = b.id
WHERE o.branch_id  = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY hour
ORDER BY hour;

-- name: GetOrderVolume :many
-- Returns daily order count and revenue for a branch within a time window.
SELECT
    DATE(o.created_at AT TIME ZONE b.timezone)  AS day,
    COUNT(*)::BIGINT                             AS order_count,
    COALESCE(SUM(o.total_amount), 0)::TEXT       AS revenue
FROM orders o
JOIN branches b ON o.branch_id = b.id
WHERE o.branch_id  = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY day
ORDER BY day;

-- name: GetOrganizationTopOrderedItems :many
-- Returns the most ordered menu items across an organization within a time window.
SELECT
    mi.id                          AS menu_item_id,
    mi.name                        AS menu_item_name,
    SUM(oi.quantity)::BIGINT       AS total_quantity
FROM order_items oi
JOIN menu_items mi ON oi.menu_item_id = mi.id
JOIN orders o      ON oi.order_id     = o.id
JOIN branches b    ON o.branch_id     = b.id
WHERE b.organization_id = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY mi.id, mi.name
ORDER BY total_quantity DESC
LIMIT 10;

-- name: GetOrganizationBusyHours :many
-- Returns order count per UTC hour-of-day (0-23) across an organization.
SELECT
    EXTRACT(HOUR FROM o.created_at AT TIME ZONE 'UTC')::SMALLINT AS hour,
    COUNT(*)::BIGINT                                             AS order_count
FROM orders o
JOIN branches b ON o.branch_id = b.id
WHERE b.organization_id = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY hour
ORDER BY hour;

-- name: GetOrganizationOrderVolume :many
-- Returns daily UTC order count and revenue across an organization.
SELECT
    DATE(o.created_at AT TIME ZONE 'UTC') AS day,
    COUNT(*)::BIGINT                      AS order_count,
    COALESCE(SUM(o.total_amount), 0)::TEXT AS revenue
FROM orders o
JOIN branches b ON o.branch_id = b.id
WHERE b.organization_id = $1
  AND o.created_at >= $2
  AND o.created_at <  $3
  AND o.status    <> 'cancelled'
GROUP BY day
ORDER BY day;
