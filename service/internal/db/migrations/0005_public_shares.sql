-- +goose Up
-- Public shares (docs/memory/content/features/public-shares.mdx): a member
-- turns a recipe into a read-only link anyone can open without an account,
-- until it is revoked or expires. At most one link per person and recipe;
-- revoking deletes the row outright, there is no soft-delete state.
CREATE TABLE shares (
    id         TEXT PRIMARY KEY,
    token      TEXT NOT NULL UNIQUE,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT,
    UNIQUE (recipe_id, created_by)
);
CREATE INDEX shares_created_by_idx ON shares(created_by);
-- Whether this person may create or keep a public link at all; an admin
-- withdraws it without touching the household-wide switch below.
ALTER TABLE users ADD COLUMN can_share_publicly BOOLEAN NOT NULL DEFAULT 1;
-- Whether the instance allows public links at all (owner-only, off by
-- default: nothing changes for an upgraded instance until the owner turns
-- it on), and the default and maximum lifetime new links get, in days.
-- NULL means permanent for both. SQLite's ALTER TABLE cannot add a
-- table-level constraint, so the cross-column rule - the default may never
-- exceed the maximum - has to be a column constraint, and a column
-- constraint can only name columns that already exist by the time it is
-- added. That is why it sits on public_share_max_days, the column added
-- last: ADD COLUMN checks it against every existing row as soon as the
-- column exists, including public_share_default_days added just before it.
ALTER TABLE instance_settings ADD COLUMN public_shares BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE instance_settings ADD COLUMN public_share_default_days INTEGER
    CHECK (public_share_default_days IS NULL OR public_share_default_days IN (1, 7, 30, 365));
ALTER TABLE instance_settings ADD COLUMN public_share_max_days INTEGER
    CHECK (public_share_max_days IS NULL OR (public_share_max_days IN (1, 7, 30, 365)
        AND public_share_default_days IS NOT NULL
        AND public_share_default_days <= public_share_max_days));

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN public_share_max_days;
ALTER TABLE instance_settings DROP COLUMN public_share_default_days;
ALTER TABLE instance_settings DROP COLUMN public_shares;
ALTER TABLE users DROP COLUMN can_share_publicly;
DROP TABLE shares;
