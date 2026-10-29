package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

func TestSchedulerRunsAtStartAndEveryInterval(t *testing.T) {
	d := dbtest.New(t)
	clock := &fakeClock{t: testNow}
	tick := make(chan time.Time)
	q := New(d, Options{
		Now:           clock.Now,
		PollInterval:  time.Hour,
		SchedulerTick: tick,
	})
	ran := make(chan struct{}, 8)
	q.Register("periodic", func(context.Context, json.RawMessage) error {
		ran <- struct{}{}
		return nil
	})
	q.Every("periodic", time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Every schedule runs once at startup.
	waitSignal(t, ran, "the job scheduled at startup")

	// It doesn't run again until the interval has passed.
	select {
	case <-ran:
		t.Fatal("the scheduled job ran again before its interval")
	case <-time.After(50 * time.Millisecond):
	}

	clock.Advance(time.Hour)
	tick <- clock.Now()
	waitSignal(t, ran, "the job scheduled for the next interval")

	cancel()
	q.Wait()
}

// A restart while the first run is still pending must not enqueue a second
// copy, and then the next interval must add exactly one more.
func TestScheduledJobDedupedAcrossRestart(t *testing.T) {
	q1, d, clock := newQueue(t)
	ctx := context.Background()
	q1.Register("periodic", func(context.Context, json.RawMessage) error { return nil })
	q1.Every("periodic", time.Hour)

	// What Start does on boot.
	if err := q1.enqueueScheduled(ctx, clock.Now()); err != nil {
		t.Fatal(err)
	}
	if n := countJobs(t, d, "kind = ? AND status = 'queued'", "periodic"); n != 1 {
		t.Fatalf("%d periodic jobs after the first boot, want 1", n)
	}

	// A second boot while that job is still pending.
	q2 := New(d, Options{Now: clock.Now, PollInterval: time.Hour})
	q2.Register("periodic", func(context.Context, json.RawMessage) error { return nil })
	q2.Every("periodic", time.Hour)
	if err := q2.enqueueScheduled(ctx, clock.Now()); err != nil {
		t.Fatal(err)
	}
	if n := countJobs(t, d, "kind = ? AND status = 'queued'", "periodic"); n != 1 {
		t.Fatalf("%d periodic jobs after a restart, want 1 (deduplicated by unique_key)", n)
	}

	// Once the pending job has run, the next interval enqueues a new one.
	for {
		ran, err := q1.runOne(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ran {
			break
		}
	}
	clock.Advance(time.Hour)
	if err := q2.enqueueScheduled(ctx, clock.Now()); err != nil {
		t.Fatal(err)
	}
	if n := countJobs(t, d, "kind = ? AND status = 'queued'", "periodic"); n != 1 {
		t.Errorf("%d periodic jobs after the interval, want 1", n)
	}
	if n := countJobs(t, d, "kind = ?", "periodic"); n != 2 {
		t.Errorf("%d periodic jobs in total, want 2", n)
	}
}

func TestRegisterAndEveryValidation(t *testing.T) {
	q, _, _ := newQueue(t)
	handler := func(context.Context, json.RawMessage) error { return nil }

	assertPanics(t, "Register with an empty kind", func() { q.Register("", handler) })
	assertPanics(t, "Register with a nil handler", func() { q.Register("nil", nil) })
	assertPanics(t, "Every with no handler", func() { q.Every("missing", time.Hour) })
	assertPanics(t, "Every with a non-positive interval", func() {
		q.Register("x", handler)
		q.Every("x", 0)
	})
	q.Every("x", time.Hour)
	assertPanics(t, "Every twice", func() { q.Every("x", time.Hour) })
	assertPanics(t, "duplicate Register", func() { q.Register("x", handler) })

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		q.Wait()
	}()
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	assertPanics(t, "Register after Start", func() { q.Register("y", handler) })
	assertPanics(t, "Every after Start", func() { q.Every("x", 2*time.Hour) })
}

func assertPanics(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s didn't panic", what)
		}
	}()
	f()
}
