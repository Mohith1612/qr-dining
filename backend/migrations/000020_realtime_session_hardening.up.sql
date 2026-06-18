-- Phase 6: realtime and session lifecycle hardening.

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY table_id
               ORDER BY (host_participant_id IS NOT NULL) DESC, created_at DESC, id DESC
           ) AS rn
    FROM sessions
    WHERE status = 'active'
),
abandoned AS (
    UPDATE sessions s
    SET status = 'abandoned', closed_at = NOW()
    FROM ranked r
    WHERE s.id = r.id AND r.rn > 1
    RETURNING s.id
)
SELECT COUNT(*) FROM abandoned;

UPDATE tables t
SET status = 'available'
WHERE t.status = 'occupied'
  AND NOT EXISTS (
      SELECT 1 FROM sessions s
      WHERE s.table_id = t.id AND s.status = 'active'
  );

UPDATE tables t
SET status = 'occupied'
WHERE EXISTS (
    SELECT 1 FROM sessions s
    WHERE s.table_id = t.id AND s.status = 'active'
)
  AND t.status = 'available';

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
