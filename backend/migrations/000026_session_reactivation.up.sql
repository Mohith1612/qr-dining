-- Phase B: awaiting_reactivation worker pipeline.
-- Records when a session was moved into awaiting_reactivation so the worker
-- and snapshot endpoint can compare against the reactivation window without
-- guessing from created_at or closed_at.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS awaiting_reactivation_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_awaiting_reactivation
    ON sessions(awaiting_reactivation_at)
    WHERE status = 'awaiting_reactivation';
