-- A duration the author marked in a step's text, like step_references anchored
-- by the phrase it stands on. seconds is the lower end, max_seconds the upper
-- end of a range.
-- +goose Up
CREATE TABLE step_times (
    step_id     TEXT NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    phrase      TEXT NOT NULL,
    seconds     INTEGER NOT NULL,
    max_seconds INTEGER,
    position    INTEGER NOT NULL,
    PRIMARY KEY (step_id, phrase)
);

-- +goose Down
DROP TABLE step_times;
