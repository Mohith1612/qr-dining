-- Serene Hospitality becomes the new default guest theme preset (UI redesign).
-- Additive: registers the preset in the catalog so tenants/branches can select it
-- and so the server-side default resolves to a known preset.
INSERT INTO theme_presets (key, name, description) VALUES
    ('serene', 'Serene Hospitality', 'Alabaster canvas, charcoal ink, muted gold accent')
ON CONFLICT (key) DO NOTHING;
