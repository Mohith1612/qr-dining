-- Premium QR collateral: structured per-branch print/branding config consumed by the
-- collateral renderer (Theme + Branch metadata + this config). Additive only.
--
-- The config holds ONLY the physical/content concern (chosen print format, content
-- toggles, welcome/subtitle/footer text, WiFi, socials, tagline) — never theme tokens.
-- The theme stays the source of truth for colour/typography; this never duplicates it.
CREATE TABLE branch_collateral (
    branch_id                   BIGINT PRIMARY KEY REFERENCES branches(id) ON DELETE CASCADE,
    config_json                 JSONB NOT NULL DEFAULT '{}',
    updated_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
