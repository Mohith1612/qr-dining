-- Phase 6: realtime and session lifecycle hardening.

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_one_active_per_table
    ON sessions(table_id)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS session_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sequence        BIGINT      NOT NULL,
    organization_id BIGINT      NOT NULL REFERENCES organizations(id),
    branch_id       BIGINT      NOT NULL REFERENCES branches(id),
    session_id      UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    event           TEXT        NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_session_events_session_sequence
    ON session_events(session_id, sequence);

CREATE INDEX IF NOT EXISTS idx_session_events_branch_created_at
    ON session_events(branch_id, created_at DESC);
