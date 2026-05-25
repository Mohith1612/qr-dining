ALTER TABLE branches
  ADD COLUMN session_timeout_minutes SMALLINT NOT NULL DEFAULT 120;
