-- name: CreateUser :one
INSERT INTO users (
    id, username, display_name, password_hash, role, color, locale, email, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username;

-- name: GetSuperadmin :one
SELECT * FROM users WHERE role = 'superadmin';

-- name: UpdateUserRole :one
UPDATE users SET role = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: UpdateUserPasswordHash :execrows
UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- The acting admin becomes the author of every recipe the deleted user
-- wrote, and an author cannot mark their own recipe tasty: a mark the admin
-- had left on one of them goes before it changes hands.
-- name: DeleteTastyOfNewOwner :exec
DELETE FROM tasty
WHERE user_id = sqlc.arg(new_owner)
  AND recipe_id IN (SELECT id FROM recipes WHERE created_by = sqlc.arg(old_owner));

-- The acting admin becomes the author of every recipe the deleted user
-- wrote, and an author follows the entries on their recipes: without this,
-- every entry already on those recipes would show as new to the admin. Their
-- watermark rises to each recipe's newest entry and never lowers. The WHERE
-- before GROUP BY keeps SQLite's upsert parser from reading ON CONFLICT as a
-- join constraint.
-- name: SeeCommentsOfNewOwner :exec
INSERT INTO recipe_comment_reads (user_id, recipe_id, last_seen)
SELECT sqlc.arg(new_owner), c.recipe_id, MAX(c.id)
FROM recipe_comments c
WHERE c.recipe_id IN (SELECT id FROM recipes WHERE created_by = sqlc.arg(old_owner))
GROUP BY c.recipe_id
ON CONFLICT (user_id, recipe_id) DO UPDATE SET last_seen = MAX(last_seen, excluded.last_seen);

-- recipes.created_by and recipes.updated_by are both NOT NULL without ON
-- DELETE, so every mention of a user has to move before their row can go -
-- a recipe somebody else wrote but this user last edited names them in
-- updated_by alone.
-- name: ReassignRecipes :exec
UPDATE recipes
SET created_by = CASE WHEN created_by = sqlc.arg(old_owner) THEN sqlc.arg(new_owner) ELSE created_by END,
    updated_by = CASE WHEN updated_by = sqlc.arg(old_owner) THEN sqlc.arg(new_owner) ELSE updated_by END
WHERE created_by = sqlc.arg(old_owner) OR updated_by = sqlc.arg(old_owner);

-- Colors nobody holds are absent from this result; Go fills them in against
-- user.Colors, because the database does not know the palette.
-- name: CountUsersByColor :many
SELECT color, COUNT(*) AS user_count FROM users GROUP BY color;

-- Every self-service profile column at once. SetProfile reads the row first
-- and fills in whichever the caller left alone, so a partial update needs no
-- second statement.
-- name: UpdateUserProfile :one
UPDATE users
SET display_name = ?, color = ?, locale = ?, email = ?, email_verified = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- The identity provider's address lands only in an empty field: once a person
-- has an address, it is theirs to change.
-- name: SetEmailIfEmpty :exec
UPDATE users SET email = ?, email_verified = ?, updated_at = ?
WHERE id = ? AND email = '';

-- name: SetCanSharePublicly :one
UPDATE users SET can_share_publicly = ? WHERE id = ? RETURNING *;

-- name: SetUserAvatar :execrows
UPDATE users SET avatar_id = ?, updated_at = ? WHERE id = ?;

-- Verifies the address only while it is still the one the link was mailed
-- to; a change in between leaves it unverified.
-- name: VerifyEmailIfMatches :exec
UPDATE users SET email_verified = 1, updated_at = ?
WHERE id = ? AND email = ? AND email != '';
