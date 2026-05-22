-- name: ListPlans :many
SELECT * FROM subscription_plans ORDER BY price_monthly ASC;

-- name: GetPlanByTier :one
SELECT * FROM subscription_plans WHERE tier = $1;

-- name: GetSubscriptionByRestaurant :one
SELECT
    rs.id,
    rs.restaurant_id,
    rs.plan_id,
    rs.status,
    rs.trial_ends_at,
    rs.current_period_start,
    rs.current_period_end,
    rs.created_at,
    rs.updated_at,
    sp.name        AS plan_name,
    sp.tier        AS plan_tier,
    sp.price_monthly AS plan_price_monthly,
    sp.features_json AS plan_features_json
FROM restaurant_subscriptions rs
JOIN subscription_plans sp ON sp.id = rs.plan_id
WHERE rs.restaurant_id = $1;

-- name: UpsertSubscription :one
INSERT INTO restaurant_subscriptions (
    restaurant_id, plan_id, status, trial_ends_at,
    current_period_start, current_period_end
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (restaurant_id) DO UPDATE SET
    plan_id              = EXCLUDED.plan_id,
    status               = EXCLUDED.status,
    trial_ends_at        = EXCLUDED.trial_ends_at,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end   = EXCLUDED.current_period_end,
    updated_at           = NOW()
RETURNING *;
