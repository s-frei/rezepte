-- Meal plans (docs/memory/content/features/meal-plan.mdx): a plan belongs to
-- the person who made it and goes with their account, and is shared like a
-- shopping list: a member row without accepted_at is a pending invitation and
-- grants nothing, an accepted one grants everything except sharing and
-- deleting. Everyone has one personal plan (personal = 1, name NULL, the
-- client words it), which is never shared, renamed or deleted;
-- meal_plans_personal_idx keeps it to one per person. revision counts a
-- plan's changes for the live stream, as on lists.
-- An entry is a recipe or a free text on a day (YYYY-MM-DD) or on no day
-- (day NULL, the tray), ordered by position within its day. A recipe entry
-- copies the title and starts at the recipe's servings; a deleted recipe
-- leaves the title (recipe_id NULL). removed_at is a soft delete so Undo
-- brings an entry back with its list blocks; old removals are purged.
-- Staples are names a plan always has at home, keyed by shopping.NameKey.
-- calendar_feeds holds one feed secret per person as a SHA-256 digest.
-- shopping_list_entries.plan_entry_id links a list entry to the plan entry
-- it came from; list reads hide entries whose plan entry is removed.
-- +goose Up
CREATE TABLE meal_plans (
    id         TEXT PRIMARY KEY,
    name       TEXT,
    personal   INTEGER NOT NULL DEFAULT 0 CHECK (personal IN (0, 1)),
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    revision   INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX meal_plans_personal_idx ON meal_plans(created_by) WHERE personal = 1;
CREATE INDEX meal_plans_creator_idx ON meal_plans(created_by, updated_at DESC);

CREATE TABLE meal_plan_members (
    plan_id     TEXT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_at  TEXT NOT NULL,
    accepted_at TEXT,
    PRIMARY KEY (plan_id, user_id)
);
CREATE INDEX meal_plan_members_user_idx ON meal_plan_members(user_id);

CREATE TABLE meal_plan_entries (
    id         TEXT PRIMARY KEY,
    plan_id    TEXT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    day        TEXT,
    position   INTEGER NOT NULL,
    recipe_id  TEXT REFERENCES recipes(id) ON DELETE SET NULL,
    title      TEXT NOT NULL,
    servings   INTEGER CHECK (servings IS NULL OR servings BETWEEN 1 AND 99),
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    removed_at TEXT
);
CREATE INDEX meal_plan_entries_day_idx ON meal_plan_entries(plan_id, day);

CREATE TABLE meal_plan_staples (
    plan_id    TEXT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    name_key   TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (plan_id, name_key)
);

CREATE TABLE calendar_feeds (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);

ALTER TABLE shopping_list_entries ADD COLUMN plan_entry_id TEXT REFERENCES meal_plan_entries(id) ON DELETE CASCADE;
CREATE INDEX shopping_list_entries_plan_entry_idx ON shopping_list_entries(plan_entry_id);

-- +goose Down
DROP INDEX shopping_list_entries_plan_entry_idx;
ALTER TABLE shopping_list_entries DROP COLUMN plan_entry_id;
DROP TABLE calendar_feeds;
DROP TABLE meal_plan_staples;
DROP TABLE meal_plan_entries;
DROP TABLE meal_plan_members;
DROP TABLE meal_plans;
