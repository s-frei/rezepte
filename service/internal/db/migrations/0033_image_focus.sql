-- images.focus_x and focus_y are the point of a photo that stays in view
-- wherever the app crops it, as fractions of the upright image (0 = left or
-- top, 1 = right or bottom). Existing rows read as centered, which is how
-- every photo was cropped before (docs/memory/content/features/images.mdx).
-- +goose Up
ALTER TABLE images ADD COLUMN focus_x REAL NOT NULL DEFAULT 0.5 CHECK (focus_x BETWEEN 0 AND 1);
ALTER TABLE images ADD COLUMN focus_y REAL NOT NULL DEFAULT 0.5 CHECK (focus_y BETWEEN 0 AND 1);

-- +goose Down
ALTER TABLE images DROP COLUMN focus_y;
ALTER TABLE images DROP COLUMN focus_x;
