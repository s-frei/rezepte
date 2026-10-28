-- name: UpsertTag :one
INSERT INTO tags (id, name) VALUES (?, ?)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING id;

-- name: SetRecipeTag :exec
INSERT OR IGNORE INTO recipe_tags (recipe_id, tag_id) VALUES (?, ?);

-- name: DeleteRecipeTags :exec
DELETE FROM recipe_tags WHERE recipe_id = ?;

-- name: DeleteOrphanTags :exec
DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM recipe_tags);

