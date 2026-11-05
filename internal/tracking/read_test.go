package tracking

import (
	"context"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

func TestHistoryPagesNewestFirst(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	// Five kept watches, two of them at the same time, plus a merged event
	// and another user's watch, which aren't listed.
	at := testNow.Add(-10 * time.Hour)
	var ids []int64
	for _, ev := range []Event{
		{UserID: e.user, MovieID: e.movie, WatchedAt: at},
		{UserID: e.user, MovieID: e.movie2, WatchedAt: at},
		{UserID: e.user, EpisodeID: e.episode, WatchedAt: at.Add(time.Hour)},
		{UserID: e.user, EpisodeID: e.episode2, WatchedAt: at.Add(-time.Hour)},
		{UserID: e.user, MovieID: e.movie, WatchedAt: at.Add(5 * time.Hour)},
	} {
		ev.Source = SourceManual
		ids = append(ids, e.record(t, ev).ID)
	}
	if r := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: at.Add(time.Minute), Source: SourceManual}); r.MergedInto == 0 {
		t.Fatal("expected a merged event")
	}
	e.record(t, Event{UserID: e.other, MovieID: e.movie, WatchedAt: at, Source: SourceManual})

	// Newest first; equal times by ID, newest first.
	want := []int64{ids[4], ids[2], ids[1], ids[0], ids[3]}
	var got []int64
	var cur Cursor
	for range 10 {
		page, err := e.s.History(ctx, e.user, cur, 2)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, h := range page {
			got = append(got, h.ID)
		}
		last := page[len(page)-1]
		cur = Cursor{WatchedAt: last.WatchedAt, ID: last.ID}
	}
	if len(got) != len(want) {
		t.Fatalf("History IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("History IDs = %v, want %v", got, want)
		}
	}
}

func TestHistoryEntryFields(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{2: {{TMDBID: 201, ShowTMDBID: markShow, EpisodeNumber: 3, Name: "Ep", StillPath: "/s.jpg", AirDate: "2009-03-08", Runtime: 47, VoteAverage: 8.25}}})
	if _, err := e.s.MarkEpisode(ctx, e.user, 201, When{ReleaseDate: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.MarkMovie(ctx, e.user, 603, When{}); err != nil {
		t.Fatal(err)
	}
	h, err := e.s.History(ctx, e.user, Cursor{}, 10)
	if err != nil || len(h) != 2 {
		t.Fatalf("History = %+v, %v; want 2 entries", h, err)
	}
	// The movie has no runtime or votes stored: both are 0.
	if m := h[0].Movie; m == nil || h[0].Episode != nil || m.TMDBID != 603 || m.Title != "Movie" || m.Runtime != 0 || m.VoteAverage != 0 || h[0].Source != SourceManual || !h[0].WatchedAt.Equal(testNow) {
		t.Errorf("movie entry = %+v (movie %+v)", h[0], h[0].Movie)
	}
	wantEp := HistoryEpisode{TMDBID: 201, SeasonNumber: 2, EpisodeNumber: 3, Name: "Ep", StillPath: "/s.jpg", AirDate: "2009-03-08", Runtime: 47, VoteAverage: 8.25, ShowTMDBID: markShow, ShowName: "Marked"}
	if ep := h[1].Episode; ep == nil || h[1].Movie != nil || *ep != wantEp {
		t.Errorf("episode entry = %+v, want %+v", h[1].Episode, wantEp)
	}
}

func TestHistoryLimitIsClamped(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	for i := range MaxHistoryLimit + 1 {
		// 4 hours apart, so none merge.
		e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Duration(i) * 4 * time.Hour), Source: SourceManual})
	}
	for limit, want := range map[int]int{0: 1, -5: 1, 3: 3, 1000: MaxHistoryLimit} {
		h, err := e.s.History(ctx, e.user, Cursor{}, limit)
		if err != nil || len(h) != want {
			t.Errorf("History(limit %d) = %d entries, %v; want %d", limit, len(h), err, want)
		}
	}
}

func TestShowWatched(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.storeMarkShow(t, map[int][]tmdb.Episode{1: {ep(101, 1, "2008-01-20"), ep(102, 2, "2008-01-27"), ep(103, 3, "2008-02-03")}})
	for _, at := range []time.Time{testNow.Add(-48 * time.Hour), testNow.Add(-24 * time.Hour)} {
		if _, err := e.s.MarkEpisode(ctx, e.user, 101, When{At: at}); err != nil {
			t.Fatal(err)
		}
	}
	// Merged into the second watch, so not a play of its own.
	if _, err := e.s.MarkEpisode(ctx, e.user, 101, When{At: testNow.Add(-23 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.MarkEpisode(ctx, e.user, 103, When{}); err != nil {
		t.Fatal(err)
	}
	// LastWatchedOn comes from the latest watch, recorded first or not: 102's
	// latest is its air date mark, 103's is the timed one above.
	for _, m := range []struct {
		tmdbID int
		w      When
	}{
		{102, When{At: time.Date(2008, 1, 26, 0, 0, 0, 0, time.UTC)}},
		{102, When{ReleaseDate: true}},
		{103, When{ReleaseDate: true}},
	} {
		if _, err := e.s.MarkEpisode(ctx, e.user, m.tmdbID, m.w); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.s.MarkEpisode(ctx, e.other, 102, When{}); err != nil {
		t.Fatal(err)
	}
	got, err := e.s.ShowWatched(ctx, e.user, markShow)
	if err != nil {
		t.Fatal(err)
	}
	want := []EpisodeWatched{
		{TMDBID: 101, Plays: 2, LastWatchedAt: testNow.Add(-24 * time.Hour)},
		{TMDBID: 102, Plays: 2, LastWatchedAt: time.Date(2008, 1, 27, 12, 0, 0, 0, time.UTC), LastWatchedOn: "2008-01-27"},
		{TMDBID: 103, Plays: 2, LastWatchedAt: testNow},
	}
	if len(got) != len(want) {
		t.Fatalf("ShowWatched = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].TMDBID != want[i].TMDBID || got[i].Plays != want[i].Plays || !got[i].LastWatchedAt.Equal(want[i].LastWatchedAt) || got[i].LastWatchedOn != want[i].LastWatchedOn {
			t.Errorf("ShowWatched[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if none, err := e.s.ShowWatched(ctx, e.user, 5); err != nil || len(none) != 0 {
		t.Errorf("unknown show: %+v, %v", none, err)
	}
}
