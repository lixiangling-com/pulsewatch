-- name: ListDueMonitorsForUpdate :many
SELECT id, config_version, interval_minutes, next_check_at
FROM monitors
WHERE deleted_at IS NULL
  AND status <> 'paused'
  AND next_check_at <= now()
ORDER BY next_check_at, id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: AdvanceMonitorNextCheck :execrows
UPDATE monitors
SET next_check_at = sqlc.arg(next_check_at), updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND status <> 'paused';

-- name: ListPendingCheckRunIDs :many
SELECT id
FROM check_runs
WHERE status = 'queued' AND enqueued_at IS NULL
ORDER BY scheduled_at, id
LIMIT sqlc.arg(batch_size);

-- name: MarkCheckRunEnqueued :execrows
UPDATE check_runs
SET enqueued_at = COALESCE(enqueued_at, now())
WHERE id = sqlc.arg(id) AND status = 'queued';

-- name: GetCheckRunForProcessing :one
SELECT r.id, r.status, r.config_version,
       m.status AS monitor_status, m.config_version AS monitor_config_version,
       m.deleted_at AS monitor_deleted_at
FROM check_runs r
JOIN monitors m ON m.id = r.monitor_id
WHERE r.id = sqlc.arg(id)
FOR UPDATE OF r, m;

-- name: MarkCheckRunRunning :execrows
UPDATE check_runs
SET status = 'running', started_at = COALESCE(started_at, now())
WHERE id = sqlc.arg(id) AND status = 'queued';

-- name: CompleteCheckRun :execrows
UPDATE check_runs
SET status = sqlc.arg(status),
    started_at = COALESCE(started_at, now()),
    finished_at = now(),
    error_code = sqlc.narg(error_code),
    error_summary = sqlc.narg(error_summary)
WHERE id = sqlc.arg(id)
  AND status IN ('queued', 'running');
