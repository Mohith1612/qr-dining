-- Platform governance: structured theme/branding config (presets + validated tokens).
-- Additive only. Custom tokens are gated by the custom.theme entitlement at the
-- service layer. The legacy restaurants.settings_json.theme path is left untouched.

-- Preset catalog. Token VALUES live in the frontend CSS (frontend/styles/themes.css);
-- the backend validates only the preset key. Seeded to match the frontend presets.
CREATE TABLE theme_presets (
    key         TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-tenant (restaurant-scoped; org is 1:1 with restaurant today) structured theme.
CREATE TABLE tenant_themes (
    restaurant_id               BIGINT PRIMARY KEY REFERENCES restaurants(id) ON DELETE CASCADE,
    preset                      TEXT NOT NULL REFERENCES theme_presets(key),
    tokens_json                 JSONB NOT NULL DEFAULT '{}',
    updated_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO theme_presets (key, name, description) VALUES
    ('dark-luxury',    'Dark Luxury',    'Default dark, warm-gold luxury theme'),
    ('modern-minimal', 'Modern Minimal', 'Clean, light minimal theme'),
    ('warm-cafe',      'Warm Cafe',      'Warm, cozy cafe theme'),
    ('vibrant',        'Vibrant',        'Bright, high-energy theme');
