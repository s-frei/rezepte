-- A setup link replaces whatever the account had. A reset link replaces an
-- older reset link or an expired one, never an open setup link an admin
-- handed out: no row changes then.
-- name: ReplaceSetupLink :execrows
INSERT INTO setup_links (id, user_id, created_by, expires_at, created_at, sent_to, purpose)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET
    id = excluded.id, created_by = excluded.created_by,
    expires_at = excluded.expires_at, created_at = excluded.created_at,
    sent_to = excluded.sent_to, purpose = excluded.purpose
WHERE excluded.purpose = 'setup' OR setup_links.purpose = 'reset'
    OR setup_links.expires_at <= excluded.created_at;

-- name: MarkSetupLinkSent :exec
UPDATE setup_links SET sent_to = ? WHERE id = ?;

-- name: GetOpenSetupLink :one
SELECT * FROM setup_links WHERE id = ? AND expires_at > ?;

-- Delete-returning makes redemption atomic: of two requests with the same
-- token, exactly one gets the row.
-- name: ConsumeSetupLink :one
DELETE FROM setup_links WHERE id = ? AND expires_at > ? RETURNING user_id, sent_to;

-- name: DeleteExpiredSetupLinks :exec
DELETE FROM setup_links WHERE expires_at <= ?;

-- name: DeleteSetupLinkOfUser :exec
DELETE FROM setup_links WHERE user_id = ?;

-- name: ListOpenSetupLinks :many
SELECT user_id, expires_at FROM setup_links WHERE purpose = 'setup' AND expires_at > ?;
