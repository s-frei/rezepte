-- +goose Up
-- A recipe's source is a name, a link, or both (docs/memory/content/features/recipes.mdx):
-- the name is free text - a book with its page, a website, a person - that
-- source_url could not hold. NULL means absent, never ''.
ALTER TABLE recipes ADD COLUMN source_name TEXT;

-- +goose Down
ALTER TABLE recipes DROP COLUMN source_name;
