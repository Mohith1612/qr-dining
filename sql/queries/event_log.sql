-- name: InsertEventLog :exec
INSERT INTO event_log (session_id, branch_id, event_type, actor_type, actor_id, payload)
VALUES ($1, $2, $3, $4, $5, $6);
