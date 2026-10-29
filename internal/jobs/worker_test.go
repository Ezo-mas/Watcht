package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

func TestClaimOrder(t *testing.T) {
	q, _, clock := newQueue(t)
	ctx := context.Background()
	var ran []string
	for _, kind := range []string{"second", "first", "later", "future"} {
		q.Register(kind, func(_ context.Context, _ json.RawMessage) error {
			ran = append(ran, kind)
			return nil
		})
	}
	mustEnqueue(t, q, ctx, "second", nil, EnqueueOptions{RunAt: clock.Now()})
	mustEnqueue(t, q, ctx, "first", nil, EnqueueOptions{RunAt: clock.Now().Add(-time.Minute)})
	mustEnqueue(t, q, ctx, "later", nil, EnqueueOptions{RunAt: clock.Now().Add(time.Minute)})
	mustEnqueue(t, q, ctx, "future", nil, EnqueueOptions{RunAt: clock.Now().Add(time.Hour)})

	// The two already due, oldest first.
	mustRunOne(t, q, ctx)
	mustRunOne(t, q, ctx)
	if ran, err := q.runOne(ctx); err != nil || ran {
		t.Fatalf("runOne with nothing due = %v, %v; want false, nil", ran, err)
	}

	clock.Advance(time.Minute)
	mustRunOne(t, q, ctx)
	if ran, err := q.runOne(ctx); err != nil || ran {
		t.Fatalf("runOne before the future run_at = %v, %v; want false, nil", ran, err)
	}
	clock.Advance(59 * time.Minute)
	mustRunOne(t, q, ctx)

	want := []string{"first", "second", "later", "future"}
	if len(ran) != len(want) {
		t.Fatalf("ran %v, want %v", ran, want)
	}
	for i := range want {
		if ran[i] != want[i] {
			t.Fatalf("ran %v, want %v", ran, want)
		}
	}
}

func TestRetryBackoffAndMaxAttempts(t *testing.T) {
	q, d, clock := newQueue(t)
	ctx := context.Background()
	q.Register("flaky", func(context.Context, json.RawMessage) error { return errors.New("boom") })
	mustEnqueue(t, q, ctx, "flaky", nil, EnqueueOptions{MaxAttempts: 3})
	id := lastJobID(t, d)

	for attempt := int64(1); attempt <= 3; attempt++ {
		mustRunOne(t, q, ctx)
		j := getJob(t, d, id)
		if j.attempts != attempt {
			t.Errorf("after run %d: attempts = %d", attempt, j.attempts)
		}
		if !j.lastError.Valid || j.lastError.String != "boom" {
			t.Errorf("after run %d: last_error = %v, want boom", attempt, j.lastError)
		}
		if attempt == 3 {
			if j.status != "failed" || !j.finishedAt.Valid {
				t.Errorf("after the last attempt: status %s, finished_at %v; want failed", j.status, j.finishedAt)
			}
			break
		}
		if j.status != "queued" {
			t.Fatalf("after run %d: status = %s, want queued", attempt, j.status)
		}
		if want := db.Timestamp(clock.Now().Add(backoff(attempt))); j.runAt != want {
			t.Errorf("after run %d: run_at = %q, want %q", attempt, j.runAt, want)
		}
		if j.startedAt.Valid || j.finishedAt.Valid {
			t.Errorf("after run %d: started_at %v, finished_at %v; want NULL while queued", attempt, j.startedAt, j.finishedAt)
		}
		// It isn't due until the backoff has passed.
		if ran, err := q.runOne(ctx); err != nil || ran {
			t.Fatalf("job ran before its backoff: %v, %v", ran, err)
		}
		clock.Advance(backoff(attempt))
	}
}

func TestPermanentAndUnknownKindFailAtOnce(t *testing.T) {
	q, d, _ := newQueue(t)
	ctx := context.Background()
	q.Register("permanent", func(context.Context, json.RawMessage) error {
		return Permanent(errors.New("bad payload"))
	})
	q.Register("wrapped", func(context.Context, json.RawMessage) error {
		return fmt.Errorf("load: %w", Permanent(errors.New("bad")))
	})
	mustEnqueue(t, q, ctx, "permanent", nil, EnqueueOptions{MaxAttempts: 5})
	permanentID := lastJobID(t, d)
	mustEnqueue(t, q, ctx, "wrapped", nil, EnqueueOptions{MaxAttempts: 5})
	wrappedID := lastJobID(t, d)
	mustEnqueue(t, q, ctx, "nobody", nil, EnqueueOptions{MaxAttempts: 5})
	unknownID := lastJobID(t, d)

	for range 3 {
		mustRunOne(t, q, ctx)
	}
	for _, tc := range []struct {
		name string
		id   int64
		msg  string
	}{
		{"permanent", permanentID, "bad payload"},
		{"wrapped permanent", wrappedID, "load: bad"},
		{"unknown kind", unknownID, `unknown job kind "nobody"`},
	} {
		j := getJob(t, d, tc.id)
		if j.status != "failed" || j.attempts != 1 {
			t.Errorf("%s: status %s with %d attempts, want failed after 1", tc.name, j.status, j.attempts)
		}
		if !j.lastError.Valid || j.lastError.String != tc.msg {
			t.Errorf("%s: last_error = %v, want %q", tc.name, j.lastError, tc.msg)
		}
	}
}

func TestPanicRecovered(t *testing.T) {
	q, d, clock := newQueue(t)
	ctx := context.Background()
	q.Register("boom", func(context.Context, json.RawMessage) error { panic("handler exploded") })
	mustEnqueue(t, q, ctx, "boom", nil, EnqueueOptions{MaxAttempts: 2})
	id := lastJobID(t, d)

	mustRunOne(t, q, ctx)
	j := getJob(t, d, id)
	if j.status != "queued" && j.status != "failed" {
		t.Fatalf("status = %s after a panic, want a recorded failure", j.status)
	}
	if j.attempts != 1 || !strings.Contains(j.lastError.String, "panic: handler exploded") {
		t.Errorf("after the panic: %+v, want attempt 1 with a panic message", j)
	}

	// The recovered panic is an ordinary error: it uses the next attempt.
	clock.Advance(backoff(1))
	mustRunOne(t, q, ctx)
	j = getJob(t, d, id)
	if j.status != "failed" || j.attempts != 2 {
		t.Errorf("after the second panic: status %s, attempts %d; want failed after 2", j.status, j.attempts)
	}
}

func TestJobTimeout(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	q := New(d, Options{Now: clock.Now, JobTimeout: 20 * time.Millisecond})
	ctx := context.Background()
	q.Register("slow", func(ctx context.Context, _ json.RawMessage) error {
		<-ctx.Done()
		return ctx.Err()
	})
	mustEnqueue(t, q, ctx, "slow", nil, EnqueueOptions{})
	id := lastJobID(t, d)

	mustRunOne(t, q, ctx)
	j := getJob(t, d, id)
	if j.status != "queued" || j.attempts != 1 {
		t.Errorf("after the timeout: %+v, want queued for a retry", j)
	}
	if !j.lastError.Valid || !strings.Contains(j.lastError.String, "deadline exceeded") {
		t.Errorf("last_error = %v, want a deadline message", j.lastError)
	}
}

func TestShutdownRequeuesWithoutCountingAttempt(t *testing.T) {
	q, d, _ := newQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	q.Register("block", func(ctx context.Context, _ json.RawMessage) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	mustEnqueue(t, q, context.Background(), "block", nil, EnqueueOptions{})
	id := lastJobID(t, d)

	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, started, "the blocking job to start")
	cancel()
	q.Wait()

	j := getJob(t, d, id)
	if j.status != "queued" || j.attempts != 0 || j.startedAt.Valid {
		t.Errorf("after shutdown: %+v, want queued with 0 attempts and no started_at", j)
	}
}

func TestStartupRecovery(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	q := New(d, Options{Now: clock.Now})
	ctx := context.Background()
	mustEnqueue(t, q, ctx, "work", nil, EnqueueOptions{})
	claimed, err := q.write.ClaimJob(ctx, db.Timestamp(clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Kind != "work" || claimed.Attempts != 1 {
		t.Fatalf("claimed = %+v, want work on attempt 1", claimed)
	}

	// The row is now running, as after a crash. A restarted process must
	// requeue it before its worker starts.
	ran := make(chan struct{})
	release := make(chan struct{})
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q2 := New(d, Options{Now: clock.Now, PollInterval: 10 * time.Millisecond})
	q2.Register("work", func(context.Context, json.RawMessage) error {
		close(ran)
		<-release
		return nil
	})
	if err := q2.Start(runCtx); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ran, "the recovered job to run")
	j := getJob(t, d, claimed.ID)
	if j.status != "running" || j.attempts != 2 {
		t.Errorf("recovered job = %s on attempt %d, want running on attempt 2", j.status, j.attempts)
	}

	close(release)
	waitFor(t, "the recovered job to finish", func() bool {
		return getJob(t, d, claimed.ID).status == "done"
	})
	cancel()
	q2.Wait()
}

func TestStartupRecoveryFailsJobOnItsLastAttempt(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	q := New(d, Options{Now: clock.Now})
	ctx := context.Background()
	mustEnqueue(t, q, ctx, "crash", nil, EnqueueOptions{MaxAttempts: 2})
	id := lastJobID(t, d)
	// Two runs that each took the process down with them.
	for range 2 {
		if _, err := q.write.ClaimJob(ctx, db.Timestamp(clock.Now())); err != nil {
			t.Fatal(err)
		}
		if err := q.recover(ctx); err != nil {
			t.Fatal(err)
		}
	}

	j := getJob(t, d, id)
	if j.status != "failed" || j.attempts != 2 || !j.finishedAt.Valid || !j.lastError.Valid {
		t.Errorf("after crashing on its last attempt: %+v, want failed after 2 attempts with finished_at and last_error", j)
	}
}

func TestWorkerRecoversRunWhoseOutcomeWasNotRecorded(t *testing.T) {
	q, d, _ := newQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Register("work", func(context.Context, json.RawMessage) error { return nil })
	mustEnqueue(t, q, ctx, "work", nil, EnqueueOptions{})
	id := lastJobID(t, d)
	// A claimed row whose outcome write failed: running, with no run behind
	// it, which the worker flags as orphaned.
	if _, err := q.write.ClaimJob(ctx, db.Timestamp(testNow)); err != nil {
		t.Fatal(err)
	}
	q.orphaned = true

	q.wg.Add(1)
	go q.runWorker(ctx)
	waitFor(t, "the orphaned job to be rerun", func() bool {
		return getJob(t, d, id).status == "done"
	})
	cancel()
	q.Wait()
	if j := getJob(t, d, id); j.attempts != 2 {
		t.Errorf("attempts = %d, want 2 (the orphaned run counts)", j.attempts)
	}
}

func TestRunOneAfterShutdownClaimsNothing(t *testing.T) {
	q, d, _ := newQueue(t)
	q.Register("work", func(context.Context, json.RawMessage) error { return nil })
	mustEnqueue(t, q, context.Background(), "work", nil, EnqueueOptions{})
	id := lastJobID(t, d)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ran, err := q.runOne(ctx)
	if err != nil || ran {
		t.Fatalf("runOne(cancelled) = %v, %v, want false, nil", ran, err)
	}
	if j := getJob(t, d, id); j.status != "queued" || j.attempts != 0 {
		t.Errorf("job = %s on attempt %d, want queued with 0 attempts", j.status, j.attempts)
	}
}

func TestWorkerWakesOnEnqueue(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	// A long poll interval proves the enqueue wake-up is what runs it.
	q := New(d, Options{Now: clock.Now, PollInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ran := make(chan struct{}, 1)
	q.Register("wake", func(context.Context, json.RawMessage) error {
		ran <- struct{}{}
		return nil
	})
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	mustEnqueue(t, q, ctx, "wake", nil, EnqueueOptions{})
	waitSignal(t, ran, "the enqueued job to run")
	cancel()
	q.Wait()
}

func TestWorkerPolls(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	q := New(d, Options{Now: clock.Now, PollInterval: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ran := make(chan struct{}, 1)
	q.Register("polled", func(context.Context, json.RawMessage) error {
		ran <- struct{}{}
		return nil
	})
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Inserted directly, without Enqueue's wake, so only the poll can see it.
	now := db.Timestamp(clock.Now())
	if _, err := d.Write.ExecContext(ctx, `
		INSERT INTO jobs (kind, payload, status, attempts, max_attempts, run_at, created_at, updated_at)
		VALUES ('polled', '{}', 'queued', 0, 5, ?, ?, ?)`, now, now, now); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ran, "the polled job to run")
	cancel()
	q.Wait()
}

func TestStartTwice(t *testing.T) {
	q, _, _ := newQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		q.Wait()
	}()
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := q.Start(ctx); err == nil {
		t.Error("second Start = nil, want an error")
	}
}
