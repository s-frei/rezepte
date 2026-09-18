-- name: InsertRecipe :one
INSERT INTO recipes (id, slug, title, description, servings, prep_minutes, cook_minutes, source_url, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateRecipe :one
UPDATE recipes SET title = ?, description = ?, servings = ?, prep_minutes = ?, cook_minutes = ?, source_url = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteRecipe :execrows
DELETE FROM recipes WHERE id = ?;

-- name: GetRecipe :one
SELECT * FROM recipes WHERE id = ?;

-- name: GetRecipeBySlug :one
SELECT * FROM recipes WHERE slug = ?;

-- GetRecipeRowID, InsertFTS and DeleteFTS are hand-written in
-- internal/db/sqlc/fts.go: sqlc's SQLite engine only resolves the implicit
-- "rowid" pseudo-column through the FTS5 table-valued function call form
-- (e.g. "SELECT rowid FROM recipes_fts(?)", used below); it rejects "rowid"
-- as a plain column reference on any other table, real or virtual.

-- name: SlugExists :one
SELECT EXISTS(SELECT 1 FROM recipes WHERE slug = ?);

-- name: ListRecipes :many
-- The id tiebreak runs DESC, like updated_at: timestamps are RFC3339 with
-- second resolution, so everything written within the same second compares
-- equal, and ids are UUIDv7, ordered by creation time. An ASC tiebreak
-- would list such a burst oldest first, contradicting "newest first".
SELECT * FROM recipes ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: CountRecipes :one
SELECT COUNT(*) FROM recipes;

-- name: SearchRecipes :many
-- The id tiebreak runs DESC: see the comment on ListRecipes above.
-- LIMIT/OFFSET use sqlc.arg() rather than plain "?" here: mixed with the
-- explicitly numbered "?N" that sqlc.arg(query) becomes, plain "?" would
-- be auto-numbered by SQLite starting *after* the highest explicit number
-- (see https://www.sqlite.org/lang_expr.html#varparam), landing on indices
-- the Go driver never binds (it binds args 1..N positionally) and failing
-- at run time with "missing argument".
SELECT * FROM recipes
WHERE rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)))
ORDER BY updated_at DESC, id DESC LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountSearchRecipes :one
SELECT COUNT(*) FROM recipes WHERE rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)));

-- name: ListRecipesByTag :many
-- The id tiebreak runs DESC: see the comment on ListRecipes above.
SELECT r.* FROM recipes r
JOIN recipe_tags rt ON rt.recipe_id = r.id
JOIN tags t ON t.id = rt.tag_id
WHERE t.name = ?
ORDER BY r.updated_at DESC, r.id DESC LIMIT ? OFFSET ?;

-- name: CountRecipesByTag :one
SELECT COUNT(*) FROM recipes r
JOIN recipe_tags rt ON rt.recipe_id = r.id
JOIN tags t ON t.id = rt.tag_id
WHERE t.name = ?;

-- name: SearchRecipesByTag :many
-- The id tiebreak runs DESC: see the comment on ListRecipes above.
-- LIMIT/OFFSET use sqlc.arg(): see the comment on SearchRecipes above.
SELECT r.* FROM recipes r
JOIN recipe_tags rt ON rt.recipe_id = r.id
JOIN tags t ON t.id = rt.tag_id
WHERE t.name = sqlc.arg(tag) AND r.rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)))
ORDER BY r.updated_at DESC, r.id DESC LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountSearchRecipesByTag :one
SELECT COUNT(*) FROM recipes r
JOIN recipe_tags rt ON rt.recipe_id = r.id
JOIN tags t ON t.id = rt.tag_id
WHERE t.name = sqlc.arg(tag) AND r.rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)));

-- name: InsertIngredientGroup :exec
INSERT INTO ingredient_groups (id, recipe_id, name, position) VALUES (?, ?, ?, ?);

-- name: InsertIngredient :exec
INSERT INTO ingredients (id, group_id, quantity, unit, name, note, position) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertStep :exec
INSERT INTO steps (id, recipe_id, position, text) VALUES (?, ?, ?, ?);

-- name: DeleteIngredientGroupsByRecipe :exec
DELETE FROM ingredient_groups WHERE recipe_id = ?;

-- name: DeleteStepsByRecipe :exec
DELETE FROM steps WHERE recipe_id = ?;

-- name: ListIngredientGroupsByRecipe :many
SELECT * FROM ingredient_groups WHERE recipe_id = ? ORDER BY position;

-- name: ListIngredientsByRecipe :many
SELECT i.* FROM ingredients i
JOIN ingredient_groups g ON g.id = i.group_id
WHERE g.recipe_id = ? ORDER BY g.position, i.position;

-- name: ListStepsByRecipe :many
SELECT * FROM steps WHERE recipe_id = ? ORDER BY position;

-- name: ListTagNamesByRecipe :many
SELECT t.name FROM tags t JOIN recipe_tags rt ON rt.tag_id = t.id WHERE rt.recipe_id = ? ORDER BY t.name;

-- name: ListTagNamesForRecipes :many
SELECT rt.recipe_id, t.name FROM tags t JOIN recipe_tags rt ON rt.tag_id = t.id
WHERE rt.recipe_id IN (sqlc.slice(recipe_ids)) ORDER BY rt.recipe_id, t.name;
