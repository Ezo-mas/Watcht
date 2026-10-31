package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestSyncShowSeasonNotFoundContinues(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3577, 0, 1), summary(3572, 1, 1), summary(3573, 2, 1)}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1, testEpisode(101, 1))
	e.md.seasons[seasonKey{1396, 2}] = testSeason(3573, 2, testEpisode(201, 1))
	var logs bytes.Buffer
	e.s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	if err := e.s.HandleSyncShow(ctx, []byte(`{"tmdb_id":1396}`)); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.md.calls, []string{"1396/0", "1396/1", "1396/2"}) {
		t.Fatal(e.md.calls)
	}
	if got := e.textOf(t, "SELECT not_found_since FROM seasons WHERE tmdb_id=3577"); got.String != db.Timestamp(testNow) {
		t.Fatal(got)
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil || !slices.Equal(seasonNumbers(got.Seasons), []int{1, 2}) {
		t.Fatalf("show %v %v", got, err)
	}
	if !strings.Contains(logs.String(), `level=WARN msg="TMDB season not found" show=1396 season=0`) {
		t.Fatal(logs.String())
	}
	if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	e.md.calls = nil
	e.at(1)
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if len(e.md.calls) != 0 {
		t.Fatalf("missing season retried: %v", e.md.calls)
	}
	// Only the season response makes it visible again.
	e.md.seasons[seasonKey{1396, 0}] = testSeason(3577, 0)
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if e.textOf(t, "SELECT not_found_since FROM seasons WHERE tmdb_id=3577").Valid {
		t.Fatal("season did not return")
	}
}

func TestRefreshSeasonAge(t *testing.T) {
	for _, status := range []string{"Ended", "Canceled", "Returning Series"} {
		lifetime := 7 * 24 * time.Hour
		if status == "Returning Series" {
			lifetime = 24 * time.Hour
		}
		for _, delta := range []time.Duration{-time.Second, 0, time.Second} {
			for _, missing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%s", status, delta, map[bool]string{false: "listed", true: "missing"}[missing]), func(t *testing.T) {
					e := newEnv(t)
					ctx := context.Background()
					show := testShow()
					show.Status = status
					show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 0)}
					if err := e.s.UpsertShow(ctx, show); err != nil {
						t.Fatal(err)
					}
					e.clk.now = testNow.Add(-lifetime - delta)
					e.upsertSeason(t, testSeason(3572, 1))
					e.clk.now = testNow
					if missing {
						if _, err := e.d.Write.ExecContext(ctx, "UPDATE seasons SET missing_since=?", db.Timestamp(testNow)); err != nil {
							t.Fatal(err)
						}
					}
					if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
						t.Fatal(err)
					}
					e.md.shows[1396] = show
					e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1)
					if err := e.s.Refresh(ctx); err != nil {
						t.Fatal(err)
					}
					if got := len(e.md.calls) > 0; got != (delta > 0 && !missing) {
						t.Fatalf("calls %v", e.md.calls)
					}
				})
			}
		}
	}
}

func TestRefreshPartialShowRetries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.at(-200)
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 0), summary(3573, 2, 0)}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1)
	e.md.seasons[seasonKey{1396, 2}] = testSeason(3573, 2)
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	e.at(0)
	if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	failure := &providers.StatusError{StatusCode: http.StatusInternalServerError}
	e.md.seasonErr[seasonKey{1396, 1}] = failure
	if err := e.s.Refresh(ctx); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	e.clk.now = testNow.Add(30 * time.Second)
	delete(e.md.seasonErr, seasonKey{1396, 1})
	e.md.calls = nil
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.md.calls, []string{"1396/1", "1396/2"}) {
		t.Fatalf("partial sync not retried: %v", e.md.calls)
	}
}

func TestBackgroundWithoutStale(t *testing.T) {
	for _, kind := range []string{"refresh", "sync job"} {
		t.Run(kind, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			e.at(-200)
			var calls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusInternalServerError) }))
			defer srv.Close()
			e.s.md = tmdb.New(e.d, tmdb.Options{Token: "test", BaseURL: srv.URL, Now: e.clk.Now})
			key := "/movie/603?append_to_response=external_ids"
			body := `{"id":603,"title":"cached"}`
			if kind == "refresh" {
				if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
					t.Fatal(err)
				}
			} else {
				e.storeShow(t)
				key = "/tv/1396?append_to_response=external_ids"
				body = `{"id":1396}`
			}
			if err := sqlc.New(e.d.Write).UpsertProviderCache(ctx, sqlc.UpsertProviderCacheParams{Provider: "tmdb", Key: key, Response: body, FetchedAt: db.Timestamp(e.clk.now), ExpiresAt: db.Timestamp(e.clk.now.Add(time.Hour))}); err != nil {
				t.Fatal(err)
			}
			old := db.Timestamp(e.clk.now)
			e.at(0)
			var err error
			table := "shows"
			if kind == "refresh" {
				table = "movies"
				if err := e.s.MarkMovieViewed(ctx, 603); err != nil {
					t.Fatal(err)
				}
				err = e.s.Refresh(ctx)
			} else {
				err = e.s.HandleSyncShow(ctx, []byte(`{"tmdb_id":1396}`))
			}
			var status *providers.StatusError
			if !errors.As(err, &status) || status.StatusCode != http.StatusInternalServerError || calls != 1 {
				t.Fatalf("error %v calls %d", err, calls)
			}
			if got := e.textOf(t, "SELECT metadata_updated_at FROM "+table).String; got != old {
				t.Fatalf("metadata moved to %s", got)
			}
			// Page loads still use the stale fallback.
			if kind == "refresh" {
				err = e.s.FetchMovie(ctx, 603)
			} else {
				err = e.s.FetchShow(ctx, 1396)
			}
			if err != nil {
				t.Fatalf("page fallback: %v", err)
			}
		})
	}
}

// afterCommitContext reports cancellation only once a row is visible on the
// read pool. Done stays nil so cancellation cannot race the commit itself.
type afterCommitContext struct {
	context.Context
	e     *env
	table string
}

func (c afterCommitContext) Err() error {
	var updated string
	err := c.e.d.Read.QueryRowContext(context.Background(), "SELECT metadata_updated_at FROM "+c.table+" WHERE tmdb_id=1").Scan(&updated)
	if err == nil && updated == db.Timestamp(testNow) {
		return context.Canceled
	}
	return nil
}

func TestFetchShowDisconnected(t *testing.T) {
	e := newEnv(t)
	show := testShow()
	show.TMDBID = 1
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 0)}
	e.md.shows[1] = show
	e.s.jobs = jobs.New(e.d, jobs.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Hold the read connection until the show commits and the caller is
	// cancelled. The detached post-commit query must then acquire it successfully.
	e.d.Read.SetMaxOpenConns(1)
	conn, err := e.d.Read.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	result := make(chan error, 1)
	go func() {
		pollCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		defer func() { _ = conn.Close() }()
		for {
			var n int
			if err := e.d.Write.QueryRowContext(pollCtx, "SELECT count(*) FROM shows").Scan(&n); err != nil {
				result <- err
				return
			}
			if n > 0 {
				cancel()
				result <- nil
				return
			}
			select {
			case <-pollCtx.Done():
				result <- pollCtx.Err()
				return
			case <-ticker.C:
			}
		}
	}()
	err = e.s.FetchShow(ctx, 1)
	if pollErr := <-result; pollErr != nil {
		t.Fatal(pollErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("caller not cancelled")
	}
	if n := e.int64Of(t, "SELECT count(*) FROM jobs WHERE unique_key='catalog_sync_show:1'"); n != 1 {
		t.Fatal("disconnect lost enqueue")
	}
}

func TestShowNeedsEpisodesIgnoresMissing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 0))
	e.storeShow(t)
	// FetchShow lists no seasons, leaving the never-fetched season missing.
	e.md.shows[1396] = testShow()
	q := &testEnqueuer{e: e}
	e.s.jobs = q
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if q.count != 0 {
		t.Fatal("missing season triggered enqueue")
	}
}

func TestRefreshShowsStopOnError(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.at(-200)
	for i := 1; i <= 3; i++ {
		s := testShow()
		s.TMDBID = i
		e.md.shows[i] = s
		if err := e.s.UpsertShow(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	e.at(0)
	if _, err := e.d.Write.ExecContext(ctx, "UPDATE shows SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
		t.Fatal(err)
	}
	md := &refreshMetadata{fakeMetadata: e.md, fail: 2}
	e.s.md = md
	var status *providers.StatusError
	if err := e.s.Refresh(ctx); !errors.As(err, &status) || status.StatusCode != http.StatusInternalServerError {
		t.Fatal(err)
	}
	if !slices.Equal(md.showCalls, []int{1, 2}) {
		t.Fatal(md.showCalls)
	}
	if e.textOf(t, "SELECT metadata_updated_at FROM shows WHERE tmdb_id=1").String != db.Timestamp(testNow) {
		t.Fatal("earlier show lost")
	}
}

func TestRefreshCancellationGuards(t *testing.T) {
	for _, table := range []string{"movies", "shows"} {
		for _, count := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/%d items", table, count), func(t *testing.T) {
				e := newEnv(t)
				ctx := context.Background()
				e.at(-200)
				for i := 1; i <= count; i++ {
					m := testMovie()
					m.TMDBID = i
					e.md.movies[i] = m
					s := testShow()
					s.TMDBID = i
					e.md.shows[i] = s
					var err error
					if table == "movies" {
						err = e.s.UpsertMovie(ctx, m)
					} else {
						err = e.s.UpsertShow(ctx, s)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if _, err := e.d.Write.ExecContext(ctx, "UPDATE "+table+" SET last_viewed_at=?", db.Timestamp(testNow)); err != nil {
					t.Fatal(err)
				}
				e.at(0)
				md := &refreshMetadata{fakeMetadata: e.md}
				e.s.md = md
				err := e.s.Refresh(afterCommitContext{Context: ctx, e: e, table: table})
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("final cancellation lost: %v", err)
				}
				calls := md.movieCalls
				if table == "shows" {
					calls = md.showCalls
				}
				if !slices.Equal(calls, []int{1}) {
					t.Fatalf("fetch after cancellation: %v", calls)
				}
				if e.textOf(t, "SELECT metadata_updated_at FROM "+table+" WHERE tmdb_id=1").String != db.Timestamp(testNow) {
					t.Fatal("first item not committed")
				}
			})
		}
	}
}

func TestMarkSeasonNotFoundPreservesTime(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 0))
	ctx := context.Background()
	for _, hours := range []int{0, 1} {
		e.at(hours)
		if err := e.s.write.MarkSeasonNotFound(ctx, sqlc.MarkSeasonNotFoundParams{Now: nullString(db.Timestamp(e.clk.now)), TmdbID: 3572}); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.textOf(t, "SELECT not_found_since FROM seasons").String; got != db.Timestamp(testNow) {
		t.Fatalf("missing time moved: %s", got)
	}
}

func TestSeasonNotFoundSurvivesShowFetch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 1)}
	e.md.shows[1396] = show
	e.s.jobs = jobs.New(e.d, jobs.Options{})
	assertHidden := func() {
		t.Helper()
		if got := e.textOf(t, "SELECT not_found_since FROM seasons").String; got != db.Timestamp(testNow) {
			t.Fatalf("not-found time changed: %s", got)
		}
		if e.textOf(t, "SELECT missing_since FROM seasons").Valid {
			t.Fatal("404 set missing_since")
		}
		got, err := e.s.Show(ctx, 1396)
		if err != nil || len(got.Seasons) != 0 {
			t.Fatalf("not-found season visible: %v %v", got.Seasons, err)
		}
		if _, err := e.s.SeasonEpisodes(ctx, 1396, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("episodes visible: %v", err)
		}
	}
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	assertHidden()
	e.at(1)
	if err := e.s.FetchShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	assertHidden()
	if n := e.int64Of(t, "SELECT count(*) FROM jobs WHERE kind=?", JobSyncShow); n != 0 {
		t.Fatal("not-found season queued sync")
	}
	e.at(2)
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	assertHidden()
	// At the show's lifetime it is tried again even though it stays hidden.
	if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	e.at(171)
	e.md.calls = nil
	if err := e.s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.md.calls, []string{"1396/1"}) {
		t.Fatalf("not retried at show lifetime: %v", e.md.calls)
	}
	assertHidden()
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1, testEpisode(101, 1))
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if e.textOf(t, "SELECT not_found_since FROM seasons").Valid {
		t.Fatal("successful season did not clear mark")
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil || len(got.Seasons) != 1 {
		t.Fatalf("returned season hidden: %v %v", got.Seasons, err)
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || len(eps) != 1 {
		t.Fatalf("returned episodes hidden: %v %v", eps, err)
	}
}

func TestRefreshIgnoresNotFoundSeasons(t *testing.T) {
	for _, status := range []string{"Ended", "Canceled", "Returning Series"} {
		for _, fetched := range []bool{false, true} {
			t.Run(status+"/"+map[bool]string{false: "unfetched", true: "stale"}[fetched], func(t *testing.T) {
				e := newEnv(t)
				ctx := context.Background()
				show := testShow()
				show.Status = status
				show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 1)}
				e.at(-200)
				if err := e.s.UpsertShow(ctx, show); err != nil {
					t.Fatal(err)
				}
				if fetched {
					e.upsertSeason(t, testSeason(3572, 1))
				}
				if err := e.s.write.MarkSeasonNotFound(ctx, sqlc.MarkSeasonNotFoundParams{Now: nullString(db.Timestamp(e.clk.now)), TmdbID: 3572}); err != nil {
					t.Fatal(err)
				}
				e.at(0)
				// The show's own metadata is fresh; only the excluded season is due.
				if err := e.s.UpsertShow(ctx, show); err != nil {
					t.Fatal(err)
				}
				if err := e.s.MarkShowViewed(ctx, 1396); err != nil {
					t.Fatal(err)
				}
				md := &refreshMetadata{fakeMetadata: e.md}
				e.s.md = md
				if err := e.s.Refresh(ctx); err != nil {
					t.Fatal(err)
				}
				if len(md.showCalls) != 0 {
					t.Fatalf("not-found season selected: %v", md.showCalls)
				}
			})
		}
	}
}

// firstSeasonNotFound answers the first season fetch with a 404, and the rest
// from the fake. The fake keys seasons by show and number, so two listed
// seasons that share a number can't get different answers from it. SyncShow
// fetches a show's seasons in list order, so the first fetch is the first
// listed season.
type firstSeasonNotFound struct {
	*fakeMetadata
	seen bool
}

func (m *firstSeasonNotFound) Season(ctx context.Context, showID, seasonNumber int) (tmdb.Season, error) {
	season, err := m.fakeMetadata.Season(ctx, showID, seasonNumber)
	if !m.seen {
		m.seen = true
		return tmdb.Season{}, providers.ErrNotFound
	}
	return season, err
}

// A season that was synced with episodes hides them once TMDB answers 404.
func TestSeasonNotFoundAfterSyncHidesEpisodes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 1)}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1, testEpisode(101, 1))
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || len(eps) != 1 {
		t.Fatalf("episodes hidden before the 404: %v %v", eps, err)
	}
	e.at(1)
	delete(e.md.seasons, seasonKey{1396, 1})
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if got := e.textOf(t, "SELECT not_found_since FROM seasons WHERE tmdb_id=3572"); got.String != db.Timestamp(e.clk.now) {
		t.Fatalf("not-found time: %v", got)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("episodes of a not-found season visible: %v %v", eps, err)
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil || len(got.Seasons) != 0 {
		t.Fatalf("not-found season listed: %v %v", got.Seasons, err)
	}
}

// Two listed seasons share number 1. The lower TMDB ID answers 404, so the
// visible season is the other one, and its episodes are the ones returned.
func TestDuplicateNumberNotFoundUsesVisibleSeason(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 1), summary(3573, 1, 1)}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3573, 1, testEpisode(201, 1))
	e.s.md = &firstSeasonNotFound{fakeMetadata: e.md}
	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatal(err)
	}
	if got := e.textOf(t, "SELECT not_found_since FROM seasons WHERE tmdb_id=3572"); got.String != db.Timestamp(testNow) {
		t.Fatalf("lower season not marked: %v", got)
	}
	if e.textOf(t, "SELECT not_found_since FROM seasons WHERE tmdb_id=3573").Valid {
		t.Fatal("fetched season marked not found")
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil || !slices.Equal(seasonNumbers(got.Seasons), []int{1}) || got.Seasons[0].TMDBID != 3573 {
		t.Fatalf("show %v %v", got.Seasons, err)
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || !slices.Equal(tmdbIDs(eps), []int{201}) {
		t.Fatalf("episodes %v %v", tmdbIDs(eps), err)
	}
}
