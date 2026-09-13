-- Queries used by background worker routines.

-- name: ListExpiredSessions :many
-- Finds sessions whose latest durable participant activity has exceeded their
-- branch-configured timeout. Sessions without participants fall back to age.
SELECT s.id, s.branch_id, s.table_id
FROM sessions s
JOIN branches b ON b.id = s.branch_id
LEFT JOIN session_participants sp ON sp.session_id = s.id
WHERE s.status = 'active'
GROUP BY s.id, s.branch_id, s.table_id, s.created_at, b.session_timeout_minutes
HAVING COALESCE(MAX(sp.last_seen_at), s.created_at)
  < NOW() - (b.session_timeout_minutes || ' minutes')::interval
ORDER BY COALESCE(MAX(sp.last_seen_at), s.created_at) ASC;

-- name: AbandonStaleSession :exec
-- Accepts both active and awaiting_reactivation states; the latter is the
-- terminal stage of the awaiting_reactivation pipeline.
UPDATE sessions
SET status = 'abandoned', closed_at = NOW()
WHERE id = $1 AND status IN ('active', 'awaiting_reactivation');
