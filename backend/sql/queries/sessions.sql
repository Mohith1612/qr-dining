-- name: CreateSession :one
INSERT INTO sessions (
  branch_id,
  table_id,
  session_token,
  session_business_date,
  visit_number,
  session_number
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: NextSessionNumber :one
INSERT INTO session_sequences (branch_id, date, last_seq)
VALUES ($1, $2, 1)
ON CONFLICT (branch_id, date)
DO UPDATE SET last_seq = session_sequences.last_seq + 1
RETURNING last_seq;

-- name: GetSessionByToken :one
SELECT * FROM sessions WHERE session_token = $1 AND status = 'active';

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: GetSessionParticipantByID :one
SELECT * FROM session_participants WHERE id = $1;

-- name: SetSessionHost :exec
UPDATE sessions SET host_participant_id = $2 WHERE id = $1;

-- name: CloseSessionIfActive :one
UPDATE sessions
SET status = 'closed', closed_at = NOW()
WHERE id = $1 AND status IN ('active', 'payment_pending', 'awaiting_reactivation')
RETURNING id;

-- name: TransitionSessionToPaymentPending :one
UPDATE sessions
SET status = 'payment_pending'
WHERE id = $1 AND status = 'active'
RETURNING id;

-- name: TransitionSessionToActive :one
UPDATE sessions
SET status = 'active'
WHERE id = $1 AND status = 'payment_pending'
RETURNING id;

-- name: RevokeAllParticipants :exec
UPDATE session_participants
SET revoked_at = NOW(), revoked_reason = $2
WHERE session_id = $1 AND revoked_at IS NULL;

-- name: BumpAllParticipantCredentialVersions :exec
UPDATE session_participants
SET credential_version = credential_version + 1
WHERE session_id = $1;

-- name: ListSessionsExpiringSoon :many
SELECT s.id, s.branch_id, s.created_at, b.session_timeout_minutes
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE s.status = 'active'
  AND s.warned_at IS NULL
  AND s.created_at + (b.session_timeout_minutes || ' minutes')::interval
      BETWEEN NOW() AND NOW() + INTERVAL '15 minutes';

-- name: MarkSessionWarned :exec
UPDATE sessions SET warned_at = NOW() WHERE id = $1 AND warned_at IS NULL;

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
