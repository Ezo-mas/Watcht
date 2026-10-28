package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// runWorker claims and runs due jobs until ctx is cancelled. It wakes when
// Enqueue adds a job and otherwise looks again every pollInterval.
func (q *Queue) runWorker(ctx context.Context) {
	defer q.wg.Done()
	for ctx.Err() == nil {
		if q.orphaned {
			if err := q.recover(ctx); err != nil {
				q.logger.Error("job worker", "err", err)
				if !q.wait(ctx, q.pollInterval) {
					return
				}
				continue
			}
			q.orphaned = false
		}
		ran, err := q.runOne(ctx)
		if err != nil {
			// A job that ran but whose outcome wasn't recorded is still
			// running in the table and would hold its unique_key forever.
			q.orphaned = ran
			q.logger.Error("job worker", "err", err)
			if !q.wait(ctx, q.pollInterval) {
				return
			}
			continue
		}
		if ran {
			continue // maybe more are due
		}
		if !q.wait(ctx, q.pollInterval) {
			return
		}
	}
}

// wait sleeps for d, or until Enqueue adds a job or ctx is cancelled. It
// returns false when ctx is done.
func (q *Queue) wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-q.wake:
	case <-t.C:
	}
	return true
}

// signal wakes the worker if it's waiting. The buffered channel means a
// wake that arrives while the worker is busy isn't lost.
func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// runOne claims and runs at most one job, reporting whether it ran one.
// Handler failures are recorded on the job and aren't returned; an error
// here means the queue itself (the database) failed.
func (q *Queue) runOne(ctx context.Context) (bool, error) {
	if ctx.Err() != nil {
		return false, nil
	}
	// The claim must not be cut short by shutdown: a cancelled call can
	// still commit, which would leave the row running with nothing to run it.
	claimCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
	defer cancel()
	start := time.Now()
	row, err := q.write.ClaimJob(claimCtx, db.Timestamp(q.now()))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim job: %w", err)
	}
	runErr := q.run(ctx, row)
	if err := q.record(ctx, row, runErr, time.Since(start)); err != nil {
		return true, err
	}
	return true, nil
}

// run executes one claimed job's handler with the per-job timeout, turning a
// panic into an error.
func (q *Queue) run(ctx context.Context, row sqlc.ClaimJobRow) error {
	h, ok := q.handlers[row.Kind]
	if !ok {
		return fmt.Errorf("%w %q", errUnknownKind, row.Kind)
	}
	jobCtx, cancel := context.WithTimeout(ctx, q.jobTimeout)
	defer cancel()
	return invoke(jobCtx, h, json.RawMessage(row.Payload))
}

func invoke(ctx context.Context, h Handler, payload json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return h(ctx, payload)
}

// record stores a run's outcome and logs it. It never uses the worker's
// context, so a job interrupted by shutdown is still requeued: the worker's
// context is cancelled before Wait returns.
func (q *Queue) record(workerCtx context.Context, row sqlc.ClaimJobRow, runErr error, elapsed time.Duration) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(workerCtx), bookkeepingTimeout)
	defer cancel()

	now := q.now()
	fields := []any{"kind", row.Kind, "id", row.ID, "attempt", row.Attempts, "duration", elapsed}
	switch {
	case runErr == nil:
		if err := q.write.FinishJob(ctx, sqlc.FinishJobParams{Now: db.Timestamp(now), ID: row.ID}); err != nil {
			return fmt.Errorf("finish job %d: %w", row.ID, err)
		}
		q.logger.Info("job done", fields...)
		return nil

	case workerCtx.Err() != nil:
		// Shutdown cut the run short. Put the job back without the attempt
		// the claim added, so a restart gets a full set of attempts.
		if err := q.write.InterruptJob(ctx, sqlc.InterruptJobParams{Now: db.Timestamp(now), ID: row.ID}); err != nil {
			return fmt.Errorf("requeue interrupted job %d: %w", row.ID, err)
		}
		q.logger.Warn("job interrupted by shutdown; requeued", fields...)
		return nil

	default:
		msg := truncateMessage(runErr)
		if errors.Is(runErr, errUnknownKind) || isPermanent(runErr) || row.Attempts >= row.MaxAttempts {
			if err := q.write.FailJob(ctx, sqlc.FailJobParams{
				LastError: msg,
				Now:       db.Timestamp(now),
				ID:        row.ID,
			}); err != nil {
				return fmt.Errorf("fail job %d: %w", row.ID, err)
			}
			q.logger.Error("job failed", append(fields, "err", runErr)...)
			return nil
		}
		next := now.Add(backoff(row.Attempts))
		if err := q.write.RetryJob(ctx, sqlc.RetryJobParams{
			RunAt:     db.Timestamp(next),
			LastError: msg,
			Now:       db.Timestamp(now),
			ID:        row.ID,
		}); err != nil {
			return fmt.Errorf("retry job %d: %w", row.ID, err)
		}
		q.logger.Warn("job failed; will retry", append(fields, "run_at", db.Timestamp(next), "err", runErr)...)
		return nil
	}
}

// truncateMessage bounds last_error, cutting on a rune boundary so the
// stored text stays valid UTF-8.
func truncateMessage(err error) string {
	msg := err.Error()
	if len(msg) <= maxLastError {
		return msg
	}
	cut := maxLastError
	for cut > 0 && !utf8.RuneStart(msg[cut]) {
		cut--
	}
	return msg[:cut]
}
