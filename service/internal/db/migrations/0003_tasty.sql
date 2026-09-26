-- +goose Up
-- A tasty mark is one member telling a recipe's author it was good. It has
-- the shape of favorites - one row per (user, recipe), gone with either -
-- but unlike a favorite it is shown to everyone, as a count and as names.
CREATE TABLE tasty (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (user_id, recipe_id)
);
-- The primary key serves lookups by user; this one serves the counts per
-- recipe that every card and the tasty sort read.
CREATE INDEX tasty_recipe_idx ON tasty(recipe_id);

-- +goose Down
DROP TABLE tasty;
