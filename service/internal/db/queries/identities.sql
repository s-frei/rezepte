-- name: GetUserByIdentity :one
SELECT u.* FROM users u JOIN user_identities i ON i.user_id = u.id
WHERE i.issuer = ? AND i.subject = ?;

-- name: InsertIdentity :exec
INSERT INTO user_identities (user_id, issuer, subject, created_at) VALUES (?, ?, ?, ?);

-- name: GetIdentityOfUser :one
SELECT * FROM user_identities WHERE user_id = ? AND issuer = ?;

-- name: DeleteIdentityOfUser :execrows
DELETE FROM user_identities WHERE user_id = ? AND issuer = ?;

-- name: ListLinkedUserIDs :many
SELECT user_id FROM user_identities WHERE issuer = ?;

-- name: CountIdentities :one
-- Accounts connected at issuer, and how many of them have no password.
SELECT count(*) AS total,
       count(CASE WHEN u.password_hash = '' THEN 1 END) AS without_password
FROM user_identities i JOIN users u ON u.id = i.user_id
WHERE i.issuer = ?;
