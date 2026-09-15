-- name: CreateIncident :one
INSERT INTO incidents (id, monitor_id, opened_at, error_code, error_summary)
VALUES (sqlc.arg(id), sqlc.arg(monitor_id), sqlc.arg(opened_at), sqlc.arg(error_code), sqlc.arg(error_summary))
RETURNING id, monitor_id, opened_at, resolved_at, error_code, error_summary,
    created_at, updated_at;

-- name: CountIncidentsByMonitorAndUser :one
SELECT count(*)::bigint
FROM incidents i
JOIN monitors m ON m.id = i.monitor_id
WHERE i.monitor_id = sqlc.arg(monitor_id)
  AND m.user_id = sqlc.arg(user_id)
  AND m.deleted_at IS NULL;

-- name: ListIncidentsByMonitorAndUser :many
SELECT i.id, i.monitor_id, i.opened_at, i.resolved_at, i.error_code,
    i.error_summary, i.created_at, i.updated_at
FROM incidents i
JOIN monitors m ON m.id = i.monitor_id
WHERE i.monitor_id = sqlc.arg(monitor_id)
  AND m.user_id = sqlc.arg(user_id)
  AND m.deleted_at IS NULL
ORDER BY i.opened_at DESC, i.id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
