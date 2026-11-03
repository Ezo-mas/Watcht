package tracking

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

var testNow = time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)

type env struct {
	d        *db.DB
	s        *Service
	user     int64
	other    int64
	movie    int64
	movie2   int64
	episode  int64
	episode2 int64
}

// newEnv stores two users, two movies, and a show with two episodes.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	e := &env{d: d, s: New(d, Options{Now: func() time.Time { return testNow }})}

	q := sqlc.New(d.Write)
	for _, u := range []struct {
		name string
		id   *int64
	}{{"alice", &e.user}, {"bob", &e.other}} {
		row, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: u.name, CreatedAt: db.Timestamp(testNow)})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		*u.id = row.ID
	}

	c := catalog.New(d, nil, catalog.Options{Now: func() time.Time { return testNow }})
	for _, id := range []int{603, 604} {
		if err := c.UpsertMovie(ctx, tmdb.Movie{TMDBID: id, Title: "Movie"}); err != nil {
			t.Fatalf("UpsertMovie: %v", err)
		}
	}
	show := tmdb.Show{TMDBID: 1396, Name: "Show", Seasons: []tmdb.SeasonSummary{{TMDBID: 3572, SeasonNumber: 1, EpisodeCount: 2}}}
	if err := c.UpsertShow(ctx, show); err != nil {
		t.Fatalf("UpsertShow: %v", err)
	}
	season := tmdb.Season{TMDBID: 3572, ShowTMDBID: 1396, SeasonNumber: 1, Episodes: []tmdb.Episode{
		{TMDBID: 62085, ShowTMDBID: 1396, SeasonNumber: 1, EpisodeNumber: 1},
		{TMDBID: 62086, ShowTMDBID: 1396, SeasonNumber: 1, EpisodeNumber: 2},
	}}
	if err := c.UpsertSeason(ctx, 1396, season); err != nil {
		t.Fatalf("UpsertSeason: %v", err)
	}

	e.movie = e.rowID(t, "movies", 603)
	e.movie2 = e.rowID(t, "movies", 604)
	e.episode = e.rowID(t, "episodes", 62085)
	e.episode2 = e.rowID(t, "episodes", 62086)
	return e
}

func (e *env) rowID(t *testing.T, table string, tmdbID int) int64 {
	t.Helper()
	var id int64
	if err := e.d.Read.QueryRow("SELECT id FROM "+table+" WHERE tmdb_id = ?", tmdbID).Scan(&id); err != nil {
		t.Fatalf("find %s %d: %v", table, tmdbID, err)
	}
	return id
}

// record records an event and fails the test on an error.
func (e *env) record(t *testing.T, ev Event) Result {
	t.Helper()
	res, err := e.s.Record(context.Background(), ev)
	if err != nil {
		t.Fatalf("Record(%+v): %v", ev, err)
	}
	return res
}

func (e *env) count(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.d.Read.QueryRow("SELECT count(*) FROM watch_events").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// manual is a manual mark of the user's movie at testNow plus offset.
func (e *env) manual(offset time.Duration) Event {
	return Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-6 * time.Hour).Add(offset), Source: SourceManual}
}

func TestRecordStoresEveryField(t *testing.T) {
	e := newEnv(t)
	at := time.Date(2026, 11, 3, 18, 30, 15, 0, time.FixedZone("EET", 2*3600))
	res := e.record(t, Event{
		UserID: e.user, EpisodeID: e.episode, WatchedAt: at,
		Source: SourceJellyfin, SourceEventID: "jf-1", Payload: json.RawMessage(`{"a":1}`),
	})
	if res.MergedInto != 0 || res.Duplicate || !res.WatchedAt.Equal(time.Date(2026, 11, 3, 16, 30, 15, 0, time.UTC)) {
		t.Errorf("Result = %+v, want a kept new watch at 16:30:15 UTC", res)
	}
	var got sqlc.WatchEvent
	err := e.d.Read.QueryRow(`SELECT id, user_id, movie_id, episode_id, watched_at, source,
		source_event_id, payload, merged_into_id, created_at FROM watch_events`).Scan(
		&got.ID, &got.UserID, &got.MovieID, &got.EpisodeID, &got.WatchedAt, &got.Source,
		&got.SourceEventID, &got.Payload, &got.MergedIntoID, &got.CreatedAt)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := sqlc.WatchEvent{
		ID: res.ID, UserID: e.user,
		EpisodeID:     sql.NullInt64{Int64: e.episode, Valid: true},
		WatchedAt:     "2026-11-03T16:30:15Z",
		Source:        "jellyfin",
		SourceEventID: sql.NullString{String: "jf-1", Valid: true},
		Payload:       sql.NullString{String: `{"a":1}`, Valid: true},
		CreatedAt:     "2026-11-03T20:00:00Z",
	}
	if got != want {
		t.Errorf("row = %+v\nwant %+v", got, want)
	}
}

func TestRecordTruncatesToSeconds(t *testing.T) {
	e := newEnv(t)
	e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour + 999*time.Millisecond), Source: SourceManual})
	var at string
	if err := e.d.Read.QueryRow("SELECT watched_at FROM watch_events").Scan(&at); err != nil {
		t.Fatal(err)
	}
	if at != "2026-11-03T19:00:00Z" {
		t.Errorf("watched_at = %q, want 2026-11-03T19:00:00Z", at)
	}
}

func TestDedupeWindow(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		merged bool
	}{
		{"same time", 0, true},
		{"just inside after", 3*time.Hour - time.Second, true},
		{"window edge after", 3 * time.Hour, true},
		{"past window after", 3*time.Hour + time.Second, false},
		{"window edge before", -3 * time.Hour, true},
		{"past window before", -3*time.Hour - time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			first := e.record(t, e.manual(0))
			second := e.record(t, e.manual(tt.offset))
			if tt.merged && second.MergedInto != first.ID {
				t.Errorf("MergedInto = %d, want %d", second.MergedInto, first.ID)
			}
			if !tt.merged && second.MergedInto != 0 {
				t.Errorf("MergedInto = %d, want a kept watch", second.MergedInto)
			}
			if second.ID == first.ID || second.Duplicate {
				t.Errorf("second Result = %+v, want a new stored event", second)
			}
		})
	}
}

func TestDedupeWindowOption(t *testing.T) {
	e := newEnv(t)
	e.s = New(e.d, Options{DedupeWindow: 10 * time.Minute, Now: func() time.Time { return testNow }})
	e.record(t, e.manual(0))
	if res := e.record(t, e.manual(11*time.Minute)); res.MergedInto != 0 {
		t.Errorf("11 minutes apart with a 10-minute window: MergedInto = %d, want 0", res.MergedInto)
	}
}

func TestDedupeAcrossSourcesKeepsFirst(t *testing.T) {
	e := newEnv(t)
	jf := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-2 * time.Hour), Source: SourceJellyfin, SourceEventID: "jf"})
	plex := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-90 * time.Minute), Source: SourcePlex, SourceEventID: "px"})
	// Earlier in time than the kept watch, but recorded later: still merged.
	man := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-3 * time.Hour), Source: SourceManual})
	if jf.MergedInto != 0 {
		t.Errorf("first: MergedInto = %d, want 0", jf.MergedInto)
	}
	for name, r := range map[string]Result{"plex": plex, "manual": man} {
		if r.MergedInto != jf.ID {
			t.Errorf("%s: MergedInto = %d, want %d", name, r.MergedInto, jf.ID)
		}
	}
	if n := e.count(t); n != 3 {
		t.Errorf("rows = %d, want 3 (merged events are kept)", n)
	}
}

func TestDedupePicksNearestKeptWatch(t *testing.T) {
	e := newEnv(t)
	a := e.record(t, e.manual(0))
	b := e.record(t, e.manual(5*time.Hour))
	if b.MergedInto != 0 {
		t.Fatalf("5 hours apart: MergedInto = %d, want 0", b.MergedInto)
	}
	if r := e.record(t, e.manual(3*time.Hour)); r.MergedInto != b.ID {
		t.Errorf("2h from b, 3h from a: MergedInto = %d, want b %d", r.MergedInto, b.ID)
	}
	// Equally close to both: the first recorded wins.
	if r := e.record(t, e.manual(2*time.Hour+30*time.Minute)); r.MergedInto != a.ID {
		t.Errorf("tie: MergedInto = %d, want the first recorded %d", r.MergedInto, a.ID)
	}
}

func TestDedupeTieGoesToFirstRecorded(t *testing.T) {
	e := newEnv(t)
	later := e.record(t, e.manual(5*time.Hour))
	e.record(t, e.manual(0))
	if r := e.record(t, e.manual(2*time.Hour+30*time.Minute)); r.MergedInto != later.ID {
		t.Errorf("tie: MergedInto = %d, want the first recorded %d (later in time)", r.MergedInto, later.ID)
	}
}

func TestDedupeWindowDefault(t *testing.T) {
	e := newEnv(t)
	e.s = New(e.d, Options{DedupeWindow: -time.Minute, Now: func() time.Time { return testNow }})
	a := e.record(t, e.manual(0))
	if r := e.record(t, e.manual(3*time.Hour)); r.MergedInto != a.ID {
		t.Errorf("MergedInto = %d, want %d with the default 3-hour window", r.MergedInto, a.ID)
	}
}

func TestConcurrentRecordsKeepOneWatch(t *testing.T) {
	e := newEnv(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.s.Record(context.Background(), Event{
				UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Duration(i) * time.Minute),
				Source: SourcePlex, SourceEventID: fmt.Sprintf("px-%d", i%4),
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	var kept, total int
	if err := e.d.Read.QueryRow("SELECT count(*) FILTER (WHERE merged_into_id IS NULL), count(*) FROM watch_events").Scan(&kept, &total); err != nil {
		t.Fatal(err)
	}
	if kept != 1 || total != 4 {
		t.Errorf("kept = %d, total = %d; want 1 kept of 4 (one per source event)", kept, total)
	}
}

func TestMergedEventsDontExtendTheWindow(t *testing.T) {
	e := newEnv(t)
	a := e.record(t, e.manual(0))
	b := e.record(t, e.manual(2*time.Hour))
	if b.MergedInto != a.ID {
		t.Fatalf("MergedInto = %d, want %d", b.MergedInto, a.ID)
	}
	// 2 hours from b, but b is merged; a is 4 hours away.
	if r := e.record(t, e.manual(4*time.Hour)); r.MergedInto != 0 {
		t.Errorf("MergedInto = %d, want a new kept watch", r.MergedInto)
	}
}

func TestDedupeIsPerUserAndItem(t *testing.T) {
	e := newEnv(t)
	at := testNow.Add(-time.Hour)
	e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: at, Source: SourceManual})
	e.record(t, Event{UserID: e.user, EpisodeID: e.episode, WatchedAt: at, Source: SourceManual})
	for name, ev := range map[string]Event{
		"other user":    {UserID: e.other, MovieID: e.movie, WatchedAt: at, Source: SourceManual},
		"other movie":   {UserID: e.user, MovieID: e.movie2, WatchedAt: at, Source: SourceManual},
		"other episode": {UserID: e.user, EpisodeID: e.episode2, WatchedAt: at, Source: SourceManual},
	} {
		if r := e.record(t, ev); r.MergedInto != 0 {
			t.Errorf("%s: MergedInto = %d, want a kept watch", name, r.MergedInto)
		}
	}
	if r := e.record(t, Event{UserID: e.user, EpisodeID: e.episode, WatchedAt: at, Source: SourceManual}); r.MergedInto == 0 {
		t.Errorf("same episode: want merged")
	}
}

func TestSourceEventIsIdempotent(t *testing.T) {
	e := newEnv(t)
	ev := Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourceJellyfin, SourceEventID: "jf-1"}
	first := e.record(t, ev)
	// A resend may carry a different time or payload; the first is kept.
	ev.WatchedAt = testNow.Add(-10 * time.Hour)
	ev.Payload = json.RawMessage(`{}`)
	again := e.record(t, ev)
	if !again.Duplicate || again.ID != first.ID || again.MergedInto != 0 || !again.WatchedAt.Equal(first.WatchedAt) {
		t.Errorf("resend Result = %+v, want duplicate of %d", again, first.ID)
	}
	if n := e.count(t); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}

	// A duplicate of a merged event reports what it was merged into.
	merged := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourcePlex, SourceEventID: "px-1"})
	if merged.MergedInto != first.ID {
		t.Fatalf("plex MergedInto = %d, want %d", merged.MergedInto, first.ID)
	}
	dup := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourcePlex, SourceEventID: "px-1"})
	if !dup.Duplicate || dup.ID != merged.ID || dup.MergedInto != first.ID {
		t.Errorf("merged resend Result = %+v, want duplicate of %d merged into %d", dup, merged.ID, first.ID)
	}

	// The same ID from another source or another user is a different event.
	other := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourcePlex, SourceEventID: "jf-1"})
	if other.Duplicate {
		t.Errorf("same ID, other source: Duplicate")
	}
	otherUser := e.record(t, Event{UserID: e.other, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourceJellyfin, SourceEventID: "jf-1"})
	if otherUser.Duplicate || otherUser.MergedInto != 0 {
		t.Errorf("same ID, other user: Result = %+v, want a kept watch", otherUser)
	}
}

func TestRecordRejectsInvalidEvents(t *testing.T) {
	e := newEnv(t)
	at := testNow.Add(-time.Hour)
	tests := map[string]Event{
		"no user":               {MovieID: e.movie, WatchedAt: at, Source: SourceManual},
		"no item":               {UserID: e.user, WatchedAt: at, Source: SourceManual},
		"two items":             {UserID: e.user, MovieID: e.movie, EpisodeID: e.episode, WatchedAt: at, Source: SourceManual},
		"negative item":         {UserID: e.user, MovieID: -1, WatchedAt: at, Source: SourceManual},
		"no time":               {UserID: e.user, MovieID: e.movie, Source: SourceManual},
		"zero after truncation": {UserID: e.user, MovieID: e.movie, WatchedAt: time.Time{}.Add(500 * time.Millisecond), Source: SourceManual},
		"future":                {UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(5*time.Minute + time.Second), Source: SourceManual},
		"year 0":                {UserID: e.user, MovieID: e.movie, WatchedAt: time.Date(0, 12, 31, 0, 0, 0, 0, time.UTC), Source: SourceManual},
		"unknown source":        {UserID: e.user, MovieID: e.movie, WatchedAt: at, Source: "trakt"},
		"manual with event ID":  {UserID: e.user, MovieID: e.movie, WatchedAt: at, Source: SourceManual, SourceEventID: "x"},
		"import without ID":     {UserID: e.user, MovieID: e.movie, WatchedAt: at, Source: SourceTraktImport},
		"bad payload":           {UserID: e.user, MovieID: e.movie, WatchedAt: at, Source: SourcePlex, SourceEventID: "x", Payload: json.RawMessage(`{`)},
	}
	for name, ev := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := e.s.Record(context.Background(), ev); !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
		})
	}
	// Within the clock slack, and an 1890s release date, are accepted.
	e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(5 * time.Minute), Source: SourceManual})
	e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: time.Date(1895, 12, 28, 0, 0, 0, 0, time.UTC), Source: SourceManual})
	if n := e.count(t); n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
}

func TestRecordUnknownItem(t *testing.T) {
	e := newEnv(t)
	for name, ev := range map[string]Event{
		"movie":   {UserID: e.user, MovieID: 9999, WatchedAt: testNow, Source: SourceManual},
		"episode": {UserID: e.user, EpisodeID: 9999, WatchedAt: testNow, Source: SourceManual},
	} {
		if _, err := e.s.Record(context.Background(), ev); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if n := e.count(t); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

func TestRemoveDeletesTheWholeWatch(t *testing.T) {
	ctx := context.Background()
	for _, via := range []string{"kept", "merged"} {
		t.Run(via, func(t *testing.T) {
			e := newEnv(t)
			kept := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourceJellyfin, SourceEventID: "jf-1"})
			merged := e.record(t, e.manual(5*time.Hour))
			if merged.MergedInto != kept.ID {
				t.Fatalf("MergedInto = %d, want %d", merged.MergedInto, kept.ID)
			}
			// A separate watch of the same movie, and another user's watch, stay.
			e.record(t, e.manual(0))
			e.record(t, Event{UserID: e.other, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourceManual})

			id := kept.ID
			if via == "merged" {
				id = merged.ID
			}
			if err := e.s.Remove(ctx, e.user, id); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			if n := e.count(t); n != 2 {
				t.Errorf("rows = %d, want 2", n)
			}
			// The source can report the same event again, and it's a new watch.
			again := e.record(t, Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-time.Hour), Source: SourceJellyfin, SourceEventID: "jf-1"})
			if again.Duplicate || again.MergedInto != 0 {
				t.Errorf("resend after remove: Result = %+v, want a new kept watch", again)
			}
			if again.ID <= merged.ID {
				t.Errorf("new ID %d reuses a removed one (removed up to %d)", again.ID, merged.ID)
			}
		})
	}
}

func TestRemovedMergedSourceEventCanReturn(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	kept := e.record(t, e.manual(0))
	px := Event{UserID: e.user, MovieID: e.movie, WatchedAt: testNow.Add(-5 * time.Hour), Source: SourcePlex, SourceEventID: "px-1"}
	if r := e.record(t, px); r.MergedInto != kept.ID {
		t.Fatalf("MergedInto = %d, want %d", r.MergedInto, kept.ID)
	}
	if err := e.s.Remove(ctx, e.user, kept.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if r := e.record(t, px); r.Duplicate || r.MergedInto != 0 {
		t.Errorf("resend after remove: Result = %+v, want a new kept watch", r)
	}
}

func TestRemoveOnlyOwnWatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	r := e.record(t, e.manual(0))
	if err := e.s.Remove(ctx, e.other, r.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user's: err = %v, want ErrNotFound", err)
	}
	if err := e.s.Remove(ctx, e.user, r.ID+100); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: err = %v, want ErrNotFound", err)
	}
	if n := e.count(t); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestDeletingUserDeletesWatches(t *testing.T) {
	e := newEnv(t)
	e.record(t, e.manual(0))
	e.record(t, e.manual(time.Hour))
	if _, err := sqlc.New(e.d.Write).DeleteUser(context.Background(), e.user); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

// The table's CHECKs refuse rows that Record would never write.
func TestSchemaRejectsBadRows(t *testing.T) {
	e := newEnv(t)
	const insert = `INSERT INTO watch_events (user_id, movie_id, episode_id, watched_at, source,
		source_event_id, payload, merged_into_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	ts := db.Timestamp(testNow)
	tests := map[string][]any{
		"no item":              {e.user, nil, nil, ts, "manual", nil, nil, nil, ts},
		"two items":            {e.user, e.movie, e.episode, ts, "manual", nil, nil, nil, ts},
		"bad time":             {e.user, e.movie, nil, "2026-11-03 20:00:00", "manual", nil, nil, nil, ts},
		"unknown source":       {e.user, e.movie, nil, ts, "trakt", nil, nil, nil, ts},
		"manual with event ID": {e.user, e.movie, nil, ts, "manual", "x", nil, nil, ts},
		"import without ID":    {e.user, e.movie, nil, ts, "plex", nil, nil, nil, ts},
		"empty event ID":       {e.user, e.movie, nil, ts, "plex", "", nil, nil, ts},
		"bad payload":          {e.user, e.movie, nil, ts, "plex", "x", "{", nil, ts},
		"unknown movie":        {e.user, 9999, nil, ts, "manual", nil, nil, nil, ts},
	}
	for name, args := range tests {
		if _, err := e.d.Write.Exec(insert, args...); err == nil {
			t.Errorf("%s: insert succeeded", name)
		}
	}
	if _, err := e.d.Write.Exec(insert, e.user, e.movie, nil, ts, "plex", "x", nil, nil, ts); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := e.d.Write.Exec(insert, e.user, e.movie2, nil, ts, "plex", "x", nil, nil, ts); !db.IsUniqueViolation(err) {
		t.Errorf("repeated source event: err = %v, want a unique violation", err)
	}
}
