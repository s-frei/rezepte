-- +goose Up
CREATE TABLE recipes (
    id             TEXT PRIMARY KEY,
    slug           TEXT NOT NULL UNIQUE,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    servings       INTEGER NOT NULL DEFAULT 4,
    prep_minutes   INTEGER,
    cook_minutes   INTEGER,
    source_url     TEXT,
    cover_image_id TEXT,
    created_by     TEXT NOT NULL REFERENCES users(id),
    created_at     TEXT NOT NULL,
    updated_by     TEXT NOT NULL REFERENCES users(id),
    updated_at     TEXT NOT NULL
);
CREATE INDEX recipes_updated_at_idx ON recipes(updated_at DESC);
CREATE INDEX recipes_created_at_idx ON recipes(created_at DESC);

CREATE TABLE ingredient_groups (
    id        TEXT PRIMARY KEY,
    recipe_id TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    name      TEXT,
    position  INTEGER NOT NULL
);
CREATE INDEX ingredient_groups_recipe_idx ON ingredient_groups(recipe_id, position);

CREATE TABLE ingredients (
    id       TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES ingredient_groups(id) ON DELETE CASCADE,
    quantity REAL,
    unit     TEXT,
    name     TEXT NOT NULL,
    note     TEXT,
    position INTEGER NOT NULL
);
CREATE INDEX ingredients_group_idx ON ingredients(group_id, position);

CREATE TABLE steps (
    id        TEXT PRIMARY KEY,
    recipe_id TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL,
    text      TEXT NOT NULL
);
CREATE INDEX steps_recipe_idx ON steps(recipe_id, position);

CREATE TABLE tags (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE recipe_tags (
    recipe_id TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id    TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);
CREATE INDEX recipe_tags_tag_idx ON recipe_tags(tag_id);

CREATE TABLE favourites (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (user_id, recipe_id)
);
-- The primary key already covers lookups by user; this one serves the
-- reverse direction, which the card lookup uses for a page of recipes.
CREATE INDEX favourites_recipe_idx ON favourites(recipe_id);

-- Images are uploaded in Phase 4; the table exists now so recipes.cover_image_id has a target.
CREATE TABLE images (
    id         TEXT PRIMARY KEY,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    width      INTEGER NOT NULL,
    height     INTEGER NOT NULL,
    size_bytes INTEGER NOT NULL,
    position   INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX images_recipe_idx ON images(recipe_id, position);

-- Contentless FTS5 index maintained by the recipe service (rowid = recipes.rowid).
CREATE VIRTUAL TABLE recipes_fts USING fts5(
    title, description, ingredients, tags,
    content='', contentless_delete=1,
    tokenize='unicode61 remove_diacritics 2'
);

-- +goose Down
DROP TABLE recipes_fts;
DROP TABLE images;
DROP TABLE favourites;
DROP TABLE recipe_tags;
DROP TABLE tags;
DROP TABLE steps;
DROP TABLE ingredients;
DROP TABLE ingredient_groups;
DROP TABLE recipes;
