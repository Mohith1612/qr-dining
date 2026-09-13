-- Staff performance analytics. Read-only aggregation over EXISTING sources:
-- event_log (ORDER_STATUS_CHANGED / ASSISTANCE_* rows already carry staff
-- actor_id), payments (settled_by_staff_id / settled_at), and staff_sessions
-- (created_at = login moment, last_seen_at/revoked_at = activity bounds).
-- No new event capture; windows are UTC, matching the existing analytics queries.
--
-- event_log.actor_id is TEXT; staff joins go through a guarded CASE cast so a
-- non-numeric legacy/system actor_id can never abort the query.

-- name: GetStaffOrderActivity :many
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    st.role AS staff_role,
    COUNT(DISTINCT el.payload->>'order_id')::BIGINT AS orders_touched,
    COUNT(*) FILTER (WHERE el.payload->>'new_status' = 'served')::BIGINT AS orders_served,
    COUNT(DISTINCT el.session_id)::BIGINT AS sessions_touched,
    COUNT(DISTINCT s.table_id)::BIGINT AS tables_touched
FROM event_log el
JOIN staff st ON st.id = (CASE WHEN el.actor_id ~ '^[0-9]+$' THEN el.actor_id::BIGINT END)
LEFT JOIN sessions s ON s.id = el.session_id
WHERE el.branch_id  = sqlc.arg(branch_id)
  AND el.event_type = 'ORDER_STATUS_CHANGED'
  AND el.actor_type = 'staff'
  AND el.created_at >= sqlc.arg(from_time)::timestamptz
  AND el.created_at <  sqlc.arg(to_time)::timestamptz
GROUP BY st.id, st.name, st.role
ORDER BY orders_touched DESC;

-- name: GetStaffAssistanceStats :many
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    st.role AS staff_role,
    COUNT(*) FILTER (WHERE el.event_type = 'ASSISTANCE_ACKNOWLEDGED')::BIGINT AS acknowledged_count,
    COUNT(*) FILTER (WHERE el.event_type = 'ASSISTANCE_RESOLVED')::BIGINT     AS resolved_count,
    -- The ack event payload is the full assistance request, so the request's
    -- created_at rides inside the event row; response time needs no join.
    COALESCE(AVG(EXTRACT(EPOCH FROM (el.created_at - (el.payload->>'created_at')::timestamptz)))
        FILTER (WHERE el.event_type = 'ASSISTANCE_ACKNOWLEDGED'
                  AND el.payload->>'created_at' IS NOT NULL), 0)::DOUBLE PRECISION AS avg_response_seconds
FROM event_log el
JOIN staff st ON st.id = (CASE WHEN el.actor_id ~ '^[0-9]+$' THEN el.actor_id::BIGINT END)
WHERE el.branch_id  = sqlc.arg(branch_id)
  AND el.event_type IN ('ASSISTANCE_ACKNOWLEDGED', 'ASSISTANCE_RESOLVED')
  AND el.actor_type = 'staff'
  AND el.created_at >= sqlc.arg(from_time)::timestamptz
  AND el.created_at <  sqlc.arg(to_time)::timestamptz
GROUP BY st.id, st.name, st.role
ORDER BY acknowledged_count DESC;

-- name: GetStaffSettlementStats :many
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    st.role AS staff_role,
    COUNT(*)::BIGINT AS payments_settled,
    COALESCE(AVG(EXTRACT(EPOCH FROM (p.settled_at - p.initiated_at))), 0)::DOUBLE PRECISION AS avg_settlement_seconds,
    COUNT(DISTINCT p.session_id)::BIGINT AS sessions_settled
FROM payments p
JOIN staff st ON st.id = p.settled_by_staff_id
WHERE p.branch_id = sqlc.arg(branch_id)
  AND p.settled_by_staff_id IS NOT NULL
  AND p.settled_at >= sqlc.arg(from_time)::timestamptz
  AND p.settled_at <  sqlc.arg(to_time)::timestamptz
GROUP BY st.id, st.name, st.role
ORDER BY payments_settled DESC;

-- name: GetStaffLoginStats :many
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    st.role AS staff_role,
    COUNT(*)::BIGINT AS login_count,
    COALESCE(SUM(GREATEST(EXTRACT(EPOCH FROM (
        LEAST(COALESCE(ss.revoked_at, ss.last_seen_at), sqlc.arg(to_time)::timestamptz) - ss.created_at
    )), 0)), 0)::DOUBLE PRECISION AS active_seconds
FROM staff_sessions ss
JOIN staff st ON st.id = ss.staff_id
WHERE ss.branch_id  = sqlc.arg(branch_id)
  AND ss.created_at >= sqlc.arg(from_time)::timestamptz
  AND ss.created_at <  sqlc.arg(to_time)::timestamptz
GROUP BY st.id, st.name, st.role
ORDER BY login_count DESC;

-- Orders whose 'preparing' transition predates the window are excluded from
-- avg_prep_seconds (prep_started_at is NULL inside the window) but still count
-- in orders_completed.
-- name: GetKitchenPerformance :many
WITH transitions AS (
    SELECT
        el.payload->>'order_id'   AS order_id,
        el.payload->>'new_status' AS new_status,
        el.created_at,
        (CASE WHEN el.actor_id ~ '^[0-9]+$' THEN el.actor_id::BIGINT END) AS staff_id
    FROM event_log el
    WHERE el.branch_id  = sqlc.arg(branch_id)
      AND el.event_type = 'ORDER_STATUS_CHANGED'
      AND el.actor_type = 'staff'
      AND el.created_at >= sqlc.arg(from_time)::timestamptz
      AND el.created_at <  sqlc.arg(to_time)::timestamptz
),
per_order AS (
    SELECT
        order_id,
        MIN(created_at) FILTER (WHERE new_status = 'preparing') AS prep_started_at,
        MIN(created_at) FILTER (WHERE new_status = 'ready')     AS ready_at,
        (ARRAY_AGG(staff_id ORDER BY created_at) FILTER (WHERE new_status = 'ready'))[1] AS ready_staff_id
    FROM transitions
    GROUP BY order_id
)
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    COUNT(*)::BIGINT AS orders_completed,
    COALESCE(AVG(EXTRACT(EPOCH FROM (po.ready_at - po.prep_started_at)))
        FILTER (WHERE po.prep_started_at IS NOT NULL), 0)::DOUBLE PRECISION AS avg_prep_seconds
FROM per_order po
JOIN staff st ON st.id = po.ready_staff_id
WHERE po.ready_at IS NOT NULL
GROUP BY st.id, st.name
ORDER BY orders_completed DESC;

-- name: GetKitchenPeakThroughput :many
WITH hourly AS (
    SELECT
        (CASE WHEN el.actor_id ~ '^[0-9]+$' THEN el.actor_id::BIGINT END) AS staff_id,
        DATE_TRUNC('hour', el.created_at) AS hour_bucket,
        COUNT(*)::BIGINT AS ready_count
    FROM event_log el
    WHERE el.branch_id  = sqlc.arg(branch_id)
      AND el.event_type = 'ORDER_STATUS_CHANGED'
      AND el.actor_type = 'staff'
      AND el.payload->>'new_status' = 'ready'
      AND el.created_at >= sqlc.arg(from_time)::timestamptz
      AND el.created_at <  sqlc.arg(to_time)::timestamptz
    GROUP BY 1, 2
)
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    MAX(h.ready_count)::BIGINT AS peak_orders_per_hour
FROM hourly h
JOIN staff st ON st.id = h.staff_id
GROUP BY st.id, st.name
ORDER BY peak_orders_per_hour DESC;

-- name: GetStaffDailyActivity :many
WITH events AS (
    SELECT
        (CASE WHEN el.actor_id ~ '^[0-9]+$' THEN el.actor_id::BIGINT END) AS staff_id,
        DATE(el.created_at AT TIME ZONE 'UTC') AS day,
        COUNT(*)::BIGINT AS event_count
    FROM event_log el
    WHERE el.branch_id  = sqlc.arg(branch_id)
      AND el.actor_type = 'staff'
      AND el.created_at >= sqlc.arg(from_time)::timestamptz
      AND el.created_at <  sqlc.arg(to_time)::timestamptz
    GROUP BY 1, 2
),
logins AS (
    SELECT
        ss.staff_id,
        DATE(ss.created_at AT TIME ZONE 'UTC') AS day,
        COUNT(*)::BIGINT AS login_count,
        SUM(GREATEST(EXTRACT(EPOCH FROM (
            LEAST(COALESCE(ss.revoked_at, ss.last_seen_at), sqlc.arg(to_time)::timestamptz) - ss.created_at
        )), 0))::DOUBLE PRECISION AS active_seconds
    FROM staff_sessions ss
    WHERE ss.branch_id  = sqlc.arg(branch_id)
      AND ss.created_at >= sqlc.arg(from_time)::timestamptz
      AND ss.created_at <  sqlc.arg(to_time)::timestamptz
    GROUP BY 1, 2
)
SELECT
    st.id   AS staff_id,
    st.name AS staff_name,
    st.role AS staff_role,
    COALESCE(e.day, l.day) AS day,
    COALESCE(e.event_count, 0)::BIGINT AS event_count,
    COALESCE(l.login_count, 0)::BIGINT AS login_count,
    COALESCE(l.active_seconds, 0)::DOUBLE PRECISION AS active_seconds
FROM events e
FULL OUTER JOIN logins l ON l.staff_id = e.staff_id AND l.day = e.day
JOIN staff st ON st.id = COALESCE(e.staff_id, l.staff_id)
ORDER BY COALESCE(e.day, l.day) DESC, st.name ASC;
