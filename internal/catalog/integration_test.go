package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// fixtureDir holds the responses the tmdb package recorded. This test reads
// them and doesn't copy them.
const fixtureDir = "../providers/tmdb/testdata"

// route is the recorded response that answers a request, and its status.
type route struct {
	file   string
	status int
}

// fixtureTMDB answers requests from the recorded files. A request with no
// route gets a 500, so an unexpected request fails the test.
type fixtureTMDB struct {
	routes map[string]route

	mu   sync.Mutex
	uris []string
}

func (f *fixtureTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.RequestURI()
	f.mu.Lock()
	f.uris = append(f.uris, uri)
	f.mu.Unlock()

	rt, ok := f.routes[uri]
	if !ok {
		http.Error(w, "no recorded response for "+uri, http.StatusInternalServerError)
		return
	}
	body, err := os.ReadFile(filepath.Join(fixtureDir, rt.file))
	if err != nil {
		http.Error(w, "fixture unreadable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rt.status)
	_, _ = w.Write(body)
}

// requests returns the URIs the server has answered, in order.
func (f *fixtureTMDB) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.uris...)
}

// TestFetchAndSyncAgainstRecordedTMDB runs the real tmdb client against an
// httptest server that serves the recorded responses.
func TestFetchAndSyncAgainstRecordedTMDB(t *testing.T) {
	// The timeout turns a hang into a failure. If a write transaction were open
	// during a fetch, the client's cache write would wait for it indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	notFound := route{file: "movie_not_found.json", status: http.StatusNotFound}
	fake := &fixtureTMDB{routes: map[string]route{
		"/3/movie/603?append_to_response=external_ids": {file: "movie_603.json", status: http.StatusOK},
		"/3/tv/1396?append_to_response=external_ids":   {file: "tv_1396.json", status: http.StatusOK},
		"/3/tv/1396/season/0":                          {file: "tv_1396_season_0.json", status: http.StatusOK},
		"/3/tv/1396/season/1":                          {file: "tv_1396_season_1.json", status: http.StatusOK},
		// Seasons 2 to 5 aren't recorded, so they get TMDB's 404 body.
		"/3/tv/1396/season/2": notFound,
		"/3/tv/1396/season/3": notFound,
		"/3/tv/1396/season/4": notFound,
		"/3/tv/1396/season/5": notFound,
	}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	d := dbtest.New(t)
	client := tmdb.New(d, tmdb.Options{Token: "test-read-token", BaseURL: srv.URL + "/3"})
	s := New(d, client, Options{Now: func() time.Time { return testNow }})

	if err := s.FetchMovie(ctx, 603); err != nil {
		t.Fatalf("FetchMovie(603): %v", err)
	}
	m, err := s.Movie(ctx, 603)
	if err != nil {
		t.Fatalf("Movie(603): %v", err)
	}
	if m.Title != "The Matrix" || m.IMDbID != "tt0133093" || m.Runtime != 136 || m.ReleaseDate != "1999-03-31" {
		t.Errorf("Movie(603) = title %q, IMDb %q, runtime %d, released %q", m.Title, m.IMDbID, m.Runtime, m.ReleaseDate)
	}
	if want := []Genre{{TMDBID: 28, Name: "Action"}, {TMDBID: 878, Name: "Science Fiction"}}; !slices.Equal(m.Genres, want) {
		t.Errorf("Movie(603) genres = %+v, want %+v", m.Genres, want)
	}

	// Breaking Bad lists seasons 0 to 5. Only 0 and 1 are recorded, so the
	// sync stops at season 2 with TMDB's 404.
	err = s.SyncShow(ctx, 1396)
	if !errors.Is(err, providers.ErrNotFound) {
		t.Fatalf("SyncShow(1396) = %v, want an error matching providers.ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "season 2") {
		t.Errorf("SyncShow error = %q, want it to name season 2", err)
	}
	wantRequests := []string{
		"/3/movie/603?append_to_response=external_ids",
		"/3/tv/1396?append_to_response=external_ids",
		"/3/tv/1396/season/0",
		"/3/tv/1396/season/1",
		"/3/tv/1396/season/2",
	}
	if got := fake.requests(); !slices.Equal(got, wantRequests) {
		t.Errorf("requests = %v, want %v", got, wantRequests)
	}

	show, err := s.Show(ctx, 1396)
	if err != nil {
		t.Fatalf("Show(1396): %v", err)
	}
	if show.Status != "Ended" || show.IMDbID != "tt0903747" || show.TVDBID != 81189 {
		t.Errorf("Show(1396) = status %q, IMDb %q, TVDB %d", show.Status, show.IMDbID, show.TVDBID)
	}
	if want := []Genre{{TMDBID: 80, Name: "Crime"}, {TMDBID: 18, Name: "Drama"}}; !slices.Equal(show.Genres, want) {
		t.Errorf("Show(1396) genres = %+v, want %+v", show.Genres, want)
	}
	if nums := seasonNumbers(show.Seasons); !slices.Equal(nums, []int{0, 1, 2, 3, 4, 5}) {
		t.Errorf("listed seasons = %v, want 0 to 5", nums)
	}

	// Seasons 0 and 1 were fetched: they have their episodes and the overview
	// from their responses.
	specials, err := s.SeasonEpisodes(ctx, 1396, 0)
	if err != nil {
		t.Fatalf("SeasonEpisodes(0): %v", err)
	}
	if len(specials) != 9 || specials[0].TMDBID != 62131 || specials[0].EpisodeNumber != 1 {
		t.Errorf("season 0 has %d episodes, want 9 starting with episode 62131 at number 1", len(specials))
	}
	first, err := s.SeasonEpisodes(ctx, 1396, 1)
	if err != nil {
		t.Fatalf("SeasonEpisodes(1): %v", err)
	}
	if len(first) != 7 || first[0].TMDBID != 62085 || first[0].EpisodeNumber != 1 {
		t.Errorf("season 1 has %d episodes, want 7 starting with episode 62085 at number 1", len(first))
	}
	// The recorded season 0 (Specials) has no overview, and season 1's starts
	// with its recorded text.
	if show.Seasons[0].Overview != "" {
		t.Errorf("season 0 overview = %q, want none (the recorded response has none)", show.Seasons[0].Overview)
	}
	if !strings.HasPrefix(show.Seasons[1].Overview, "High school chemistry teacher") {
		t.Errorf("season 1 overview = %q, want the recorded text", show.Seasons[1].Overview)
	}
	// Season 2 was never fetched, so its episode list isn't stored.
	var stored int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM seasons WHERE season_number = 2 AND episodes_updated_at IS NOT NULL").Scan(&stored); err != nil {
		t.Fatalf("read season 2: %v", err)
	}
	if stored != 0 {
		t.Errorf("season 2 has its episode list stored, want none")
	}
}
