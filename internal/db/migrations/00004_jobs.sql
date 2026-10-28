-- +goose Up
-- Durable background jobs: one worker claims due rows and runs them, and a
-- scheduler enqueues periodic ones. See internal/jobs and the jobs section
-- of docs/agent-guides/architecture.md.
--
-- Nullable columns are plain TEXT: that's already nullable in SQLite, and
-- the explicit NULL keyword makes sqlc generate interface{} for the column.

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}'
        CHECK (json_valid(payload)),
    status TEXT NOT NULL
        CHECK (status IN ('queued', 'running', 'done', 'failed')),
    -- Set for periodic jobs: at most one queued or running row per key, so
    -- a restart or a slow run can't pile up duplicates. NULL means "not
    -- deduplicated" (NULLs are distinct in a unique index).
    unique_key TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL,
    run_at TEXT NOT NULL
        CHECK (run_at IS strftime('%Y-%m-%dT%H:%M:%SZ', run_at)),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at TEXT NOT NULL
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    started_at TEXT
        CHECK (started_at IS strftime('%Y-%m-%dT%H:%M:%SZ', started_at)),
    finished_at TEXT
        CHECK (finished_at IS strftime('%Y-%m-%dT%H:%M:%SZ', finished_at)),
    last_error TEXT
) STRICT;

-- The claim query takes the oldest due queued job.
CREATE INDEX jobs_status_run_at ON jobs (status, run_at);

-- Only pending rows take part, so a key is free again once its job is done
-- or failed.
CREATE UNIQUE INDEX jobs_unique_key_pending ON jobs (unique_key)
    WHERE status IN ('queued', 'running');
