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

-- name: UpdateMenuItemScoped :one
UPDATE menu_items
SET name = $3, description = $4, price = $5, position = $6,
    dietary_flags = $7, item_badges = $8, spice_level = $9,
    category_id = COALESCE(sqlc.narg('category_id'), category_id),
    image_url = COALESCE(sqlc.narg('image_url'), image_url)
WHERE id = $1 AND branch_id = $2
RETURNING *;

-- name: UpdateMenuItemAvailability :exec
UPDATE menu_items SET is_available = $2 WHERE id = $1;

-- name: UpdateMenuItemAvailabilityScoped :exec
UPDATE menu_items SET is_available = $3 WHERE id = $1 AND branch_id = $2;

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

-- name: UpdateMenuItemFeaturedScoped :exec
UPDATE menu_items SET is_featured = $3, featured_sort_order = $4 WHERE id = $1 AND branch_id = $2;

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
INSERT INTO item_modifiers (item_id, name, price_delta, is_required, modifier_group, single_select)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteItemModifier :exec
DELETE FROM item_modifiers WHERE id = $1;

-- name: DeleteItemModifierScoped :exec
DELETE FROM item_modifiers
USING menu_items
WHERE item_modifiers.id = $1
  AND item_modifiers.item_id = menu_items.id
  AND menu_items.branch_id = $2;

-- name: GetMenuCategoryByID :one
SELECT * FROM menu_categories WHERE id = $1;

-- name: GetModifierWithItemBranch :one
SELECT im.*, mi.branch_id
FROM item_modifiers im
JOIN menu_items mi ON mi.id = im.item_id
WHERE im.id = $1;

-- name: CreateItemModifierScoped :one
INSERT INTO item_modifiers (item_id, name, price_delta, is_required, modifier_group, single_select)
SELECT $1, $3, $4, $5, $6, $7
FROM menu_items
WHERE id = $1 AND branch_id = $2
RETURNING *;

-- name: UpdateItemModifierScoped :one
UPDATE item_modifiers im
SET name          = $3,
    price_delta   = $4,
    is_required   = $5,
    modifier_group = $6,
    single_select = $7
FROM menu_items mi
WHERE im.id = $1
  AND im.item_id = mi.id
  AND mi.branch_id = $2
RETURNING im.*;
