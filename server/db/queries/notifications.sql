-- name: CreateNotification :one
INSERT INTO notifications (
    id, user_id, monitor_id, incident_id, type, dedupe_key, title, body
)
VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(monitor_id), sqlc.arg(incident_id),
    sqlc.arg(type), sqlc.arg(dedupe_key), sqlc.arg(title), sqlc.arg(body)
)
RETURNING id, user_id, monitor_id, incident_id, type, dedupe_key, title, body,
    read_at, status, queued_at, sent_at, error_code, error_summary,
    created_at, updated_at;

-- name: CountNotificationsByUser :one
SELECT count(*)::bigint FROM notifications WHERE user_id = sqlc.arg(user_id);

-- name: ListNotificationsByUser :many
SELECT id, user_id, monitor_id, incident_id, type, dedupe_key, title, body,
    read_at, status, queued_at, sent_at, error_code, error_summary,
    created_at, updated_at
FROM notifications
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: MarkNotificationReadByUser :one
UPDATE notifications
SET read_at = COALESCE(read_at, now()), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING id, user_id, monitor_id, incident_id, type, dedupe_key, title, body,
    read_at, status, queued_at, sent_at, error_code, error_summary,
    created_at, updated_at;

-- name: ListPendingEmailNotificationIDs :many
SELECT id
FROM notifications
WHERE status IN ('pending', 'failed') AND queued_at IS NULL
ORDER BY created_at, id
LIMIT sqlc.arg(batch_size);

-- name: MarkNotificationQueued :execrows
UPDATE notifications
SET status = 'queued', queued_at = COALESCE(queued_at, now()), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending' AND error_code IS NULL;

-- name: PrepareNotificationRetry :execrows
UPDATE notifications
SET status = 'pending', error_code = NULL, error_summary = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'failed' AND queued_at IS NULL;

-- name: GetNotificationForEmail :one
SELECT n.id, n.user_id, n.monitor_id, n.incident_id, n.type, n.dedupe_key,
    n.title, n.body, n.read_at, n.status, n.queued_at, n.sent_at,
    n.error_code, n.error_summary, n.created_at, n.updated_at, u.email,
    m.name AS monitor_name
FROM notifications n
JOIN users u ON u.id = n.user_id
JOIN monitors m ON m.id = n.monitor_id
WHERE n.id = sqlc.arg(id);

-- name: MarkNotificationSent :execrows
UPDATE notifications
SET status = 'sent', sent_at = now(), error_code = NULL, error_summary = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status <> 'sent';

-- name: MarkNotificationFailed :execrows
UPDATE notifications
SET status = 'failed', queued_at = NULL,
    error_code = sqlc.arg(error_code), error_summary = sqlc.arg(error_summary), updated_at = now()
WHERE id = sqlc.arg(id) AND status <> 'sent';
