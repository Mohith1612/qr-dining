-- name: GetStaffByID :one
SELECT * FROM staff WHERE id = $1;

-- name: GetStaffByBranchAndCode :one
SELECT * FROM staff
WHERE branch_id = $1 AND staff_code = $2 AND is_active = TRUE;

-- name: ListStaffForBranch :many
SELECT * FROM staff WHERE branch_id = $1 ORDER BY name ASC;

-- name: CreateStaff :one
INSERT INTO staff (branch_id, name, role, pin_hash, staff_code)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetBranchByID :one
SELECT * FROM branches WHERE id = $1;

-- name: GetBranchByCode :one
SELECT * FROM branches WHERE branch_code = $1;

-- name: GetRestaurantByID :one
SELECT * FROM restaurants WHERE id = $1;

-- name: UpdateStaffPIN :exec
UPDATE staff
SET pin_hash = $2,
    pin_version = pin_version + 1,
    token_version = token_version + 1
WHERE id = $1;

-- name: DeactivateStaff :exec
UPDATE staff
SET is_active = FALSE,
    token_version = token_version + 1
WHERE id = $1;

-- name: CreateStaffSession :one
INSERT INTO staff_sessions (
  staff_id, branch_id, token_hash, device_name, token_version, pin_version, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetActiveStaffSessionByTokenHash :one
SELECT * FROM staff_sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: TouchStaffSession :exec
UPDATE staff_sessions SET last_seen_at = NOW() WHERE id = $1;

-- name: RevokeStaffSessionsForStaff :exec
UPDATE staff_sessions SET revoked_at = NOW()
WHERE staff_id = $1 AND revoked_at IS NULL;

-- name: ListActiveStaffForBranch :many
SELECT * FROM staff WHERE branch_id = $1 AND is_active = TRUE ORDER BY name ASC;

-- Pin-hash-free projection for the staff roster shown to managers/owners.
-- name: ListStaffRosterForBranch :many
SELECT id, branch_id, name, role, staff_code, is_active, created_at
FROM staff
WHERE branch_id = $1 AND is_active = TRUE
ORDER BY name ASC;

-- name: UpdateBranchSessionTimeout :exec
UPDATE branches SET session_timeout_minutes = $2 WHERE id = $1;

-- name: UpdateBranchOrderPrefix :exec
UPDATE branches SET order_prefix = $2 WHERE id = $1;
