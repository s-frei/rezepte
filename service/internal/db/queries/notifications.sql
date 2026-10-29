-- name: ListOptOuts :many
SELECT kind, channel FROM notification_optouts WHERE user_id = ? ORDER BY kind, channel;

-- name: AddOptOut :exec
INSERT INTO notification_optouts (user_id, kind, channel) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: RemoveOptOut :exec
DELETE FROM notification_optouts WHERE user_id = ? AND kind = ? AND channel = ?;

-- name: ListGroupOptOuts :many
SELECT grp, channel FROM notification_group_optouts WHERE user_id = ? ORDER BY grp, channel;

-- name: AddGroupOptOut :exec
INSERT INTO notification_group_optouts (user_id, grp, channel) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: RemoveGroupOptOut :exec
DELETE FROM notification_group_optouts WHERE user_id = ? AND grp = ? AND channel = ?;

-- name: HasOptOut :one
-- Whether user_id switched kind, or its group grp, off on channel.
SELECT CAST(EXISTS (
    SELECT 1 FROM notification_optouts o
    WHERE o.user_id = sqlc.arg(user_id) AND o.kind = sqlc.arg(kind) AND o.channel = sqlc.arg(channel)
) OR EXISTS (
    SELECT 1 FROM notification_group_optouts g
    WHERE g.user_id = sqlc.arg(user_id) AND g.grp = sqlc.arg(grp) AND g.channel = sqlc.arg(channel)
) AS BOOLEAN);

-- name: GetMailAlertState :one
SELECT mail_failing_since, mail_alert_last_at FROM instance_settings WHERE id = 1;

-- name: SetMailAlertState :exec
UPDATE instance_settings SET mail_failing_since = ?, mail_alert_last_at = ? WHERE id = 1;

-- name: ListCommentMailRecipients :many
-- Who a new comment by author_id on recipe_id is mailed to: its followers
-- (the creator and everyone with a comment on it, as for the unread dot)
-- with a confirmed address and the comment mail and the household mail
-- switch on, never the author.
SELECT u.id, u.email, u.locale, CAST(u.id = r.created_by AS BOOLEAN) AS is_creator
FROM users u
JOIN recipes r ON r.id = sqlc.arg(recipe_id)
WHERE u.email != '' AND u.email_verified
  AND u.id != sqlc.arg(author_id)
  AND (u.id = r.created_by OR EXISTS (
      SELECT 1 FROM recipe_comments c WHERE c.recipe_id = r.id AND c.author_id = u.id))
  AND NOT EXISTS (
      SELECT 1 FROM notification_optouts o WHERE o.user_id = u.id AND o.kind = 'comment' AND o.channel = 'mail')
  AND NOT EXISTS (
      SELECT 1 FROM notification_group_optouts g WHERE g.user_id = u.id AND g.grp = 'household' AND g.channel = 'mail')
ORDER BY u.username;

-- name: ListRecipeDigestRecipients :many
-- Who gets the daily new recipes digest: everyone with a confirmed address
-- and the recipe mail and the household mail switch on. Whose own recipes to leave out is the caller's.
SELECT id, email, locale
FROM users
WHERE email != '' AND email_verified
  AND NOT EXISTS (
      SELECT 1 FROM notification_optouts o WHERE o.user_id = users.id AND o.kind = 'recipe' AND o.channel = 'mail')
  AND NOT EXISTS (
      SELECT 1 FROM notification_group_optouts g WHERE g.user_id = users.id AND g.grp = 'household' AND g.channel = 'mail')
ORDER BY username;
