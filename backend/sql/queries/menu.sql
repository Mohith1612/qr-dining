-- name: GetMenuItemByID :one
SELECT * FROM menu_items WHERE id = $1;

-- name: GetMenuItemsByIDs :many
SELECT * FROM menu_items WHERE id = ANY($1::bigint[]);

-- name: ListModifiersForItems :many
SELECT * FROM item_modifiers WHERE item_id = ANY($1::bigint[]) ORDER BY item_id, id ASC;

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

-- name: InsertMenuCategory :one
INSERT INTO menu_categories (branch_id, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: InsertMenuItem :one
INSERT INTO menu_items (category_id, branch_id, name, description, price, is_available, position)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateMenuItem :one
UPDATE menu_items
SET name = $2, description = $3, price = $4, position = $5,
    dietary_flags = $6, item_badges = $7, spice_level = $8,
    category_id = COALESCE(sqlc.narg('category_id'), category_id),
    image_url = COALESCE(sqlc.narg('image_url'), image_url)
WHERE id = $1
RETURNING *;

-- name: UpdateMenuItemAvailability :exec
UPDATE menu_items SET is_available = $2 WHERE id = $1;

-- name: CreateTable :one
INSERT INTO tables (branch_id, identifier, capacity, qr_code_token)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RefreshTableQRToken :one
UPDATE tables SET qr_code_token = $2 WHERE id = $1 RETURNING *;

-- name: ListFeaturedMenuItems :many
SELECT * FROM menu_items
WHERE branch_id = $1
  AND is_featured = TRUE
  AND is_available = TRUE
ORDER BY featured_sort_order ASC, id ASC;

-- name: UpdateMenuItemFeatured :exec
UPDATE menu_items SET is_featured = $2, featured_sort_order = $3 WHERE id = $1;

-- name: ListAllMenuCategoriesForBranch :many
SELECT * FROM menu_categories
WHERE branch_id = $1
ORDER BY position ASC, id ASC;

-- name: ListAllMenuItemsForCategory :many
SELECT * FROM menu_items
WHERE category_id = $1
ORDER BY position ASC, id ASC;

-- name: DeleteMenuItem :exec
DELETE FROM menu_items WHERE id = $1 AND branch_id = $2;

-- name: CountItemsInCategory :one
SELECT COUNT(*) FROM menu_items WHERE category_id = $1;

-- name: DeleteMenuCategory :exec
DELETE FROM menu_categories WHERE id = $1 AND branch_id = $2;

-- name: UpdateMenuCategory :one
UPDATE menu_categories
SET name = $2, position = $3, is_active = $4
WHERE id = $1 AND branch_id = $5
RETURNING *;

-- name: CreateItemModifier :one
INSERT INTO item_modifiers (item_id, name, price_delta, is_required, modifier_group)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteItemModifier :exec
DELETE FROM item_modifiers WHERE id = $1;
