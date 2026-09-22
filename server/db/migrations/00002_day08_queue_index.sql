-- +goose Up

CREATE INDEX check_runs_pending_enqueue_idx
    ON check_runs (scheduled_at, id)
    WHERE status = 'queued' AND enqueued_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS check_runs_pending_enqueue_idx;
