ALTER TABLE menu_items
  ADD COLUMN dietary_flags TEXT[]   NOT NULL DEFAULT '{}',
  ADD COLUMN item_badges   TEXT[]   NOT NULL DEFAULT '{}',
  ADD COLUMN spice_level   SMALLINT NOT NULL DEFAULT 0
    CONSTRAINT menu_items_spice_level_range CHECK (spice_level BETWEEN 0 AND 3);
