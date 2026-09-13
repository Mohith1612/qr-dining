-- Backfill promo_redemptions.phone_e164 to the canonical form used by the
-- per-phone redemption cap.
--
-- The cap query (CountPromoRedemptionsByPhone) is an exact string match against
-- a normalized phone. Two defects put non-canonical values in this column:
--   1. The payment-initiation redemption path stored the raw client string, so
--      any punctuation variant wrote a row the cap query could never see.
--   2. The normalizer itself only kept "+" at index 0, so a leading "(" dropped
--      it and produced a second canonical form for the same number.
-- Both are fixed in services/promo.go and services/payment.go. This backfill
-- collapses existing rows to the same canonical form (digits only, "+"-prefixed)
-- so historical redemptions still count against the cap.
UPDATE promo_redemptions
SET phone_e164 = '+' || regexp_replace(phone_e164, '[^0-9]', '', 'g')
WHERE phone_e164 IS NOT NULL
  AND regexp_replace(phone_e164, '[^0-9]', '', 'g') <> ''
  AND phone_e164 IS DISTINCT FROM '+' || regexp_replace(phone_e164, '[^0-9]', '', 'g');

-- Rows that held no digits at all were never usable as a cap key.
UPDATE promo_redemptions
SET phone_e164 = NULL
WHERE phone_e164 IS NOT NULL
  AND regexp_replace(phone_e164, '[^0-9]', '', 'g') = '';
