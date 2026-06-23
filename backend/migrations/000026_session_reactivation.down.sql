DROP INDEX IF EXISTS idx_sessions_awaiting_reactivation;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS awaiting_reactivation_at;
