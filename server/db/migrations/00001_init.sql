-- +goose Up

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 320),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id, created_at DESC);
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens (expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE monitors (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    url text NOT NULL CHECK (
        char_length(url) BETWEEN 1 AND 2048
        AND url ~ '^https?://[^[:space:]]+$'
    ),
    interval_minutes integer NOT NULL DEFAULT 5
        CHECK (interval_minutes IN (1, 5, 10)),
    expected_status integer NOT NULL DEFAULT 200
        CHECK (expected_status BETWEEN 100 AND 599),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'up', 'confirming_down', 'down', 'confirming_up', 'paused')),
    config_version integer NOT NULL DEFAULT 1 CHECK (config_version >= 1),
    next_check_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz,
    last_latency_ms bigint CHECK (last_latency_ms IS NULL OR last_latency_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX monitors_user_list_idx
    ON monitors (user_id, deleted_at, created_at DESC);
CREATE INDEX monitors_due_idx
    ON monitors (next_check_at)
    WHERE deleted_at IS NULL AND status <> 'paused';

CREATE TABLE check_runs (
    id uuid PRIMARY KEY,
    monitor_id uuid NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    config_version integer NOT NULL CHECK (config_version >= 1),
    status text NOT NULL
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    scheduled_at timestamptz NOT NULL,
    enqueued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    status_code integer CHECK (status_code IS NULL OR status_code BETWEEN 100 AND 599),
    latency_ms bigint CHECK (latency_ms IS NULL OR latency_ms >= 0),
    error_code text,
    error_summary text CHECK (error_summary IS NULL OR char_length(error_summary) <= 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT check_runs_monitor_schedule_unique UNIQUE (monitor_id, scheduled_at)
);

CREATE INDEX check_runs_monitor_history_idx
    ON check_runs (monitor_id, scheduled_at DESC);

CREATE TABLE incidents (
    id uuid PRIMARY KEY,
    monitor_id uuid NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    opened_at timestamptz NOT NULL,
    resolved_at timestamptz,
    error_code text NOT NULL,
    error_summary text CHECK (error_summary IS NULL OR char_length(error_summary) <= 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX incidents_monitor_history_idx
    ON incidents (monitor_id, opened_at DESC);
CREATE UNIQUE INDEX one_open_incident_per_monitor
    ON incidents (monitor_id)
    WHERE resolved_at IS NULL;

CREATE TABLE notifications (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    monitor_id uuid NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    incident_id uuid NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('incident_opened', 'incident_resolved')),
    dedupe_key text NOT NULL UNIQUE,
    title text NOT NULL,
    body text NOT NULL,
    read_at timestamptz,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'queued', 'sent', 'failed')),
    queued_at timestamptz,
    sent_at timestamptz,
    error_code text,
    error_summary text CHECK (error_summary IS NULL OR char_length(error_summary) <= 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_history_idx
    ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_monitor_idx ON notifications (monitor_id, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS check_runs;
DROP TABLE IF EXISTS monitors;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
