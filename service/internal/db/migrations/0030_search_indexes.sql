-- The search indexes hold folded text (lower-cased, "ä"/"ae" and "ß"/"ss"
-- as one; fold is the Go function db.Fold, registered with the driver), so
-- either spelling finds the other. recipes_fts is rebuilt that way, and
-- recipes_trigram is new: a second index over the same four columns,
-- tokenized into trigrams, so a search finds a word inside another - "brot"
-- in "Bauernbrot". Both are contentless, keyed by recipes.rowid and written
-- by the recipe service; this migration fills them once from the rows
-- already there.
--
-- The ingredient names come in no set order, unlike the service's: a search
-- word never holds a space, so it cannot match across two names, and the
-- order changes nothing. (sqlc cannot parse ORDER BY inside group_concat.)
-- +goose Up
CREATE VIRTUAL TABLE recipes_trigram USING fts5(
    title, description, ingredients, tags,
    content='', contentless_delete=1,
    tokenize='trigram remove_diacritics 1'
);

INSERT INTO recipes_fts (recipes_fts) VALUES ('delete-all');

INSERT INTO recipes_fts (rowid, title, description, ingredients, tags)
SELECT r.rowid, fold(r.title), fold(r.description),
       fold(COALESCE((SELECT group_concat(i.name, ' ')
                      FROM ingredient_groups g
                      JOIN ingredients i ON i.group_id = g.id
                      WHERE g.recipe_id = r.id), '')),
       fold(COALESCE((SELECT group_concat(t.name, ' ')
                      FROM recipe_tags rt
                      JOIN tags t ON t.id = rt.tag_id
                      WHERE rt.recipe_id = r.id), ''))
FROM recipes r;

INSERT INTO recipes_trigram (rowid, title, description, ingredients, tags)
SELECT r.rowid, fold(r.title), fold(r.description),
       fold(COALESCE((SELECT group_concat(i.name, ' ')
                      FROM ingredient_groups g
                      JOIN ingredients i ON i.group_id = g.id
                      WHERE g.recipe_id = r.id), '')),
       fold(COALESCE((SELECT group_concat(t.name, ' ')
                      FROM recipe_tags rt
                      JOIN tags t ON t.id = rt.tag_id
                      WHERE rt.recipe_id = r.id), ''))
FROM recipes r;

-- +goose Down
DROP TABLE recipes_trigram;

INSERT INTO recipes_fts (recipes_fts) VALUES ('delete-all');

INSERT INTO recipes_fts (rowid, title, description, ingredients, tags)
SELECT r.rowid, r.title, r.description,
       COALESCE((SELECT group_concat(i.name, ' ')
                 FROM ingredient_groups g
                 JOIN ingredients i ON i.group_id = g.id
                 WHERE g.recipe_id = r.id), ''),
       COALESCE((SELECT group_concat(t.name, ' ')
                 FROM recipe_tags rt
                 JOIN tags t ON t.id = rt.tag_id
                 WHERE rt.recipe_id = r.id), '')
FROM recipes r;
