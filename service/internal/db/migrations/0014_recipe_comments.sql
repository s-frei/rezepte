-- +goose Up
-- The kitchen diary: a flat conversation per recipe. The id is an
-- AUTOINCREMENT integer, unlike every other table, because it doubles as the
-- watermark recipe_comment_reads compares against: without AUTOINCREMENT,
-- SQLite hands a deleted newest row's id to the next insert, which would
-- then count as already seen. author_id is SET NULL rather than CASCADE: an
-- entry is a statement of a person and outlives their account as "former
-- member"; recipes are reassigned to an admin instead, entries are not.
CREATE TABLE recipe_comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    author_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    edited_at  TEXT
);
CREATE INDEX recipe_comments_recipe_idx ON recipe_comments(recipe_id, id);
CREATE INDEX recipe_comments_author_idx ON recipe_comments(author_id);

-- One watermark per person and recipe: the highest comment id they have
-- seen on the recipe page. "New" is id > last_seen. Timestamps are not used
-- because they have second precision.
CREATE TABLE recipe_comment_reads (
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    last_seen INTEGER NOT NULL,
    PRIMARY KEY (user_id, recipe_id)
);

-- +goose Down
DROP TABLE recipe_comment_reads;
DROP TABLE recipe_comments;
