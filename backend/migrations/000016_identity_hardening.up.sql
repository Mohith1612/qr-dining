ALTER TABLE branches
  ADD COLUMN branch_code TEXT;

UPDATE branches
SET branch_code = 'BR-' || id
WHERE branch_code IS NULL;

ALTER TABLE branches
  ALTER COLUMN branch_code SET NOT NULL;

CREATE UNIQUE INDEX idx_branches_branch_code_unique ON branches(branch_code);

ALTER TABLE staff
  ADD COLUMN staff_code TEXT,
  ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN pin_version INTEGER NOT NULL DEFAULT 1;

UPDATE staff
SET staff_code = 'STAFF-' || id
WHERE staff_code IS NULL;

ALTER TABLE staff
  ALTER COLUMN staff_code SET NOT NULL;

CREATE UNIQUE INDEX idx_staff_branch_staff_code_unique ON staff(branch_id, staff_code);

ALTER TABLE session_participants
  ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE staff_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  staff_id        BIGINT NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
  branch_id       BIGINT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  token_hash      TEXT NOT NULL UNIQUE,
  device_name     TEXT NOT NULL DEFAULT '',
  token_version   INTEGER NOT NULL,
  pin_version     INTEGER NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at      TIMESTAMPTZ NOT NULL,
  revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_staff_sessions_staff_active
  ON staff_sessions(staff_id, expires_at)
  WHERE revoked_at IS NULL;
