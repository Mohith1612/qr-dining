-- Drop in reverse dependency order.
-- Enums are dropped last because tables reference them.
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS assistance_requests CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS cart_items CASCADE;
DROP TABLE IF EXISTS carts CASCADE;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_host_participant;
DROP TABLE IF EXISTS session_participants CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS item_modifiers CASCADE;
DROP TABLE IF EXISTS menu_items CASCADE;
DROP TABLE IF EXISTS menu_categories CASCADE;
DROP TABLE IF EXISTS staff CASCADE;
DROP TABLE IF EXISTS tables CASCADE;
DROP TABLE IF EXISTS branches CASCADE;
DROP TABLE IF EXISTS restaurants CASCADE;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS assistance_status;
DROP TYPE IF EXISTS assistance_type;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS session_status;
DROP TYPE IF EXISTS staff_role;
DROP TYPE IF EXISTS table_status;
