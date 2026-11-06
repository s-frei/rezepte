-- +goose Up
-- When the account last showed activity (see auth.touchActive). Existing
-- accounts start from what is already recorded: a session's last renewal is its expiry minus the 30-day session
-- lifetime (auth.SessionTTL), and a token's last use is stored. An account
-- with neither stays NULL.
ALTER TABLE users ADD COLUMN last_active_at TEXT;
UPDATE users SET last_active_at = (
    SELECT MAX(t) FROM (
        SELECT strftime('%Y-%m-%dT%H:%M:%SZ', expires_at, '-30 days') AS t
        FROM sessions WHERE user_id = users.id
        UNION ALL
        SELECT last_used_at FROM api_tokens WHERE user_id = users.id
    )
);

-- +goose Down
ALTER TABLE users DROP COLUMN last_active_at;
