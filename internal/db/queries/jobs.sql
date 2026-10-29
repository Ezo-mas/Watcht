-- The jobs queue. The worker claims the oldest due job with one
-- UPDATE ... RETURNING on the write pool, so a job can never be claimed
-- twice. Internal/jobs owns the rules (retry, recovery, retention); these
-- queries are the single state transitions.

-- EnqueueJob ignores a second pending row with the same unique_key: the
-- partial unique index only covers queued and running jobs, so ON CONFLICT
-- applies exactly while one is pending. It returns the number of rows
-- inserted (0 or 1).
-- name: EnqueueJob :execrows
INSERT INTO jobs (
    kind, payload, status, unique_key, attempts, max_attempts,
    run_at, created_at, updated_at
) VALUES (
    sqlc.arg(kind),
    sqlc.arg(payload),
    'queued',
    sqlc.narg(unique_key),
    0,
    sqlc.arg(max_attempts),
    sqlc.arg(run_at),
    sqlc.arg(now),
    sqlc.arg(now)
)
ON CONFLICT DO NOTHING;

-- ClaimJob starts the oldest queued job that is due. Attempts counts runs,
-- including the one that just started.
-- name: ClaimJob :one
UPDATE jobs
SET status = 'running',
    attempts = attempts + 1,
    started_at = CAST(sqlc.arg(now) AS TEXT),
    updated_at = sqlc.arg(now)
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'queued'
        AND run_at <= sqlc.arg(now)
    ORDER BY run_at, id
    LIMIT 1
)
RETURNING id, kind, payload, attempts, max_attempts;

-- The outcome updates (FinishJob, RetryJob, FailJob) only change a row
-- that is still running, so a late write can't overwrite a newer state.

-- FinishJob records a successful run. A previous failure's message is no
-- longer true, so it's cleared.
-- name: FinishJob :exec
UPDATE jobs
SET status = 'done',
    last_error = NULL,
    updated_at = sqlc.arg(now),
    finished_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'running';

-- RetryJob puts a failed run back in the queue for its next attempt.
-- name: RetryJob :exec
UPDATE jobs
SET status = 'queued',
    run_at = sqlc.arg(run_at),
    last_error = CAST(sqlc.arg(last_error) AS TEXT),
    updated_at = sqlc.arg(now),
    started_at = NULL,
    finished_at = NULL
WHERE id = sqlc.arg(id) AND status = 'running';

-- FailJob ends a job that can't succeed or has used its last attempt.
-- name: FailJob :exec
UPDATE jobs
SET    status = 'failed',
    last_error = CAST(sqlc.arg(last_error) AS TEXT),
    updated_at = sqlc.arg(now),
    finished_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'running';

-- InterruptJob returns a job whose run was cut short by shutdown to the
-- queue without the attempt the claim added.
-- name: InterruptJob :exec
UPDATE jobs
SET status = 'queued',
    attempts = attempts - 1,
    started_at = NULL,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'running';

-- FailExhaustedRunningJobs fails jobs left running whose last attempt was
-- the one cut short. Without it a job that kills the process (out of memory,
-- a fatal runtime error) would be requeued at every start and crash-loop the
-- server. Run it before RequeueRunningJobs.
-- name: FailExhaustedRunningJobs :execrows
UPDATE jobs
SET status = 'failed',
    last_error = 'stopped during its last attempt (process exited or the outcome was not recorded)',
    updated_at = sqlc.arg(now),
    finished_at = sqlc.arg(now)
WHERE status = 'running' AND attempts >= max_attempts;

-- RequeueRunningJobs returns jobs left running by a crash (or whose outcome
-- couldn't be recorded) to the queue. The attempt stays counted: it may have
-- failed partway through.
-- name: RequeueRunningJobs :execrows
UPDATE jobs
SET status = 'queued',
    started_at = NULL,
    updated_at = sqlc.arg(now)
WHERE status = 'running';

-- DeleteFinishedJobs prunes finished jobs past their retention. Queued,
-- running, and unfinished rows are never touched.
-- name: DeleteFinishedJobs :execrows
DELETE FROM jobs
WHERE (status = 'done' AND finished_at <= CAST(sqlc.arg(done_before) AS TEXT))
   OR (status = 'failed' AND finished_at <= CAST(sqlc.arg(failed_before) AS TEXT));
