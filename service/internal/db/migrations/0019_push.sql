-- +goose Up
-- Web Push for cook timers (docs/memory/content/features/cook-timers.mdx):
-- the server keeps the devices that asked for a push and the alarms they
-- scheduled, never a timer's state. An endpoint belongs to one account at a
-- time; a device that signs in as someone else moves it.
CREATE TABLE push_subscriptions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     BLOB NOT NULL,
    auth       BLOB NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions(user_id);

-- One push to send at fire_at. The copy is built in the browser in the
-- member's language, so the server needs no catalog for it.
CREATE TABLE timer_alarms (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES push_subscriptions(id) ON DELETE CASCADE,
    fire_at         TEXT NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    url             TEXT NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX timer_alarms_fire_at_idx ON timer_alarms(fire_at);
CREATE INDEX timer_alarms_subscription_idx ON timer_alarms(subscription_id);

-- The VAPID signing key (PKCS #8), written on first use.
ALTER TABLE instance_settings ADD COLUMN vapid_key BLOB;

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN vapid_key;
DROP TABLE timer_alarms;
DROP TABLE push_subscriptions;
