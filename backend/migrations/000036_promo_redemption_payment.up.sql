-- Promo redemption moves from order placement to payment initiation: a
-- redemption is now recorded against the payment that consumed the promo (the
-- discount lands in the immutable bill snapshot). Additive — existing
-- order-linked redemptions keep their order_id.

ALTER TABLE promo_redemptions
  ADD COLUMN payment_id BIGINT REFERENCES payments(id);

-- order_id was NOT NULL (order-time redemption); payment-time redemptions have
-- no order_id, so relax it. Existing rows are unaffected.
ALTER TABLE promo_redemptions
  ALTER COLUMN order_id DROP NOT NULL;

-- Idempotency for payment-linked redemptions (mirrors the existing
-- (promo_id, order_id) unique index).
CREATE UNIQUE INDEX idx_redemptions_promo_payment
  ON promo_redemptions(promo_id, payment_id)
  WHERE payment_id IS NOT NULL;
