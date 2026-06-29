-- name: CreateParticipant :one
INSERT INTO session_participants (session_id, display_name, device_fingerprint, is_host, phone_e164)
VALUES ($1, $2, $3, $4, $5)
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

-- name: GetOldestActiveParticipant :one
SELECT * FROM session_participants
WHERE session_id = $1 AND revoked_at IS NULL
ORDER BY joined_at ASC, id ASC
LIMIT 1;

-- name: SetParticipantHostFlags :exec
-- Sets is_host = true for exactly the new host and false for everyone else in
-- the session, in a single statement. Used for host reassignment.
UPDATE session_participants
SET is_host = (id = $2)
WHERE session_id = $1;
