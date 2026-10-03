-- name: GetInstanceSettings :one
-- Lists its columns so the link preview key never rides along.
SELECT recipes_locked_by_default, link_previews, link_preview_minutes,
       public_shares, public_share_default_days, public_share_max_days,
       public_share_attribution
FROM instance_settings WHERE id = 1;

-- name: SetRecipesLockedByDefault :exec
UPDATE instance_settings SET recipes_locked_by_default = ? WHERE id = 1;

-- name: GetLinkPreviewKey :one
SELECT link_preview_key FROM instance_settings WHERE id = 1;

-- name: SetLinkPreviews :exec
UPDATE instance_settings SET link_previews = ? WHERE id = 1;

-- name: SetLinkPreviewMinutes :exec
UPDATE instance_settings SET link_preview_minutes = ? WHERE id = 1;

-- name: SetPublicShares :exec
UPDATE instance_settings SET public_shares = ? WHERE id = 1;

-- name: SetPublicShareAttribution :exec
UPDATE instance_settings SET public_share_attribution = ? WHERE id = 1;

-- Both lifetime columns at once: a maximum that lowers the default has to
-- land with it, in the one statement, or a reader between the two writes
-- could see a default the new maximum already forbids.
-- name: SetShareLifetimes :exec
UPDATE instance_settings
SET public_share_default_days = ?, public_share_max_days = ?
WHERE id = 1;

-- name: GetMailSettings :one
SELECT smtp_host, smtp_port, smtp_security, smtp_username, smtp_password,
       smtp_from, smtp_from_name
FROM instance_settings WHERE id = 1;

-- name: SetMailSettings :exec
UPDATE instance_settings
SET smtp_host = ?, smtp_port = ?, smtp_security = ?, smtp_username = ?,
    smtp_password = ?, smtp_from = ?, smtp_from_name = ?
WHERE id = 1;

-- name: ClearMailSettings :exec
UPDATE instance_settings
SET smtp_host = '', smtp_port = 587, smtp_security = 'starttls',
    smtp_username = '', smtp_password = '', smtp_from = '', smtp_from_name = 'Rezepte'
WHERE id = 1;
