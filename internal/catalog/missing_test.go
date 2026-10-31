package catalog

import (
	"context"
	"slices"
	"testing"
)

// An empty list of TMDB IDs must still mark everything it doesn't name. The
// NOT IN check reads the list as a JSON array, and an empty array matches
// every row; a null would match none.
func TestEmptyListMarksEverythingMissing(t *testing.T) {
	ctx := context.Background()
	t.Run("show with no seasons", func(t *testing.T) {
		e := newEnv(t)
		e.storeShow(t, summary(3577, 0, 1), summary(3572, 1, 2))
		e.at(1)
		e.storeShow(t)
		if n := e.int64Of(t, "SELECT count(*) FROM seasons WHERE missing_since IS NOT NULL"); n != 2 {
			t.Errorf("missing seasons = %d, want 2", n)
		}
		show, err := e.s.Show(ctx, 1396)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if len(show.Seasons) != 0 {
			t.Errorf("listed seasons = %v, want none", seasonNumbers(show.Seasons))
		}
	})
	t.Run("season with no episodes", func(t *testing.T) {
		e := newEnv(t)
		e.storeShow(t, summary(3572, 1, 2))
		e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2)))
		e.at(1)
		e.upsertSeason(t, testSeason(3572, 1))
		if n := e.int64Of(t, "SELECT count(*) FROM episodes WHERE missing_since IS NOT NULL"); n != 2 {
			t.Errorf("missing episodes = %d, want 2", n)
		}
		eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
		if err != nil || len(eps) != 0 {
			t.Errorf("SeasonEpisodes = %v, %v; want none", tmdbIDs(eps), err)
		}
	})
}

// A show's season list names only that show's seasons, so storing it must not
// mark another show's seasons missing.
func TestOtherShowsSeasonsAreNotMarkedMissing(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 1), summary(3573, 2, 1))
	e.storeOtherShow(t, summary(4001, 1, 1), summary(4002, 2, 1))

	// At hour 1 show 1396 drops season 2. Show 1399's seasons aren't in its
	// list, so they stay listed.
	e.at(1)
	e.storeShow(t, summary(3572, 1, 1))
	if got := e.missingTMDBIDs(t, "seasons"); !slices.Equal(got, []int{3573}) {
		t.Errorf("missing seasons = %v, want [3573]", got)
	}

	// At hour 2 show 1399 drops season 2. Show 1396's season 2 is still missing.
	e.at(2)
	e.storeOtherShow(t, summary(4001, 1, 1))
	if got := e.missingTMDBIDs(t, "seasons"); !slices.Equal(got, []int{3573, 4002}) {
		t.Errorf("missing seasons = %v, want [3573 4002]", got)
	}
}

// An episode can only go missing within its own season. A season's response
// that drops an episode must not touch the episodes of other seasons or shows.
func TestMissingEpisodesStayWithinTheirSeason(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 2), summary(3573, 2, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2)))
	e.upsertSeason(t, testSeason(3573, 2, testEpisode(201, 1)))
	e.storeOtherShow(t, summary(4001, 1, 1))
	e.upsertOtherSeason(t, testSeason(4001, 1, testEpisode(301, 1)))

	// At hour 1 season 1 of show 1396 drops episode 102. Episodes 201 and 301
	// are in other seasons, so they stay listed.
	e.at(1)
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))
	if got := e.missingTMDBIDs(t, "episodes"); !slices.Equal(got, []int{102}) {
		t.Errorf("missing episodes = %v, want [102]", got)
	}
}

// A season's own response means TMDB lists it, so it clears the season's
// missing time, as the show's season list does.
func TestSeasonResponseClearsItsMissingTime(t *testing.T) {
	e := newEnv(t)
	e.storeShow(t, summary(3572, 1, 1))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))

	// At hour 1 the show's list drops season 1.
	e.at(1)
	e.storeShow(t)
	if got := e.missingTMDBIDs(t, "seasons"); !slices.Equal(got, []int{3572}) {
		t.Fatalf("missing seasons = %v, want [3572]", got)
	}

	// At hour 2 TMDB returns season 1 with its episodes, and that clears the mark.
	e.at(2)
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))
	if got := e.missingTMDBIDs(t, "seasons"); len(got) != 0 {
		t.Errorf("missing seasons = %v, want none", got)
	}
	show, err := e.s.Show(context.Background(), 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got := seasonNumbers(show.Seasons); !slices.Equal(got, []int{1}) {
		t.Errorf("listed seasons = %v, want [1]", got)
	}
}
