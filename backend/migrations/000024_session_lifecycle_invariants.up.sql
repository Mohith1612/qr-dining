-- Phase A: extend the "one non-terminal session per table" invariant to cover
-- payment_pending and awaiting_reactivation. The original partial unique index
-- only protected rows in 'active' status; once a session transitions to
-- payment_pending the partial index would stop covering it, which would let a
-- second session start on the same table.

DROP INDEX IF EXISTS idx_sessions_one_active_per_table;

CREATE UNIQUE INDEX idx_sessions_one_active_per_table
    ON sessions(table_id)
    WHERE status IN ('active', 'payment_pending', 'awaiting_reactivation');
