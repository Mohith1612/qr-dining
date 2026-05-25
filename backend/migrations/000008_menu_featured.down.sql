DROP INDEX IF EXISTS idx_menu_items_featured;
ALTER TABLE menu_items
  DROP COLUMN is_featured,
  DROP COLUMN featured_sort_order;
