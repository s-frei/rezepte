-- name: CreateUser :one
INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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

-- Lives here rather than in recipes.sql so Phase 4 (running concurrently)
-- and this phase never edit the same query file. recipes.created_by and
-- recipes.updated_by are both NOT NULL without ON DELETE, so every mention
-- of a user has to move before their row can go - a recipe somebody else
-- wrote but this user last edited names them in updated_by alone.
-- name: ReassignRecipes :exec
UPDATE recipes
SET created_by = CASE WHEN created_by = sqlc.arg(old_owner) THEN sqlc.arg(new_owner) ELSE created_by END,
    updated_by = CASE WHEN updated_by = sqlc.arg(old_owner) THEN sqlc.arg(new_owner) ELSE updated_by END
WHERE created_by = sqlc.arg(old_owner) OR updated_by = sqlc.arg(old_owner);

-- Colours nobody holds are absent from this result; Go fills them in against
-- user.Colors, because the database does not know the palette.
-- name: CountUsersByColor :many
SELECT color, COUNT(*) AS user_count FROM users GROUP BY color;

-- Both profile columns at once. SetProfile reads the row first and fills in
-- whichever of the two the caller left alone, so a partial update needs no
-- second statement.
-- name: UpdateUserProfile :one
UPDATE users SET display_name = ?, color = ?, updated_at = ? WHERE id = ? RETURNING *;
