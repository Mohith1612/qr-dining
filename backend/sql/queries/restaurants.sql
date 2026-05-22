-- name: GetRestaurantBySlug :one
SELECT * FROM restaurants WHERE slug = $1;

-- name: GetRestaurantByBranchID :one
SELECT r.*
FROM restaurants r
JOIN branches b ON b.restaurant_id = r.id
WHERE b.id = $1;
