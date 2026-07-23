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

-- name: TransitionSessionToAwaitingReactivation :one
UPDATE sessions
SET status = 'awaiting_reactivation',
    awaiting_reactivation_at = NOW()
WHERE id = $1 AND status = 'active'
RETURNING id;

-- name: ReactivateSession :one
-- warned_at is cleared so a recovered session can be warned again by the
-- expiry warner before its (unchanged) timeout.
UPDATE sessions
SET status = 'active',
    awaiting_reactivation_at = NULL,
    warned_at = NULL
WHERE id = $1 AND status = 'awaiting_reactivation'
RETURNING id;

-- name: ListSessionsAwaitingReactivationExpired :many
-- Returns sessions whose reactivation window has been exceeded. Caller is
-- responsible for verifying no non-terminal payment exists before abandoning
-- (TIM-1 in payment-finalization-invariants.md).
SELECT s.id, s.branch_id, s.table_id, b.organization_id
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE s.status = 'awaiting_reactivation'
  AND s.awaiting_reactivation_at IS NOT NULL
  AND s.awaiting_reactivation_at < $1::timestamptz
ORDER BY s.awaiting_reactivation_at ASC;

-- name: ListActiveSessionsForReactivationScan :many
-- Returns active sessions older than the grace floor — candidates for the
-- awaiting_reactivation transition. The worker still has to verify Redis
-- presence absence before transitioning.
SELECT s.id, s.branch_id, s.table_id, b.organization_id
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE s.status = 'active'
  AND s.created_at < $1::timestamptz
ORDER BY s.created_at ASC;

-- name: HasNonTerminalPaymentForSession :one
SELECT EXISTS (
    SELECT 1 FROM payments
    WHERE session_id = $1
      AND status NOT IN ('completed', 'failed', 'cancelled', 'refunded', 'partially_refunded')
) AS has_pending;

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
-- Returns the table's in-progress session. Must match the non-terminal statuses
-- the one-active-per-table unique index blocks, so a QR scan of an occupied table
-- resolves the joinable session instead of falling through to a blocked create.
SELECT * FROM sessions
WHERE table_id = $1 AND status IN ('active', 'payment_pending', 'awaiting_reactivation')
ORDER BY created_at DESC
LIMIT 1;
