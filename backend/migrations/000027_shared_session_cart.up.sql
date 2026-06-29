-- Shared session cart: a single cart per session, collaboratively edited by all
-- participants, identified by participant_id IS NULL.
--
-- The table-level UNIQUE(session_id, participant_id) does NOT enforce one shared
-- cart, because Postgres treats NULLs as distinct in unique constraints. This
-- partial unique index guarantees at most one shared cart per session so
-- GetOrCreateSessionCart can rely on ON CONFLICT for the NULL-participant row.
CREATE UNIQUE INDEX IF NOT EXISTS carts_session_shared_uniq
    ON carts (session_id)
    WHERE participant_id IS NULL;
