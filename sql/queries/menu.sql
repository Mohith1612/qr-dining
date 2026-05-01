-- name: GetMenuItemByID :one
SELECT * FROM menu_items WHERE id = $1;

-- name: ListMenuCategoriesForBranch :many
SELECT * FROM menu_categories
WHERE branch_id = $1 AND is_active = TRUE
ORDER BY position ASC;

-- name: ListMenuItemsForCategory :many
SELECT * FROM menu_items
WHERE category_id = $1 AND is_available = TRUE
ORDER BY position ASC;

-- name: ListModifiersForItem :many
SELECT * FROM item_modifiers WHERE item_id = $1 ORDER BY id ASC;

-- name: GetTableByQRToken :one
SELECT * FROM tables WHERE qr_code_token = $1;

-- name: GetTableByID :one
SELECT * FROM tables WHERE id = $1;

-- name: UpdateTableStatus :exec
UPDATE tables SET status = $2 WHERE id = $1;

-- name: ListTablesForBranch :many
SELECT * FROM tables WHERE branch_id = $1 ORDER BY identifier ASC;
