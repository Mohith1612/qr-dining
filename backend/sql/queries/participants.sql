-- name: CreateParticipant :one
INSERT INTO session_participants (session_id, display_name, device_fingerprint, is_host)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetParticipantByID :one
SELECT * FROM session_participants WHERE id = $1;

-- name: ListParticipantsBySession :many
SELECT * FROM session_participants
WHERE session_id = $1
ORDER BY joined_at ASC;

-- name: UpdateParticipantLastSeen :exec
UPDATE session_participants
SET last_seen_at = NOW()
WHERE id = $1;
