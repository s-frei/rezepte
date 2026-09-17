-- name: CreateUser :one
INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username;

-- name: CountAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin';

-- name: UpdateUserRole :one
UPDATE users SET role = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: UpdateUserPasswordHash :execrows
UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- Lives here rather than in recipes.sql so Phase 4 (running concurrently)
-- and this phase never edit the same query file. recipes.created_by is
-- NOT NULL without ON DELETE, so a user's recipes must move before the
-- user row can go.
-- name: ReassignRecipes :exec
UPDATE recipes SET created_by = sqlc.arg(new_owner) WHERE created_by = sqlc.arg(old_owner);
