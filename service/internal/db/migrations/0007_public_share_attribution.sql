-- +goose Up
-- Whether a public share page names Rezepte at its foot
-- (docs/memory/content/features/public-shares.mdx). On by default, also for
-- an upgraded instance: it only shows while public shares are on at all.
ALTER TABLE instance_settings ADD COLUMN public_share_attribution BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN public_share_attribution;
