-- Photos on recipe comments (docs/memory/content/features/recipe-comments.mdx). They
-- are never rows in images: the recipe photo cap, cover, export and public
-- page all read images by recipe. comment_id is NULL while a photo is staged
-- (uploaded, not yet sent with a comment). uploader_id is SET NULL so a
-- former member's comments keep their photos.
-- +goose Up
CREATE TABLE comment_images (
    id              TEXT PRIMARY KEY,
    recipe_id       TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    comment_id      INTEGER REFERENCES recipe_comments(id) ON DELETE CASCADE,
    uploader_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    recipe_image_id TEXT REFERENCES images(id) ON DELETE SET NULL,
    width           INTEGER NOT NULL,
    height          INTEGER NOT NULL,
    size_bytes      INTEGER NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX comment_images_comment_idx ON comment_images(comment_id);
CREATE INDEX comment_images_staged_idx ON comment_images(created_at) WHERE comment_id IS NULL;
CREATE INDEX comment_images_recipe_idx ON comment_images(recipe_id);
CREATE INDEX comment_images_recipe_image_idx ON comment_images(recipe_image_id) WHERE recipe_image_id IS NOT NULL;

-- +goose Down
DROP INDEX comment_images_recipe_image_idx;
DROP INDEX comment_images_recipe_idx;
DROP INDEX comment_images_staged_idx;
DROP INDEX comment_images_comment_idx;
DROP TABLE comment_images;
