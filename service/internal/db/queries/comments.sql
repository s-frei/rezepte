-- name: InsertComment :one
INSERT INTO recipe_comments (recipe_id, author_id, body, created_at)
VALUES (?, ?, ?, ?)
RETURNING id;

-- name: GetComment :one
SELECT id, recipe_id, author_id, body, created_at, edited_at
FROM recipe_comments
WHERE id = ?;

-- name: ListComments :many
-- Oldest first. Authors are loaded with ListAuthorsForIDs, not joined here:
-- a LEFT JOIN on a removed author would need nullable user columns.
SELECT id, recipe_id, author_id, body, created_at, edited_at
FROM recipe_comments
WHERE recipe_id = ?
ORDER BY id;

-- name: UpdateCommentBody :exec
UPDATE recipe_comments SET body = ?, edited_at = ? WHERE id = ?;

-- name: DeleteComment :exec
DELETE FROM recipe_comments WHERE id = ?;

-- name: GetCommentWatermark :one
SELECT last_seen FROM recipe_comment_reads WHERE user_id = ? AND recipe_id = ?;

-- name: RaiseCommentWatermark :exec
-- Never lowers: an older page view arriving late must not resurrect dots.
INSERT INTO recipe_comment_reads (user_id, recipe_id, last_seen)
VALUES (sqlc.arg(user_id), sqlc.arg(recipe_id), sqlc.arg(last_seen))
ON CONFLICT (user_id, recipe_id) DO UPDATE SET last_seen = MAX(last_seen, excluded.last_seen);

-- name: MaxCommentID :one
SELECT CAST(COALESCE(MAX(id), 0) AS INTEGER) AS max_id
FROM recipe_comments
WHERE recipe_id = ?;

-- name: ListRecipesWithNewComments :many
-- Which recipes of a page show the dot for user_id: they wrote the recipe
-- or an entry on it, and somebody else (or a former member) wrote an entry
-- above their watermark. json_each for the same reason as
-- ListFavoriteRecipeIDs.
SELECT r.id
FROM recipes r
WHERE r.id IN (SELECT value FROM json_each(sqlc.arg(recipe_ids)))
  AND (r.created_by = sqlc.arg(user_id)
       OR EXISTS (SELECT 1 FROM recipe_comments mine
                  WHERE mine.recipe_id = r.id AND mine.author_id = sqlc.arg(user_id)))
  AND EXISTS (SELECT 1 FROM recipe_comments c
              WHERE c.recipe_id = r.id
                AND (c.author_id IS NULL OR c.author_id <> sqlc.arg(user_id))
                AND c.id > COALESCE((SELECT w.last_seen FROM recipe_comment_reads w
                                     WHERE w.user_id = sqlc.arg(user_id) AND w.recipe_id = r.id), 0));
