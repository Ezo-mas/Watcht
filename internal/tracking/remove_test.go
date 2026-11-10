package tracking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestRemoveAllWatches(t *testing.T) {
	for _, kind := range []string{"movie", "episode", "season", "show"} {
		t.Run(kind, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			remove := func(id int) (int, error) { return e.s.RemoveMovieWatches(ctx, e.user, id) }
			id := 603
			event := Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow, Source: SourceManual}
			if kind != "movie" {
				event.MovieID, event.EpisodeID = 0, e.episode
				id = 62085
				remove = func(id int) (int, error) { return e.s.RemoveEpisodeWatches(ctx, e.user, id) }
			}
			if kind == "season" {
				id = 1396
				remove = func(id int) (int, error) { return e.s.RemoveSeasonWatches(ctx, e.user, id, 1) }
			}
			if kind == "show" {
				id = 1396
				remove = func(id int) (int, error) { return e.s.RemoveShowWatches(ctx, e.user, id) }
			}
			e.record(t, event)
			e.record(t, event) // merged: removed too, but doesn't count as another play
			event.WatchedAt = testNow.Add(-24 * time.Hour)
			e.record(t, event)
			event.UserID = e.other
			e.record(t, event)
			event.UserID, event.MovieID, event.EpisodeID = e.user, e.movie2, 0
			e.record(t, event)
			if n, err := remove(id); n != 2 || err != nil {
				t.Fatalf("remove = %d, %v; want 2", n, err)
			}
			if n := e.count(t); n != 2 {
				t.Fatalf("events left = %d; want other user's watch and unrelated movie", n)
			}
			var own int
			if err := e.d.Read.QueryRow(`SELECT count(*) FROM watch_events WHERE user_id = ? AND movie_id = ?`, e.user, e.movie2).Scan(&own); err != nil || own != 1 {
				t.Fatalf("unrelated movie left = %d, %v", own, err)
			}
			var other int
			if err := e.d.Read.QueryRow(`SELECT count(*) FROM watch_events WHERE user_id = ?`, e.other).Scan(&other); err != nil || other != 1 {
				t.Fatalf("other user's events = %d, %v", other, err)
			}
			if n, err := remove(id); n != 0 || err != nil {
				t.Fatalf("second remove = %d, %v; want 0", n, err)
			}
			if _, err := remove(99999); !errors.Is(err, ErrNotFound) {
				t.Fatalf("missing title = %v", err)
			}
		})
	}
}

func TestRemoveSeasonAndShowScope(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeMarkShow(t, map[int][]tmdb.Episode{0: {ep(1, 1, "2008-01-20")}, 1: {ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-20")}, 2: {ep(201, 1, "2008-01-20")}})
	for _, id := range []int{1, 101, 102, 201} {
		if _, err := e.s.MarkEpisode(ctx, e.user, id, When{}); err != nil {
			t.Fatal(err)
		}
	}
	// Metadata can renumber an episode; season membership stays tied to its identity.
	if _, err := e.d.Write.Exec(`UPDATE episodes SET season_number = 7, missing_since = '2026-11-03T00:00:00Z' WHERE tmdb_id = 102`); err != nil {
		t.Fatal(err)
	}
	if n, err := e.s.RemoveSeasonWatches(ctx, e.user, markShow, 1); err != nil || n != 2 {
		t.Fatalf("season remove = %d, %v", n, err)
	}
	if _, err := e.s.RemoveSeasonWatches(ctx, e.user, markShow, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing season = %v", err)
	}
	if n, err := e.s.RemoveShowWatches(ctx, e.user, markShow); err != nil || n != 2 {
		t.Fatalf("show remove including specials = %d, %v", n, err)
	}
	if n := e.count(t); n != 0 {
		t.Fatalf("events left = %d", n)
	}
}

func TestRemoveSeasonUsesListedIdentity(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	if err := c.UpsertShow(ctx, tmdb.Show{TMDBID: markShow, Seasons: []tmdb.SeasonSummary{{TMDBID: 9001, SeasonNumber: 1}, {TMDBID: 9101, SeasonNumber: 1}}}); err != nil {
		t.Fatal(err)
	}
	for seasonID, episodeID := range map[int]int{9001: 101, 9101: 111} {
		if err := c.UpsertSeason(ctx, markShow, tmdb.Season{TMDBID: seasonID, ShowTMDBID: markShow, SeasonNumber: 1, Episodes: []tmdb.Episode{ep(episodeID, 1, "2008-01-20")}}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.s.MarkEpisode(ctx, e.user, episodeID, When{}); err != nil {
			t.Fatal(err)
		}
	}
	if removed, err := e.s.RemoveSeasonWatches(ctx, e.user, markShow, 1); err != nil || removed != 1 {
		t.Fatalf("remove = %d, %v; want 1", removed, err)
	}
	if watches := e.watchTimes(t, 111); len(watches) != 1 {
		t.Fatalf("other season identity's watches = %v", watches)
	}
}
