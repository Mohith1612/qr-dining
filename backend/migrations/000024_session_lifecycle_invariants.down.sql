DROP INDEX IF EXISTS idx_sessions_one_active_per_table;

CREATE UNIQUE INDEX idx_sessions_one_active_per_table
    ON sessions(table_id)
    WHERE status = 'active';
