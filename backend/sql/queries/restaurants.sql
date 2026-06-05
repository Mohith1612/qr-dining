-- name: GetRestaurantBySlug :one
SELECT * FROM restaurants WHERE slug = $1;

-- name: GetRestaurantByBranchID :one
SELECT r.*
FROM restaurants r
JOIN branches b ON b.restaurant_id = r.id
WHERE b.id = $1;

-- name: UpdateRestaurantLogoByBranchID :exec
UPDATE restaurants SET logo_url = $2
WHERE id = (SELECT b.restaurant_id FROM branches b WHERE b.id = $1);
