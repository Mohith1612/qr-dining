-- Phase A: session lifecycle state machine.
-- Adds three intermediate session statuses required by
-- session-lifecycle-state-machine.md, plus participant credential revocation
-- fields needed to prevent closed-session resurrection.
--
-- The non-terminal index in 000024 needs these enum values committed first, so
-- the index migration is split into its own file (Postgres rejects use of new
-- enum values inside the transaction that adds them).

ALTER TYPE session_status ADD VALUE IF NOT EXISTS 'payment_pending';
ALTER TYPE session_status ADD VALUE IF NOT EXISTS 'awaiting_reactivation';
ALTER TYPE session_status ADD VALUE IF NOT EXISTS 'expired';

ALTER TABLE session_participants
  ADD COLUMN IF NOT EXISTS revoked_at      TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS revoked_reason  TEXT;

CREATE INDEX IF NOT EXISTS idx_session_participants_active
  ON session_participants(session_id)
  WHERE revoked_at IS NULL;
