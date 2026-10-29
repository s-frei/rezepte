-- name: InsertCommentImage :exec
-- A new photo is staged: comment_id stays NULL until the comment is sent.
INSERT INTO comment_images (id, recipe_id, uploader_id, width, height, size_bytes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CountStagedCommentImages :one
SELECT COUNT(*) FROM comment_images WHERE uploader_id = ? AND comment_id IS NULL;

-- name: DeleteOldestStagedCommentImage :one
-- Makes room for a new staged photo; the caller removes the files.
DELETE FROM comment_images
WHERE id = (
    SELECT s.id FROM comment_images s
    WHERE s.uploader_id = sqlc.arg(uploader_id) AND s.comment_id IS NULL
    ORDER BY s.created_at, s.rowid LIMIT 1
)
RETURNING recipe_id, id;

-- name: AttachCommentImages :execrows
-- Only the caller's own photos of this recipe, staged or already on this
-- comment, attach; callers compare the row count with the ids they sent.
UPDATE comment_images
SET comment_id = sqlc.arg(comment_id)
WHERE id IN (SELECT value FROM json_each(sqlc.arg(ids)))
  AND recipe_id = sqlc.arg(recipe_id)
  AND uploader_id = sqlc.arg(uploader_id)
  AND (comment_id IS NULL OR comment_id = sqlc.arg(comment_id));

-- name: ListCommentImageIDs :many
SELECT id FROM comment_images WHERE comment_id = ? ORDER BY rowid;

-- name: DeleteCommentImages :many
-- Drops the photos of a comment that are not in keep; the caller removes the files.
DELETE FROM comment_images
WHERE comment_id = sqlc.arg(comment_id)
  AND id NOT IN (SELECT value FROM json_each(sqlc.arg(keep)))
RETURNING id;

-- name: ListCommentImagesForRecipe :many
-- Insertion order; staged photos are not part of any comment yet.
SELECT * FROM comment_images WHERE recipe_id = ? AND comment_id IS NOT NULL ORDER BY rowid;

-- name: ListCommentImagesForComment :many
SELECT * FROM comment_images WHERE comment_id = ? ORDER BY rowid;

-- name: GetAttachedCommentImage :one
SELECT * FROM comment_images WHERE id = ? AND recipe_id = ? AND comment_id IS NOT NULL;

-- name: SweepStagedCommentImages :many
-- Staged photos that were never sent and are older than the cutoff, or whose uploader is gone (any age).
DELETE FROM comment_images
WHERE comment_id IS NULL AND (uploader_id IS NULL OR created_at < ?)
RETURNING recipe_id, id;

-- name: SetCommentImageRecipeImage :exec
UPDATE comment_images SET recipe_image_id = ? WHERE id = ?;
