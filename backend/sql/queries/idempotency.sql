-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
  scope_type, scope_id, actor_type, actor_id, key, request_hash, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (scope_type, scope_id, actor_type, actor_id, key) DO NOTHING
RETURNING *;

-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE scope_type = $1
  AND scope_id = $2
  AND actor_type = $3
  AND actor_id = $4
  AND key = $5;

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_keys
SET response_resource_type = $6,
    response_resource_id = $7,
    status = 'completed'
WHERE scope_type = $1
  AND scope_id = $2
  AND actor_type = $3
  AND actor_id = $4
  AND key = $5;

-- name: FailIdempotencyKey :exec
UPDATE idempotency_keys
SET status = 'failed'
WHERE scope_type = $1
  AND scope_id = $2
  AND actor_type = $3
  AND actor_id = $4
  AND key = $5
  AND status = 'pending';
