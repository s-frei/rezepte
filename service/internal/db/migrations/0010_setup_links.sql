-- A setup link lets one person set their own password or connect an
-- identity provider, so an admin never has to know or hand over a password.
-- Only the SHA-256 of the token is stored, like sessions.id. One open link per
-- account: issuing another replaces it.
-- +goose Up
CREATE TABLE setup_links (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX setup_links_user_idx ON setup_links(user_id);

-- +goose Down
DROP TABLE setup_links;
