-- Daily time windows are owner-entered wall-clock times for the BRANCH, so
-- they are compared against branch-local time, not the DB server's LOCALTIME
-- (UTC in production — a lunch-hour promo would silently never match at
-- Indian lunch hours).

-- name: GetPromoByCode :one
SELECT * FROM promos
WHERE branch_id = $1
  AND LOWER(code) = LOWER($2)
  AND is_active = TRUE
  AND valid_from <= now()
  AND valid_until >= now()
  AND (
    time_window_start IS NULL
    OR (
      time_window_start <= (NOW() AT TIME ZONE (SELECT b.timezone FROM branches b WHERE b.id = promos.branch_id))::time
      AND (NOW() AT TIME ZONE (SELECT b.timezone FROM branches b WHERE b.id = promos.branch_id))::time <= time_window_end
    )
  );

-- name: GetPromoByCodeForUpdate :one
SELECT * FROM promos
WHERE branch_id = $1
  AND LOWER(code) = LOWER($2)
  AND is_active = TRUE
  AND valid_from <= now()
  AND valid_until >= now()
  AND (
    time_window_start IS NULL
    OR (
      time_window_start <= (NOW() AT TIME ZONE (SELECT b.timezone FROM branches b WHERE b.id = promos.branch_id))::time
      AND (NOW() AT TIME ZONE (SELECT b.timezone FROM branches b WHERE b.id = promos.branch_id))::time <= time_window_end
    )
  )
FOR UPDATE;

-- name: GetPromoByID :one
SELECT * FROM promos WHERE id = $1;

-- name: CountPromoRedemptions :one
SELECT COUNT(*) FROM promo_redemptions WHERE promo_id = $1;

-- name: CountPromoRedemptionsByPhone :one
SELECT COUNT(*) FROM promo_redemptions
WHERE promo_id = $1 AND phone_e164 = $2;

-- name: CreatePromoRedemption :one
INSERT INTO promo_redemptions (promo_id, order_id, phone_e164)
VALUES ($1, $2, $3) RETURNING *;

-- name: IncrementPromoRedemptionCount :exec
UPDATE promos
SET redeemed_count = redeemed_count + 1
WHERE id = $1;

-- name: CreatePromo :one
INSERT INTO promos (
  branch_id, code, type, value, min_order_amount,
  max_uses, uses_per_phone, valid_from, valid_until,
  time_window_start, time_window_end, description, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: ListPromosForBranch :many
SELECT * FROM promos WHERE branch_id = $1 ORDER BY created_at DESC;

-- name: DeactivatePromo :exec
UPDATE promos SET is_active = FALSE WHERE id = $1 AND branch_id = $2;
