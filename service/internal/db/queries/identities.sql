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
