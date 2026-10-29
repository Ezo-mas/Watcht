package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

// testNow is a whole second, like every stored timestamp.
var testNow = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

// fakeClock lets a test move time without sleeping. The mutex matters for
// tests that run the worker goroutine.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newQueue(t *testing.T) (*Queue, *db.DB, *fakeClock) {
	t.Helper()
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	q := New(d, Options{
		Now:          clock.Now,
		PollInterval: 10 * time.Millisecond,
		JobTimeout:   2 * time.Second,
	})
	return q, d, clock
}

func mustEnqueue(t *testing.T, q *Queue, ctx context.Context, kind string, payload any, opts EnqueueOptions) {
	t.Helper()
	if err := q.Enqueue(ctx, kind, payload, opts); err != nil {
		t.Fatalf("Enqueue(%s): %v", kind, err)
	}
}

func mustRunOne(t *testing.T, q *Queue, ctx context.Context) {
	t.Helper()
	ran, err := q.runOne(ctx)
	if err != nil {
		t.Fatalf("runOne: %v", err)
	}
	if !ran {
		t.Fatal("runOne claimed no job, want one")
	}
}

// job is the row parts the tests assert on.
type job struct {
	id          int64
	kind        string
	status      string
	attempts    int64
	maxAttempts int64
	runAt       string
	startedAt   sql.NullString
	finishedAt  sql.NullString
	lastError   sql.NullString
}

func getJob(t *testing.T, d *db.DB, id int64) job {
	t.Helper()
	var j job
	err := d.Read.QueryRowContext(context.Background(), `
		SELECT id, kind, status, attempts, max_attempts, run_at,
		       started_at, finished_at, last_error
		FROM jobs WHERE id = ?`, id).
		Scan(&j.id, &j.kind, &j.status, &j.attempts, &j.maxAttempts,
			&j.runAt, &j.startedAt, &j.finishedAt, &j.lastError)
	if err != nil {
		t.Fatalf("read job %d: %v", id, err)
	}
	return j
}

// lastJobID returns the newest job, for tests that enqueued one at a time.
func lastJobID(t *testing.T, d *db.DB) int64 {
	t.Helper()
	var id int64
	if err := d.Read.QueryRowContext(context.Background(), "SELECT max(id) FROM jobs").Scan(&id); err != nil {
		t.Fatalf("read newest job: %v", err)
	}
	return id
}

// countJobs counts jobs matching a WHERE condition, such as "kind = ?".
func countJobs(t *testing.T, d *db.DB, where string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM jobs WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatalf("count jobs where %s: %v", where, err)
	}
	return n
}

// waitFor polls f until it's true, so a test never waits longer than needed.
func waitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestEnqueuePayloadAndDefaults(t *testing.T) {
	q, d, clock := newQueue(t)
	ctx := context.Background()
	var got string
	q.Register("echo", func(_ context.Context, payload json.RawMessage) error {
		got = string(payload)
		return nil
	})

	mustEnqueue(t, q, ctx, "echo", map[string]int64{"movie": 603}, EnqueueOptions{})
	id := lastJobID(t, d)
	j := getJob(t, d, id)
	if j.status != "queued" || j.attempts != 0 || j.maxAttempts != defaultMaxAttempts {
		t.Errorf("new job = %+v, want queued with 0/%d attempts", j, defaultMaxAttempts)
	}
	if j.runAt != db.Timestamp(clock.Now()) {
		t.Errorf("run_at = %q, want now (%q)", j.runAt, db.Timestamp(clock.Now()))
	}
	if j.startedAt.Valid || j.finishedAt.Valid || j.lastError.Valid {
		t.Errorf("new job has started_at %v, finished_at %v, last_error %v; want NULL", j.startedAt, j.finishedAt, j.lastError)
	}

	mustRunOne(t, q, ctx)
	if got != `{"movie":603}` {
		t.Errorf("handler payload = %q, want the enqueued JSON", got)
	}
	if j := getJob(t, d, id); j.status != "done" || !j.finishedAt.Valid {
		t.Errorf("after success = %+v, want done with finished_at", j)
	}

	// A nil payload is the schema's empty object, not JSON null.
	mustEnqueue(t, q, ctx, "echo", nil, EnqueueOptions{})
	mustRunOne(t, q, ctx)
	if got != "{}" {
		t.Errorf("nil payload reached the handler as %q, want {}", got)
	}
}

func TestUniqueKeyDedupes(t *testing.T) {
	ctx := context.Background()
	opts := EnqueueOptions{UniqueKey: "same"}

	t.Run("while queued", func(t *testing.T) {
		q, d, _ := newQueue(t)
		q.Register("k", func(context.Context, json.RawMessage) error { return nil })
		mustEnqueue(t, q, ctx, "k", nil, opts)
		mustEnqueue(t, q, ctx, "k", nil, opts)
		if n := countJobs(t, d, "kind = ?", "k"); n != 1 {
			t.Errorf("%d jobs enqueued with the same key, want 1", n)
		}
	})

	t.Run("while running", func(t *testing.T) {
		q, d, clock := newQueue(t)
		mustEnqueue(t, q, ctx, "k", nil, opts)
		if _, err := q.write.ClaimJob(ctx, db.Timestamp(clock.Now())); err != nil {
			t.Fatal(err)
		}
		mustEnqueue(t, q, ctx, "k", nil, opts)
		if n := countJobs(t, d, "kind = ?", "k"); n != 1 {
			t.Errorf("%d jobs after enqueueing while running, want 1", n)
		}
	})

	t.Run("after done", func(t *testing.T) {
		q, d, _ := newQueue(t)
		q.Register("k", func(context.Context, json.RawMessage) error { return nil })
		mustEnqueue(t, q, ctx, "k", nil, opts)
		mustRunOne(t, q, ctx)
		if j := getJob(t, d, lastJobID(t, d)); j.status != "done" {
			t.Fatalf("first job = %s, want done", j.status)
		}
		// The key is free again once the previous job is finished.
		mustEnqueue(t, q, ctx, "k", nil, opts)
		if n := countJobs(t, d, "kind = ?", "k"); n != 2 {
			t.Errorf("%d jobs after the first finished, want 2", n)
		}
	})

	t.Run("after failed", func(t *testing.T) {
		q, d, _ := newQueue(t)
		q.Register("k", func(context.Context, json.RawMessage) error { return errors.New("failed") })
		// One attempt, so the first failure is final.
		mustEnqueue(t, q, ctx, "k", nil, EnqueueOptions{UniqueKey: "same", MaxAttempts: 1})
		mustRunOne(t, q, ctx)
		if j := getJob(t, d, lastJobID(t, d)); j.status != "failed" {
			t.Fatalf("first job = %s, want failed", j.status)
		}
		mustEnqueue(t, q, ctx, "k", nil, EnqueueOptions{UniqueKey: "same", MaxAttempts: 1})
		if n := countJobs(t, d, "kind = ?", "k"); n != 2 {
			t.Errorf("%d jobs after the first failed, want 2", n)
		}
	})

	t.Run("null keys are not deduplicated", func(t *testing.T) {
		q, d, _ := newQueue(t)
		q.Register("plain", func(context.Context, json.RawMessage) error { return nil })
		mustEnqueue(t, q, ctx, "plain", nil, EnqueueOptions{})
		mustEnqueue(t, q, ctx, "plain", nil, EnqueueOptions{})
		if n := countJobs(t, d, "kind = ?", "plain"); n != 2 {
			t.Errorf("%d jobs without a key, want 2", n)
		}
	})
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempt int64
		want    time.Duration
	}{
		{1, 30 * time.Second},
		{2, time.Minute},
		{3, 2 * time.Minute},
		{4, 4 * time.Minute},
		{5, 8 * time.Minute},
		{6, 16 * time.Minute},
		{7, 32 * time.Minute},
		{8, time.Hour}, // 30s doubling would pass the cap
		{9, time.Hour},
		{100, time.Hour},
	}
	for _, tt := range tests {
		if got := backoff(tt.attempt); got != tt.want {
			t.Errorf("backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestEnqueueValidation(t *testing.T) {
	q, d, _ := newQueue(t)
	ctx := context.Background()
	if err := q.Enqueue(ctx, "", nil, EnqueueOptions{}); err == nil {
		t.Error("Enqueue with an empty kind = nil, want an error")
	}
	if err := q.Enqueue(ctx, "k", nil, EnqueueOptions{MaxAttempts: -1}); err == nil {
		t.Error("Enqueue with negative max attempts = nil, want an error")
	}
	if err := q.Enqueue(ctx, "k", make(chan int), EnqueueOptions{}); err == nil {
		t.Error("Enqueue with an unmarshalable payload = nil, want an error")
	}
	if n := countJobs(t, d, "1 = 1"); n != 0 {
		t.Errorf("%d jobs stored by rejected enqueues, want 0", n)
	}
}

func TestMaintenancePrunesOnlyOldFinishedJobs(t *testing.T) {
	q, d, clock := newQueue(t)
	ctx := context.Background()
	q.Register("ok", func(context.Context, json.RawMessage) error { return nil })
	q.Register("bad", func(context.Context, json.RawMessage) error { return errors.New("nope") })

	// Each job is finished at the time it's enqueued, so finished_at is set
	// exactly at these times.
	finished := []struct {
		name   string
		at     time.Time
		failed bool
		pruned bool
	}{
		{"done-edge", testNow.Add(-doneRetention), false, true}, // exactly 7 days
		{"done-old", testNow.Add(-8 * 24 * time.Hour), false, true},
		{"done-new", testNow.Add(-6 * 24 * time.Hour), false, false},
		{"failed-edge", testNow.Add(-failedRetention), true, true}, // exactly 30 days
		{"failed-old", testNow.Add(-31 * 24 * time.Hour), true, true},
		{"failed-new", testNow.Add(-29 * 24 * time.Hour), true, false},
	}
	for _, j := range finished {
		clock.Set(j.at)
		opts := EnqueueOptions{}
		kind, want := "ok", "done"
		if j.failed {
			opts.MaxAttempts = 1
			kind, want = "bad", "failed"
		}
		mustEnqueue(t, q, ctx, kind, nil, opts)
		id := lastJobID(t, d)
		mustRunOne(t, q, ctx)
		if got := getJob(t, d, id).status; got != want {
			t.Fatalf("%s finished as %s, want %s", j.name, got, want)
		}
	}

	// A pending job and a running one must survive pruning.
	clock.Set(testNow)
	mustEnqueue(t, q, ctx, "pending", nil, EnqueueOptions{RunAt: testNow.Add(time.Hour)})
	mustEnqueue(t, q, ctx, "running", nil, EnqueueOptions{})
	if _, err := q.write.ClaimJob(ctx, db.Timestamp(testNow)); err != nil {
		t.Fatal(err)
	}

	if err := q.prune(ctx, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// The finished jobs share two kinds, so match each by its run_at.
	for _, j := range finished {
		want := 1
		if j.pruned {
			want = 0
		}
		if got := countJobs(t, d, "run_at = ? AND kind IN ('ok', 'bad')", db.Timestamp(j.at)); got != want {
			t.Errorf("%s (finished %v): %d rows left, want %d", j.name, j.at, got, want)
		}
	}
	if n := countJobs(t, d, "kind = ? AND status = 'queued'", "pending"); n != 1 {
		t.Errorf("pending jobs after prune = %d, want 1", n)
	}
	if n := countJobs(t, d, "kind = ? AND status = 'running'", "running"); n != 1 {
		t.Errorf("running jobs after prune = %d, want 1", n)
	}
}

func TestTruncateMessage(t *testing.T) {
	if got := truncateMessage(errors.New("short")); got != "short" {
		t.Errorf("truncateMessage(short) = %q", got)
	}

	long := strings.Repeat("x", maxLastError+100)
	if got := truncateMessage(errors.New(long)); len(got) != maxLastError || !strings.HasPrefix(long, got) {
		t.Errorf("long message truncated to %d bytes, want %d", len(got), maxLastError)
	}

	// A cut must not split a multi-byte rune.
	wide := strings.Repeat("€", maxLastError)
	got := truncateMessage(errors.New(wide))
	if len(got) > maxLastError || !utf8.ValidString(got) || !strings.HasPrefix(wide, got) {
		t.Errorf("wide message truncated to %d bytes, valid UTF-8 = %v", len(got), utf8.ValidString(got))
	}
}
