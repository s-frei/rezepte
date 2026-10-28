-- +goose Up
-- Notification preferences (docs/memory/content/features/email.mdx): every
-- switchable mail is on until its account opts out, so a row here is a kind
-- the account switched off.
CREATE TABLE notification_optouts (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    PRIMARY KEY (user_id, kind)
);

-- +goose Down
DROP TABLE notification_optouts;
