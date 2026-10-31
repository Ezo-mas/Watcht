package catalog

import (
	"context"
	"slices"
	"testing"
)

// The episode identity rule: an episode is matched by its TMDB episode ID.
// Its season and numbers are display data that TMDB can change.

func TestRenumberingKeepsEpisodeIDs(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 3))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2), testEpisode(103, 3)))
	before := e.episodeIDs(t)

	// TMDB swaps episodes 2 and 3. Both rows are updated in one transaction.
	e.at(1)
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 3), testEpisode(103, 2)))

	after := e.episodeIDs(t)
	for _, tmdbID := range []int{101, 102, 103} {
		if after[tmdbID] != before[tmdbID] {
			t.Errorf("episode %d: id = %d, want %d (kept)", tmdbID, after[tmdbID], before[tmdbID])
		}
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil {
		t.Fatalf("SeasonEpisodes: %v", err)
	}
	if got := tmdbIDs(eps); !slices.Equal(got, []int{101, 103, 102}) {
		t.Fatalf("episodes in order = %v, want [101 103 102]", got)
	}
	if eps[2].EpisodeNumber != 3 || eps[1].EpisodeNumber != 2 {
		t.Errorf("numbers after the swap = %d and %d, want 3 and 2", eps[2].EpisodeNumber, eps[1].EpisodeNumber)
	}
}

func TestMovedEpisodeEndsTheSameInEitherOrder(t *testing.T) {
	// Episode 201 is episode 5 of season 1. TMDB moves it to season 0, as
	// special 2, and season 1's new list doesn't have it.
	orders := []struct {
		name string
		// seasons is the order the two seasons are fetched in, by number.
		seasons []int
	}{
		{"season 1 then season 0", []int{1, 0}},
		{"season 0 then season 1", []int{0, 1}},
	}
	snapshots := map[string]string{}
	for _, order := range orders {
		t.Run(order.name, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			e.storeShow(t, summary(3577, 0, 2), summary(3572, 1, 2))
			e.upsertSeason(t, testSeason(3572, 1, testEpisode(201, 5), testEpisode(202, 6)))
			e.upsertSeason(t, testSeason(3577, 0, testEpisode(204, 1)))
			id := e.episodeIDs(t)[201]

			e.at(1)
			season1 := testSeason(3572, 1, testEpisode(202, 6))
			season0 := testSeason(3577, 0, testEpisode(204, 1), testEpisode(201, 2))
			for _, n := range order.seasons {
				if n == 1 {
					e.upsertSeason(t, season1)
				} else {
					e.upsertSeason(t, season0)
				}
			}

			if got := e.episodeIDs(t)[201]; got != id {
				t.Errorf("episode 201: id = %d, want %d (kept)", got, id)
			}
			if n := e.int64Of(t, "SELECT season_number FROM episodes WHERE tmdb_id = 201"); n != 0 {
				t.Errorf("episode 201: season_number = %d, want 0", n)
			}
			if n := e.int64Of(t, "SELECT episode_number FROM episodes WHERE tmdb_id = 201"); n != 2 {
				t.Errorf("episode 201: episode_number = %d, want 2", n)
			}
			if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{202}) {
				t.Errorf("season 1 episodes = %v, %v; want only 202", tmdbIDs(eps), err)
			}
			if eps, err := e.s.SeasonEpisodes(ctx, 1396, 0); err != nil || !slices.Equal(tmdbIDs(eps), []int{204, 201}) {
				t.Errorf("season 0 episodes = %v, %v; want [204 201]", tmdbIDs(eps), err)
			}
			snapshots[order.name] = e.snapshot(t)
		})
	}
	if len(snapshots) == len(orders) {
		if a, b := snapshots[orders[0].name], snapshots[orders[1].name]; a != b {
			t.Errorf("end states differ by fetch order:\n%s\nvs\n%s", a, b)
		}
	}
}

func TestDroppedEpisodeIsHiddenAndReturns(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 3))
	full := testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2), testEpisode(103, 3))
	e.upsertSeason(t, full)
	id := e.episodeIDs(t)[102]
	withoutTwo := testSeason(3572, 1, testEpisode(101, 1), testEpisode(103, 3))

	// At hour 1 TMDB no longer lists episode 102.
	e.at(1)
	e.upsertSeason(t, withoutTwo)
	since := e.textOf(t, "SELECT missing_since FROM episodes WHERE tmdb_id = 102").String
	if since == "" {
		t.Fatalf("episode 102 has no missing_since after TMDB dropped it")
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{101, 103}) {
		t.Errorf("SeasonEpisodes after the drop = %v, %v; want [101 103]", tmdbIDs(eps), err)
	}

	// At hour 2 it's still missing, and the first time is kept.
	e.at(2)
	e.upsertSeason(t, withoutTwo)
	if got := e.textOf(t, "SELECT missing_since FROM episodes WHERE tmdb_id = 102").String; got != since {
		t.Errorf("missing_since = %s, want the first time %s", got, since)
	}

	// At hour 3 it's listed again: back, with the same ID and no missing time.
	e.at(3)
	e.upsertSeason(t, full)
	if n := e.int64Of(t, "SELECT count(*) FROM episodes WHERE tmdb_id = 102 AND missing_since IS NULL"); n != 1 {
		t.Errorf("episode 102 is still missing after TMDB listed it again")
	}
	if got := e.episodeIDs(t)[102]; got != id {
		t.Errorf("episode 102: id = %d, want %d (kept)", got, id)
	}
	if eps, err := e.s.SeasonEpisodes(ctx, 1396, 1); err != nil || !slices.Equal(tmdbIDs(eps), []int{101, 102, 103}) {
		t.Errorf("SeasonEpisodes after it returns = %v, %v; want [101 102 103]", tmdbIDs(eps), err)
	}
}

func TestEpisodeNumbersCanCollideTransiently(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 3))

	// TMDB lists two episodes as number 1 in one response. That's stored
	// without error, and the TMDB ID breaks the tie when reading.
	if err := e.s.UpsertSeason(ctx, 1396, testSeason(3572, 1, testEpisode(103, 2), testEpisode(102, 1), testEpisode(101, 1))); err != nil {
		t.Fatalf("UpsertSeason with two episodes numbered 1: %v", err)
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil {
		t.Fatalf("SeasonEpisodes: %v", err)
	}
	if got := tmdbIDs(eps); !slices.Equal(got, []int{101, 102, 103}) {
		t.Errorf("episodes = %v, want [101 102 103]", got)
	}
}

// An episode that TMDB moves to another show keeps its internal ID, and its
// show and season become the new show's. The old season no longer lists it, so
// fetching that season again doesn't mark the episode missing.
func TestEpisodeMovesToAnotherShow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 2))
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1), testEpisode(102, 2)))
	e.storeOtherShow(t, summary(4001, 1, 1))
	id := e.episodeIDs(t)[101]
	wantShow := e.int64Of(t, "SELECT id FROM shows WHERE tmdb_id = 1399")
	wantSeason := e.int64Of(t, "SELECT id FROM seasons WHERE tmdb_id = 4001")

	// At hour 1 show 1399 lists episode 101 in its season 1.
	e.at(1)
	e.upsertOtherSeason(t, testSeason(4001, 1, testEpisode(101, 1)))
	if got := e.episodeIDs(t)[101]; got != id {
		t.Errorf("episode 101: id = %d, want %d (kept)", got, id)
	}
	// Checked in SQL, since no read returns an episode's show.
	if got := e.int64Of(t, "SELECT show_id FROM episodes WHERE tmdb_id = 101"); got != wantShow {
		t.Errorf("episode 101: show_id = %d, want show 1399 (%d)", got, wantShow)
	}
	if got := e.int64Of(t, "SELECT season_id FROM episodes WHERE tmdb_id = 101"); got != wantSeason {
		t.Errorf("episode 101: season_id = %d, want season 4001 (%d)", got, wantSeason)
	}

	// At hour 2 show 1396's season 1 is fetched without episode 101. The episode
	// is linked to show 1399's season now, so it isn't marked missing.
	e.at(2)
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(102, 2)))
	if n := e.int64Of(t, "SELECT count(*) FROM episodes WHERE tmdb_id = 101 AND missing_since IS NULL"); n != 1 {
		t.Errorf("episode 101 is missing, want it listed under show 1399")
	}
	if got := e.int64Of(t, "SELECT show_id FROM episodes WHERE tmdb_id = 101"); got != wantShow {
		t.Errorf("after show 1396's season was fetched: show_id = %d, want %d", got, wantShow)
	}
	eps, err := e.s.SeasonEpisodes(ctx, 1399, 1)
	if err != nil || !slices.Equal(tmdbIDs(eps), []int{101}) {
		t.Errorf("show 1399 season 1 episodes = %v, %v; want [101]", tmdbIDs(eps), err)
	}
	eps, err = e.s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil || !slices.Equal(tmdbIDs(eps), []int{102}) {
		t.Errorf("show 1396 season 1 episodes = %v, %v; want [102]", tmdbIDs(eps), err)
	}
}
