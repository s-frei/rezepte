-- name: GetInstanceSettings :one
SELECT * FROM instance_settings WHERE id = 1;

-- name: SetRecipesLockedByDefault :exec
UPDATE instance_settings SET recipes_locked_by_default = ? WHERE id = 1;
