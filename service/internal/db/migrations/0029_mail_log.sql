-- Every mail Rezepte hands to its SMTP server, for the owner's outbox: what
-- kind it was, to whom, its subject and how it went. Never the body: a
-- setup or reset mail carries a link that signs somebody in. error is the
-- last failed attempt's reason, kept when a retry then got through;
-- next_attempt_at is set while a retry waits. Rows older than 30 days are
-- dropped by the mail worker.
-- +goose Up
CREATE TABLE mail_log (
    id          INTEGER PRIMARY KEY,
    kind        TEXT NOT NULL,
    recipient   TEXT NOT NULL,
    subject     TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('queued', 'sent', 'failed')),
    attempts    INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    next_attempt_at TEXT,
    created_at  TEXT NOT NULL,
    finished_at TEXT
);

-- The history lists newest first; the prune deletes by age.
CREATE INDEX mail_log_created_idx ON mail_log(created_at);

-- +goose Down
DROP INDEX mail_log_created_idx;
DROP TABLE mail_log;
