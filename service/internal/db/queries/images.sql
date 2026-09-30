-- name: InsertImage :one
INSERT INTO images (id, recipe_id, filename, width, height, size_bytes, position, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListImagesByRecipe :many
SELECT * FROM images WHERE recipe_id = ? ORDER BY position, id;

-- name: GetImage :one
SELECT * FROM images WHERE recipe_id = ? AND id = ?;

-- name: DeleteImage :execrows
DELETE FROM images WHERE recipe_id = ? AND id = ?;

-- name: CountImagesByRecipe :one
SELECT COUNT(*) FROM images WHERE recipe_id = ?;

-- name: NextImagePosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) FROM images WHERE recipe_id = ?;

-- name: UpdateImagePosition :exec
UPDATE images SET position = ? WHERE recipe_id = ? AND id = ?;

-- name: SetRecipeCover :exec
UPDATE recipes
SET cover_image_id = ?, updated_by = ?, updated_at = ?
WHERE id = ?;

-- name: TouchRecipe :exec
UPDATE recipes SET updated_by = ?, updated_at = ? WHERE id = ?;

-- How many photos each recipe of a page has and the bytes of all three
-- variants on disk, batched like ListTagNamesForRecipes. An export carries
-- only the largest, so the export page's size estimate runs high; covered
-- by images_recipe_idx.
-- name: ImageStatsForRecipes :many
SELECT recipe_id,
       CAST(COUNT(*) AS INTEGER) AS image_count,
       CAST(COALESCE(SUM(size_bytes), 0) AS INTEGER) AS image_bytes
FROM images
WHERE recipe_id IN (sqlc.slice(recipe_ids))
GROUP BY recipe_id;
