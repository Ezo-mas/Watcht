package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// TestProviderCachePruneJobRuns starts the job queue on a database holding an
// entry expired 31 days ago and one expired an hour ago. The scheduled prune
// runs at startup, and must delete only the first.
func TestProviderCachePruneJobRuns(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	w := sqlc.New(d.Write)
	for key, expires := range map[string]time.Time{
		"/old":    now.Add(-31 * 24 * time.Hour),
		"/recent": now.Add(-time.Hour),
	} {
		if err := w.UpsertProviderCache(ctx, sqlc.UpsertProviderCacheParams{
			Provider:  "test",
			Key:       key,
			Response:  "{}",
			FetchedAt: db.Timestamp(expires),
			ExpiresAt: db.Timestamp(expires),
		}); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	jobCtx, stop := context.WithCancel(ctx)
	cheap := auth.Argon2Params{Memory: 64, Time: 1, Threads: 1}
	q, err := startJobs(jobCtx, d, slog.New(slog.DiscardHandler), auth.New(d, auth.Options{Argon2: cheap}))
	if err != nil {
		t.Fatalf("startJobs: %v", err)
	}
	t.Cleanup(func() {
		stop()
		q.Wait()
	})

	deadline := time.Now().Add(10 * time.Second)
	for cacheHas(t, d, "/old") {
		if time.Now().After(deadline) {
			t.Fatal("the prune job didn't delete the entry expired 31 days ago")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cacheHas(t, d, "/recent") {
		t.Error("the prune job deleted an entry expired an hour ago")
	}
}

// cacheHas reports whether provider_cache has a row for key.
func cacheHas(t *testing.T, d *db.DB, key string) bool {
	t.Helper()
	var n int
	if err := d.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM provider_cache WHERE key = ?", key).Scan(&n); err != nil {
		t.Fatalf("count provider_cache: %v", err)
	}
	return n > 0
}
