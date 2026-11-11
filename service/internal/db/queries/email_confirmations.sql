-- name: ReplaceEmailConfirmation :exec
INSERT INTO email_confirmations (id, user_id, address, expires_at, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET
    id = excluded.id, address = excluded.address,
    expires_at = excluded.expires_at, created_at = excluded.created_at;

-- Delete-returning, like ConsumeSetupLink: of two requests with the same
-- token, exactly one gets the row.
-- name: ConsumeEmailConfirmation :one
DELETE FROM email_confirmations WHERE id = ? AND expires_at > ? RETURNING user_id, address;

-- name: DeleteEmailConfirmation :exec
DELETE FROM email_confirmations WHERE id = ?;

-- Every account's one confirmation, open or expired: what the email row shows.
-- name: ListEmailConfirmations :many
SELECT user_id, address, created_at, expires_at FROM email_confirmations;
