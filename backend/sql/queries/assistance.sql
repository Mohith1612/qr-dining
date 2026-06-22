-- name: CreateAssistanceRequest :one
INSERT INTO assistance_requests (session_id, table_id, participant_id, type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAssistanceRequestByID :one
SELECT * FROM assistance_requests WHERE id = $1;

-- name: UpdateAssistanceStatus :one
UPDATE assistance_requests
SET status = $2,
    resolved_at = CASE WHEN $2::assistance_status = 'resolved' THEN NOW() ELSE resolved_at END
WHERE id = $1
RETURNING *;

-- name: UpdateAssistanceStatusScoped :one
UPDATE assistance_requests ar
SET status = $3,
    resolved_at = CASE WHEN $3::assistance_status = 'resolved' THEN NOW() ELSE ar.resolved_at END
FROM sessions s
WHERE ar.id = $1
  AND ar.session_id = s.id
  AND s.branch_id = $2
RETURNING ar.*;

-- name: ListActiveAssistanceForBranch :many
SELECT
  ar.*,
  t.identifier AS table_identifier,
  s.session_number
FROM assistance_requests ar
JOIN sessions s ON s.id = ar.session_id
JOIN tables t ON t.id = ar.table_id
WHERE t.branch_id = $1
  AND ar.status IN ('pending', 'acknowledged')
ORDER BY ar.created_at ASC;

-- name: ListAssistanceForSession :many
SELECT * FROM assistance_requests
WHERE session_id = $1
ORDER BY created_at DESC;
