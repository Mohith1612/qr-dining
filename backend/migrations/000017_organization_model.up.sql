CREATE TABLE organizations (
    id                    BIGSERIAL PRIMARY KEY,
    code                  TEXT NOT NULL UNIQUE,
    name                  TEXT NOT NULL,
    legal_name            TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
    primary_contact_email TEXT NOT NULL DEFAULT '',
    settings_json         JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE restaurants
    ADD COLUMN organization_id BIGINT;

ALTER TABLE branches
    ADD COLUMN organization_id BIGINT,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
    ADD COLUMN support_metadata_json JSONB NOT NULL DEFAULT '{}';

INSERT INTO organizations (code, name, settings_json, created_at, updated_at)
SELECT slug, name, settings_json, created_at, NOW()
FROM restaurants;

UPDATE restaurants r
SET organization_id = o.id
FROM organizations o
WHERE o.code = r.slug;

UPDATE branches b
SET organization_id = r.organization_id
FROM restaurants r
WHERE r.id = b.restaurant_id;

ALTER TABLE restaurants
    ALTER COLUMN organization_id SET NOT NULL,
    ADD CONSTRAINT fk_restaurants_organization
        FOREIGN KEY (organization_id) REFERENCES organizations(id);

ALTER TABLE branches
    ALTER COLUMN organization_id SET NOT NULL,
    ADD CONSTRAINT fk_branches_organization
        FOREIGN KEY (organization_id) REFERENCES organizations(id);

CREATE UNIQUE INDEX idx_restaurants_organization_id_unique
    ON restaurants(organization_id);

CREATE INDEX idx_branches_organization_id
    ON branches(organization_id);

CREATE UNIQUE INDEX idx_branches_organization_branch_code_unique
    ON branches(organization_id, branch_code);

CREATE TABLE organization_members (
    id                  BIGSERIAL PRIMARY KEY,
    organization_id     BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    staff_id            BIGINT NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    role                TEXT NOT NULL CHECK (role IN ('owner', 'admin')),
    status              TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'removed')),
    invited_by_staff_id BIGINT REFERENCES staff(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, staff_id)
);

CREATE INDEX idx_organization_members_staff_active
    ON organization_members(staff_id, organization_id)
    WHERE status = 'active';

INSERT INTO organization_members (organization_id, staff_id, role, status)
SELECT DISTINCT
    b.organization_id,
    s.id,
    CASE WHEN s.role = 'owner' THEN 'owner' ELSE 'admin' END,
    'active'
FROM staff s
JOIN branches b ON b.id = s.branch_id
WHERE s.role IN ('owner', 'manager')
  AND s.is_active = TRUE;

CREATE TABLE organization_branch_memberships (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    branch_id       BIGINT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, branch_id)
);

INSERT INTO organization_branch_memberships (organization_id, branch_id)
SELECT organization_id, id
FROM branches;
