-- The daily digest of new recipes (docs/memory/content/features/email.mdx).
-- recipes.announce_pending marks a recipe created through the app, the API
-- or MCP that no digest has announced yet; archive import and the demo seed
-- leave it false. The digest goes out once a day at recipe_digest_hour in
-- the server's time zone; recipe_digest_last_day is the local date
-- (YYYY-MM-DD) of the last run, empty before the first, so a restart never
-- sends a second digest the same day.
-- +goose Up
ALTER TABLE recipes ADD COLUMN announce_pending BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX recipes_announce_pending_idx ON recipes(created_at) WHERE announce_pending;
ALTER TABLE instance_settings ADD COLUMN recipe_digest_hour INTEGER NOT NULL DEFAULT 17;
ALTER TABLE instance_settings ADD COLUMN recipe_digest_last_day TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN recipe_digest_last_day;
ALTER TABLE instance_settings DROP COLUMN recipe_digest_hour;
DROP INDEX recipes_announce_pending_idx;
ALTER TABLE recipes DROP COLUMN announce_pending;
