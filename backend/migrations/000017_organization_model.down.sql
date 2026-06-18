DROP TABLE IF EXISTS organization_branch_memberships;
DROP INDEX IF EXISTS idx_organization_members_staff_active;
DROP TABLE IF EXISTS organization_members;

DROP INDEX IF EXISTS idx_branches_organization_branch_code_unique;
DROP INDEX IF EXISTS idx_branches_organization_id;
DROP INDEX IF EXISTS idx_restaurants_organization_id_unique;

ALTER TABLE branches
    DROP CONSTRAINT IF EXISTS fk_branches_organization,
    DROP COLUMN IF EXISTS support_metadata_json,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS organization_id;

ALTER TABLE restaurants
    DROP CONSTRAINT IF EXISTS fk_restaurants_organization,
    DROP COLUMN IF EXISTS organization_id;

DROP TABLE IF EXISTS organizations;
