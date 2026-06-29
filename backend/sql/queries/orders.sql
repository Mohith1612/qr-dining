-- name: CreateOrder :one
INSERT INTO orders (
  session_id,
  branch_id,
  placed_by_participant_id,
  idempotency_key,
  total_amount,
  order_number,
  promo_id,
  discount_amount,
  order_business_date,
  order_number_display,
  order_operational_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: NextOrderNumber :one
INSERT INTO order_sequences (branch_id, date, last_seq)
VALUES ($1, $2, 1001)
ON CONFLICT (branch_id, date)
DO UPDATE SET last_seq = order_sequences.last_seq + 1
RETURNING last_seq;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByIdempotencyKey :one
SELECT * FROM orders WHERE idempotency_key = $1;

-- name: GetOrderByScopedIdempotencyKey :one
SELECT * FROM orders
WHERE session_id = $1
  AND placed_by_participant_id = $2
  AND idempotency_key = $3;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateOrderStatusScoped :one
UPDATE orders
SET status = $3, updated_at = NOW()
WHERE id = $1 AND branch_id = $2
RETURNING *;

-- name: UpdateOrderStatusExpected :one
UPDATE orders
SET status = $4, updated_at = NOW()
WHERE id = $1 AND branch_id = $2 AND status = $3
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

-- name: ListActiveOrderItemsForBranch :many
SELECT
    oi.order_id,
    oi.menu_item_id,
    oi.quantity,
    oi.selected_modifiers_json,
    oi.note,
    mi.name AS menu_item_name
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN menu_items mi ON mi.id = oi.menu_item_id
WHERE o.branch_id = $1
  AND o.status IN ('pending', 'confirmed', 'preparing', 'ready')
ORDER BY oi.order_id, oi.id ASC;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price, selected_modifiers_json, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id ASC;
