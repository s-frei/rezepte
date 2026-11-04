-- Shopping lists (docs/memory/content/features/shopping-lists.mdx): a list
-- belongs to the person who made it and goes with their account; updated_by
-- names who changed it last and revision counts its changes, which the live
-- stream sends so a page that shows that revision already skips the refetch.
-- The creator invites people: a member row without accepted_at is a pending
-- invitation and grants nothing, an accepted one grants everything except
-- sharing and deleting the list. An entry is a recipe copied onto a list at a
-- serving count; its items hold the recipe's unscaled amounts and are scaled
-- when read, so changing the servings rescales them and keeps their ticks.
-- The copy is a snapshot: the recipe may be edited or deleted later
-- (recipe_id becomes NULL, title stays). A free item (entry_id NULL) holds an
-- absolute amount and who typed it; checked_by is who ticked an item, set and
-- cleared with checked_at. A department override is instance-wide and keyed
-- by the normalized item name. A person whose account is gone leaves NULL in
-- updated_by, added_by and checked_by.
-- +goose Up
CREATE TABLE shopping_lists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    revision   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX shopping_lists_creator_idx ON shopping_lists(created_by, updated_at DESC);

CREATE TABLE shopping_list_members (
    list_id     TEXT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_at  TEXT NOT NULL,
    accepted_at TEXT,
    PRIMARY KEY (list_id, user_id)
);
CREATE INDEX shopping_list_members_user_idx ON shopping_list_members(user_id);

CREATE TABLE shopping_list_entries (
    id            TEXT PRIMARY KEY,
    list_id       TEXT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    recipe_id     TEXT REFERENCES recipes(id) ON DELETE SET NULL,
    title         TEXT NOT NULL,
    base_servings INTEGER NOT NULL CHECK (base_servings > 0),
    servings      INTEGER NOT NULL CHECK (servings BETWEEN 1 AND 99),
    created_at    TEXT NOT NULL
);
CREATE INDEX shopping_list_entries_list_idx ON shopping_list_entries(list_id);

CREATE TABLE shopping_list_items (
    id         TEXT PRIMARY KEY,
    list_id    TEXT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    entry_id   TEXT REFERENCES shopping_list_entries(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    name_key   TEXT NOT NULL,
    quantity   REAL,
    unit       TEXT,
    note       TEXT,
    group_name TEXT,
    position   INTEGER NOT NULL,
    added_by   TEXT REFERENCES users(id) ON DELETE SET NULL,
    checked_at TEXT,
    checked_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX shopping_list_items_list_idx ON shopping_list_items(list_id);

CREATE TABLE shopping_department_overrides (
    name_key   TEXT PRIMARY KEY,
    department TEXT NOT NULL,
    updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE shopping_department_overrides;
DROP TABLE shopping_list_items;
DROP TABLE shopping_list_entries;
DROP TABLE shopping_list_members;
DROP TABLE shopping_lists;
