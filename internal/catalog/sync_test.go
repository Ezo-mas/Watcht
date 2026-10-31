package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestSyncShowStoresEverySeason(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{summary(3572, 1, 2), summary(3577, 0, 1)}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 0}] = testSeason(3577, 0, testEpisode(61, 1))
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2))

	if err := e.s.SyncShow(ctx, 1396); err != nil {
		t.Fatalf("SyncShow: %v", err)
	}
	if want := []string{"1396/1", "1396/0"}; !slices.Equal(e.md.calls, want) {
		t.Errorf("season fetches = %v, want %v", e.md.calls, want)
	}
	got, err := e.s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if nums := seasonNumbers(got.Seasons); !slices.Equal(nums, []int{0, 1}) {
		t.Errorf("seasons = %v, want [0 1]", nums)
	}
	if got.Seasons[0].Overview != "About season 0" {
		t.Errorf("season 0 overview = %q, want the one from its response", got.Seasons[0].Overview)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 0); err != nil || !slices.Equal(tmdbIDs(eps), []int{61}) {
		t.Errorf("season 0 episodes = %v, %v; want [61]", tmdbIDs(eps), err)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{101, 102}) {
		t.Errorf("season 1 episodes = %v, %v; want [101 102]", tmdbIDs(eps), err)
	}
}

func TestSyncShowStopsAtFailedSeason(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	show := testShow()
	show.Seasons = []tmdb.SeasonSummary{
		summary(3577, 0, 1),
		summary(3572, 1, 2),
		summary(3573, 2, 1),
		summary(3575, 3, 1),
	}
	e.md.shows[1396] = show
	e.md.seasons[seasonKey{1396, 0}] = testSeason(3577, 0, testEpisode(61, 1))
	e.md.seasons[seasonKey{1396, 1}] = testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2))
	e.md.seasonErr[seasonKey{1396, 2}] = fmt.Errorf("season 2 of show 1396: %w", providers.ErrNotFound)
	e.md.seasons[seasonKey{1396, 3}] = testSeason(3575, 3, testEpisode(301, 1))

	err := e.s.SyncShow(ctx, 1396)
	if !errors.Is(err, providers.ErrNotFound) {
		t.Fatalf("SyncShow error = %v, want it to match providers.ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "season 2") {
		t.Errorf("SyncShow error = %q, want it to name season 2", err)
	}
	// Season 3 is never asked for: the sync stops at the failure.
	if want := []string{"1396/0", "1396/1", "1396/2"}; !slices.Equal(e.md.calls, want) {
		t.Errorf("season fetches = %v, want %v", e.md.calls, want)
	}
	// Seasons 0 and 1 stay stored with their episodes.
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 0); err != nil || !slices.Equal(tmdbIDs(eps), []int{61}) {
		t.Errorf("season 0 episodes = %v, %v; want [61]", tmdbIDs(eps), err)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{101, 102}) {
		t.Errorf("season 1 episodes = %v, %v; want [101 102]", tmdbIDs(eps), err)
	}
	// Season 3 is listed by its show, but its episodes were never stored.
	if n := e.int64Of(t, "SELECT count(*) FROM seasons WHERE tmdb_id = 3575 AND episodes_updated_at IS NULL"); n != 1 {
		t.Errorf("season 3 has episodes_updated_at set, want it unset")
	}
}

func TestFetchNotFoundWritesNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.s.FetchMovie(ctx, 999); !errors.Is(err, providers.ErrNotFound) {
		t.Errorf("FetchMovie error = %v, want providers.ErrNotFound", err)
	}
	if err := e.s.FetchShow(ctx, 999); !errors.Is(err, providers.ErrNotFound) {
		t.Errorf("FetchShow error = %v, want providers.ErrNotFound", err)
	}
	for _, table := range []string{"movies", "shows"} {
		if n := e.int64Of(t, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%s has %d rows after a 404, want 0", table, n)
		}
	}
}
