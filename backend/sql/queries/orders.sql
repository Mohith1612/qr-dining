-- name: CreateOrder :one
INSERT INTO orders (session_id, branch_id, placed_by_participant_id, idempotency_key, total_amount)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByIdempotencyKey :one
SELECT * FROM orders WHERE idempotency_key = $1;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListOrdersForSession :many
SELECT * FROM orders
WHERE session_id = $1
ORDER BY created_at DESC;

-- name: ListActiveOrdersForBranch :many
SELECT
    o.*,
    t.identifier AS table_identifier
FROM orders o
JOIN sessions s ON s.id = o.session_id
JOIN tables t ON t.id = s.table_id
WHERE o.branch_id = $1
  AND o.status IN ('pending', 'confirmed', 'preparing', 'ready')
ORDER BY o.created_at ASC;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price, selected_modifiers_json, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id ASC;
