-- name: GetOrCreateCart :one
INSERT INTO carts (session_id, participant_id)
VALUES ($1, $2)
ON CONFLICT (session_id, participant_id) DO UPDATE SET session_id = EXCLUDED.session_id
RETURNING *;

-- name: GetCartByID :one
SELECT * FROM carts WHERE id = $1;

-- name: GetCartBySessionAndParticipant :one
SELECT * FROM carts WHERE session_id = $1 AND participant_id = $2;

-- name: AddCartItem :one
INSERT INTO cart_items (cart_id, menu_item_id, quantity, selected_modifiers_json, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCartItem :one
SELECT * FROM cart_items WHERE id = $1;

-- name: UpdateCartItemQuantity :one
UPDATE cart_items SET quantity = $2 WHERE id = $1
RETURNING *;

-- name: RemoveCartItem :exec
DELETE FROM cart_items WHERE id = $1 AND cart_id = $2;

-- name: ClearCart :exec
DELETE FROM cart_items WHERE cart_id = $1;

-- name: ListCartItems :many
SELECT
    ci.id,
    ci.cart_id,
    ci.menu_item_id,
    ci.quantity,
    ci.selected_modifiers_json,
    ci.note,
    mi.name AS item_name,
    mi.price AS item_price,
    mi.is_available
FROM cart_items ci
JOIN menu_items mi ON mi.id = ci.menu_item_id
WHERE ci.cart_id = $1
ORDER BY ci.id ASC;
