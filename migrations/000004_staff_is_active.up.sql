-- Add is_active flag to staff for soft deactivation.
-- Existing staff are all active by default.
ALTER TABLE staff ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- Index for efficient filtering of active staff per branch.
CREATE INDEX idx_staff_branch_active ON staff(branch_id, is_active);
