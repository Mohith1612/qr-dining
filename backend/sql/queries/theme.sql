-- name: ListThemePresets :many
SELECT * FROM theme_presets ORDER BY key;

-- name: GetThemePreset :one
SELECT * FROM theme_presets WHERE key = $1;

-- name: GetTenantThemeByRestaurant :one
SELECT * FROM tenant_themes WHERE restaurant_id = $1;

-- name: UpsertTenantTheme :one
INSERT INTO tenant_themes (restaurant_id, preset, tokens_json, updated_by_platform_user_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (restaurant_id)
DO UPDATE SET preset = EXCLUDED.preset, tokens_json = EXCLUDED.tokens_json,
              updated_by_platform_user_id = EXCLUDED.updated_by_platform_user_id, updated_at = NOW()
RETURNING *;
