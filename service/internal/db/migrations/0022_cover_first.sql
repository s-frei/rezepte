-- +goose Up
-- The cover is always the first photo: move each recipe's cover to position 0
-- and shift the photos that were ahead of it down by one.
UPDATE images
SET position = CASE
    WHEN id = (SELECT r.cover_image_id FROM recipes r WHERE r.id = images.recipe_id) THEN 0
    ELSE position + 1
END
WHERE position <= (
    SELECT c.position FROM recipes r JOIN images c ON c.id = r.cover_image_id
    WHERE r.id = images.recipe_id
);

-- Older data may hold photos but no cover: the first photo becomes the cover.
UPDATE recipes SET cover_image_id = (
    SELECT i.id FROM images i WHERE i.recipe_id = recipes.id ORDER BY i.position, i.rowid LIMIT 1
)
WHERE cover_image_id IS NULL AND EXISTS (SELECT 1 FROM images i WHERE i.recipe_id = recipes.id);

-- +goose Down
SELECT 1;
