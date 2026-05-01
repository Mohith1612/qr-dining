-- name: GetStaffByID :one
SELECT * FROM staff WHERE id = $1;

-- name: ListStaffForBranch :many
SELECT * FROM staff WHERE branch_id = $1 ORDER BY name ASC;

-- name: CreateStaff :one
INSERT INTO staff (branch_id, name, role, pin_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetBranchByID :one
SELECT * FROM branches WHERE id = $1;

-- name: GetRestaurantByID :one
SELECT * FROM restaurants WHERE id = $1;
