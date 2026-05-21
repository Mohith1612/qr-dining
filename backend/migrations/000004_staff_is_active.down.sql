DROP INDEX IF EXISTS idx_staff_branch_active;
ALTER TABLE staff DROP COLUMN IF EXISTS is_active;
