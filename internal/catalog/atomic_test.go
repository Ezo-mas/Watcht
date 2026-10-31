package catalog

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// Each store runs in one transaction, so a failure part way through leaves
// nothing of it behind. Each test below fails a store on a later step, after
// the earlier steps have written rows, and checks that those rows rolled back.

func TestUpsertSeasonIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 2))
	season := testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2))
	// The second episode's air date doesn't exist. Its step fails after the
	// season row and the first episode are written.
	season.Episodes[1].AirDate = "2008-02-30"
	err := e.s.UpsertSeason(context.Background(), 1396, season)
	if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("UpsertSeason error = %v, want a CHECK constraint failure", err)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM episodes"); n != 0 {
		t.Errorf("episodes = %d, want 0: the first episode must roll back too", n)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM seasons WHERE tmdb_id = 3572 AND episodes_updated_at IS NULL"); n != 1 {
		t.Errorf("season 3572 has episodes_updated_at set, want it NULL")
	}
}

func TestUpsertMovieIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatalf("UpsertMovie: %v", err)
	}

	// A day later the title changes, and a genre with no TMDB ID fails after the
	// movie row is written. The new title must roll back with it.
	e.at(24)
	m := testMovie()
	m.Title = "The Matrix (Remastered)"
	m.Genres = append(m.Genres, tmdb.Genre{ID: 0, Name: "No ID"})
	err := e.s.UpsertMovie(ctx, m)
	if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("UpsertMovie error = %v, want a CHECK constraint failure", err)
	}
	if got := e.textOf(t, "SELECT title FROM movies WHERE tmdb_id = 603").String; got != "The Matrix" {
		t.Errorf("title = %q, want the stored title", got)
	}
	if got := e.textOf(t, "SELECT metadata_updated_at FROM movies WHERE tmdb_id = 603").String; got != db.Timestamp(testNow) {
		t.Errorf("metadata_updated_at = %s, want %s", got, db.Timestamp(testNow))
	}
	if n := e.int64Of(t, "SELECT count(*) FROM movie_genres"); n != 2 {
		t.Errorf("movie_genres links = %d, want the 2 stored before", n)
	}
}

func TestUpsertShowIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 7))

	// A day later the show is renamed and gets new genres, and season 1 gets a
	// new episode count. The second season of the list has an air date that
	// doesn't exist, so its step fails after all of those are written.
	e.at(24)
	show := testShow()
	show.Name = "Breaking Bad (Remastered)"
	show.Genres = []tmdb.Genre{{ID: 9648, Name: "Mystery"}}
	bad := summary(3573, 2, 1)
	bad.AirDate = "2009-02-29"
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 8), bad}
	err := e.s.UpsertShow(ctx, show)
	if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("UpsertShow error = %v, want a CHECK constraint failure", err)
	}

	got, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.Name != "Breaking Bad" {
		t.Errorf("Name = %q, want the stored name", got.Name)
	}
	if !got.MetadataUpdatedAt.Equal(testNow) {
		t.Errorf("MetadataUpdatedAt = %v, want %v", got.MetadataUpdatedAt, testNow)
	}
	if want := []Genre{{TMDBID: 80, Name: "Crime"}, {TMDBID: 18, Name: "Drama"}}; !reflect.DeepEqual(got.Genres, want) {
		t.Errorf("Genres = %+v, want %+v", got.Genres, want)
	}
	if len(got.Seasons) != 1 || got.Seasons[0].EpisodeCount != 7 {
		t.Errorf("Seasons = %+v, want season 1 with 7 episodes", got.Seasons)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM seasons WHERE tmdb_id = 3573"); n != 0 {
		t.Errorf("season 3573 is stored after the failed write, want none")
	}
}

// A TMDB ID of 0 means the response had none. Storing it would merge unrelated
// rows under one key, so each write fails and the row counts don't change.
// Rolling back earlier rows is covered by the *IsAllOrNothing tests.
func TestIDLessRowsWriteNothing(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		// store makes the write that carries the ID-less row.
		store func(e *env) error
	}{
		{
			name: "movie",
			store: func(e *env) error {
				m := testMovie()
				m.TMDBID = 0
				return e.s.UpsertMovie(ctx, m)
			},
		},
		{
			name: "genre of a movie",
			store: func(e *env) error {
				m := testMovie()
				m.Genres[1].ID = 0
				return e.s.UpsertMovie(ctx, m)
			},
		},
		{
			name: "show",
			store: func(e *env) error {
				show := testShow()
				show.TMDBID = 0
				return e.s.UpsertShow(ctx, show)
			},
		},
		{
			name: "season listed by a show",
			store: func(e *env) error {
				show := testShow()
				show.Seasons = []tmdb.SeasonSummary{summary(0, 1, 1)}
				return e.s.UpsertShow(ctx, show)
			},
		},
		{
			name: "season",
			store: func(e *env) error {
				return e.s.UpsertSeason(ctx, 1396, testSeason(0, 1, testEpisode(101, 1)))
			},
		},
		{
			name: "episode",
			store: func(e *env) error {
				return e.s.UpsertSeason(ctx, 1396, testSeason(3572, 1, testEpisode(0, 1)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.storeShow(t, summary(3572, 1, 1))
			before := e.rowCounts(t)
			err := tt.store(e)
			if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Fatalf("error = %v, want a CHECK constraint failure", err)
			}
			if after := e.rowCounts(t); !reflect.DeepEqual(after, before) {
				t.Errorf("row counts after the failed write = %v, want %v", after, before)
			}
		})
	}
}
