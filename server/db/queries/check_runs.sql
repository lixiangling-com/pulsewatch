-- name: CreateCheckRun :one
INSERT INTO check_runs (id, monitor_id, config_version, status, scheduled_at)
VALUES (sqlc.arg(id), sqlc.arg(monitor_id), sqlc.arg(config_version), sqlc.arg(status), sqlc.arg(scheduled_at))
RETURNING id, monitor_id, config_version, status, scheduled_at, enqueued_at,
    started_at, finished_at, status_code, latency_ms, error_code, error_summary, created_at;

-- name: GetCheckRunByIDAndUser :one
SELECT r.id, r.monitor_id, r.config_version, r.status, r.scheduled_at, r.enqueued_at,
    r.started_at, r.finished_at, r.status_code, r.latency_ms, r.error_code,
    r.error_summary, r.created_at
FROM check_runs r
JOIN monitors m ON m.id = r.monitor_id
WHERE r.id = sqlc.arg(id) AND m.user_id = sqlc.arg(user_id) AND m.deleted_at IS NULL;

-- name: CountCheckRunsByMonitorAndUser :one
SELECT count(*)::bigint
FROM check_runs r
JOIN monitors m ON m.id = r.monitor_id
WHERE r.monitor_id = sqlc.arg(monitor_id)
  AND m.user_id = sqlc.arg(user_id)
  AND m.deleted_at IS NULL;

-- name: ListCheckRunsByMonitorAndUser :many
SELECT r.id, r.monitor_id, r.config_version, r.status, r.scheduled_at, r.enqueued_at,
    r.started_at, r.finished_at, r.status_code, r.latency_ms, r.error_code,
    r.error_summary, r.created_at
FROM check_runs r
JOIN monitors m ON m.id = r.monitor_id
WHERE r.monitor_id = sqlc.arg(monitor_id)
  AND m.user_id = sqlc.arg(user_id)
  AND m.deleted_at IS NULL
ORDER BY r.scheduled_at DESC, r.id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
