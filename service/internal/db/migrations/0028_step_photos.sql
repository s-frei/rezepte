-- A photo of the recipe the author tied to a phrase of a step's text, like
-- step_times anchored by the phrase it stands on. The link follows the image,
-- not its position, so reordering photos keeps it; deleting the image drops
-- the link and leaves the step's text as written.
-- +goose Up
CREATE TABLE step_photos (
    step_id  TEXT NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    phrase   TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY (step_id, phrase)
);

-- Deleting an image looks its links up by image_id for the cascade.
CREATE INDEX step_photos_image_idx ON step_photos(image_id);

-- +goose Down
DROP INDEX step_photos_image_idx;
DROP TABLE step_photos;
