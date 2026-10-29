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

-- +goose Down
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
