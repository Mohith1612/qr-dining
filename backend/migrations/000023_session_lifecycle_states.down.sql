DROP INDEX IF EXISTS idx_session_participants_active;

ALTER TABLE session_participants
  DROP COLUMN IF EXISTS revoked_reason,
  DROP COLUMN IF EXISTS revoked_at;

-- Enum values cannot be dropped from a Postgres enum type; leaving the three
-- added values in place is the supported rollback behavior.
