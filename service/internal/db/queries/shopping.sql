-- name: InsertShoppingList :exec
INSERT INTO shopping_lists (id, name, created_by, created_at, updated_at, updated_by) VALUES (?, ?, ?, ?, ?, ?);

-- A list the person created or accepted, with its creator.
-- name: GetShoppingList :one
SELECT l.id, l.name, l.created_by, l.created_at, l.updated_at,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id
FROM shopping_lists l
JOIN users o ON o.id = l.created_by
WHERE l.id = sqlc.arg(id) AND (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- Whether the person created or accepted the list.
-- name: CanAccessShoppingList :one
SELECT EXISTS (SELECT 1 FROM shopping_lists l WHERE l.id = sqlc.arg(id) AND (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL)));

-- name: GetShoppingListCreator :one
SELECT created_by FROM shopping_lists WHERE id = ?;

-- Every list one person created or accepted, with its open and done row counts, newest first.
-- A row is a name_key, as the list page's department view shows it: open
-- while any of its items is unticked, done once all are.
-- name: ListShoppingLists :many
SELECT l.id, l.name, l.updated_at, l.created_by,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id,
       l.updated_by, u.display_name AS updater_name, u.color AS updater_color, u.avatar_id AS updater_avatar_id,
       (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i WHERE i.list_id = l.id AND i.checked_at IS NULL) AS open_count,
       (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i WHERE i.list_id = l.id)
         - (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i WHERE i.list_id = l.id AND i.checked_at IS NULL) AS done_count
FROM shopping_lists l
JOIN users o ON o.id = l.created_by
LEFT JOIN users u ON u.id = l.updated_by
WHERE (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY l.updated_at DESC, l.id DESC;

-- The recipe entries of every list one person created or accepted, for the list overview.
-- name: ListShoppingEntryTitlesForUser :many
SELECT e.list_id, e.title, e.servings
FROM shopping_list_entries e
JOIN shopping_lists l ON l.id = e.list_id
WHERE (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY e.created_at, e.id;

-- name: RenameShoppingList :execrows
UPDATE shopping_lists SET name = sqlc.arg(name), updated_at = sqlc.arg(updated_at), updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND (shopping_lists.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = shopping_lists.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- Marks a list as changed by the person; zero rows when they have no access.
-- name: TouchShoppingList :execrows
UPDATE shopping_lists SET updated_at = sqlc.arg(updated_at), updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND (shopping_lists.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = shopping_lists.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- name: DeleteShoppingList :execrows
DELETE FROM shopping_lists WHERE id = sqlc.arg(id) AND created_by = sqlc.arg(created_by);

-- Accepted and pending members of lists, oldest first.
-- name: ListShoppingListMembers :many
SELECT m.list_id, m.user_id, m.accepted_at, u.display_name, u.color, u.avatar_id
FROM shopping_list_members m
JOIN users u ON u.id = m.user_id
WHERE m.list_id IN (sqlc.slice(list_ids))
ORDER BY m.accepted_at, m.invited_at, m.user_id;

-- Ends a membership whatever its state: leaving and declining are the same.
-- name: DeleteShoppingMembership :execrows
DELETE FROM shopping_list_members WHERE list_id = sqlc.arg(list_id) AND user_id = sqlc.arg(user_id);

-- A new invitation; zero rows when the person already has a row, pending or accepted.
-- name: InsertShoppingMember :execrows
INSERT INTO shopping_list_members (list_id, user_id, invited_at) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: AcceptShoppingMembership :execrows
UPDATE shopping_list_members SET accepted_at = sqlc.arg(accepted_at)
WHERE list_id = sqlc.arg(list_id) AND user_id = sqlc.arg(user_id) AND accepted_at IS NULL;

-- The creator withdraws an invitation or removes a member.
-- name: DeleteShoppingMember :execrows
DELETE FROM shopping_list_members
WHERE list_id = sqlc.arg(list_id) AND user_id = sqlc.arg(member_id)
  AND EXISTS (SELECT 1 FROM shopping_lists l WHERE l.id = sqlc.arg(list_id) AND l.created_by = sqlc.arg(user_id));

-- The person's pending invitations, newest first.
-- name: ListShoppingInvitations :many
SELECT l.id, l.name, m.invited_at, l.created_by,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id,
       (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i WHERE i.list_id = l.id AND i.checked_at IS NULL) AS open_count
FROM shopping_list_members m
JOIN shopping_lists l ON l.id = m.list_id
JOIN users o ON o.id = l.created_by
WHERE m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NULL
ORDER BY m.invited_at DESC, l.id DESC;

-- name: CountShoppingListsByCreator :many
SELECT created_by, COUNT(*) AS lists FROM shopping_lists GROUP BY created_by;

-- name: InsertShoppingEntry :exec
INSERT INTO shopping_list_entries (id, list_id, recipe_id, title, base_servings, servings, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListShoppingEntries :many
SELECT e.id, e.recipe_id, e.title, e.base_servings, e.servings,
       r.slug AS recipe_slug, r.cover_image_id
FROM shopping_list_entries e
LEFT JOIN recipes r ON r.id = e.recipe_id
WHERE e.list_id = ?
ORDER BY e.created_at, e.id;

-- name: SetShoppingEntryServings :execrows
UPDATE shopping_list_entries SET servings = sqlc.arg(servings)
WHERE id = sqlc.arg(id) AND list_id = sqlc.arg(list_id);

-- name: DeleteShoppingEntry :execrows
DELETE FROM shopping_list_entries WHERE id = sqlc.arg(id) AND list_id = sqlc.arg(list_id);

-- name: MaxShoppingItemPosition :one
SELECT CAST(COALESCE(MAX(position), 0) AS INTEGER) FROM shopping_list_items WHERE list_id = ?;

-- name: InsertShoppingItem :exec
INSERT INTO shopping_list_items (id, list_id, entry_id, name, name_key, quantity, unit, note, group_name, position, added_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListShoppingItems :many
SELECT i.id, i.entry_id, i.name, i.name_key, i.quantity, i.unit, i.note, i.group_name,
       i.checked_at, i.added_by,
       u.display_name AS adder_name, u.color AS adder_color, u.avatar_id AS adder_avatar_id
FROM shopping_list_items i
LEFT JOIN users u ON u.id = i.added_by
WHERE i.list_id = ?
ORDER BY i.position, i.id;

-- name: SetShoppingItemsChecked :exec
UPDATE shopping_list_items SET checked_at = sqlc.narg(checked_at)
WHERE list_id = sqlc.arg(list_id) AND id IN (sqlc.slice(ids));

-- name: DeleteShoppingItems :exec
DELETE FROM shopping_list_items WHERE list_id = sqlc.arg(list_id) AND id IN (sqlc.slice(ids));

-- name: DeleteCheckedShoppingItems :exec
DELETE FROM shopping_list_items WHERE list_id = ? AND checked_at IS NOT NULL;

-- name: ListDepartmentOverrides :many
SELECT name_key, department FROM shopping_department_overrides;

-- name: UpsertDepartmentOverride :exec
INSERT INTO shopping_department_overrides (name_key, department, updated_by, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (name_key) DO UPDATE SET department = excluded.department,
    updated_by = excluded.updated_by, updated_at = excluded.updated_at;

-- name: GetShoppingListName :one
SELECT name FROM shopping_lists WHERE id = ?;
