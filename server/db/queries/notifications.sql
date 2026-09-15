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
