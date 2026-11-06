-- +goose Up
-- Whether a public share page names the member who shared it
-- (docs/memory/content/features/public-shares.mdx). Off by default, also for
-- an upgraded instance: a stranger learns a member's name only once the
-- owner chose so.
ALTER TABLE instance_settings ADD COLUMN public_share_sharer BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN public_share_sharer;
