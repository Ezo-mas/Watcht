// Package jobs runs watcht's background work from a durable queue in
// SQLite. One worker goroutine claims due jobs and runs their registered
// handler; a scheduler enqueues periodic ones. The rules are in the jobs
// section of docs/agent-guides/architecture.md.
//
// A handler must respect the context it's given: shutdown cancels it and
// then waits for the worker, so a handler that ignores it delays the server
// stopping. Never log a payload: later jobs carry provider data.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

const (
	// kindMaintenance is the scheduled job that prunes finished jobs. The
	// package registers and schedules it itself.
	kindMaintenance = "jobs_maintenance"

	defaultMaxAttempts  = 5
	defaultPollInterval = 5 * time.Second
	defaultJobTimeout   = 5 * time.Minute

	// The scheduler compares due times every this often. A scheduled job
	// can therefore start up to this late, which is negligible for
	// intervals measured in hours.
	schedulerTick = 5 * time.Second

	// Retries back off from 30 seconds, doubling, up to an hour.
	backoffBase = 30 * time.Second
	backoffMax  = time.Hour

	maintenanceInterval = 24 * time.Hour
	doneRetention       = 7 * 24 * time.Hour
	failedRetention     = 30 * 24 * time.Hour

	// maxLastError bounds the stored error message, so a handler that
	// returns a large body (an HTML error page, say) can't bloat the row.
	maxLastError = 2000

	// bookkeepingTimeout bounds the update that records a run's outcome.
	// That update uses a context detached from shutdown's cancellation, so
	// an interrupted job can still be requeued.
	bookkeepingTimeout = 10 * time.Second
)

// errUnknownKind fails a job whose kind has no registered handler. It isn't
// retried: no handler will appear while the job runs.
var errUnknownKind = errors.New("unknown job kind")

// Permanent marks an error as not worth retrying: the job fails at once
// instead of using another attempt. Use it for failures a retry can't fix,
// such as a payload that fails validation.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err}
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func isPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}

// Handler runs one job. The context carries the per-job timeout and is
// cancelled when the server shuts down.
type Handler func(ctx context.Context, payload json.RawMessage) error

// EnqueueOptions controls a new job. The zero value queues it now with the
// default attempt limit and no deduplication.
type EnqueueOptions struct {
	// RunAt delays the first attempt. Zero means now.
	RunAt time.Time
	// UniqueKey makes the enqueue a no-op while another job with the same
	// key is queued or running.
	UniqueKey string
	// MaxAttempts is how many runs it may take before the job fails. Zero
	// means the default (5).
	MaxAttempts int
}

// Options configures a Queue. Zero values mean the defaults.
type Options struct {
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
	// Logger receives job outcomes and queue errors. Nil discards them.
	Logger *slog.Logger
	// PollInterval is how often the worker looks for due jobs when nothing
	// wakes it. Zero means 5 seconds.
	PollInterval time.Duration
	// JobTimeout bounds one run. Zero means 5 minutes.
	JobTimeout time.Duration
	// SchedulerTick replaces the scheduler's timer, so tests can fire
	// scheduler checks on demand. Production leaves it nil.
	SchedulerTick <-chan time.Time
}

type schedule struct {
	kind     string
	interval time.Duration
	next     time.Time
}

// Queue is a durable job queue with one worker and a scheduler. Create it
// with New, register handlers and schedules, then call Start; call Wait
// after cancelling the context to let a running job finish.
type Queue struct {
	write     *sqlc.Queries
	handlers  map[string]Handler
	schedules []schedule

	now          func() time.Time
	logger       *slog.Logger
	pollInterval time.Duration
	jobTimeout   time.Duration
	tick         <-chan time.Time

	wake chan struct{}
	wg   sync.WaitGroup

	// orphaned is set by the worker when a run's outcome couldn't be
	// recorded, leaving its row running; the worker recovers it before the
	// next claim. Only the worker goroutine touches it.
	orphaned bool

	started bool
}

// New returns a Queue on d. It registers the internal maintenance handler
// and schedules it; the maintenance job prunes finished jobs daily.
func New(d *db.DB, opts Options) *Queue {
	q := &Queue{
		write:        sqlc.New(d.Write),
		handlers:     make(map[string]Handler),
		now:          opts.Now,
		logger:       opts.Logger,
		pollInterval: opts.PollInterval,
		jobTimeout:   opts.JobTimeout,
		tick:         opts.SchedulerTick,
		wake:         make(chan struct{}, 1),
	}
	if q.now == nil {
		q.now = time.Now
	}
	if q.logger == nil {
		q.logger = slog.New(slog.DiscardHandler)
	}
	if q.pollInterval <= 0 {
		q.pollInterval = defaultPollInterval
	}
	if q.jobTimeout <= 0 {
		q.jobTimeout = defaultJobTimeout
	}
	q.Register(kindMaintenance, q.prune)
	q.Every(kindMaintenance, maintenanceInterval)
	return q
}

// Register adds the handler for kind. Call it before Start; it panics on an
// empty kind, a duplicate, or after Start, because all three are wiring bugs.
func (q *Queue) Register(kind string, h Handler) {
	switch {
	case q.started:
		panic("jobs: Register after Start")
	case kind == "":
		panic("jobs: Register with an empty kind")
	case h == nil:
		panic("jobs: Register(" + kind + ") with a nil handler")
	}
	if _, dup := q.handlers[kind]; dup {
		panic("jobs: duplicate handler for " + kind)
	}
	q.handlers[kind] = h
}

// Every schedules kind to run at startup and then once per interval. It
// panics unless kind has a handler; like Register, call it before Start.
// Scheduled enqueues use the kind as their unique_key, so a restart or a run
// slower than the interval never leaves two pending copies.
func (q *Queue) Every(kind string, interval time.Duration) {
	switch {
	case q.started:
		panic("jobs: Every after Start")
	case interval <= 0:
		panic("jobs: Every(" + kind + ") with a non-positive interval")
	}
	if _, ok := q.handlers[kind]; !ok {
		panic("jobs: Every(" + kind + ") has no registered handler")
	}
	for _, s := range q.schedules {
		if s.kind == kind {
			panic("jobs: Every(" + kind + ") scheduled twice")
		}
	}
	q.schedules = append(q.schedules, schedule{kind: kind, interval: interval})
}

// Enqueue adds a job and wakes the worker. payload is marshalled as JSON;
// nil stores an empty object. With a unique key that is already queued or
// running, it does nothing and returns nil.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any, opts EnqueueOptions) error {
	if kind == "" {
		return errors.New("jobs: enqueue with an empty kind")
	}
	if opts.MaxAttempts < 0 {
		return fmt.Errorf("jobs: enqueue %s: negative max attempts %d", kind, opts.MaxAttempts)
	}
	p, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload for %s job: %w", kind, err)
	}
	if payload == nil {
		p = []byte("{}")
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	now := q.now()
	runAt := opts.RunAt
	if runAt.IsZero() {
		runAt = now
	}
	key := sql.NullString{String: opts.UniqueKey, Valid: opts.UniqueKey != ""}
	n, err := q.write.EnqueueJob(ctx, sqlc.EnqueueJobParams{
		Kind:        kind,
		Payload:     string(p),
		UniqueKey:   key,
		MaxAttempts: int64(maxAttempts),
		RunAt:       db.Timestamp(runAt),
		Now:         db.Timestamp(now),
	})
	if err != nil {
		return fmt.Errorf("enqueue %s job: %w", kind, err)
	}
	if n > 0 {
		q.logger.Debug("job enqueued", "kind", kind, "run_at", db.Timestamp(runAt))
		q.signal()
	}
	return nil
}

// Start recovers jobs left running by a crash, enqueues the scheduled jobs
// for their first run, and starts the worker and scheduler goroutines. They
// stop when ctx is cancelled; Wait then returns once they have.
func (q *Queue) Start(ctx context.Context) error {
	if q.started {
		return errors.New("jobs: already started")
	}
	q.started = true
	if err := q.recover(ctx); err != nil {
		return fmt.Errorf("recover jobs left running: %w", err)
	}
	now := q.now()
	for i := range q.schedules {
		q.schedules[i].next = now
	}
	if err := q.enqueueScheduled(ctx, now); err != nil {
		return err
	}
	q.wg.Add(2)
	go q.runWorker(ctx)
	go q.runScheduler(ctx)
	return nil
}

// Wait blocks until the worker and scheduler have stopped, which happens
// after ctx passed to Start is cancelled and any running job has returned.
func (q *Queue) Wait() {
	q.wg.Wait()
}

// recover deals with jobs left running with no worker behind them: by a
// previous process that died, or by an outcome write that failed. Their
// attempt stays counted, since it may have failed partway through, and a job
// whose last attempt it was fails instead of running again. It's only safe
// while this queue's worker isn't running a job.
func (q *Queue) recover(ctx context.Context) error {
	now := db.Timestamp(q.now())
	failed, err := q.write.FailExhaustedRunningJobs(ctx, now)
	if err != nil {
		return fmt.Errorf("fail exhausted running jobs: %w", err)
	}
	if failed > 0 {
		q.logger.Error("failed jobs left running on their last attempt", "count", failed)
	}
	n, err := q.write.RequeueRunningJobs(ctx, now)
	if err != nil {
		return fmt.Errorf("requeue running jobs: %w", err)
	}
	if n > 0 {
		q.logger.Warn("requeued jobs left running", "count", n)
	}
	return nil
}

// prune is the maintenance job: it deletes finished jobs past their
// retention. Queued, running, and recently finished jobs stay.
func (q *Queue) prune(ctx context.Context, _ json.RawMessage) error {
	now := q.now()
	n, err := q.write.DeleteFinishedJobs(ctx, sqlc.DeleteFinishedJobsParams{
		DoneBefore:   db.Timestamp(now.Add(-doneRetention)),
		FailedBefore: db.Timestamp(now.Add(-failedRetention)),
	})
	if err != nil {
		return fmt.Errorf("delete finished jobs: %w", err)
	}
	if n > 0 {
		q.logger.Info("pruned finished jobs", "count", n)
	}
	return nil
}

// backoff returns the delay before the next attempt after attempt failed
// runs: 30 seconds, doubling, capped at an hour.
func backoff(attempt int64) time.Duration {
	d := backoffBase
	for i := int64(1); i < attempt && d < backoffMax; i++ {
		d *= 2
	}
	if d > backoffMax {
		d = backoffMax
	}
	return d
}
