-- A setup link's purpose: "setup" lets a person set a password or connect an
-- identity provider; "reset" is a forgotten-password link the person asked for
-- themselves, valid for an hour and never usable to connect a provider.
-- email_confirmations proves a typed address: only the SHA-256 of the token is
-- stored, like setup_links.id, one open confirmation per account, and
-- confirming verifies the address only while it is still the one mailed to.
-- A reset link went to the address an account had when it was asked for, for
-- the password it had then: changing either closes it, whichever code path
-- writes the row. The same address in other letters is no change. An admin's
-- setup link stays.
-- +goose Up
ALTER TABLE setup_links ADD COLUMN purpose TEXT NOT NULL DEFAULT 'setup' CHECK (purpose IN ('setup', 'reset'));
CREATE TABLE email_confirmations (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address    TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX email_confirmations_user_idx ON email_confirmations(user_id);
-- +goose StatementBegin
CREATE TRIGGER users_change_closes_reset_link AFTER UPDATE OF email, password_hash ON users
WHEN lower(OLD.email) IS NOT lower(NEW.email) OR OLD.password_hash IS NOT NEW.password_hash
BEGIN
    DELETE FROM setup_links WHERE user_id = NEW.id AND purpose = 'reset';
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER users_change_closes_reset_link;
DROP TABLE email_confirmations;
DELETE FROM setup_links WHERE purpose = 'reset';
ALTER TABLE setup_links DROP COLUMN purpose;
