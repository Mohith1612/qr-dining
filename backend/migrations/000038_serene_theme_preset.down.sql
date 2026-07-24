-- Remove the serene preset. Drop dependent tenant_themes rows first (FK to
-- theme_presets.key) so the delete is safe.
DELETE FROM tenant_themes WHERE preset = 'serene';
DELETE FROM theme_presets WHERE key = 'serene';
