-- Shopping lists (docs/memory/content/features/shopping-lists.mdx): a list
-- belongs to the person who made it and goes with their account. An entry is
-- a recipe copied onto a list at a serving count; its items hold the recipe's
-- unscaled amounts and are scaled when read, so changing the servings
-- rescales them and keeps their ticks. The copy is a snapshot: the recipe
-- may be edited or deleted later (recipe_id becomes NULL, title stays). A
-- free item (entry_id NULL) holds an absolute amount. A department override
-- is instance-wide and keyed by the normalized item name.
-- +goose Up
CREATE TABLE shopping_lists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX shopping_lists_creator_idx ON shopping_lists(created_by, updated_at DESC);

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
DROP TABLE shopping_lists;
