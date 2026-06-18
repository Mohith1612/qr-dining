DROP INDEX IF EXISTS idx_staff_sessions_staff_active;
DROP TABLE IF EXISTS staff_sessions;

ALTER TABLE session_participants
  DROP COLUMN IF EXISTS credential_version;

DROP INDEX IF EXISTS idx_staff_branch_staff_code_unique;

ALTER TABLE staff
  DROP COLUMN IF EXISTS pin_version,
  DROP COLUMN IF EXISTS token_version,
  DROP COLUMN IF EXISTS staff_code;

DROP INDEX IF EXISTS idx_branches_branch_code_unique;

ALTER TABLE branches
  DROP COLUMN IF EXISTS branch_code;
