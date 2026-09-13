DROP INDEX IF EXISTS idx_redemptions_promo_payment;

-- Restore the NOT NULL on order_id. Any payment-only redemptions must be
-- cleared first or this will fail (acceptable: down-migrations run on throwaway
-- DBs, never with live payment-linked redemptions).
DELETE FROM promo_redemptions WHERE order_id IS NULL;
ALTER TABLE promo_redemptions ALTER COLUMN order_id SET NOT NULL;

ALTER TABLE promo_redemptions DROP COLUMN IF EXISTS payment_id;
