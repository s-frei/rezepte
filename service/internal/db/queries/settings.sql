-- name: GetInstanceSettings :one
-- Lists its columns so the link preview key never rides along.
SELECT recipes_locked_by_default, link_previews, link_preview_minutes
FROM instance_settings WHERE id = 1;

-- name: SetRecipesLockedByDefault :exec
UPDATE instance_settings SET recipes_locked_by_default = ? WHERE id = 1;

-- name: GetLinkPreviewKey :one
SELECT link_preview_key FROM instance_settings WHERE id = 1;

-- name: SetLinkPreviews :exec
UPDATE instance_settings SET link_previews = ? WHERE id = 1;

-- name: SetLinkPreviewMinutes :exec
UPDATE instance_settings SET link_preview_minutes = ? WHERE id = 1;
