-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?);

-- name: GetSession :one
SELECT * FROM sessions WHERE id = ?;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = ? WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < ?;

-- name: DeleteUserSessionsExcept :exec
DELETE FROM sessions WHERE user_id = ? AND id != ?;
