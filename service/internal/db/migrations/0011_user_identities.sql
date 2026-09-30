-- An identity at an OIDC provider, linked to one account by a person who was
-- signed in to it at the time. (issuer, subject) is the only thing an OIDC
-- login is matched by; email never is. issuer is kept so that pointing the
-- instance at another provider cannot match old subjects by accident.
-- +goose Up
CREATE TABLE user_identities (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer     TEXT NOT NULL,
    subject    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (issuer, subject)
);
CREATE UNIQUE INDEX user_identities_user_idx ON user_identities(user_id, issuer);

-- +goose Down
DROP TABLE user_identities;
