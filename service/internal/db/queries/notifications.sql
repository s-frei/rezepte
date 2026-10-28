-- name: ListOptOuts :many
SELECT kind FROM notification_optouts WHERE user_id = ? ORDER BY kind;

-- name: AddOptOut :exec
INSERT INTO notification_optouts (user_id, kind) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: RemoveOptOut :exec
DELETE FROM notification_optouts WHERE user_id = ? AND kind = ?;

-- name: HasOptOut :one
SELECT EXISTS (
    SELECT 1 FROM notification_optouts WHERE user_id = ? AND kind = ?
);

-- name: ListCommentMailRecipients :many
-- Who a new comment by author_id on recipe_id is mailed to: its followers
-- (the creator and everyone with a comment on it, as for the unread dot)
-- with a confirmed address and the comment mail on, never the author.
SELECT u.id, u.email, u.locale, CAST(u.id = r.created_by AS BOOLEAN) AS is_creator
FROM users u
JOIN recipes r ON r.id = sqlc.arg(recipe_id)
WHERE u.email != '' AND u.email_verified
  AND u.id != sqlc.arg(author_id)
  AND (u.id = r.created_by OR EXISTS (
      SELECT 1 FROM recipe_comments c WHERE c.recipe_id = r.id AND c.author_id = u.id))
  AND NOT EXISTS (
      SELECT 1 FROM notification_optouts o WHERE o.user_id = u.id AND o.kind = 'comment')
ORDER BY u.username;

-- name: ListRecipeDigestRecipients :many
-- Who gets the daily new recipes digest: everyone with a confirmed address
-- and the recipe mail on. Whose own recipes to leave out is the caller's.
SELECT id, email, locale
FROM users
WHERE email != '' AND email_verified
  AND NOT EXISTS (
      SELECT 1 FROM notification_optouts o WHERE o.user_id = users.id AND o.kind = 'recipe')
ORDER BY username;
