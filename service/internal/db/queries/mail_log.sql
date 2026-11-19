-- name: InsertMailLog :one
INSERT INTO mail_log (kind, recipient, subject, status, created_at)
VALUES (?, ?, ?, 'queued', ?)
RETURNING id;

-- A failed attempt that will be tried again.
-- name: RetryMailLog :exec
UPDATE mail_log SET attempts = ?, error = ?, next_attempt_at = ? WHERE id = ?;

-- error stays as the last failed attempt left it when a retry got through,
-- so the outbox can say what the first attempt ran into.
-- name: FinishMailLog :exec
UPDATE mail_log SET status = ?, attempts = ?, error = ?, finished_at = ?, next_attempt_at = NULL WHERE id = ?;

-- A mail still queued when the process ended was never sent: the queue
-- lives in memory.
-- name: InterruptQueuedMailLog :exec
UPDATE mail_log SET status = 'failed', error = ?, finished_at = ?, next_attempt_at = NULL WHERE status = 'queued';

-- One page, newest first: ids grow with every insert, so the id of the last
-- row shown is the cursor of the next page. An empty status lists all.
-- name: ListMailLog :many
SELECT * FROM mail_log
WHERE id < sqlc.arg(before) AND (CAST(sqlc.arg(status) AS TEXT) = '' OR status = sqlc.arg(status))
ORDER BY id DESC LIMIT sqlc.arg(lim);

-- name: CountMailLog :one
SELECT COUNT(*) AS total,
       CAST(COALESCE(SUM(status = 'failed'), 0) AS INTEGER) AS failed,
       CAST(COALESCE(SUM(status = 'queued'), 0) AS INTEGER) AS queued
FROM mail_log;

-- name: PruneMailLog :exec
DELETE FROM mail_log WHERE created_at < ?;
