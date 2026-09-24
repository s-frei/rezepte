-- name: InsertRecipe :one
INSERT INTO recipes (id, slug, title, description, servings, prep_minutes, cook_minutes, source_url, created_by, created_at, updated_by, updated_at, edit_policy)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateRecipe :one
UPDATE recipes SET title = ?, description = ?, servings = ?, prep_minutes = ?, cook_minutes = ?, source_url = ?, updated_by = ?, updated_at = ?, edit_policy = ?
WHERE id = ?
RETURNING *;

-- name: DeleteRecipe :execrows
DELETE FROM recipes WHERE id = ?;

-- name: GetRecipe :one
SELECT * FROM recipes WHERE id = ?;

-- name: GetRecipeBySlug :one
SELECT * FROM recipes WHERE slug = ?;

-- Hand-written beside the generated code in internal/db/sqlc: GetRecipeRowID,
-- InsertFTS and DeleteFTS (fts.go), because sqlc rejects the "rowid"
-- pseudo-column, and ListRecipes and CountRecipes (recipe_list.go), because
-- their conditions and order depend on the request.

-- name: SlugExists :one
SELECT EXISTS(SELECT 1 FROM recipes WHERE slug = ?);

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

-- name: InsertStepReference :exec
INSERT INTO step_references (step_id, ingredient_id, word, position) VALUES (?, ?, ?, ?);

-- name: ListStepReferencesByRecipe :many
SELECT sr.step_id, sr.word, g.name AS group_name, i.name AS ingredient_name
FROM step_references sr
JOIN steps s ON s.id = sr.step_id
JOIN ingredients i ON i.id = sr.ingredient_id
JOIN ingredient_groups g ON g.id = i.group_id
WHERE s.recipe_id = ?
ORDER BY s.position, sr.position;

-- name: ListTagNamesByRecipe :many
SELECT t.name FROM tags t JOIN recipe_tags rt ON rt.tag_id = t.id WHERE rt.recipe_id = ? ORDER BY t.name;

-- name: ListTagNamesForRecipes :many
SELECT rt.recipe_id, t.name FROM tags t JOIN recipe_tags rt ON rt.tag_id = t.id
WHERE rt.recipe_id IN (sqlc.slice(recipe_ids)) ORDER BY rt.recipe_id, t.name;

-- The people behind a page of recipes, batched like ListTagNamesForRecipes.
-- name: ListAuthorsForIDs :many
SELECT id, username, display_name, color FROM users WHERE id IN (sqlc.slice(ids));

-- Everyone who has written at least one recipe, most recipes first, for the
-- "Angelegt von" filter. Counting here rather than in Go keeps the list and
-- its counts one statement, the way ListTagsWithCount does.
-- name: ListAuthorsWithCount :many
SELECT u.username, u.display_name, u.color, COUNT(r.id) AS recipe_count
FROM users u JOIN recipes r ON r.created_by = u.id
GROUP BY u.id, u.username, u.display_name, u.color
ORDER BY recipe_count DESC, u.username;

-- name: SetFavorite :exec
INSERT OR IGNORE INTO favorites (user_id, recipe_id, created_at) VALUES (?, ?, ?);

-- name: DeleteFavorite :exec
DELETE FROM favorites WHERE user_id = ? AND recipe_id = ?;

-- name: ListFavoriteRecipeIDs :many
-- One query per page rather than one per card: toCards already batches the
-- tag lookup the same way. The recipe ids travel as a JSON array matched
-- with json_each(), not sqlc.slice(): sqlc.slice() cannot be combined with
-- sqlc.arg() in the same query. See
-- docs/memory/content/features/recipes.mdx.
SELECT recipe_id FROM favorites
WHERE user_id = sqlc.arg(user_id)
  AND recipe_id IN (SELECT value FROM json_each(sqlc.arg(recipe_ids)));

-- The people behind the detail view. They come from a query of their own
-- rather than a join in GetRecipe because that row is also what the image
-- service reads to check a recipe exists, and it has no use for people.
-- name: GetRecipeAuthors :one
SELECT c.username AS created_by_username, c.display_name AS created_by_display_name,
       c.color AS created_by_color,
       u.username AS updated_by_username, u.display_name AS updated_by_display_name,
       u.color AS updated_by_color
FROM recipes r
JOIN users c ON c.id = r.created_by
JOIN users u ON u.id = r.updated_by
WHERE r.id = ?;
