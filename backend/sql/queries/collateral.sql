-- name: GetBranchCollateral :one
SELECT * FROM branch_collateral WHERE branch_id = $1;

-- name: UpsertBranchCollateral :one
INSERT INTO branch_collateral (branch_id, config_json, updated_by_platform_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (branch_id)
DO UPDATE SET config_json = EXCLUDED.config_json,
              updated_by_platform_user_id = EXCLUDED.updated_by_platform_user_id, updated_at = NOW()
RETURNING *;
