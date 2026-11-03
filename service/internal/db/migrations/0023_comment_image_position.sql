-- +goose Up
-- A comment's photos keep the order the writer chose; existing ones keep their
-- insertion order.
ALTER TABLE comment_images ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
UPDATE comment_images SET position = (
    SELECT COUNT(*) FROM comment_images o
    WHERE o.comment_id = comment_images.comment_id AND o.rowid < comment_images.rowid
) WHERE comment_id IS NOT NULL;

-- +goose Down
ALTER TABLE comment_images DROP COLUMN position;
