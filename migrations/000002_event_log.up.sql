-- Operational audit log. Insert-only — no updates, no deletes.
-- Each row records one operational fact at a point in time.
-- This is NOT event sourcing and does not drive application state.
-- PostgreSQL remains the source of truth. This table is supplemental:
-- debugging, auditing, analytics, and future replay support.
-- Wire incrementally via repo.LogEvent() in services as needed.

CREATE TABLE event_log (
    id          BIGSERIAL   PRIMARY KEY,
    session_id  UUID        REFERENCES sessions(id) ON DELETE SET NULL,
    branch_id   BIGINT      REFERENCES branches(id) ON DELETE SET NULL,
    -- event_type matches the WebSocket event envelope constants.
    event_type  TEXT        NOT NULL,
    -- actor_type: "participant" | "staff" | "system"
    actor_type  TEXT        NOT NULL,
    -- actor_id: string representation of participant_id, staff_id, or null for system events.
    actor_id    TEXT,
    payload     JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_event_log_session_id ON event_log(session_id);
CREATE INDEX idx_event_log_branch_id  ON event_log(branch_id);
CREATE INDEX idx_event_log_event_type ON event_log(event_type);
CREATE INDEX idx_event_log_created_at ON event_log(created_at DESC);
