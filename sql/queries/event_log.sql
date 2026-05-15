-- name: InsertEventLog :exec
INSERT INTO event_log (session_id, branch_id, event_type, actor_type, actor_id, payload)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEventsBySession :many
SELECT * FROM event_log
WHERE session_id = $1
ORDER BY created_at ASC
LIMIT 200;

-- name: GetRecentEventsByBranch :many
SELECT * FROM event_log
WHERE branch_id = $1
ORDER BY created_at DESC
LIMIT 100;
