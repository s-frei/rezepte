-- +goose Up
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('superadmin', 'admin', 'user')),
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

-- At most one owner per instance: a partial unique index makes that a property
-- of the database rather than of the code that writes to it.
CREATE UNIQUE INDEX users_superadmin_idx ON users(role) WHERE role = 'superadmin';

-- The refusals a person sees come from service/internal/user/permission.go,
-- with a specific error and status code. These two are the net underneath: on
-- every connection the service opens they hold for a raw query and for a code
-- path that forgets to ask. They catch an INSERT OR REPLACE over the owner's
-- row only because the DSN in service/internal/db/db.go sets
-- recursive_triggers(ON); that pragma is per connection, so another client
-- opening the file without it - a sqlite3 CLI session, which defaults to off -
-- can still REPLACE its way past a BEFORE DELETE trigger.
-- +goose StatementBegin
CREATE TRIGGER users_superadmin_undeletable BEFORE DELETE ON users
WHEN OLD.role = 'superadmin'
BEGIN
    SELECT RAISE(ABORT, 'the superadmin cannot be deleted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER users_superadmin_role_locked BEFORE UPDATE OF role ON users
WHEN OLD.role = 'superadmin'
BEGIN
    SELECT RAISE(ABORT, 'the superadmin cannot be demoted');
END;
-- +goose StatementEnd

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE api_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes       TEXT NOT NULL,
    expires_at   TEXT,
    last_used_at TEXT,
    created_at   TEXT NOT NULL
);

CREATE INDEX api_tokens_user_id_idx ON api_tokens(user_id);

-- +goose Down
DROP TABLE api_tokens;
DROP TABLE sessions;
DROP TABLE users;
