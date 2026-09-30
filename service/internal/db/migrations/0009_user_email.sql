-- email is profile data like display_name, never an identity: nothing signs
-- in or links an account by it, which is why it carries no UNIQUE. Empty
-- means none. email_verified is true only when an identity provider vouched
-- for the address (its email_verified claim); a typed address is unverified
-- until Rezepte can send a confirmation mail.
-- +goose Up
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN email_verified;
ALTER TABLE users DROP COLUMN email;
