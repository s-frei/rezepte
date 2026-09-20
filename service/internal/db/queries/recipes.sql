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

-- name: ListRecipesFiltered :many
-- The id tiebreak runs DESC, like updated_at: timestamps are RFC3339 with
-- second resolution, so everything written within the same second compares
-- equal, and ids are UUIDv7, ordered by creation time. An ASC tiebreak
-- would list such a burst oldest first, contradicting "newest first".
--
-- Every parameter is sqlc.arg() on purpose - mixing plain "?" with the
-- explicitly numbered "?N" that sqlc.arg() becomes makes SQLite number the
-- plain ones after the highest explicit index, which the Go driver never
-- binds. See the note that used to sit on SearchRecipes.
--
-- tag_count = 0 switches the tag filter off; when it is set, the subquery
-- keeps only recipes carrying *all* of tag_names (AND, not OR). The HAVING
-- sits inside the subquery so the outer query stays one row per recipe. It
-- is wrapped in CAST(... AS INTEGER) even though it appears twice - once
-- against a literal 0, once against a HAVING COUNT(...) - for the same
-- type-inference reason as max_minutes below.
--
-- max_minutes = 0 switches the time filter off the same way. It is wrapped
-- in CAST(... AS INTEGER); the CAST isn't about correctness (SQLite's own
-- type affinity already applies), it's what gives sqlc enough to infer
-- int64: without it, its Go zero value would be nil rather than 0, and a
-- caller that leaves the field unset (meaning "off") would bind NULL
-- instead.
--
-- favourites_only = 0 switches the favourites filter off the same way,
-- CAST for the same reason as max_minutes. When it is set, only recipes
-- present in user_id's own favourites row match - user_id is never taken
-- from a path, query or body parameter, only from the authenticated
-- caller (see ADR 0017), so this condition can only ever narrow a caller's
-- own list to their own favourites.
--
-- sort never reaches SQL as an identifier, only as a value each CASE
-- compares against - sqlc cannot parameterise ORDER BY itself. An unknown
-- sort (including anything injection-shaped) matches neither of the first
-- two WHEN clauses, so sort_created and sort_title are both NULL and
-- sort_updated (the third) is what actually orders the rows, falling back
-- to the default order rather than erroring. r.id DESC is the same
-- burst-write tiebreak as the default order for every sort, including
-- title: ids are UUIDv7, so it still reads newest-first among equal titles.
--
-- The CASEs live in a CTE's SELECT list rather than directly in ORDER BY:
-- sqlc's SQLite engine (confirmed against v1.31.1) does not rewrite
-- sqlc.arg() into a bind parameter when it appears solely inside an ORDER
-- BY expression - the call is left as literal, un-substituted SQL text,
-- which SQLite then rejects at run time ("near '(': syntax error"), and no
-- Sort field is added to the params struct at all. Computing the sort keys
-- as columns of "ordered" and only then ordering by their aliases keeps
-- sqlc.arg(sort) inside a SELECT list, where its parameter extraction does
-- work; CAST(... AS TEXT), same as the max_minutes CAST above, is what
-- gives it a concrete string type instead of interface{}. Because
-- "ordered" carries three extra sort_* columns beyond the recipes columns,
-- the outer SELECT must list recipes' columns explicitly rather than
-- "SELECT *" (which would otherwise leak them into the Recipe struct).
WITH ordered AS (
  SELECT r.*,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) = 'created' THEN r.created_at END AS sort_created,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) = 'title' THEN LOWER(r.title) END AS sort_title,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) != 'created'
          AND CAST(sqlc.arg(sort) AS TEXT) != 'title' THEN r.updated_at END AS sort_updated
  FROM recipes r
  WHERE (CAST(sqlc.arg(tag_count) AS INTEGER) = 0 OR r.id IN (
          SELECT rt.recipe_id FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
          WHERE t.name IN (SELECT value FROM json_each(sqlc.arg(tag_names)))
          GROUP BY rt.recipe_id HAVING COUNT(DISTINCT t.name) = CAST(sqlc.arg(tag_count) AS INTEGER)))
    AND (CAST(sqlc.arg(max_minutes) AS INTEGER) = 0
         OR (COALESCE(r.prep_minutes, 0) + COALESCE(r.cook_minutes, 0)
               BETWEEN 1 AND CAST(sqlc.arg(max_minutes) AS INTEGER)))
    AND (CAST(sqlc.arg(favourites_only) AS INTEGER) = 0 OR r.id IN (
          SELECT recipe_id FROM favourites WHERE user_id = sqlc.arg(user_id)))
)
SELECT id, slug, title, description, servings, prep_minutes, cook_minutes,
       source_url, cover_image_id, created_by, created_at, updated_at
FROM ordered
ORDER BY sort_created DESC, sort_title ASC, sort_updated DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountRecipesFiltered :one
SELECT COUNT(*) FROM recipes r
WHERE (CAST(sqlc.arg(tag_count) AS INTEGER) = 0 OR r.id IN (
        SELECT rt.recipe_id FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
        WHERE t.name IN (SELECT value FROM json_each(sqlc.arg(tag_names)))
        GROUP BY rt.recipe_id HAVING COUNT(DISTINCT t.name) = CAST(sqlc.arg(tag_count) AS INTEGER)))
  AND (CAST(sqlc.arg(max_minutes) AS INTEGER) = 0
       OR (COALESCE(r.prep_minutes, 0) + COALESCE(r.cook_minutes, 0)
             BETWEEN 1 AND CAST(sqlc.arg(max_minutes) AS INTEGER)))
  AND (CAST(sqlc.arg(favourites_only) AS INTEGER) = 0 OR r.id IN (
        SELECT recipe_id FROM favourites WHERE user_id = sqlc.arg(user_id)));

-- name: SearchRecipesFiltered :many
-- The full-text half of ListRecipesFiltered. It is a separate query rather
-- than a switchable condition because the match cannot be turned off from
-- inside: recipes_fts(NULL) and recipes_fts('') are both errors, and SQLite
-- does not promise to skip the subquery of an OR whose left side is true.
--
-- sort works the same way as on ListRecipesFiltered - see the note there
-- for why the ordering CASEs live in a CTE's SELECT list rather than
-- directly in ORDER BY.
WITH ordered AS (
  SELECT r.*,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) = 'created' THEN r.created_at END AS sort_created,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) = 'title' THEN LOWER(r.title) END AS sort_title,
    CASE WHEN CAST(sqlc.arg(sort) AS TEXT) != 'created'
          AND CAST(sqlc.arg(sort) AS TEXT) != 'title' THEN r.updated_at END AS sort_updated
  FROM recipes r
  WHERE r.rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)))
    AND (CAST(sqlc.arg(tag_count) AS INTEGER) = 0 OR r.id IN (
          SELECT rt.recipe_id FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
          WHERE t.name IN (SELECT value FROM json_each(sqlc.arg(tag_names)))
          GROUP BY rt.recipe_id HAVING COUNT(DISTINCT t.name) = CAST(sqlc.arg(tag_count) AS INTEGER)))
    AND (CAST(sqlc.arg(max_minutes) AS INTEGER) = 0
         OR (COALESCE(r.prep_minutes, 0) + COALESCE(r.cook_minutes, 0)
               BETWEEN 1 AND CAST(sqlc.arg(max_minutes) AS INTEGER)))
    AND (CAST(sqlc.arg(favourites_only) AS INTEGER) = 0 OR r.id IN (
          SELECT recipe_id FROM favourites WHERE user_id = sqlc.arg(user_id)))
)
SELECT id, slug, title, description, servings, prep_minutes, cook_minutes,
       source_url, cover_image_id, created_by, created_at, updated_at
FROM ordered
ORDER BY sort_created DESC, sort_title ASC, sort_updated DESC, id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountSearchRecipesFiltered :one
SELECT COUNT(*) FROM recipes r
WHERE r.rowid IN (SELECT rowid FROM recipes_fts(sqlc.arg(query)))
  AND (CAST(sqlc.arg(tag_count) AS INTEGER) = 0 OR r.id IN (
        SELECT rt.recipe_id FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
        WHERE t.name IN (SELECT value FROM json_each(sqlc.arg(tag_names)))
        GROUP BY rt.recipe_id HAVING COUNT(DISTINCT t.name) = CAST(sqlc.arg(tag_count) AS INTEGER)))
  AND (CAST(sqlc.arg(max_minutes) AS INTEGER) = 0
       OR (COALESCE(r.prep_minutes, 0) + COALESCE(r.cook_minutes, 0)
             BETWEEN 1 AND CAST(sqlc.arg(max_minutes) AS INTEGER)))
  AND (CAST(sqlc.arg(favourites_only) AS INTEGER) = 0 OR r.id IN (
        SELECT recipe_id FROM favourites WHERE user_id = sqlc.arg(user_id)));

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

-- name: SetFavourite :exec
INSERT OR IGNORE INTO favourites (user_id, recipe_id, created_at) VALUES (?, ?, ?);

-- name: DeleteFavourite :exec
DELETE FROM favourites WHERE user_id = ? AND recipe_id = ?;

-- name: ListFavouriteRecipeIDs :many
-- One query per page rather than one per card: toCards already batches the
-- tag lookup the same way. The recipe ids travel as a JSON array matched
-- with json_each(), not sqlc.slice() - see the note on ListRecipesFiltered
-- for why sqlc.slice() cannot be combined with sqlc.arg() in the same query.
SELECT recipe_id FROM favourites
WHERE user_id = sqlc.arg(user_id)
  AND recipe_id IN (SELECT value FROM json_each(sqlc.arg(recipe_ids)));
