package main

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/jobs"
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
	q, _, err := startJobs(jobCtx, d, slog.New(slog.DiscardHandler), auth.New(d, auth.Options{Argon2: cheap}), "")
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

func TestCatalogJobsWiring(t *testing.T) {
	for _, token := range []string{"", "test-token"} {
		t.Run(map[bool]string{false: "without token", true: "with token"}[token != ""], func(t *testing.T) {
			d := dbtest.New(t)
			ctx, stop := context.WithCancel(context.Background())
			q, service, err := startJobs(ctx, d, slog.New(slog.DiscardHandler), auth.New(d, auth.Options{}), token)
			if err != nil {
				stop()
				t.Fatal(err)
			}
			defer func() { stop(); q.Wait() }()
			if (service != nil) != (token != "") {
				t.Fatal("catalog availability")
			}
			// Empty catalog: the scheduled refresh never needs a network request.
			deadline := time.Now().Add(10 * time.Second)
			for {
				var n int
				if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE kind=? AND status='done'", catalog.JobRefresh).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if token == "" {
					if n != 0 {
						t.Fatal("refresh scheduled without token")
					}
					break
				}
				if n == 1 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("refresh not scheduled or handler missing")
				}
				time.Sleep(5 * time.Millisecond)
			}
			// An invalid payload distinguishes the registered sync handler from an
			// unknown kind without making a TMDB request.
			if err := q.Enqueue(ctx, catalog.JobSyncShow, map[string]int{"tmdb_id": 0}, jobs.EnqueueOptions{}); err != nil {
				t.Fatal(err)
			}
			for {
				var status string
				var last sql.NullString
				err := d.Read.QueryRowContext(ctx, "SELECT status,last_error FROM jobs WHERE kind=?", catalog.JobSyncShow).Scan(&status, &last)
				if err != nil {
					t.Fatal(err)
				}
				if status == "failed" {
					registered := strings.Contains(last.String, "invalid show sync payload")
					if registered != (token != "") {
						t.Fatalf("handler result %s", last.String)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("sync job not handled")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if service != nil {
				// Stop the worker before FetchShow: only the enqueue is under test.
				stop()
				q.Wait()
				fresh := time.Now().UTC()
				if err := sqlc.New(d.Write).UpsertProviderCache(context.Background(), sqlc.UpsertProviderCacheParams{
					Provider: "tmdb", Key: "/tv/1396?append_to_response=external_ids",
					Response:  `{"id":1396,"seasons":[{"id":3572,"season_number":1}]}`,
					FetchedAt: db.Timestamp(fresh), ExpiresAt: db.Timestamp(fresh.Add(time.Hour)),
				}); err != nil {
					t.Fatal(err)
				}
				if err := service.FetchShow(context.Background(), 1396); err != nil {
					t.Fatal(err)
				}
				var n int
				if err := d.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM jobs WHERE unique_key='catalog_sync_show:1396'").Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != 1 {
					t.Fatal("catalog not wired to queue")
				}
			}

		})
	}
}
