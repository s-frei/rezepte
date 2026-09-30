-- +goose Up
-- The account's picture (docs/memory/content/features/users-and-auth.mdx):
-- the id of the file under <data>/avatars/<user id>/, fresh for every upload
-- so the URL changes with the picture. NULL is no picture.
ALTER TABLE users ADD COLUMN avatar_id TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_id;
