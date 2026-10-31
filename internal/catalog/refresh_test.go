package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestMarkViewed(t *testing.T) {
	for _, table := range []string{"movies", "shows"} {
		t.Run(table, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			var mark func(context.Context, int) error
			var id int
			if table == "movies" {
				if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
					t.Fatal(err)
				}
				mark = e.s.MarkMovieViewed
				id = 603
			} else {
				e.storeShow(t)
				mark = e.s.MarkShowViewed
				id = 1396
			}
			for _, seconds := range []int{0, 3599, 3600, 3601} {
				e.clk.now = testNow.Add(time.Duration(seconds) * time.Second)
				if err := mark(ctx, id); err != nil {
					t.Fatal(err)
				}
				want := testNow
				if seconds > 3600 {
					want = e.clk.now
				}
				if got := e.textOf(t, "SELECT last_viewed_at FROM "+table); got.String != db.Timestamp(want) {
					t.Fatalf("at %d: %v", seconds, got)
				}
			}
			if err := mark(ctx, 999); !errors.Is(err, ErrNotFound) {
				t.Fatalf("unknown: %v", err)
			}
		})
	}
}

type testEnqueuer struct {
	e     *env
	count int
	err   error
}

func (q *testEnqueuer) Enqueue(ctx context.Context, kind string, payload any, opts jobs.EnqueueOptions) error {
	q.count++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
		return errors.New("enqueue needs a short timeout")
	}
	if kind != JobSyncShow || opts.UniqueKey != "catalog_sync_show:1396" {
		return errors.New("wrong kind or unique key")
	}
	b, _ := json.Marshal(payload)
	if string(b) != `{"tmdb_id":1396}` {
		return errors.New("wrong payload")
	}
	if err := q.e.md.writeFree("enqueue"); err != nil {
		return err
	}
	if q.e.storedShowCount(ctx) != 1 {
		return errors.New("show not committed")
	}
	return q.err
}
func (e *env) storedShowCount(ctx context.Context) int {
	var n int
	_ = e.d.Read.QueryRowContext(ctx, "SELECT count(*) FROM shows").Scan(&n)
	return n
}
func TestFetchShowEnqueue(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = append(show.Seasons, summary(3572, 1, 0))
	e.md.shows[1396] = show
	q := &testEnqueuer{e: e}
	e.s.jobs = q
	var logs bytes.Buffer
	e.s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if q.count != 1 || logs.Len() != 0 {
		t.Fatalf("enqueue count %d, logs %s", q.count, logs.String())
	}
	q.err = errors.New("queue unavailable")
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if logs.Len() == 0 {
		t.Fatal("missing warning")
	}
	e.upsertSeason(t, testSeason(3572, 1))
	before := q.count
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if q.count != before {
		t.Fatal("enqueued complete show")
	}
	e.s.jobs = nil
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
}

// The queue's permanent wrapper is private; compare the wrapper type that
// jobs.Permanent itself returns.
func permanent(err error) bool {
	return err != nil && reflect.TypeOf(err) == reflect.TypeOf(jobs.Permanent(errors.New("test")))
}
func TestHandleSyncShow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, p := range []string{"{", "null", "{}", `{"tmdb_id":0}`, `{"tmdb_id":-1}`, `{"tmdb_id":"bad"}`} {
		if err := e.s.HandleSyncShow(ctx, []byte(p)); !permanent(err) {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if err := e.s.HandleSyncShow(ctx, []byte(`{"tmdb_id":1396}`)); !permanent(err) {
		t.Fatalf("404: %v", err)
	}
	show := testShow()
	show.Seasons = append(show.Seasons, summary(3572, 1, 0))
	e.md.shows[1396] = show
	failure := errors.New("provider failed")
	e.md.seasonErr[seasonKey{1396, 1}] = failure
	if err := e.s.HandleSyncShow(ctx, []byte(`{"tmdb_id":1396}`)); !errors.Is(err, failure) || permanent(err) {
		t.Fatal(err)
	}
	delete(e.md.seasonErr, seasonKey{1396, 1})
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1)
	if err := e.s.HandleSyncShow(ctx, []byte(`{"tmdb_id":1396}`)); err != nil {
		t.Fatal(err)
	}
	if got := e.textOf(t, "SELECT episodes_updated_at FROM seasons"); !got.Valid {
		t.Fatal("not synced")
	}
}

func TestRefreshSelection(t *testing.T) {
	for _, kind := range []string{"movie", "Ended", "Canceled", "Returning Series"} {
		life := 7 * 24 * time.Hour
		if kind == "Returning Series" {
			life = 24 * time.Hour
		}
		for _, ageDelta := range []time.Duration{-time.Second, 0, time.Second} {
			for _, viewedAge := range []time.Duration{0, 30*24*time.Hour - time.Second, 30 * 24 * time.Hour, 30*24*time.Hour + time.Second, -1} {
				t.Run(fmt.Sprintf("%s/%s/%s", kind, ageDelta, viewedAge), func(t *testing.T) {
					e := newEnv(t)
					ctx := context.Background()
					e.clk.now = testNow.Add(-life - ageDelta)
					table := "shows"
					id := 1396
					if kind == "movie" {
						table = "movies"
						id = 603
						if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
							t.Fatal(err)
						}
						e.md.movies[id] = testMovie()
					} else {
						show := testShow()
						show.Status = kind
						if err := e.s.UpsertShow(ctx, show); err != nil {
							t.Fatal(err)
						}
						e.md.shows[id] = show
					}
					if viewedAge >= 0 {
						if _, err := e.d.Write.ExecContext(ctx, "UPDATE "+table+" SET last_viewed_at=?", db.Timestamp(testNow.Add(-viewedAge))); err != nil {
							t.Fatal(err)
						}
					}
					e.clk.now = testNow
					if err := e.s.Refresh(ctx); err != nil {
						t.Fatal(err)
					}
					got := e.textOf(t, "SELECT metadata_updated_at FROM "+table).String == db.Timestamp(testNow)
					want := ageDelta > 0 && viewedAge >= 0 && viewedAge <= 30*24*time.Hour
					if got != want {
						t.Fatalf("refreshed %v want %v", got, want)
					}
				})
			}
		}
	}
}

func TestRefreshCatchup(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "listed season", true: "missing season"}[missing], func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			e.storeShow(t, summary(3572, 1, 0))
			if missing {
				e.storeShow(t)
			}
			if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
				t.Fatal(err)
			}
			show := testShow()
			show.Seasons = append(show.Seasons, summary(3572, 1, 0))
			e.md.shows[1396] = show
			e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1)
			if err := e.s.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			if (len(e.md.calls) > 0) == missing {
				t.Fatalf("calls %v", e.md.calls)
			}
		})
	}
}

func TestRefreshDeletedMovie(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.at(-200)
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatal(err)
	}
	e.at(0)
	if err := e.s.MarkMovieViewed(ctx, 603); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	e.s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if e.textOf(t, "SELECT metadata_updated_at FROM movies").String != db.Timestamp(testNow) || !strings.Contains(logs.String(), `level=WARN msg="TMDB movie deleted" tmdb_id=603`) {
		t.Fatal("404 not touched or logged")
	}
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if e.int64Of(t, "SELECT count(*) FROM movies") != 1 {
		t.Fatal("deleted row")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := e.s.Refresh(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// refreshMetadata records order and injects failures after earlier writes.
type refreshMetadata struct {
	*fakeMetadata
	movieCalls []int
	showCalls  []int
	fail       int
	cancel     context.CancelFunc
}

func (m *refreshMetadata) Movie(ctx context.Context, id int) (tmdb.Movie, error) {
	m.movieCalls = append(m.movieCalls, id)
	if id == m.fail {
		return tmdb.Movie{}, &providers.StatusError{StatusCode: 500}
	}
	if m.cancel != nil {
		m.cancel()
	}
	return m.fakeMetadata.Movie(ctx, id)
}
func (m *refreshMetadata) Show(ctx context.Context, id int) (tmdb.Show, error) {
	m.showCalls = append(m.showCalls, id)
	if id == m.fail {
		return tmdb.Show{}, &providers.StatusError{StatusCode: 500}
	}
	return m.fakeMetadata.Show(ctx, id)
}

func TestRefreshBatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	md := &refreshMetadata{fakeMetadata: e.md}
	e.s.md = md
	for i := 1; i <= 55; i++ {
		e.clk.now = testNow.Add(-time.Duration(300+i) * time.Hour)
		movie := testMovie()
		movie.TMDBID = i
		e.md.movies[i] = movie
		if err := e.s.UpsertMovie(ctx, movie); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 12; i++ {
		e.clk.now = testNow.Add(-time.Duration(300+i) * time.Hour)
		show := testShow()
		show.TMDBID = i
		e.md.shows[i] = show
		if err := e.s.UpsertShow(ctx, show); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"movies", "shows"} {
		if _, err := e.d.Write.ExecContext(ctx, "UPDATE "+table+" SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
			t.Fatal(err)
		}
	}
	e.clk.now = testNow
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if len(md.movieCalls) != 50 || len(md.showCalls) != 10 {
		t.Fatalf("counts %d %d", len(md.movieCalls), len(md.showCalls))
	}
	for i, id := range md.movieCalls {
		if id != 55-i {
			t.Fatalf("movie order %v", md.movieCalls)
		}
	}
	for i, id := range md.showCalls {
		if id != 12-i {
			t.Fatalf("show order %v", md.showCalls)
		}
	}
}

func TestRefreshStops(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "provider failure", true: "cancelled"}[cancelled], func(t *testing.T) {
			e := newEnv(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			md := &refreshMetadata{fakeMetadata: e.md, fail: 2}
			e.s.md = md
			if cancelled {
				md.cancel = cancel
			}
			e.at(-200)
			for i := 1; i <= 3; i++ {
				movie := testMovie()
				movie.TMDBID = i
				e.md.movies[i] = movie
				if err := e.s.UpsertMovie(ctx, movie); err != nil {
					t.Fatal(err)
				}
			}
			e.at(0)
			if _, err := e.d.Write.ExecContext(ctx, "UPDATE movies SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
				t.Fatal(err)
			}
			err := e.s.Refresh(ctx)
			if cancelled {
				if !errors.Is(err, context.Canceled) || len(md.movieCalls) != 1 {
					t.Fatalf("cancel %v %v", err, md.movieCalls)
				}
			} else {
				var status *providers.StatusError
				if !errors.As(err, &status) || len(md.movieCalls) != 2 {
					t.Fatalf("failure %v %v", err, md.movieCalls)
				}
				if e.textOf(t, "SELECT metadata_updated_at FROM movies WHERE tmdb_id=1").String != db.Timestamp(testNow) {
					t.Fatal("earlier refresh lost")
				}
			}
		})
	}
}

func TestRefreshDeletedContinues(t *testing.T) {
	e := newEnv(t)
	var logs bytes.Buffer
	e.s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	ctx := context.Background()
	e.at(-200)
	for i := 1; i <= 2; i++ {
		movie := testMovie()
		movie.TMDBID = i
		show := testShow()
		show.TMDBID = i
		if err := e.s.UpsertMovie(ctx, movie); err != nil {
			t.Fatal(err)
		}
		if err := e.s.UpsertShow(ctx, show); err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			e.md.movies[i] = movie
			e.md.shows[i] = show
		}
	}
	e.at(0)
	for _, table := range []string{"movies", "shows"} {
		if _, err := e.d.Write.ExecContext(ctx, "UPDATE "+table+" SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"movies", "shows"} {
		if n := e.int64Of(t, "SELECT count(*) FROM "+table+" WHERE metadata_updated_at=?", db.Timestamp(testNow)); n != 2 {
			t.Fatalf("%s updated %d", table, n)
		}
	}
	if !strings.Contains(logs.String(), `level=INFO msg="refreshed catalog" movies=1 shows=1 not_found=2`) {
		t.Fatalf("counts: %s", logs.String())
	}

}

func TestFetchShowQueueDeduplicates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = append(show.Seasons, summary(3572, 1, 1))
	e.md.shows[1396] = show
	// Missing episodes are safe with no queue configured.
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	e.s.jobs = jobs.New(e.d, jobs.Options{})
	for i := 0; i < 2; i++ {
		if err := e.s.FetchShow(ctx, 1396); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.int64Of(t, "SELECT count(*) FROM jobs WHERE kind=?", JobSyncShow); n != 1 {
		t.Fatalf("queued %d jobs", n)
	}
}

// cancelAfterRefresh makes cancellation deterministic immediately after a
// committed first item, before Refresh reaches the next provider call.
type cancelAfterRefresh struct {
	context.Context
	e *env
}

func (c cancelAfterRefresh) Err() error {
	var updated string
	err := c.e.d.Read.QueryRowContext(context.Background(), "SELECT metadata_updated_at FROM movies WHERE tmdb_id=1").Scan(&updated)
	if err == nil && updated == db.Timestamp(testNow) {
		return context.Canceled
	}
	return nil
}
func TestRefreshCancellationBetweenItems(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.at(-200)
	for i := 1; i <= 2; i++ {
		m := testMovie()
		m.TMDBID = i
		e.md.movies[i] = m
		if err := e.s.UpsertMovie(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.d.Write.ExecContext(ctx, "UPDATE movies SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
		t.Fatal(err)
	}
	e.at(0)
	md := &refreshMetadata{fakeMetadata: e.md}
	e.s.md = md
	err := e.s.Refresh(cancelAfterRefresh{Context: ctx, e: e})
	if !errors.Is(err, context.Canceled) || len(md.movieCalls) != 1 {
		t.Fatalf("cancel: %v calls %v", err, md.movieCalls)
	}
	if e.textOf(t, "SELECT metadata_updated_at FROM movies WHERE tmdb_id=1").String != db.Timestamp(testNow) {
		t.Fatal("first item not committed")
	}
}
