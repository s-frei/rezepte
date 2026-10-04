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

-- name: DeleteExpiredEmailConfirmations :exec
DELETE FROM email_confirmations WHERE expires_at <= ?;

-- Whether a link mailed to address is still open: what lets the profile say
-- "Confirmation mail sent" truthfully.
-- name: HasOpenEmailConfirmation :one
SELECT EXISTS (
    SELECT 1 FROM email_confirmations WHERE user_id = ? AND address = ? AND expires_at > ?
);

-- name: DeleteEmailConfirmation :exec
DELETE FROM email_confirmations WHERE id = ?;
