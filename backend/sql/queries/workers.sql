-- Queries used by background worker routines.

-- name: ListStaleSessions :many
-- Finds sessions active for more than the given interval (passed as an interval string, e.g. '2 hours').
SELECT s.*, t.id AS table_id_ref
FROM sessions s
JOIN tables t ON t.id = s.table_id
WHERE s.status = 'active'
  AND s.created_at < NOW() - $1::interval
ORDER BY s.created_at ASC;

-- name: AbandonStaleSession :exec
UPDATE sessions
SET status = 'abandoned', closed_at = NOW()
WHERE id = $1 AND status = 'active';
