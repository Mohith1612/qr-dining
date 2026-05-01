-- ============================================================
-- ENUMS
-- All custom types defined before any table that references them.
-- ============================================================

CREATE TYPE table_status AS ENUM ('available', 'occupied', 'reserved');
CREATE TYPE staff_role AS ENUM ('owner', 'manager', 'waiter', 'kitchen');
CREATE TYPE session_status AS ENUM ('active', 'closed', 'abandoned');
CREATE TYPE order_status AS ENUM (
    'pending', 'confirmed', 'preparing', 'ready', 'served', 'cancelled'
);
CREATE TYPE assistance_type AS ENUM ('waiter', 'bill', 'other');
CREATE TYPE assistance_status AS ENUM ('pending', 'acknowledged', 'resolved');
CREATE TYPE payment_method AS ENUM ('cash', 'card', 'digital');
CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');

-- ============================================================
-- RESTAURANTS
-- ============================================================

CREATE TABLE restaurants (
    id            BIGSERIAL     PRIMARY KEY,
    name          TEXT          NOT NULL,
    slug          TEXT          NOT NULL UNIQUE,
    -- JSONB policy: flexible restaurant-level config (tax rate, tipping, receipt footer).
    -- Intentionally schemaless — varies per restaurant, rarely queried structurally.
    -- Never use for operational data that needs filtering or aggregation.
    settings_json JSONB         NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ============================================================
-- BRANCHES
-- ============================================================

CREATE TABLE branches (
    id            BIGSERIAL     PRIMARY KEY,
    restaurant_id BIGINT        NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name          TEXT          NOT NULL,
    address       TEXT          NOT NULL DEFAULT '',
    timezone      TEXT          NOT NULL DEFAULT 'UTC',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_branches_restaurant_id ON branches(restaurant_id);

-- ============================================================
-- TABLES
-- ============================================================

CREATE TABLE tables (
    id            BIGSERIAL     PRIMARY KEY,
    branch_id     BIGINT        NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    identifier    TEXT          NOT NULL,
    capacity      SMALLINT      NOT NULL DEFAULT 4,
    -- qr_code_token: 32-byte cryptographically random hex, embedded in QR code URL.
    qr_code_token TEXT          NOT NULL UNIQUE,
    status        table_status  NOT NULL DEFAULT 'available',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE(branch_id, identifier)
);

CREATE INDEX idx_tables_branch_id     ON tables(branch_id);
CREATE INDEX idx_tables_qr_code_token ON tables(qr_code_token);

-- ============================================================
-- STAFF
-- ============================================================

CREATE TABLE staff (
    id         BIGSERIAL   PRIMARY KEY,
    branch_id  BIGINT      NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    role       staff_role  NOT NULL,
    -- pin_hash: bcrypt hash of the staff PIN (4–6 digits). Never store plaintext PINs.
    pin_hash   TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_staff_branch_id ON staff(branch_id);

-- ============================================================
-- MENU CATEGORIES
-- ============================================================

CREATE TABLE menu_categories (
    id        BIGSERIAL   PRIMARY KEY,
    branch_id BIGINT      NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    name      TEXT        NOT NULL,
    position  SMALLINT    NOT NULL DEFAULT 0,
    is_active BOOLEAN     NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_menu_categories_branch_position ON menu_categories(branch_id, position);

-- ============================================================
-- MENU ITEMS
-- ============================================================

CREATE TABLE menu_items (
    id           BIGSERIAL       PRIMARY KEY,
    category_id  BIGINT          NOT NULL REFERENCES menu_categories(id) ON DELETE CASCADE,
    branch_id    BIGINT          NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    name         TEXT            NOT NULL,
    description  TEXT            NOT NULL DEFAULT '',
    price        NUMERIC(10, 2)  NOT NULL CHECK (price >= 0),
    is_available BOOLEAN         NOT NULL DEFAULT TRUE,
    position     SMALLINT        NOT NULL DEFAULT 0
);

CREATE INDEX idx_menu_items_category_id ON menu_items(category_id);
CREATE INDEX idx_menu_items_branch_id   ON menu_items(branch_id);

-- ============================================================
-- ITEM MODIFIERS
-- ============================================================

CREATE TABLE item_modifiers (
    id          BIGSERIAL      PRIMARY KEY,
    item_id     BIGINT         NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    name        TEXT           NOT NULL,
    price_delta NUMERIC(10, 2) NOT NULL DEFAULT 0,
    is_required BOOLEAN        NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_item_modifiers_item_id ON item_modifiers(item_id);

-- ============================================================
-- SESSIONS
-- host_participant_id is nullable here; the FK back to session_participants
-- is added DEFERRABLE INITIALLY DEFERRED below to break the circular dependency.
-- Both rows (session + host participant) are inserted in a single transaction;
-- the deferred FK is verified at COMMIT time.
-- ============================================================

CREATE TABLE sessions (
    id                  UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id           BIGINT         NOT NULL REFERENCES branches(id),
    table_id            BIGINT         NOT NULL REFERENCES tables(id),
    host_participant_id BIGINT,
    status              session_status NOT NULL DEFAULT 'active',
    session_token       TEXT           NOT NULL UNIQUE,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    closed_at           TIMESTAMPTZ
);

CREATE INDEX idx_sessions_branch_id     ON sessions(branch_id);
CREATE INDEX idx_sessions_table_id      ON sessions(table_id);
CREATE INDEX idx_sessions_session_token ON sessions(session_token);
-- Partial index: fast lookup of active sessions only.
CREATE INDEX idx_sessions_active_status ON sessions(status) WHERE status = 'active';

-- ============================================================
-- SESSION PARTICIPANTS
-- ============================================================

CREATE TABLE session_participants (
    id                 BIGSERIAL   PRIMARY KEY,
    session_id         UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    display_name       TEXT        NOT NULL,
    device_fingerprint TEXT        NOT NULL DEFAULT '',
    joined_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_host            BOOLEAN     NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_session_participants_session_id ON session_participants(session_id);

-- Add the deferred FK now that session_participants exists.
ALTER TABLE sessions
    ADD CONSTRAINT fk_sessions_host_participant
    FOREIGN KEY (host_participant_id)
    REFERENCES session_participants(id)
    DEFERRABLE INITIALLY DEFERRED;

-- ============================================================
-- CARTS
-- participant_id is nullable to support a shared-cart mode where the entire
-- table collaborates on a single cart rather than per-person carts.
-- UNIQUE(session_id, participant_id) handles both cases correctly:
-- NULL participant_id means the shared cart; each participant gets one cart.
-- ============================================================

CREATE TABLE carts (
    id             BIGSERIAL  PRIMARY KEY,
    session_id     UUID       NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    participant_id BIGINT     REFERENCES session_participants(id) ON DELETE SET NULL,
    UNIQUE(session_id, participant_id)
);

CREATE INDEX idx_carts_session_id ON carts(session_id);

-- ============================================================
-- CART ITEMS
-- selected_modifiers_json: snapshot of selected modifier IDs and names
-- at add-to-cart time. Denormalized by design — menu changes do not affect
-- items already in the cart. Display only; not filtered or aggregated.
-- ============================================================

CREATE TABLE cart_items (
    id                      BIGSERIAL      PRIMARY KEY,
    cart_id                 BIGINT         NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    menu_item_id            BIGINT         NOT NULL REFERENCES menu_items(id),
    quantity                SMALLINT       NOT NULL DEFAULT 1 CHECK (quantity > 0),
    selected_modifiers_json JSONB          NOT NULL DEFAULT '[]',
    note                    TEXT           NOT NULL DEFAULT ''
);

CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);

-- ============================================================
-- ORDERS
-- idempotency_key: client-generated unique key per order attempt.
-- Service checks for existing key before insert; DB UNIQUE constraint
-- is the safety net against TOCTOU races.
-- ============================================================

CREATE TABLE orders (
    id                       UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id               UUID           NOT NULL REFERENCES sessions(id),
    branch_id                BIGINT         NOT NULL REFERENCES branches(id),
    placed_by_participant_id BIGINT         REFERENCES session_participants(id) ON DELETE SET NULL,
    status                   order_status   NOT NULL DEFAULT 'pending',
    idempotency_key          TEXT           NOT NULL UNIQUE,
    total_amount             NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_session_id  ON orders(session_id);
CREATE INDEX idx_orders_branch_id   ON orders(branch_id);
CREATE INDEX idx_orders_status      ON orders(status);
CREATE INDEX idx_orders_created_at  ON orders(created_at DESC);

-- ============================================================
-- ORDER ITEMS
-- unit_price + selected_modifiers_json: price snapshot at order time.
-- Menu price changes do not retroactively affect placed orders.
-- Correct financial behavior — display only, not aggregated.
-- ============================================================

CREATE TABLE order_items (
    id                      BIGSERIAL      PRIMARY KEY,
    order_id                UUID           NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id            BIGINT         NOT NULL REFERENCES menu_items(id),
    quantity                SMALLINT       NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price              NUMERIC(10, 2) NOT NULL,
    selected_modifiers_json JSONB          NOT NULL DEFAULT '[]',
    note                    TEXT           NOT NULL DEFAULT ''
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);

-- ============================================================
-- ASSISTANCE REQUESTS
-- ============================================================

CREATE TABLE assistance_requests (
    id             BIGSERIAL         PRIMARY KEY,
    session_id     UUID              NOT NULL REFERENCES sessions(id),
    table_id       BIGINT            NOT NULL REFERENCES tables(id),
    participant_id BIGINT            REFERENCES session_participants(id) ON DELETE SET NULL,
    type           assistance_type   NOT NULL DEFAULT 'waiter',
    status         assistance_status NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    resolved_at    TIMESTAMPTZ
);

CREATE INDEX idx_assistance_session_id ON assistance_requests(session_id);
-- Partial index: waiter dashboard queries only active requests.
CREATE INDEX idx_assistance_active ON assistance_requests(table_id, status)
    WHERE status IN ('pending', 'acknowledged');

-- ============================================================
-- PAYMENTS
-- order_id is nullable: supports session-level payment (pay entire bill)
-- as well as per-order payment.
-- ============================================================

CREATE TABLE payments (
    id           BIGSERIAL      PRIMARY KEY,
    session_id   UUID           NOT NULL REFERENCES sessions(id),
    order_id     UUID           REFERENCES orders(id) ON DELETE SET NULL,
    amount       NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    method       payment_method NOT NULL,
    status       payment_status NOT NULL DEFAULT 'pending',
    initiated_at TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_payments_session_id ON payments(session_id);
CREATE INDEX idx_payments_order_id   ON payments(order_id);
