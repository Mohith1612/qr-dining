-- Customer loyalty (migration 000035). Accounts are org-scoped; the
-- transactions table is an append-only signed points ledger.

-- name: UpsertLoyaltyProgram :one
INSERT INTO organization_loyalty_programs (organization_id, is_active, earn_rate_points, earn_rate_amount, updated_by_staff_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id) DO UPDATE
SET is_active           = EXCLUDED.is_active,
    earn_rate_points    = EXCLUDED.earn_rate_points,
    earn_rate_amount    = EXCLUDED.earn_rate_amount,
    updated_by_staff_id = EXCLUDED.updated_by_staff_id,
    updated_at          = NOW()
RETURNING *;

-- name: GetLoyaltyProgram :one
SELECT * FROM organization_loyalty_programs WHERE organization_id = $1;

-- Accrual upsert: creates the account on first earn, otherwise adds points,
-- spend, and (first earn per session only — caller decides) a visit.
-- name: UpsertLoyaltyAccountForEarn :one
INSERT INTO customer_loyalty_accounts
    (customer_id, organization_id, points_balance, lifetime_points_earned, visit_count, lifetime_spend)
VALUES
    (sqlc.arg(customer_id), sqlc.arg(organization_id), sqlc.arg(points), sqlc.arg(points), sqlc.arg(visit_increment), sqlc.arg(amount))
ON CONFLICT (customer_id, organization_id) DO UPDATE
SET points_balance         = customer_loyalty_accounts.points_balance + EXCLUDED.points_balance,
    lifetime_points_earned = customer_loyalty_accounts.lifetime_points_earned + EXCLUDED.lifetime_points_earned,
    visit_count            = customer_loyalty_accounts.visit_count + EXCLUDED.visit_count,
    lifetime_spend         = customer_loyalty_accounts.lifetime_spend + EXCLUDED.lifetime_spend,
    updated_at             = NOW()
RETURNING *;

-- name: InsertLoyaltyTransaction :one
INSERT INTO customer_loyalty_transactions
    (account_id, type, points, amount, payment_id, session_id, performed_by_actor_type, performed_by_staff_id, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CountEarnTransactionsForAccountSession :one
SELECT COUNT(*)::BIGINT
FROM customer_loyalty_transactions
WHERE account_id = $1 AND session_id = $2 AND type = 'earn';

-- Guarded deduction: returns no row when the balance is insufficient. The row
-- lock serializes concurrent redeems; the points_balance >= 0 CHECK is the
-- backstop.
-- name: DeductLoyaltyPoints :one
UPDATE customer_loyalty_accounts
SET points_balance           = points_balance - sqlc.arg(points)::BIGINT,
    lifetime_points_redeemed = lifetime_points_redeemed + sqlc.arg(points)::BIGINT,
    updated_at               = NOW()
WHERE id = sqlc.arg(account_id)
  AND organization_id = sqlc.arg(organization_id)
  AND points_balance >= sqlc.arg(points)::BIGINT
RETURNING *;

-- Signed manual adjustment; floors at zero. Positive part counts as earned,
-- negative part as redeemed so balance = earned - redeemed stays invariant.
-- name: AdjustLoyaltyPoints :one
UPDATE customer_loyalty_accounts
SET points_balance           = points_balance + sqlc.arg(points_delta)::BIGINT,
    lifetime_points_earned   = lifetime_points_earned + GREATEST(sqlc.arg(points_delta)::BIGINT, 0),
    lifetime_points_redeemed = lifetime_points_redeemed + GREATEST(-sqlc.arg(points_delta)::BIGINT, 0),
    updated_at               = NOW()
WHERE id = sqlc.arg(account_id)
  AND organization_id = sqlc.arg(organization_id)
  AND points_balance + sqlc.arg(points_delta)::BIGINT >= 0
RETURNING *;

-- name: GetLoyaltyAccountByID :one
SELECT * FROM customer_loyalty_accounts WHERE id = $1 AND organization_id = $2;

-- name: GetLoyaltyAccountByCustomer :one
SELECT * FROM customer_loyalty_accounts WHERE customer_id = $1 AND organization_id = $2;

-- name: ListLoyaltyTransactions :many
SELECT * FROM customer_loyalty_transactions
WHERE account_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetLoyaltyPointsSummary :one
SELECT
    COALESCE(SUM(t.points)  FILTER (WHERE t.type = 'earn'), 0)::BIGINT   AS points_issued,
    COALESCE(SUM(-t.points) FILTER (WHERE t.type = 'redeem'), 0)::BIGINT AS points_redeemed,
    COUNT(*) FILTER (WHERE t.type = 'earn')::BIGINT                      AS earn_count,
    COUNT(*) FILTER (WHERE t.type = 'redeem')::BIGINT                    AS redeem_count,
    COUNT(DISTINCT a.customer_id)::BIGINT                                AS active_customers
FROM customer_loyalty_transactions t
JOIN customer_loyalty_accounts a ON a.id = t.account_id
WHERE a.organization_id = sqlc.arg(organization_id)
  AND t.created_at >= sqlc.arg(from_time)::timestamptz
  AND t.created_at <  sqlc.arg(to_time)::timestamptz;

-- Participation: sessions in the window that produced a loyalty earn vs all
-- sessions, org-scoped via branches.
-- name: GetLoyaltyParticipation :one
SELECT
    COUNT(DISTINCT s.id)::BIGINT           AS total_sessions,
    COUNT(DISTINCT t.session_id)::BIGINT   AS loyalty_sessions
FROM sessions s
JOIN branches b ON b.id = s.branch_id
LEFT JOIN customer_loyalty_transactions t ON t.session_id = s.id AND t.type = 'earn'
WHERE b.organization_id = sqlc.arg(organization_id)
  AND s.created_at >= sqlc.arg(from_time)::timestamptz
  AND s.created_at <  sqlc.arg(to_time)::timestamptz;

-- name: CountLoyaltyAccounts :one
SELECT COUNT(*)::BIGINT FROM customer_loyalty_accounts WHERE organization_id = $1;

-- name: GetTopLoyaltyCustomers :many
SELECT
    a.id AS account_id,
    a.customer_id,
    c.display_name,
    c.phone_e164,
    a.points_balance,
    a.lifetime_points_earned,
    a.visit_count,
    a.lifetime_spend
FROM customer_loyalty_accounts a
JOIN customers c ON c.id = a.customer_id
WHERE a.organization_id = $1
ORDER BY a.lifetime_points_earned DESC, a.lifetime_spend DESC
LIMIT 10;
