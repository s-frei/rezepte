-- name: CreateAPIToken :exec
INSERT INTO api_tokens (
    id, user_id, name, token_hash, token_prefix, scopes, expires_at, last_used_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?);

-- name: GetAPITokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = ?;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = ? WHERE id = ?;

-- name: ListAPITokens :many
SELECT t.*, u.username AS owner_name
FROM api_tokens t
JOIN users u ON u.id = t.user_id
ORDER BY t.created_at DESC;

-- name: DeleteAPIToken :execrows
DELETE FROM api_tokens WHERE id = ?;
