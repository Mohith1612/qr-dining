ALTER TABLE menu_items
  ADD COLUMN is_featured         BOOLEAN  NOT NULL DEFAULT FALSE,
  ADD COLUMN featured_sort_order SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX idx_menu_items_featured ON menu_items(branch_id, is_featured)
  WHERE is_featured = TRUE;
