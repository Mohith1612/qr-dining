-- Reserves a key for a request. Two outcomes, distinguished by whether a row
-- comes back:
--
--   * no existing row, or an existing row that is already past expires_at —
--     the reservation succeeds and a row is returned. An expired row is
--     reclaimed in place (fresh request_hash and expires_at, status back to
--     pending, stale response pointer cleared) so it can never replay again.
--   * a live existing row — the DO UPDATE's WHERE is false, nothing is
--     returned, and the caller falls through to the replay path.
--
-- Doing the expiry check inside ON CONFLICT rather than as a read-then-write
-- leaves no window where two callers both believe they reserved the key.
-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
  scope_type, scope_id, actor_type, actor_id, key, request_hash, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (scope_type, scope_id, actor_type, actor_id, key) DO UPDATE
SET request_hash           = EXCLUDED.request_hash,
    expires_at             = EXCLUDED.expires_at,
    status                 = 'pending',
    response_resource_type = NULL,
    response_resource_id   = NULL,
    created_at             = NOW()
WHERE idempotency_keys.expires_at <= NOW()
RETURNING *;

-- Expired keys are invisible to lookup. Without the expires_at predicate the
-- recorded expiry was decoration and a key of any age replayed to its original
-- resource.
-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE scope_type = $1
  AND scope_id = $2
  AND actor_type = $3
  AND actor_id = $4
  AND key = $5
  AND expires_at > NOW();

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

-- Reaps keys whose expiry passed longer ago than the retention grace period.
-- Bounded by $2 so a single statement can never run long on a backlogged
-- table; the worker loops batches within its run budget. Driven by
-- idx_idempotency_keys_expires_at.
-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys
WHERE id IN (
  SELECT stale.id FROM idempotency_keys AS stale
  WHERE stale.expires_at < $1
  ORDER BY stale.expires_at
  LIMIT $2
);
