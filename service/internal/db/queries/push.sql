-- name: GetVapidKey :one
SELECT vapid_key FROM instance_settings WHERE id = 1;

-- Writes the key only when none is stored, so two first requests at once
-- keep one key: the loser reads the winner's back.
-- name: SetVapidKeyIfMissing :exec
UPDATE instance_settings SET vapid_key = ? WHERE id = 1 AND vapid_key IS NULL;

-- Renews the keys of an endpoint the account already has. A move from
-- another account deletes the old row first, so its alarms go with it.
-- name: UpsertPushSubscription :exec
INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (endpoint) DO UPDATE
SET user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions WHERE endpoint = ?;

-- name: CountPushSubscriptionsByUser :one
SELECT COUNT(*) FROM push_subscriptions WHERE user_id = ?;

-- name: GetPushSubscriptionForUser :one
SELECT id, user_id, endpoint, p256dh, auth, created_at
FROM push_subscriptions WHERE user_id = ? AND endpoint = ?;

-- name: CountAlarmsBySubscription :one
SELECT COUNT(*) FROM timer_alarms WHERE subscription_id = ?;

-- name: InsertTimerAlarm :exec
INSERT INTO timer_alarms (id, subscription_id, fire_at, title, body, url, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteTimerAlarmForUser :exec
DELETE FROM timer_alarms
WHERE timer_alarms.id = sqlc.arg(id) AND timer_alarms.subscription_id IN (
    SELECT s.id FROM push_subscriptions s WHERE s.user_id = sqlc.arg(user_id)
);

-- Oldest first, so a backlog sends in order.
-- name: ListDueTimerAlarms :many
SELECT a.id, a.fire_at, a.title, a.body, a.url,
       s.endpoint, s.p256dh, s.auth
FROM timer_alarms a
JOIN push_subscriptions s ON s.id = a.subscription_id
WHERE a.fire_at <= ?
ORDER BY a.fire_at;

-- name: DeleteTimerAlarm :execrows
DELETE FROM timer_alarms WHERE id = ?;
