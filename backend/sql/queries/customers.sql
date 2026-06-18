-- name: UpsertCustomer :one
INSERT INTO customers (restaurant_id, phone_e164, display_name, opted_in, opted_in_at, last_seen_at)
VALUES ($1, $2, $3, TRUE, NOW(), NOW())
ON CONFLICT (restaurant_id, phone_e164) DO UPDATE
SET
    display_name = CASE WHEN EXCLUDED.display_name != '' THEN EXCLUDED.display_name ELSE customers.display_name END,
    last_seen_at = NOW(),
    visit_count  = customers.visit_count + 1,
    opted_in     = TRUE,
    opted_in_at  = COALESCE(customers.opted_in_at, NOW())
RETURNING *;

-- name: LinkSessionToCustomer :exec
UPDATE sessions SET customer_id = $2 WHERE id = $1;

-- name: GetCustomerByPhone :one
SELECT * FROM customers WHERE restaurant_id = $1 AND phone_e164 = $2;

-- name: GetCustomerByID :one
SELECT * FROM customers WHERE id = $1;

-- name: GetCustomerSessionHistory :many
SELECT
    s.id AS session_id,
    s.created_at,
    s.closed_at,
    t.identifier AS table_identifier,
    COALESCE(SUM(o.total_amount), 0::numeric) AS total_spent
FROM sessions s
JOIN tables t ON t.id = s.table_id
LEFT JOIN orders o ON o.session_id = s.id AND o.status != 'cancelled'
WHERE s.customer_id = $1
GROUP BY s.id, s.created_at, s.closed_at, t.identifier
ORDER BY s.created_at DESC
LIMIT 20;

-- name: GetCustomerSessionHistoryScoped :many
SELECT
    s.id AS session_id,
    s.created_at,
    s.closed_at,
    t.identifier AS table_identifier,
    COALESCE(SUM(o.total_amount), 0::numeric) AS total_spent
FROM sessions s
JOIN customers c ON c.id = s.customer_id
JOIN tables t ON t.id = s.table_id
LEFT JOIN orders o ON o.session_id = s.id AND o.status != 'cancelled'
WHERE s.customer_id = $1
  AND c.restaurant_id = $2
GROUP BY s.id, s.created_at, s.closed_at, t.identifier
ORDER BY s.created_at DESC
LIMIT 20;

-- name: DeleteCustomer :exec
DELETE FROM customers WHERE id = $1 AND restaurant_id = $2;

-- name: SearchCustomersByPhone :many
SELECT * FROM customers
WHERE restaurant_id = $1 AND phone_e164 LIKE $2 || '%'
ORDER BY last_seen_at DESC
LIMIT 20;
