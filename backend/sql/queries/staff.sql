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

-- name: UpdateStaffPIN :exec
UPDATE staff SET pin_hash = $2 WHERE id = $1;

-- name: DeactivateStaff :exec
UPDATE staff SET is_active = FALSE WHERE id = $1;

-- name: ListActiveStaffForBranch :many
SELECT * FROM staff WHERE branch_id = $1 AND is_active = TRUE ORDER BY name ASC;

-- name: UpdateBranchSessionTimeout :exec
UPDATE branches SET session_timeout_minutes = $2 WHERE id = $1;

-- name: UpdateBranchOrderPrefix :exec
UPDATE branches SET order_prefix = $2 WHERE id = $1;
