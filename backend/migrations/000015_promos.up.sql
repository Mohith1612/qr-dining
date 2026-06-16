CREATE TYPE promo_type AS ENUM ('flat_amount', 'percentage');

CREATE TABLE promos (
  id               BIGSERIAL PRIMARY KEY,
  branch_id        BIGINT NOT NULL REFERENCES branches(id),
  code             TEXT NOT NULL,
  type             promo_type NOT NULL,
  value            NUMERIC(10,2) NOT NULL,
  min_order_amount NUMERIC(10,2) NOT NULL DEFAULT 0,
  max_uses         INT,
  uses_per_phone   INT NOT NULL DEFAULT 1,
  valid_from       TIMESTAMPTZ NOT NULL,
  valid_until      TIMESTAMPTZ NOT NULL,
  time_window_start TIME,
  time_window_end   TIME,
  is_active        BOOLEAN NOT NULL DEFAULT TRUE,
  description      TEXT,
  created_by       BIGINT REFERENCES staff(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_promos_branch_code ON promos(branch_id, LOWER(code)) WHERE is_active;

CREATE TABLE promo_redemptions (
  id           BIGSERIAL PRIMARY KEY,
  promo_id     BIGINT NOT NULL REFERENCES promos(id),
  order_id     UUID NOT NULL REFERENCES orders(id),
  phone_e164   TEXT,
  redeemed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_redemptions_promo ON promo_redemptions(promo_id);
CREATE INDEX idx_redemptions_phone ON promo_redemptions(promo_id, phone_e164) WHERE phone_e164 IS NOT NULL;

ALTER TABLE orders
  ADD COLUMN promo_id        BIGINT REFERENCES promos(id),
  ADD COLUMN discount_amount NUMERIC(10,2) NOT NULL DEFAULT 0;
