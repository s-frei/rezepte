-- name: CreateShare :one
INSERT INTO shares (id, token, recipe_id, created_by, created_at, starts_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- The creator's can_share_publicly rides along so the share service can work
-- out status (active/paused/limited) without a second query.
-- name: GetShareByToken :one
SELECT s.*, u.can_share_publicly AS creator_can_share_publicly
FROM shares s
JOIN users u ON u.id = s.created_by
WHERE s.token = ?;

-- name: GetShareByRecipeAndCreator :one
SELECT * FROM shares WHERE recipe_id = ? AND created_by = ?;

-- A member's own shares, with what the "my shared links" list shows: the
-- recipe the link points at and whether the member may still share at all.
-- name: ListSharesByCreator :many
SELECT s.*,
       r.slug AS recipe_slug, r.title AS recipe_title,
       u.display_name AS creator_display_name, u.color AS creator_color,
       u.can_share_publicly AS creator_can_share_publicly
FROM shares s
JOIN recipes r ON r.id = s.recipe_id
JOIN users u ON u.id = s.created_by
WHERE s.created_by = ?
ORDER BY s.created_at DESC, s.id DESC;

-- Every share in the household, for the admin view; adds the creator's
-- display name and color, which ListSharesByCreator has no use for.
-- name: ListAllShares :many
SELECT s.*,
       r.slug AS recipe_slug, r.title AS recipe_title,
       u.display_name AS creator_display_name, u.color AS creator_color,
       u.can_share_publicly AS creator_can_share_publicly
FROM shares s
JOIN recipes r ON r.id = s.recipe_id
JOIN users u ON u.id = s.created_by
ORDER BY s.created_at DESC, s.id DESC;

-- name: DeleteShare :execrows
DELETE FROM shares WHERE id = ?;

-- A member revoking their own share: the created_by check makes an id that
-- belongs to somebody else's share a no-op rather than a cross-account
-- delete, without a read first.
-- name: DeleteShareByCreator :execrows
DELETE FROM shares WHERE id = ? AND created_by = ?;

-- Admin "revoke all": every public link in the household, at once.
-- name: DeleteAllShares :exec
DELETE FROM shares;

-- The hourly sweep, like DeleteExpiredSessions: only a link past its own
-- expires_at goes. A link limited solely by the household maximum stays -
-- raising the maximum again must revive it, which a deleted row could not.
-- name: DeleteExpiredShares :exec
DELETE FROM shares WHERE expires_at IS NOT NULL AND expires_at < ?;

-- Re-timing restarts the clock: the new lifetime and the owner's maximum both
-- count from starts_at. Only the creator's own link that has not passed its
-- own expiry matches (stored times are RFC3339 UTC text, so they compare as
-- strings, as in DeleteExpiredShares); anything else updates no row.
-- name: RetimeShare :one
UPDATE shares SET starts_at = ?, expires_at = ?
WHERE id = ? AND created_by = ? AND (expires_at IS NULL OR expires_at > sqlc.arg(now))
RETURNING *;
