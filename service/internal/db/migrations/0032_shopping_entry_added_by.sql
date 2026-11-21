-- shopping_list_entries.added_by is who put a recipe on a list (by hand or
-- from a plan), so a list member outside the plan a block came from reads
-- "Geplant von Mila" - a list member when she added it - never the plan's
-- name or its owner (docs/memory/content/features/shopping-lists.mdx). Rows
-- from before stay NULL, as do those of a deleted account.
-- +goose Up
ALTER TABLE shopping_list_entries ADD COLUMN added_by TEXT REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE shopping_list_entries DROP COLUMN added_by;
