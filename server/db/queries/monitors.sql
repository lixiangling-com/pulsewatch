-- name: CreateMonitor :one
INSERT INTO monitors (
    id, user_id, name, url, interval_minutes, expected_status
)
VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(url),
    sqlc.arg(interval_minutes), sqlc.arg(expected_status)
)
RETURNING id, user_id, name, url, interval_minutes, expected_status, status,
    config_version, next_check_at, last_checked_at, last_latency_ms,
    created_at, updated_at, deleted_at;

-- name: LockUserForMonitorCreate :one
SELECT id
FROM users
WHERE id = sqlc.arg(user_id)
FOR UPDATE;

-- name: GetMonitorByIDAndUser :one
SELECT id, user_id, name, url, interval_minutes, expected_status, status,
    config_version, next_check_at, last_checked_at, last_latency_ms,
    created_at, updated_at, deleted_at
FROM monitors
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL;

-- name: GetMonitorByIDAndUserForUpdate :one
SELECT id, user_id, name, url, interval_minutes, expected_status, status,
    config_version, next_check_at, last_checked_at, last_latency_ms,
    created_at, updated_at, deleted_at
FROM monitors
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL
FOR UPDATE;

-- name: CountMonitorsByUser :one
SELECT count(*)::bigint
FROM monitors
WHERE user_id = sqlc.arg(user_id) AND deleted_at IS NULL;

-- name: ListMonitorsByUser :many
SELECT id, user_id, name, url, interval_minutes, expected_status, status,
    config_version, next_check_at, last_checked_at, last_latency_ms,
    created_at, updated_at, deleted_at
FROM monitors
WHERE user_id = sqlc.arg(user_id) AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: UpdateMonitor :one
UPDATE monitors
SET name = sqlc.arg(name),
    url = sqlc.arg(url),
    interval_minutes = sqlc.arg(interval_minutes),
    expected_status = sqlc.arg(expected_status),
    status = sqlc.arg(status),
    config_version = sqlc.arg(config_version),
    next_check_at = sqlc.arg(next_check_at),
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL
RETURNING id, user_id, name, url, interval_minutes, expected_status, status,
    config_version, next_check_at, last_checked_at, last_latency_ms,
    created_at, updated_at, deleted_at;

-- name: SoftDeleteMonitor :execrows
UPDATE monitors
SET deleted_at = COALESCE(deleted_at, now()),
    config_version = config_version + 1,
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL;
