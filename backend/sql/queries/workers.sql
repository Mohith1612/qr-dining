-- Queries used by background worker routines.

-- name: ListExpiredSessions :many
-- Finds sessions that have exceeded their branch-configured timeout.
SELECT s.id, s.branch_id, s.table_id
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE s.status = 'active'
  AND s.created_at < NOW() - (b.session_timeout_minutes || ' minutes')::interval
ORDER BY s.created_at ASC;

-- name: AbandonStaleSession :exec
-- Accepts both active and awaiting_reactivation states; the latter is the
-- terminal stage of the awaiting_reactivation pipeline.
UPDATE sessions
SET status = 'abandoned', closed_at = NOW()
WHERE id = $1 AND status IN ('active', 'awaiting_reactivation');
