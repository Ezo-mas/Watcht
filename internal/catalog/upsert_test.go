package catalog

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestUpsertMovieStoresEveryField(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatalf("UpsertMovie: %v", err)
	}
	got, err := e.s.Movie(ctx, 603)
	if err != nil {
		t.Fatalf("Movie: %v", err)
	}
	if !got.MetadataUpdatedAt.Equal(testNow) {
		t.Errorf("MetadataUpdatedAt = %v, want %v", got.MetadataUpdatedAt, testNow)
	}
	got.MetadataUpdatedAt = time.Time{}
	want := Movie{
		TMDBID:           603,
		IMDbID:           "tt0133093",
		Title:            "The Matrix",
		OriginalTitle:    "The Matrix",
		OriginalLanguage: "en",
		Overview:         "A computer hacker learns the truth about reality.",
		Tagline:          "Welcome to the Real World.",
		Status:           "Released",
		PosterPath:       "/poster.jpg",
		BackdropPath:     "/backdrop.jpg",
		ReleaseDate:      "1999-03-31",
		Runtime:          136,
		Genres:           []Genre{{TMDBID: 28, Name: "Action"}, {TMDBID: 878, Name: "Science Fiction"}},
		VoteAverage:      8.2,
		VoteCount:        29114,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Movie = %+v\nwant %+v", got, want)
	}
}

func TestUpsertMovieUnknownValuesAreNull(t *testing.T) {
	tests := []struct {
		name   string
		column string
		unset  func(*tmdb.Movie)
		field  func(Movie) any
		zero   any
	}{
		{
			name:   "IMDb ID",
			column: "imdb_id",
			unset:  func(m *tmdb.Movie) { m.IMDbID = "" },
			field:  func(m Movie) any { return m.IMDbID },
			zero:   "",
		},
		{
			name:   "release date",
			column: "release_date",
			unset:  func(m *tmdb.Movie) { m.ReleaseDate = "" },
			field:  func(m Movie) any { return m.ReleaseDate },
			zero:   "",
		},
		{
			name:   "runtime",
			column: "runtime",
			unset:  func(m *tmdb.Movie) { m.Runtime = 0 },
			field:  func(m Movie) any { return m.Runtime },
			zero:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			m := testMovie()
			tt.unset(&m)
			if err := e.s.UpsertMovie(ctx, m); err != nil {
				t.Fatalf("UpsertMovie: %v", err)
			}
			if n := e.int64Of(t, "SELECT count(*) FROM movies WHERE tmdb_id = 603 AND "+tt.column+" IS NULL"); n != 1 {
				t.Errorf("%s is not NULL in the database", tt.column)
			}
			got, err := e.s.Movie(ctx, 603)
			if err != nil {
				t.Fatalf("Movie: %v", err)
			}
			if tt.field(got) != tt.zero {
				t.Errorf("%s = %v, want %v", tt.column, tt.field(got), tt.zero)
			}
		})
	}
}

func TestUpsertMovieUpdatesInPlace(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatalf("UpsertMovie: %v", err)
	}
	id := e.int64Of(t, "SELECT id FROM movies WHERE tmdb_id = 603")
	created := e.textOf(t, "SELECT created_at FROM movies WHERE tmdb_id = 603").String

	// A day later TMDB has a new title and vote count, drops Action, and
	// renames Science Fiction.
	e.at(24)
	m := testMovie()
	m.Title = "The Matrix (Remastered)"
	m.VoteCount = 30000
	m.Genres = []tmdb.Genre{{ID: 878, Name: "Sci-Fi"}}
	if err := e.s.UpsertMovie(ctx, m); err != nil {
		t.Fatalf("UpsertMovie again: %v", err)
	}

	if got := e.int64Of(t, "SELECT id FROM movies WHERE tmdb_id = 603"); got != id {
		t.Errorf("id = %d, want %d (kept)", got, id)
	}
	if got := e.textOf(t, "SELECT created_at FROM movies WHERE tmdb_id = 603").String; got != created {
		t.Errorf("created_at = %s, want %s (kept)", got, created)
	}
	got, err := e.s.Movie(ctx, 603)
	if err != nil {
		t.Fatalf("Movie: %v", err)
	}
	if got.Title != "The Matrix (Remastered)" || got.VoteCount != 30000 {
		t.Errorf("Title, VoteCount = %q, %d; want the new values", got.Title, got.VoteCount)
	}
	if want := testNow.Add(24 * time.Hour); !got.MetadataUpdatedAt.Equal(want) {
		t.Errorf("MetadataUpdatedAt = %v, want %v", got.MetadataUpdatedAt, want)
	}
	if want := []Genre{{TMDBID: 878, Name: "Sci-Fi"}}; !reflect.DeepEqual(got.Genres, want) {
		t.Errorf("Genres = %+v, want %+v", got.Genres, want)
	}
	// The dropped genre's row stays, and only its link goes.
	if n := e.int64Of(t, "SELECT count(*) FROM genres WHERE tmdb_id = 28"); n != 1 {
		t.Errorf("genre 28 rows = %d, want 1", n)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM movie_genres WHERE movie_id = ?", id); n != 1 {
		t.Errorf("movie_genres links = %d, want 1", n)
	}
	if name := e.textOf(t, "SELECT name FROM genres WHERE tmdb_id = 878").String; name != "Sci-Fi" {
		t.Errorf("genre 878 name = %q, want Sci-Fi", name)
	}
}

func TestSharedExternalIDsStoreBoth(t *testing.T) {
	ctx := context.Background()
	t.Run("movies share an IMDb ID", func(t *testing.T) {
		e := newEnv(t)
		first := testMovie()
		second := testMovie()
		second.TMDBID = 604
		second.Title = "Another Entry"
		for _, m := range []tmdb.Movie{first, second} {
			if err := e.s.UpsertMovie(ctx, m); err != nil {
				t.Fatalf("UpsertMovie(%d): %v", m.TMDBID, err)
			}
		}
		for _, id := range []int{603, 604} {
			got, err := e.s.Movie(ctx, id)
			if err != nil || got.IMDbID != "tt0133093" {
				t.Errorf("Movie(%d) = %q, %v; want IMDb tt0133093", id, got.IMDbID, err)
			}
		}
	})
	t.Run("shows share a TVDB ID", func(t *testing.T) {
		e := newEnv(t)
		first := testShow()
		second := testShow()
		second.TMDBID = 1397
		second.Name = "Another Entry"
		for _, s := range []tmdb.Show{first, second} {
			if err := e.s.UpsertShow(ctx, s); err != nil {
				t.Fatalf("UpsertShow(%d): %v", s.TMDBID, err)
			}
		}
		if n := e.int64Of(t, "SELECT count(*) FROM shows WHERE tvdb_id = 81189"); n != 2 {
			t.Errorf("shows with TVDB 81189 = %d, want 2", n)
		}
	})
}

func TestShowSeasonsGoMissingAndReturn(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	all := []tmdb.SeasonSummary{summary(3577, 0, 9), summary(3572, 1, 7), summary(3573, 2, 13)}
	e.storeShow(t, all...)

	// At hour 1 TMDB stops listing season 2.
	e.at(1)
	e.storeShow(t, all[:2]...)
	since := e.textOf(t, "SELECT missing_since FROM seasons WHERE tmdb_id = 3573").String
	if want := db.Timestamp(testNow.Add(time.Hour)); since != want {
		t.Errorf("missing_since = %s, want %s", since, want)
	}
	show, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got := seasonNumbers(show.Seasons); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("listed seasons = %v, want [0 1]", got)
	}

	// Still missing at hour 2, and the first time is kept.
	e.at(2)
	e.storeShow(t, all[:2]...)
	if got := e.textOf(t, "SELECT missing_since FROM seasons WHERE tmdb_id = 3573").String; got != since {
		t.Errorf("missing_since = %s, want the first time %s", got, since)
	}

	// Listed again at hour 3: back, and no longer missing.
	e.at(3)
	e.storeShow(t, all...)
	if n := e.int64Of(t, "SELECT count(*) FROM seasons WHERE tmdb_id = 3573 AND missing_since IS NULL"); n != 1 {
		t.Errorf("season 2 is still missing after it was listed again")
	}
	show, err = e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got := seasonNumbers(show.Seasons); !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("listed seasons = %v, want [0 1 2]", got)
	}
}

func TestSeasonSummaryKeepsStoredOverview(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 7))
	e.upsertSeason(t, testSeason(3572, 1))

	// The show's list has no overview, so the one from the season response stays.
	e.at(1)
	e.storeShow(t, summary(3572, 1, 8))
	show, err := e.s.Show(context.Background(), 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if len(show.Seasons) != 1 {
		t.Fatalf("seasons = %d, want 1", len(show.Seasons))
	}
	if got := show.Seasons[0]; got.Overview != "About season 1" || got.EpisodeCount != 8 {
		t.Errorf("season = overview %q, count %d; want the stored overview and count 8", got.Overview, got.EpisodeCount)
	}
}

func TestUpsertSeasonNeedsStoredShow(t *testing.T) {
	e := newEnv(t)
	err := e.s.UpsertSeason(context.Background(), 1396, testSeason(3572, 1, testEpisode(101, 1)))
	if !errors.Is(err, ErrNotStored) {
		t.Fatalf("UpsertSeason error = %v, want ErrNotStored", err)
	}
	for _, table := range []string{"shows", "seasons", "episodes"} {
		if n := e.int64Of(t, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%s has %d rows after a rejected season, want 0", table, n)
		}
	}
}

func TestUpsertShowUnknownValuesAreNull(t *testing.T) {
	tests := []struct {
		name   string
		column string
		unset  func(*tmdb.Show)
		field  func(Show) any
		zero   any
	}{
		{
			name:   "IMDb ID",
			column: "imdb_id",
			unset:  func(s *tmdb.Show) { s.IMDbID = "" },
			field:  func(s Show) any { return s.IMDbID },
			zero:   "",
		},
		{
			name:   "TVDB ID",
			column: "tvdb_id",
			unset:  func(s *tmdb.Show) { s.TVDBID = 0 },
			field:  func(s Show) any { return s.TVDBID },
			zero:   0,
		},
		{
			name:   "first air date",
			column: "first_air_date",
			unset:  func(s *tmdb.Show) { s.FirstAirDate = "" },
			field:  func(s Show) any { return s.FirstAirDate },
			zero:   "",
		},
		{
			name:   "last air date",
			column: "last_air_date",
			unset:  func(s *tmdb.Show) { s.LastAirDate = "" },
			field:  func(s Show) any { return s.LastAirDate },
			zero:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			show := testShow()
			tt.unset(&show)
			if err := e.s.UpsertShow(ctx, show); err != nil {
				t.Fatalf("UpsertShow: %v", err)
			}
			if n := e.int64Of(t, "SELECT count(*) FROM shows WHERE tmdb_id = 1396 AND "+tt.column+" IS NULL"); n != 1 {
				t.Errorf("%s is not NULL in the database", tt.column)
			}
			got, err := e.s.Show(ctx, 1396)
			if err != nil {
				t.Fatalf("Show: %v", err)
			}
			if tt.field(got) != tt.zero {
				t.Errorf("%s = %v, want %v", tt.column, tt.field(got), tt.zero)
			}
		})
	}
}

func TestUpsertShowUpdatesInPlace(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t)
	id := e.int64Of(t, "SELECT id FROM shows WHERE tmdb_id = 1396")
	created := e.textOf(t, "SELECT created_at FROM shows WHERE tmdb_id = 1396").String

	// A day later TMDB has a new name and overview, and the show is back in
	// production with a new vote count.
	e.at(24)
	show := testShow()
	show.Name = "Breaking Bad (Remastered)"
	show.Overview = "A rewritten overview."
	show.Status = "Returning Series"
	show.InProduction = true
	show.VoteCount = 13000
	if err := e.s.UpsertShow(ctx, show); err != nil {
		t.Fatalf("UpsertShow again: %v", err)
	}

	if got := e.int64Of(t, "SELECT id FROM shows WHERE tmdb_id = 1396"); got != id {
		t.Errorf("id = %d, want %d (kept)", got, id)
	}
	if got := e.textOf(t, "SELECT created_at FROM shows WHERE tmdb_id = 1396").String; got != created {
		t.Errorf("created_at = %s, want %s (kept)", got, created)
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.Name != "Breaking Bad (Remastered)" || got.Overview != "A rewritten overview." ||
		got.Status != "Returning Series" || !got.InProduction || got.VoteCount != 13000 {
		t.Errorf("Name, Overview, Status, InProduction, VoteCount = %q, %q, %q, %v, %d; want the new values",
			got.Name, got.Overview, got.Status, got.InProduction, got.VoteCount)
	}
	if want := testNow.Add(24 * time.Hour); !got.MetadataUpdatedAt.Equal(want) {
		t.Errorf("MetadataUpdatedAt = %v, want %v", got.MetadataUpdatedAt, want)
	}
}

// A show's genres are exactly those of its latest response. A genre TMDB drops
// is unlinked, and a new one is linked.
func TestShowGenresAreReplacedExactly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t)
	id := e.int64Of(t, "SELECT id FROM shows WHERE tmdb_id = 1396")

	// A day later TMDB drops Crime and adds Mystery.
	e.at(24)
	show := testShow()
	show.Genres = []tmdb.Genre{{ID: 18, Name: "Drama"}, {ID: 9648, Name: "Mystery"}}
	if err := e.s.UpsertShow(ctx, show); err != nil {
		t.Fatalf("UpsertShow again: %v", err)
	}

	got, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if want := []Genre{{TMDBID: 18, Name: "Drama"}, {TMDBID: 9648, Name: "Mystery"}}; !reflect.DeepEqual(got.Genres, want) {
		t.Errorf("Show genres = %+v, want %+v", got.Genres, want)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM show_genres WHERE show_id = ?", id); n != 2 {
		t.Errorf("show_genres links = %d, want 2", n)
	}
	// The dropped genre's row stays, and only its link goes.
	if n := e.int64Of(t, "SELECT count(*) FROM genres WHERE tmdb_id = 80"); n != 1 {
		t.Errorf("genre 80 rows = %d, want 1", n)
	}
}

func TestUpsertEpisodeUpdatesInPlace(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))
	id := e.episodeIDs(t)[101]
	created := e.textOf(t, "SELECT created_at FROM episodes WHERE tmdb_id = 101").String

	// A day later TMDB renames the episode and rewrites its overview.
	e.at(24)
	ep := testEpisode(101, 1)
	ep.Name = "Pilot (Director's Cut)"
	ep.Overview = "A rewritten overview."
	e.upsertSeason(t, testSeason(3572, 1, ep))

	if got := e.episodeIDs(t)[101]; got != id {
		t.Errorf("episode 101: id = %d, want %d (kept)", got, id)
	}
	if got := e.textOf(t, "SELECT created_at FROM episodes WHERE tmdb_id = 101").String; got != created {
		t.Errorf("created_at = %s, want %s (kept)", got, created)
	}
	if got := e.textOf(t, "SELECT metadata_updated_at FROM episodes WHERE tmdb_id = 101").String; got != db.Timestamp(testNow.Add(24*time.Hour)) {
		t.Errorf("metadata_updated_at = %s, want the day-later time", got)
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || len(eps) != 1 {
		t.Fatalf("SeasonEpisodes = %v, %v; want one episode", tmdbIDs(eps), err)
	}
	if eps[0].Name != "Pilot (Director's Cut)" || eps[0].Overview != "A rewritten overview." {
		t.Errorf("episode = name %q, overview %q; want the new values", eps[0].Name, eps[0].Overview)
	}
}

// TMDB sends a runtime of 0 for an episode it doesn't know, and that is stored
// as NULL, not as 0.
func TestEpisodeUnknownRuntimeIsNull(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 1))
	ep := testEpisode(101, 1)
	ep.Runtime = 0
	e.upsertSeason(t, testSeason(3572, 1, ep))
	if n := e.int64Of(t, "SELECT count(*) FROM episodes WHERE tmdb_id = 101 AND runtime IS NULL"); n != 1 {
		t.Errorf("runtime of episode 101 is not NULL in the database")
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || len(eps) != 1 || eps[0].Runtime != 0 {
		t.Errorf("SeasonEpisodes = %v, %v; want one episode with runtime 0", tmdbIDs(eps), err)
	}
}
