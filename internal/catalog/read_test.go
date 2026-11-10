package catalog

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestReadsReportNotFound(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.s.Movie(ctx, 603); !errors.Is(err, ErrNotFound) {
		t.Errorf("Movie(603) on an empty catalog = %v, want ErrNotFound", err)
	}
	if _, err := e.s.Show(ctx, 1396); !errors.Is(err, ErrNotFound) {
		t.Errorf("Show(1396) on an empty catalog = %v, want ErrNotFound", err)
	}
	if _, err := e.s.SeasonEpisodes(ctx, 1396, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("SeasonEpisodes on an empty catalog = %v, want ErrNotFound", err)
	}

	e.storeShow(t, summary(3572, 1, 1))
	if _, err := e.s.SeasonEpisodes(ctx, 1396, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("SeasonEpisodes for a season the show doesn't list = %v, want ErrNotFound", err)
	}
	// A listed season with no episodes stored yet is found, with none in it.
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || len(eps) != 0 {
		t.Errorf("SeasonEpisodes for a listed season with no episodes = %v, %v; want none and no error", tmdbIDs(eps), err)
	}
}

func TestReadsAreOrdered(t *testing.T) {
	ctx := context.Background()
	t.Run("genres by name", func(t *testing.T) {
		e := newEnv(t)
		m := testMovie()
		// The IDs sort differently from the names, so this checks the name order.
		m.Genres = []tmdb.Genre{{ID: 878, Name: "Science Fiction"}, {ID: 28, Name: "Action"}, {ID: 10, Name: "Documentary"}}
		if err := e.s.UpsertMovie(ctx, m); err != nil {
			t.Fatalf("UpsertMovie: %v", err)
		}
		got, err := e.s.Movie(ctx, 603)
		if err != nil {
			t.Fatalf("Movie: %v", err)
		}
		want := []Genre{{TMDBID: 28, Name: "Action"}, {TMDBID: 10, Name: "Documentary"}, {TMDBID: 878, Name: "Science Fiction"}}
		if !reflect.DeepEqual(got.Genres, want) {
			t.Errorf("movie genres = %+v, want %+v", got.Genres, want)
		}

		e.storeShow(t, summary(3572, 1, 7))
		show, err := e.s.Show(ctx, 1396)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if want := []Genre{{TMDBID: 80, Name: "Crime"}, {TMDBID: 18, Name: "Drama"}}; !reflect.DeepEqual(show.Genres, want) {
			t.Errorf("show genres = %+v, want %+v", show.Genres, want)
		}
	})
	t.Run("seasons by number, with their episode counts", func(t *testing.T) {
		e := newEnv(t)
		e.storeShow(t, summary(3573, 2, 13), summary(3577, 0, 9), summary(3572, 1, 7))
		show, err := e.s.Show(ctx, 1396)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if nums := seasonNumbers(show.Seasons); !slices.Equal(nums, []int{0, 1, 2}) {
			t.Errorf("seasons = %v, want [0 1 2]", nums)
		}
		if counts := []int{show.Seasons[0].EpisodeCount, show.Seasons[1].EpisodeCount, show.Seasons[2].EpisodeCount}; !slices.Equal(counts, []int{9, 7, 13}) {
			t.Errorf("episode counts = %v, want [9 7 13]", counts)
		}
	})
	t.Run("episodes by number, then TMDB ID", func(t *testing.T) {
		e := newEnv(t)
		e.storeShow(t, summary(3572, 1, 4))
		e.upsertSeason(t, testSeason(3572, 1,
			testEpisode(105, 2), testEpisode(101, 3), testEpisode(103, 1), testEpisode(102, 1)))
		eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
		if err != nil {
			t.Fatalf("SeasonEpisodes: %v", err)
		}
		if got := tmdbIDs(eps); !slices.Equal(got, []int{102, 103, 105, 101}) {
			t.Errorf("episodes = %v, want [102 103 105 101]", got)
		}
	})
}

func TestMissingRowsAreHidden(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 2), summary(3573, 2, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2)))
	e.upsertSeason(t, testSeason(3573, 2, testEpisode(201, 1)))

	// At hour 1, season 2 leaves the list and episode 102 leaves season 1.
	e.at(1)
	e.storeShow(t, summary(3572, 1, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))

	show, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if nums := seasonNumbers(show.Seasons); !slices.Equal(nums, []int{1}) {
		t.Errorf("listed seasons = %v, want [1]", nums)
	}
	if _, err := e.s.SeasonEpisodes(ctx, 1396, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("SeasonEpisodes for a missing season = %v, want ErrNotFound", err)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{101}) {
		t.Errorf("SeasonEpisodes(1) = %v, %v; want [101]", tmdbIDs(eps), err)
	}
}

// TMDB's list can give two seasons the same number. The number then means the
// listed season with the lowest TMDB season ID, and Show lists both.
func TestRepeatedSeasonNumberResolvesToLowestTMDBID(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(300, 1, 1), summary(200, 1, 1))
	e.upsertSeason(t, testSeason(300, 1, testEpisode(3001, 1)))
	e.upsertSeason(t, testSeason(200, 1, testEpisode(2001, 1)))

	show, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	var seasonIDs []int
	for _, s := range show.Seasons {
		seasonIDs = append(seasonIDs, s.TMDBID)
		if s.SeasonNumber != 1 {
			t.Errorf("season %d has number %d, want 1", s.TMDBID, s.SeasonNumber)
		}
	}
	if !slices.Equal(seasonIDs, []int{200, 300}) {
		t.Errorf("Show lists seasons %v, want both, by TMDB ID [200 300]", seasonIDs)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{2001}) {
		t.Errorf("SeasonEpisodes(1) = %v, %v; want the season 200 episodes [2001]", tmdbIDs(eps), err)
	}

	// Season 200 leaves the list, so the number now means season 300.
	e.at(1)
	e.storeShow(t, summary(300, 1, 1))
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{3001}) {
		t.Errorf("SeasonEpisodes(1) after 200 left = %v, %v; want [3001]", tmdbIDs(eps), err)
	}
}

func TestSeasonEpisodesPending(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 7))
	season, episodes, err := e.s.Season(ctx, 1396, 1)
	if err != nil || !season.EpisodesPending || episodes != nil {
		t.Fatalf("unfetched season = %+v, %v, %v; want pending with nil episodes", season, episodes, err)
	}
	show, err := e.s.Show(ctx, 1396)
	if err != nil || !show.Seasons[0].EpisodesPending {
		t.Fatalf("show before fetch = %+v, %v", show, err)
	}

	// A fetched season with no episodes is distinct from an unfetched season.
	e.upsertSeason(t, testSeason(3572, 1))
	season, episodes, err = e.s.Season(ctx, 1396, 1)
	if err != nil || season.EpisodesPending || episodes == nil || len(episodes) != 0 {
		t.Fatalf("fetched empty season = %+v, %v, %v; want not pending with []", season, episodes, err)
	}
	show, err = e.s.Show(ctx, 1396)
	if err != nil || show.Seasons[0].EpisodesPending {
		t.Fatalf("show after fetch = %+v, %v", show, err)
	}
	if _, _, err := e.s.Season(ctx, 1396, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown season = %v, want ErrNotFound", err)
	}
}

// ShowEpisodes lists the episodes of every visible season, specials included,
// by season number, then episode number, then TMDB ID.
func TestShowEpisodesAreOrderedAcrossSeasons(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	// Episode 1 is in season 0 and in season 1, and the specials come first.
	e.storeShow(t, summary(3573, 2, 1), summary(3572, 1, 3), summary(3577, 0, 2))
	e.upsertSeason(t, testSeason(3573, 2, testEpisode(201, 1)))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(105, 2), testEpisode(101, 1), testEpisode(103, 2)))
	e.upsertSeason(t, testSeason(3577, 0, testEpisode(9002, 2), testEpisode(9001, 1)))

	eps, pending, err := e.s.ShowEpisodes(ctx, 1396)
	if err != nil || pending {
		t.Fatalf("ShowEpisodes = %v, %v, %v; want fetched episodes", tmdbIDs(eps), pending, err)
	}
	if got := tmdbIDs(eps); !slices.Equal(got, []int{9001, 9002, 101, 103, 105, 201}) {
		t.Errorf("episodes = %v, want [9001 9002 101 103 105 201]", got)
	}
	var numbers []int
	for _, ep := range eps {
		numbers = append(numbers, ep.SeasonNumber)
	}
	if !slices.Equal(numbers, []int{0, 0, 1, 1, 1, 2}) {
		t.Errorf("season numbers = %v, want [0 0 1 1 1 2]", numbers)
	}
}

// TMDB's list can give two seasons the same number. ShowEpisodes uses the
// season that Season returns, the lowest listed TMDB season ID that isn't
// hidden, and only that season decides whether the show is pending.
func TestShowEpisodesUseTheVisibleSeasonOnly(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(300, 1, 1), summary(200, 1, 1))
	e.upsertSeason(t, testSeason(200, 1, testEpisode(2001, 1)))
	// Season 300 is hidden and not fetched, so it doesn't make the show pending.
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || pending || !slices.Equal(tmdbIDs(eps), []int{2001}) {
		t.Fatalf("ShowEpisodes = %v, %v, %v; want the season 200 episodes [2001] and not pending", tmdbIDs(eps), pending, err)
	}
	e.upsertSeason(t, testSeason(300, 1, testEpisode(3001, 1)))
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || pending || !slices.Equal(tmdbIDs(eps), []int{2001}) {
		t.Fatalf("ShowEpisodes after season 300 is fetched = %v, %v, %v; want [2001]", tmdbIDs(eps), pending, err)
	}

	// Season 200 is hidden as not found, so the number means season 300.
	if err := e.s.write.MarkSeasonNotFound(ctx, sqlc.MarkSeasonNotFoundParams{Now: nullString(db.Timestamp(testNow)), TmdbID: 200}); err != nil {
		t.Fatal(err)
	}
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || pending || !slices.Equal(tmdbIDs(eps), []int{3001}) {
		t.Errorf("ShowEpisodes after season 200 is not found = %v, %v, %v; want [3001]", tmdbIDs(eps), pending, err)
	}
}

func TestShowEpisodesExcludeMissing(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 2), summary(3573, 2, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2)))
	e.upsertSeason(t, testSeason(3573, 2, testEpisode(201, 1)))

	// At hour 1, episode 102 leaves season 1 and season 2 leaves the list.
	e.at(1)
	e.storeShow(t, summary(3572, 1, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || pending || !slices.Equal(tmdbIDs(eps), []int{101}) {
		t.Errorf("ShowEpisodes = %v, %v, %v; want [101] and not pending", tmdbIDs(eps), pending, err)
	}
}

func TestShowEpisodesPending(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeShow(t, summary(3577, 0, 1), summary(3572, 1, 7))
	e.upsertSeason(t, testSeason(3577, 0, testEpisode(9001, 1)))
	// Season 1 has no episodes stored yet, so the whole list is pending.
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || !pending || eps != nil {
		t.Fatalf("ShowEpisodes with season 1 unfetched = %v, %v, %v; want pending with nil", tmdbIDs(eps), pending, err)
	}
	// A fetched season with no episodes isn't pending.
	e.upsertSeason(t, testSeason(3572, 1))
	if eps, pending, err := e.s.ShowEpisodes(ctx, 1396); err != nil || pending || eps == nil || !slices.Equal(tmdbIDs(eps), []int{9001}) {
		t.Errorf("ShowEpisodes after season 1 is fetched = %v, %v, %v; want [9001] and not pending", tmdbIDs(eps), pending, err)
	}
}

func TestShowEpisodesNotStoredOrNoSeasons(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	if _, _, err := e.s.ShowEpisodes(ctx, 1396); !errors.Is(err, ErrNotFound) {
		t.Errorf("ShowEpisodes for an unstored show = %v, want ErrNotFound", err)
	}
	// A stored show that lists no seasons has no episodes, and nothing is pending.
	e.storeShow(t)
	eps, pending, err := e.s.ShowEpisodes(ctx, 1396)
	if err != nil || pending || eps == nil || len(eps) != 0 {
		t.Errorf("ShowEpisodes for a show with no seasons = %v, %v, %v; want [] and not pending", tmdbIDs(eps), pending, err)
	}
}
