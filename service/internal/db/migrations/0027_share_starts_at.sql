-- +goose Up
-- When a public share's current lifetime began: its creation, or the last time
-- its lifetime was changed (docs/memory/content/features/public-shares.mdx).
-- The instance maximum counts from here, not from created_at. Existing rows
-- start at their creation.
ALTER TABLE shares ADD COLUMN starts_at TEXT NOT NULL DEFAULT '';
UPDATE shares SET starts_at = created_at;

-- +goose Down
ALTER TABLE shares DROP COLUMN starts_at;
