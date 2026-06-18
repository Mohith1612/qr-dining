-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationByCode :one
SELECT * FROM organizations WHERE code = $1;

-- name: GetOrganizationByBranchID :one
SELECT o.*
FROM organizations o
JOIN branches b ON b.organization_id = o.id
WHERE b.id = $1;

-- name: GetOrganizationByRestaurantID :one
SELECT o.*
FROM organizations o
JOIN restaurants r ON r.organization_id = o.id
WHERE r.id = $1;

-- name: GetRestaurantByOrganizationID :one
SELECT * FROM restaurants WHERE organization_id = $1;

-- name: GetOrganizationMembershipForStaff :one
SELECT * FROM organization_members
WHERE organization_id = $1
  AND staff_id = $2
  AND status = 'active';

-- name: ListBranchesForOrganization :many
SELECT *
FROM branches
WHERE organization_id = $1
ORDER BY name ASC, id ASC;

-- name: UpdateOrganizationSettings :one
UPDATE organizations
SET
    name = $2,
    legal_name = $3,
    primary_contact_email = $4,
    settings_json = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
