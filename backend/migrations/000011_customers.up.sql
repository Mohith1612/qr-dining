CREATE TABLE customers (
    id            BIGSERIAL    PRIMARY KEY,
    restaurant_id BIGINT       NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    phone_e164    TEXT         NOT NULL,
    display_name  TEXT         NOT NULL DEFAULT '',
    opted_in      BOOLEAN      NOT NULL DEFAULT TRUE,
    opted_in_at   TIMESTAMPTZ,
    last_seen_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    visit_count   INTEGER      NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(restaurant_id, phone_e164)
);

CREATE INDEX idx_customers_restaurant_id ON customers(restaurant_id);
CREATE INDEX idx_customers_phone ON customers(restaurant_id, phone_e164);

ALTER TABLE sessions ADD COLUMN customer_id BIGINT REFERENCES customers(id) ON DELETE SET NULL;
CREATE INDEX idx_sessions_customer_id ON sessions(customer_id);
