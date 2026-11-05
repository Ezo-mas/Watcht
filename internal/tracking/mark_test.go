package tracking

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// Show 1399 for marking tests: season 1 has episodes 101-105, season 2 has
// 201, specials (season 0) have 1, and the show lists them all.
const markShow = 1399

func ep(tmdbID, n int, airDate string) tmdb.Episode {
	return tmdb.Episode{TMDBID: tmdbID, ShowTMDBID: markShow, EpisodeNumber: n, AirDate: airDate}
}

func (e *env) storeMarkShow(t *testing.T, seasons map[int][]tmdb.Episode) {
	t.Helper()
	ctx := context.Background()
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	show := tmdb.Show{TMDBID: markShow, Name: "Marked"}
	for n := range 3 {
		if _, ok := seasons[n]; ok {
			show.Seasons = append(show.Seasons, tmdb.SeasonSummary{TMDBID: 9000 + n, SeasonNumber: n})
		}
	}
	if err := c.UpsertShow(ctx, show); err != nil {
		t.Fatalf("UpsertShow: %v", err)
	}
	for n, eps := range seasons {
		for i := range eps {
			eps[i].SeasonNumber = n
		}
		if err := c.UpsertSeason(ctx, markShow, tmdb.Season{TMDBID: 9000 + n, ShowTMDBID: markShow, SeasonNumber: n, Episodes: eps}); err != nil {
			t.Fatalf("UpsertSeason %d: %v", n, err)
		}
	}
}

// watchedAt returns the watch times stored for an episode, by TMDB ID.
func (e *env) watchTimes(t *testing.T, tmdbID int) []string {
	t.Helper()
	rows, err := e.d.Read.Query(`SELECT w.watched_at FROM watch_events w JOIN episodes e ON e.id = w.episode_id
		WHERE e.tmdb_id = ? AND w.user_id = ? ORDER BY w.id`, tmdbID, e.user)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestReleaseTime(t *testing.T) {
	got, ok := ReleaseTime("2008-01-20")
	if !ok || !got.Equal(time.Date(2008, 1, 20, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("ReleaseTime = %v, %v; want 2008-01-20 12:00 UTC", got, ok)
	}
	for _, bad := range []string{"", "2008-02-30", "20 Jan 2008"} {
		if _, ok := ReleaseTime(bad); ok {
			t.Errorf("ReleaseTime(%q) ok", bad)
		}
	}
}

func TestMarkMovie(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	for id, date := range map[int]string{700: "1895-12-28", 701: "", 702: "2026-11-04"} {
		if err := c.UpsertMovie(ctx, tmdb.Movie{TMDBID: id, Title: "M", ReleaseDate: date}); err != nil {
			t.Fatal(err)
		}
	}

	now, err := e.s.MarkMovie(ctx, e.user, 700, When{})
	if err != nil || now.MergedInto != 0 {
		t.Fatalf("MarkMovie now: %+v, %v", now, err)
	}
	at := testNow.Add(-48 * time.Hour)
	if _, err := e.s.MarkMovie(ctx, e.user, 700, When{At: at}); err != nil {
		t.Fatalf("MarkMovie at: %v", err)
	}
	rel, err := e.s.MarkMovie(ctx, e.user, 700, When{ReleaseDate: true})
	if err != nil {
		t.Fatalf("MarkMovie release: %v", err)
	}
	// A second mark just now is the same watch.
	again, err := e.s.MarkMovie(ctx, e.user, 700, When{})
	if err != nil || again.MergedInto != now.ID {
		t.Errorf("second mark now: %+v, %v; want merged into %d", again, err, now.ID)
	}
	watches, err := e.s.MovieWatches(ctx, e.user, 700)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{testNow, at, time.Date(1895, 12, 28, 12, 0, 0, 0, time.UTC)}
	if len(watches) != len(want) {
		t.Fatalf("MovieWatches = %+v, want %d kept watches", watches, len(want))
	}
	for i, w := range watches {
		if !w.WatchedAt.Equal(want[i]) || w.Source != SourceManual {
			t.Errorf("watch %d = %+v, want manual at %v", i, w, want[i])
		}
	}
	if watches[2].ID != rel.ID {
		t.Errorf("release watch ID = %d, want %d", watches[2].ID, rel.ID)
	}
	// The release date mark is stored as the date; the timed ones aren't.
	if rel.WatchedOn != "1895-12-28" || watches[2].WatchedOn != "1895-12-28" || watches[0].WatchedOn != "" || watches[1].WatchedOn != "" {
		t.Errorf("WatchedOn: mark %q, watches %+v", rel.WatchedOn, watches)
	}

	for name, tt := range map[string]struct {
		id   int
		w    When
		want error
	}{
		"no release date": {701, When{ReleaseDate: true}, ErrNoReleaseDate},
		"not released":    {702, When{ReleaseDate: true}, ErrNotReleased},
		"not stored":      {999, When{}, ErrNotFound},
		"future time":     {700, When{At: testNow.Add(time.Hour)}, ErrInvalid},
		"another user":    {700, When{}, nil},
	} {
		user := e.user
		if tt.want == nil {
			user = e.other
		}
		if _, err := e.s.MarkMovie(ctx, user, tt.id, tt.w); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}
	// A movie with no release date can still be marked now.
	if _, err := e.s.MarkMovie(ctx, e.user, 701, When{}); err != nil {
		t.Errorf("no release date, now: %v", err)
	}
}

func TestMarkEpisode(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20"), ep(102, 2, "")}})
	if _, err := e.s.MarkEpisode(ctx, e.user, 101, When{ReleaseDate: true}); err != nil {
		t.Fatalf("MarkEpisode: %v", err)
	}
	if got := e.watchTimes(t, 101); len(got) != 1 || got[0] != "2008-01-20T12:00:00Z" {
		t.Errorf("watch times = %v, want [2008-01-20T12:00:00Z]", got)
	}
	if _, err := e.s.MarkEpisode(ctx, e.user, 102, When{ReleaseDate: true}); !errors.Is(err, ErrNoReleaseDate) {
		t.Errorf("no air date: err = %v, want ErrNoReleaseDate", err)
	}
	if _, err := e.s.MarkEpisode(ctx, e.user, 102, When{}); err != nil {
		t.Errorf("no air date, now: %v", err)
	}
	if _, err := e.s.MarkEpisode(ctx, e.user, 999, When{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("not stored: err = %v, want ErrNotFound", err)
	}
}

func TestMarkSeasonSkipsWatchedUnairedAndMissing(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {
		ep(101, 1, "2008-01-20"),
		ep(102, 2, "2008-01-27"), // watched already
		ep(103, 3, ""),           // no air date: not aired
		ep(104, 4, "2026-11-04"), // tomorrow
		ep(105, 5, "2026-11-03"), // today: aired at noon UTC, before testNow
		ep(106, 6, "2008-02-10"), // dropped below
	}})
	// TMDB stops listing 106.
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	if err := c.UpsertSeason(ctx, markShow, tmdb.Season{TMDBID: 9001, ShowTMDBID: markShow, SeasonNumber: 1, Episodes: []tmdb.Episode{
		ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-27"), ep(103, 3, ""), ep(104, 4, "2026-11-04"), ep(105, 5, "2026-11-03"),
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.MarkEpisode(ctx, e.user, 102, When{At: testNow.Add(-24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	added, err := e.s.MarkSeason(ctx, e.user, markShow, 1, When{ReleaseDate: true})
	if err != nil || added != 2 {
		t.Fatalf("MarkSeason = %d, %v; want 2", added, err)
	}
	want := map[int][]string{
		101: {"2008-01-20T12:00:00Z"},
		102: {"2026-11-02T20:00:00Z"},
		103: nil,
		104: nil,
		105: {"2026-11-03T12:00:00Z"},
		106: nil,
	}
	for id, w := range want {
		if got := e.watchTimes(t, id); len(got) != len(w) || (len(w) == 1 && got[0] != w[0]) {
			t.Errorf("episode %d: watch times = %v, want %v", id, got, w)
		}
	}
	// The air date marks are stored as dates; 102's earlier timed watch isn't.
	for id, w := range map[int]string{101: "2008-01-20", 102: "", 105: "2026-11-03"} {
		var on sql.NullString
		if err := e.d.Read.QueryRow(`SELECT w.watched_on FROM watch_events w JOIN episodes e ON e.id = w.episode_id WHERE e.tmdb_id = ?`, id).Scan(&on); err != nil {
			t.Fatal(err)
		}
		if on.String != w {
			t.Errorf("episode %d: watched_on = %q, want %q", id, on.String, w)
		}
	}

	// Everything aired is watched now, so a second mark adds nothing.
	if added, err := e.s.MarkSeason(ctx, e.user, markShow, 1, When{}); err != nil || added != 0 {
		t.Errorf("second MarkSeason = %d, %v; want 0", added, err)
	}
	// Another user's history is separate.
	if added, err := e.s.MarkSeason(ctx, e.other, markShow, 1, When{}); err != nil || added != 3 {
		t.Errorf("other user MarkSeason = %d, %v; want 3", added, err)
	}
}

func TestMarkSeasonAtOneTime(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-27")}})
	at := testNow.Add(-time.Hour)
	if added, err := e.s.MarkSeason(ctx, e.user, markShow, 1, When{At: at}); err != nil || added != 2 {
		t.Fatalf("MarkSeason = %d, %v; want 2", added, err)
	}
	for _, id := range []int{101, 102} {
		if got := e.watchTimes(t, id); len(got) != 1 || got[0] != "2026-11-03T19:00:00Z" {
			t.Errorf("episode %d: %v, want [2026-11-03T19:00:00Z]", id, got)
		}
	}
}

func TestMarkSeasonNotFound(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20")}, 2: {ep(201, 1, "2009-03-08")}})
	// The show stops listing season 2.
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	if err := c.UpsertShow(ctx, tmdb.Show{TMDBID: markShow, Seasons: []tmdb.SeasonSummary{{TMDBID: 9001, SeasonNumber: 1}}}); err != nil {
		t.Fatal(err)
	}
	for name, tt := range map[string][2]int{"hidden season": {markShow, 2}, "no such season": {markShow, 7}, "no such show": {5, 1}} {
		if _, err := e.s.MarkSeason(ctx, e.user, tt[0], tt[1], When{}); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if n := e.count(t); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

func TestMarkShowSkipsSpecialsAndHiddenSeasons(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{
		0: {ep(1, 1, "2008-01-01")},
		1: {ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-27")},
		2: {ep(201, 1, "2009-03-08")},
	})
	if added, err := e.s.MarkShow(ctx, e.user, markShow, When{}); err != nil || added != 3 {
		t.Fatalf("MarkShow = %d, %v; want 3", added, err)
	}
	if got := e.watchTimes(t, 1); len(got) != 0 {
		t.Errorf("special was marked: %v", got)
	}
	// Specials are marked as their own season.
	if added, err := e.s.MarkSeason(ctx, e.user, markShow, 0, When{}); err != nil || added != 1 {
		t.Errorf("MarkSeason(0) = %d, %v; want 1", added, err)
	}
	if _, err := e.s.MarkShow(ctx, e.user, 5, When{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such show: err = %v, want ErrNotFound", err)
	}

	// A hidden season isn't marked.
	e2 := newEnv(t)
	e2.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20")}, 2: {ep(201, 1, "2009-03-08")}})
	c := catalog.New(e2.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	if err := c.UpsertShow(ctx, tmdb.Show{TMDBID: markShow, Seasons: []tmdb.SeasonSummary{{TMDBID: 9001, SeasonNumber: 1}}}); err != nil {
		t.Fatal(err)
	}
	if added, err := e2.s.MarkShow(ctx, e2.user, markShow, When{}); err != nil || added != 1 {
		t.Errorf("MarkShow with a hidden season = %d, %v; want 1", added, err)
	}
}

func TestMarkSeasonRejectsFutureTime(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-27")}})
	// A future time is refused, and nothing is stored.
	if _, err := e.s.MarkSeason(ctx, e.user, markShow, 1, When{At: testNow.Add(time.Hour)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

func TestMarkRefusesWhileEpisodesArePending(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{0: {ep(1, 1, "2008-01-01")}, 1: {ep(101, 1, "2008-01-20")}})
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	// The show now lists season 2, whose episodes aren't fetched yet.
	show := tmdb.Show{TMDBID: markShow, Seasons: []tmdb.SeasonSummary{
		{TMDBID: 9000, SeasonNumber: 0}, {TMDBID: 9001, SeasonNumber: 1}, {TMDBID: 9002, SeasonNumber: 2},
	}}
	if err := c.UpsertShow(ctx, show); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.MarkShow(ctx, e.user, markShow, When{}); !errors.Is(err, ErrEpisodesPending) {
		t.Errorf("MarkShow: err = %v, want ErrEpisodesPending", err)
	}
	if _, err := e.s.MarkSeason(ctx, e.user, markShow, 2, When{}); !errors.Is(err, ErrEpisodesPending) {
		t.Errorf("MarkSeason(2): err = %v, want ErrEpisodesPending", err)
	}
	if n := e.count(t); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
	// A fetched season can still be marked on its own.
	if added, err := e.s.MarkSeason(ctx, e.user, markShow, 1, When{}); err != nil || added != 1 {
		t.Errorf("MarkSeason(1) = %d, %v; want 1", added, err)
	}
	// Unfetched specials (8000 now has the lowest ID for season 0) and an
	// unfetched higher-ID duplicate of season 1 aren't part of a show mark,
	// so they don't block the one below.
	show.Seasons = append(show.Seasons, tmdb.SeasonSummary{TMDBID: 8000, SeasonNumber: 0}, tmdb.SeasonSummary{TMDBID: 9201, SeasonNumber: 1})
	if err := c.UpsertShow(ctx, show); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.MarkSeason(ctx, e.user, markShow, 0, When{}); !errors.Is(err, ErrEpisodesPending) {
		t.Errorf("MarkSeason(0) on unfetched specials: err = %v, want ErrEpisodesPending", err)
	}
	// Once season 2 is stored (empty), the show can be marked.
	if err := c.UpsertSeason(ctx, markShow, tmdb.Season{TMDBID: 9002, ShowTMDBID: markShow, SeasonNumber: 2}); err != nil {
		t.Fatal(err)
	}
	if added, err := e.s.MarkShow(ctx, e.user, markShow, When{}); err != nil || added != 0 {
		t.Errorf("MarkShow after sync = %d, %v; want 0", added, err)
	}
}

func TestMarkShowUsesTheSeasonThePageShows(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	c := catalog.New(e.d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	// TMDB lists two seasons numbered 1; the page and MarkSeason use the one
	// with the lower TMDB ID, and so does MarkShow.
	show := tmdb.Show{TMDBID: markShow, Seasons: []tmdb.SeasonSummary{{TMDBID: 9001, SeasonNumber: 1}, {TMDBID: 9101, SeasonNumber: 1}}}
	if err := c.UpsertShow(ctx, show); err != nil {
		t.Fatal(err)
	}
	for id, epID := range map[int]int{9001: 101, 9101: 111} {
		if err := c.UpsertSeason(ctx, markShow, tmdb.Season{TMDBID: id, ShowTMDBID: markShow, SeasonNumber: 1, Episodes: []tmdb.Episode{ep(epID, 1, "2008-01-20")}}); err != nil {
			t.Fatal(err)
		}
	}
	if added, err := e.s.MarkShow(ctx, e.user, markShow, When{}); err != nil || added != 1 {
		t.Fatalf("MarkShow = %d, %v; want 1", added, err)
	}
	if got := e.watchTimes(t, 111); len(got) != 0 {
		t.Errorf("the other season's episode was marked: %v", got)
	}
}
