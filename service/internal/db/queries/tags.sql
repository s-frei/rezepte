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

-- name: ListTagsWithCount :many
SELECT t.name, COUNT(rt.recipe_id) AS count FROM tags t
JOIN recipe_tags rt ON rt.tag_id = t.id
GROUP BY t.id ORDER BY count DESC, t.name;
