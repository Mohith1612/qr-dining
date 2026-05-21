-- name: CreateSession :one
INSERT INTO sessions (branch_id, table_id, session_token)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByToken :one
SELECT * FROM sessions WHERE session_token = $1 AND status = 'active';

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: SetSessionHost :exec
UPDATE sessions SET host_participant_id = $2 WHERE id = $1;

-- name: CloseSession :exec
UPDATE sessions
SET status = 'closed', closed_at = NOW()
WHERE id = $1 AND status = 'active';

-- name: AbandonSession :exec
UPDATE sessions
SET status = 'abandoned', closed_at = NOW()
WHERE id = $1 AND status = 'active';

-- name: ListActiveSessionsForBranch :many
SELECT * FROM sessions
WHERE branch_id = $1 AND status = 'active'
ORDER BY created_at DESC;

-- name: GetActiveSessionForTable :one
SELECT * FROM sessions
WHERE table_id = $1 AND status = 'active'
LIMIT 1;
