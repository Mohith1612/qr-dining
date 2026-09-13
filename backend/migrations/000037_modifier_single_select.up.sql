-- Single-select modifier groups. A modifier group (item_id + modifier_group)
-- is single-select when ANY of its rows carry this flag; the admin UI writes
-- the flag group-wide. Guests may then pick at most one option from the group
-- (e.g. lemonade: sweet OR salty, not both), enforced server-side in the cart.
ALTER TABLE item_modifiers
  ADD COLUMN single_select BOOLEAN NOT NULL DEFAULT FALSE;
