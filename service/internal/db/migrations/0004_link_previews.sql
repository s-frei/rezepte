-- +goose Up
-- Link previews of internal shares (docs/memory/content/features/link-previews.mdx):
-- whether a recipe link shared from the app may show the recipe's title,
-- description and cover to a crawler that is not signed in, for how many
-- minutes a link does, and the key its tokens are signed with.
ALTER TABLE instance_settings ADD COLUMN link_previews BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE instance_settings ADD COLUMN link_preview_minutes INTEGER NOT NULL DEFAULT 15
    CHECK (link_preview_minutes IN (15, 60, 1440));
-- SQLite refuses a non-constant default in ADD COLUMN, so the key is written
-- by the UPDATE below; the one row always exists.
ALTER TABLE instance_settings ADD COLUMN link_preview_key BLOB;
UPDATE instance_settings SET link_preview_key = randomblob(32);

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN link_preview_key;
ALTER TABLE instance_settings DROP COLUMN link_preview_minutes;
ALTER TABLE instance_settings DROP COLUMN link_previews;
