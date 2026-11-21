-- Access to a plan is its creator or an accepted member, repeated verbatim
-- in every query that needs it, as on shopping lists.

-- name: InsertMealPlan :exec
INSERT INTO meal_plans (id, name, personal, created_by, created_at, updated_at, updated_by) VALUES (?, ?, ?, ?, ?, ?, ?);

-- The person's personal plan, unless they have one already.
-- name: InsertPersonalMealPlan :exec
INSERT INTO meal_plans (id, name, personal, created_by, created_at, updated_at, updated_by)
VALUES (sqlc.arg(id), NULL, 1, sqlc.arg(user_id), sqlc.arg(now), sqlc.arg(now), sqlc.arg(user_id))
ON CONFLICT DO NOTHING;

-- A plan the person created or accepted, with its creator.
-- name: GetMealPlan :one
SELECT p.id, p.name, p.personal, p.created_by, p.created_at, p.updated_at, p.revision,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id
FROM meal_plans p
JOIN users o ON o.id = p.created_by
WHERE p.id = sqlc.arg(id) AND (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- The plan's revision when the person may read it; no row otherwise.
-- name: GetMealPlanRevision :one
SELECT p.revision FROM meal_plans p WHERE p.id = sqlc.arg(id) AND (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- name: GetMealPlanCreator :one
SELECT created_by, personal FROM meal_plans WHERE id = ?;

-- Marks a plan as changed by the person; zero rows when they have no access.
-- name: TouchMealPlan :execrows
UPDATE meal_plans SET updated_at = sqlc.arg(updated_at), updated_by = sqlc.arg(user_id), revision = revision + 1
WHERE id = sqlc.arg(id) AND (meal_plans.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = meal_plans.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL));

-- A change that does not touch the plan row itself (members).
-- name: BumpMealPlanRevision :exec
UPDATE meal_plans SET revision = revision + 1 WHERE id = ?;

-- Every plan one person created or accepted: the personal plan first, then
-- the most recently changed.
-- name: ListMealPlans :many
SELECT p.id, p.name, p.personal, p.updated_at, p.created_by,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id
FROM meal_plans p
JOIN users o ON o.id = p.created_by
WHERE (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY p.personal DESC, p.updated_at DESC, p.id DESC;

-- Only shared plans are renamed; zero rows for a personal plan. Runs after
-- TouchMealPlan, which checks access.
-- name: RenameMealPlan :execrows
UPDATE meal_plans SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id) AND personal = 0;

-- name: DeleteMealPlan :execrows
DELETE FROM meal_plans WHERE id = sqlc.arg(id) AND created_by = sqlc.arg(created_by) AND personal = 0;

-- name: CountMealPlansByCreator :many
SELECT created_by, COUNT(*) AS plans FROM meal_plans GROUP BY created_by;

-- Accepted and pending members of plans, oldest first.
-- name: ListMealPlanMembers :many
SELECT m.plan_id, m.user_id, m.accepted_at, u.display_name, u.color, u.avatar_id
FROM meal_plan_members m
JOIN users u ON u.id = m.user_id
WHERE m.plan_id IN (sqlc.slice(plan_ids))
ORDER BY m.accepted_at, m.invited_at, m.user_id;

-- A new invitation; zero rows when the person already has a row, pending or accepted.
-- name: InsertMealPlanMember :execrows
INSERT INTO meal_plan_members (plan_id, user_id, invited_at) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: AcceptMealPlanMembership :execrows
UPDATE meal_plan_members SET accepted_at = sqlc.arg(accepted_at)
WHERE plan_id = sqlc.arg(plan_id) AND user_id = sqlc.arg(user_id) AND accepted_at IS NULL;

-- Ends a membership whatever its state: leaving and declining are the same.
-- name: DeleteMealPlanMembership :execrows
DELETE FROM meal_plan_members WHERE plan_id = sqlc.arg(plan_id) AND user_id = sqlc.arg(user_id);

-- The creator withdraws an invitation or removes a member.
-- name: DeleteMealPlanMember :execrows
DELETE FROM meal_plan_members
WHERE plan_id = sqlc.arg(plan_id) AND user_id = sqlc.arg(member_id)
  AND EXISTS (SELECT 1 FROM meal_plans p WHERE p.id = sqlc.arg(plan_id) AND p.created_by = sqlc.arg(user_id));

-- The person's pending invitations, newest first.
-- name: ListMealPlanInvitations :many
SELECT p.id, p.name, m.invited_at, p.created_by,
       o.display_name AS owner_name, o.color AS owner_color, o.avatar_id AS owner_avatar_id
FROM meal_plan_members m
JOIN meal_plans p ON p.id = m.plan_id
JOIN users o ON o.id = p.created_by
WHERE m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NULL
ORDER BY m.invited_at DESC, p.id DESC;

-- The entries of a plan between two days (inclusive) and on no day, the
-- tray first, then by day and position.
-- name: ListMealPlanEntries :many
SELECT e.id, e.day, e.position, e.recipe_id, e.title, e.servings, e.created_by,
       r.slug AS recipe_slug, r.cover_image_id,
       u.display_name AS creator_name, u.color AS creator_color, u.avatar_id AS creator_avatar_id
FROM meal_plan_entries e
LEFT JOIN recipes r ON r.id = e.recipe_id
LEFT JOIN users u ON u.id = e.created_by
WHERE e.plan_id = sqlc.arg(plan_id) AND e.removed_at IS NULL
  AND (e.day IS NULL OR (e.day >= sqlc.arg(from_day) AND e.day <= sqlc.arg(to_day)))
ORDER BY e.day NULLS FIRST, e.position, e.created_at, e.id;

-- name: GetMealPlanEntry :one
SELECT id, day, position, recipe_id, servings, removed_at
FROM meal_plan_entries WHERE id = sqlc.arg(id) AND plan_id = sqlc.arg(plan_id);

-- The position after the last entry of a day (or the tray, day NULL); the
-- live entries of a day always sit at 0..n-1.
-- name: NextMealPlanEntryPosition :one
SELECT COUNT(*) FROM meal_plan_entries
WHERE plan_id = sqlc.arg(plan_id) AND day IS sqlc.narg(day) AND removed_at IS NULL;

-- The entries of one day (or the tray, day NULL) in order.
-- name: ListMealPlanDayEntryIDs :many
SELECT id FROM meal_plan_entries
WHERE plan_id = sqlc.arg(plan_id) AND day IS sqlc.narg(day) AND removed_at IS NULL
ORDER BY position, created_at, id;

-- name: InsertMealPlanEntry :exec
INSERT INTO meal_plan_entries (id, plan_id, day, position, recipe_id, title, servings, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: PlaceMealPlanEntry :exec
UPDATE meal_plan_entries SET day = sqlc.narg(day), position = sqlc.arg(position)
WHERE id = sqlc.arg(id) AND plan_id = sqlc.arg(plan_id);

-- Only an entry with servings (a recipe) takes new ones.
-- name: SetMealPlanEntryServings :execrows
UPDATE meal_plan_entries SET servings = sqlc.arg(servings)
WHERE id = sqlc.arg(id) AND plan_id = sqlc.arg(plan_id) AND removed_at IS NULL AND servings IS NOT NULL;

-- name: RemoveMealPlanEntry :execrows
UPDATE meal_plan_entries SET removed_at = sqlc.arg(removed_at)
WHERE id = sqlc.arg(id) AND plan_id = sqlc.arg(plan_id) AND removed_at IS NULL;

-- name: RestoreMealPlanEntry :execrows
UPDATE meal_plan_entries SET removed_at = NULL
WHERE id = sqlc.arg(id) AND plan_id = sqlc.arg(plan_id) AND removed_at IS NOT NULL;

-- Every entry of a plan, removed ones too.
-- name: ListMealPlanEntryIDs :many
SELECT id FROM meal_plan_entries WHERE plan_id = ?;

-- name: ListRemovedMealPlanEntryIDs :many
SELECT id FROM meal_plan_entries WHERE plan_id = sqlc.arg(plan_id) AND removed_at < sqlc.arg(cutoff);

-- Drops a plan's entries removed before the cutoff; their list entries cascade.
-- name: PurgeRemovedMealPlanEntries :exec
DELETE FROM meal_plan_entries WHERE plan_id = sqlc.arg(plan_id) AND removed_at < sqlc.arg(cutoff);

-- The accessible lists each entry of a plan is on, with each list's open
-- rows (as ListShoppingLists counts them); lists the person cannot access
-- never show.
-- name: ListMealPlanEntryLists :many
SELECT se.plan_entry_id, l.id AS list_id, l.name AS list_name,
       (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i
        LEFT JOIN shopping_list_entries ie ON ie.id = i.entry_id
        LEFT JOIN meal_plan_entries ip ON ip.id = ie.plan_entry_id
        WHERE i.list_id = l.id AND i.checked_at IS NULL AND ip.removed_at IS NULL) AS open_count
FROM shopping_list_entries se
JOIN meal_plan_entries e ON e.id = se.plan_entry_id
JOIN shopping_lists l ON l.id = se.list_id
WHERE e.plan_id = sqlc.arg(plan_id) AND e.removed_at IS NULL
  AND (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY se.created_at, se.id;

-- name: ListMealPlanStaples :many
SELECT name_key, name FROM meal_plan_staples WHERE plan_id = ? ORDER BY name_key;

-- name: InsertMealPlanStaple :exec
INSERT INTO meal_plan_staples (plan_id, name_key, name, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: DeleteMealPlanStaple :exec
DELETE FROM meal_plan_staples WHERE plan_id = sqlc.arg(plan_id) AND name_key = sqlc.arg(name_key);

-- Creates or replaces the person's feed secret.
-- name: UpsertCalendarFeed :exec
INSERT INTO calendar_feeds (user_id, token_hash, created_at) VALUES (?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET token_hash = excluded.token_hash, created_at = excluded.created_at;

-- When the person's feed secret was made; no row while they have none.
-- name: GetCalendarFeedCreated :one
SELECT created_at FROM calendar_feeds WHERE user_id = ?;

-- name: GetCalendarFeedUser :one
SELECT user_id FROM calendar_feeds WHERE token_hash = ?;

-- name: DeleteCalendarFeed :exec
DELETE FROM calendar_feeds WHERE user_id = ?;

-- The live dated entries from a day on of every plan the person created or
-- accepted, for the calendar feed.
-- name: ListCalendarFeedEntries :many
SELECT e.id, e.day, e.title, r.slug AS recipe_slug, p.name AS plan_name
FROM meal_plan_entries e
JOIN meal_plans p ON p.id = e.plan_id
LEFT JOIN recipes r ON r.id = e.recipe_id
WHERE e.removed_at IS NULL AND e.day IS NOT NULL AND e.day >= sqlc.arg(from_day)
  AND (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY e.day, e.position, e.id;

-- A shared plan's name, for the invitation notice.
-- name: GetMealPlanName :one
SELECT name FROM meal_plans WHERE id = ?;

-- The live entries between two days (inclusive) and on the tray of every
-- plan one person created or accepted, for the plans overview; ordered as
-- ListMealPlanEntries within each plan.
-- name: ListMealPlanWeekEntries :many
SELECT e.plan_id, e.id, e.day, e.recipe_id, e.title, e.servings, r.cover_image_id
FROM meal_plan_entries e
JOIN meal_plans p ON p.id = e.plan_id
LEFT JOIN recipes r ON r.id = e.recipe_id
WHERE e.removed_at IS NULL
  AND (e.day IS NULL OR (e.day >= sqlc.arg(from_day) AND e.day <= sqlc.arg(to_day)))
  AND (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY e.plan_id, e.day NULLS FIRST, e.position, e.created_at, e.id;

-- ListMealPlanEntryLists for the entries ListMealPlanWeekEntries returns.
-- name: ListMealPlanWeekEntryLists :many
SELECT se.plan_entry_id, l.id AS list_id, l.name AS list_name,
       (SELECT COUNT(DISTINCT i.name_key) FROM shopping_list_items i
        LEFT JOIN shopping_list_entries ie ON ie.id = i.entry_id
        LEFT JOIN meal_plan_entries ip ON ip.id = ie.plan_entry_id
        WHERE i.list_id = l.id AND i.checked_at IS NULL AND ip.removed_at IS NULL) AS open_count
FROM shopping_list_entries se
JOIN meal_plan_entries e ON e.id = se.plan_entry_id
JOIN meal_plans p ON p.id = e.plan_id
JOIN shopping_lists l ON l.id = se.list_id
WHERE e.removed_at IS NULL
  AND (e.day IS NULL OR (e.day >= sqlc.arg(from_day) AND e.day <= sqlc.arg(to_day)))
  AND (p.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM meal_plan_members m WHERE m.plan_id = p.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
  AND (l.created_by = sqlc.arg(user_id) OR EXISTS (SELECT 1 FROM shopping_list_members m WHERE m.list_id = l.id AND m.user_id = sqlc.arg(user_id) AND m.accepted_at IS NOT NULL))
ORDER BY se.created_at, se.id;
