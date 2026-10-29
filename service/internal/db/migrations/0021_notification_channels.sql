-- Notification channels (docs/memory/content/features/notifications.mdx):
-- every switchable kind goes out by mail and by push, each on until its
-- account opts out. A row in notification_optouts is a kind switched off on
-- one channel; a row in notification_group_optouts is a group's master
-- switch off on one channel, which silences every kind of the group there,
-- kinds added later too, and leaves the kind rows untouched. 0020's rows are
-- mail rows; where an account had every kind of a group off (what the old
-- group switch wrote), they become that group's mail row. The group is grp:
-- group is an SQL keyword.
-- instance_settings.mail_failing_since is when real sends began failing for
-- the instance's mail setup, empty while mail works; mail_alert_last_at is
-- when the owner was last warned by push, empty before the first.
-- A push subscription belongs to the session that stored it and ends with
-- it, so a signed-out or revoked device stops getting a member's
-- notifications. Rows from before have no session and are dropped, with
-- their alarms; a device stores its subscription again on its next sign-in.
-- +goose Up
CREATE TABLE notification_optouts_new (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    channel TEXT NOT NULL CHECK (channel IN ('mail', 'push')),
    PRIMARY KEY (user_id, kind, channel)
);
INSERT INTO notification_optouts_new (user_id, kind, channel)
SELECT user_id, kind, 'mail' FROM notification_optouts;
DROP TABLE notification_optouts;
ALTER TABLE notification_optouts_new RENAME TO notification_optouts;

CREATE TABLE notification_group_optouts (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    grp TEXT NOT NULL,
    channel TEXT NOT NULL CHECK (channel IN ('mail', 'push')),
    PRIMARY KEY (user_id, grp, channel)
);
INSERT INTO notification_group_optouts (user_id, grp, channel)
SELECT user_id, 'household', 'mail' FROM notification_optouts
WHERE kind IN ('comment', 'recipe') GROUP BY user_id HAVING COUNT(*) = 2;
INSERT INTO notification_group_optouts (user_id, grp, channel)
SELECT user_id, 'admin', 'mail' FROM notification_optouts WHERE kind = 'invite_accepted';
DELETE FROM notification_optouts WHERE kind IN ('comment', 'recipe')
  AND user_id IN (SELECT user_id FROM notification_group_optouts WHERE grp = 'household');
DELETE FROM notification_optouts WHERE kind = 'invite_accepted'
  AND user_id IN (SELECT user_id FROM notification_group_optouts WHERE grp = 'admin');

ALTER TABLE instance_settings ADD COLUMN mail_failing_since TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN mail_alert_last_at TEXT NOT NULL DEFAULT '';

-- Emptying the table first takes the alarms with it, so dropping it leaves
-- timer_alarms nothing that points nowhere; its reference resolves to the
-- new table by name.
DELETE FROM push_subscriptions;
CREATE TABLE push_subscriptions_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     BLOB NOT NULL,
    auth       BLOB NOT NULL,
    created_at TEXT NOT NULL
);
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions_new RENAME TO push_subscriptions;
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions(user_id);
CREATE INDEX push_subscriptions_session_idx ON push_subscriptions(session_id);

-- +goose Down
-- The alarms are kept aside while the table is rebuilt: dropping it
-- cascades to them.
CREATE TEMP TABLE timer_alarms_kept AS SELECT * FROM timer_alarms;
CREATE TABLE push_subscriptions_old (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     BLOB NOT NULL,
    auth       BLOB NOT NULL,
    created_at TEXT NOT NULL
);
INSERT INTO push_subscriptions_old (id, user_id, endpoint, p256dh, auth, created_at)
SELECT id, user_id, endpoint, p256dh, auth, created_at FROM push_subscriptions;
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions_old RENAME TO push_subscriptions;
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions(user_id);
INSERT INTO timer_alarms SELECT * FROM timer_alarms_kept;
DROP TABLE timer_alarms_kept;

ALTER TABLE instance_settings DROP COLUMN mail_alert_last_at;
ALTER TABLE instance_settings DROP COLUMN mail_failing_since;

CREATE TABLE notification_optouts_old (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    PRIMARY KEY (user_id, kind)
);
INSERT OR IGNORE INTO notification_optouts_old (user_id, kind)
SELECT user_id, kind FROM notification_optouts
WHERE channel = 'mail' AND kind IN ('comment', 'recipe', 'invite_accepted')
UNION SELECT user_id, 'comment' FROM notification_group_optouts WHERE grp = 'household' AND channel = 'mail'
UNION SELECT user_id, 'recipe' FROM notification_group_optouts WHERE grp = 'household' AND channel = 'mail'
UNION SELECT user_id, 'invite_accepted' FROM notification_group_optouts WHERE grp = 'admin' AND channel = 'mail';
DROP TABLE notification_group_optouts;
DROP TABLE notification_optouts;
ALTER TABLE notification_optouts_old RENAME TO notification_optouts;
