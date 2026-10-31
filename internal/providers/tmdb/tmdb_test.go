package tmdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/providers"
)

// testToken stands in for a read token. The tests check it never appears in a
// path, a query, or an error.
const testToken = "test-read-token"

// testNow is a whole second, like every stored timestamp.
var testNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// clock is a clock the test moves by hand. The client calls it from the test's
// own goroutine, so it needs no lock.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// route is the response that answers one request: a file in testdata/, or an
// inline body for a case no fixture covers.
type route struct {
	file   string // a file in testdata/; "" when body is set
	body   string // an inline body
	status int
}

func answer(file string) route   { return route{file: file, status: http.StatusOK} }
func notFound(file string) route { return route{file: file, status: http.StatusNotFound} }
func inline(body string) route   { return route{body: body, status: http.StatusOK} }

// tmdbRoutes maps each request URI (path and query, as sent) to the response
// that answers it. testdata/README.md says which request recorded each file.
// movie_not_found.json was recorded from /movie/999999999 alone. Its 404 body is
// the same for every endpoint, so it also answers the other 404 routes here,
// where only the status matters.
func tmdbRoutes() map[string]route {
	return map[string]route{
		"/3/search/multi?include_adult=false&page=1&query=Matrix":                 answer("search_multi.json"),
		"/3/search/multi?include_adult=false&page=1&query=Harrison+Ford":          answer("search_multi_person.json"),
		"/3/search/multi?include_adult=false&page=1&query=zzqxjvnonexistenttitle": answer("search_multi_empty.json"),
		"/3/search/multi?include_adult=false&page=1&query=nosuchtitle":            notFound("movie_not_found.json"),
		"/3/movie/603?append_to_response=external_ids":                            answer("movie_603.json"),
		"/3/movie/999999999?append_to_response=external_ids":                      notFound("movie_not_found.json"),
		"/3/tv/1396?append_to_response=external_ids":                              answer("tv_1396.json"),
		"/3/tv/1437?append_to_response=external_ids":                              answer("tv_1437.json"),
		"/3/tv/94997?append_to_response=external_ids":                             answer("tv_94997.json"),
		"/3/tv/999999999?append_to_response=external_ids":                         notFound("movie_not_found.json"),
		"/3/tv/999999999/season/1":                                                notFound("movie_not_found.json"),
		"/3/tv/1396/season/0":                                                     answer("tv_1396_season_0.json"),
		"/3/tv/1396/season/1":                                                     answer("tv_1396_season_1.json"),
		"/3/tv/1396/season/1/episode/1?append_to_response=external_ids":           answer("tv_1396_season_1_episode_1.json"),
		"/3/tv/999999999/season/1/episode/1?append_to_response=external_ids":      notFound("movie_not_found.json"),
		"/3/find/tt0903747?external_source=imdb_id":                               answer("find_imdb_tt0903747.json"),
		"/3/find/tt0133093?external_source=imdb_id":                               answer("find_imdb_tt0133093.json"),
		"/3/find/tt0000000?external_source=imdb_id":                               answer("find_none.json"),
		"/3/find/tt9999999?external_source=imdb_id":                               notFound("movie_not_found.json"),
		"/3/find/81189?external_source=tvdb_id":                                   answer("find_tvdb_81189.json"),
		"/3/find/349232?external_source=tvdb_id":                                  answer("find_tvdb_349232.json"),
		"/3/find/999999999?external_source=tvdb_id":                               notFound("movie_not_found.json"),
	}
}

// fixtureServer answers requests from testdata/ and records each one. A URI
// with no route gets a 500, so a test that makes an unexpected request fails
// on its request list.
type fixtureServer struct {
	routes map[string]route

	mu   sync.Mutex
	reqs []request
}

// request is one request the server received.
type request struct {
	uri  string // path and query, as sent
	auth string
	ua   string
}

func (s *fixtureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.reqs = append(s.reqs, request{
		uri:  r.URL.RequestURI(),
		auth: r.Header.Get("Authorization"),
		ua:   r.UserAgent(),
	})
	s.mu.Unlock()

	rt, found := s.routes[r.URL.RequestURI()]
	if !found {
		http.Error(w, "no recorded response for this request", http.StatusInternalServerError)
		return
	}
	body := []byte(rt.body)
	if rt.file != "" {
		var err error
		body, err = os.ReadFile(filepath.Join("testdata", rt.file))
		if err != nil {
			http.Error(w, "fixture unreadable", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rt.status)
	_, _ = w.Write(body)
}

// env is a Client on a fresh database, talking to a fixture server on loopback
// with BaseURL set to its /3 root.
type env struct {
	client *Client
	srv    *fixtureServer
	d      *db.DB
	clock  *clock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, nil)
}

// newEnvWith is newEnv with extra routes, which replace the route for their
// URIs. They're set before the server starts, so the handler reads them without
// a lock.
func newEnvWith(t *testing.T, extra map[string]route) *env {
	t.Helper()
	routes := tmdbRoutes()
	maps.Copy(routes, extra)
	srv := &fixtureServer{routes: routes}
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	d := dbtest.New(t)
	clk := &clock{now: testNow}
	return &env{
		client: New(d, Options{Token: testToken, BaseURL: hs.URL + "/3", Now: clk.Now}),
		srv:    srv,
		d:      d,
		clock:  clk,
	}
}

// requests returns the requests the server has received, in order.
func (e *env) requests() []request {
	e.srv.mu.Lock()
	defer e.srv.mu.Unlock()
	return append([]request(nil), e.srv.reqs...)
}

// wantRequests checks the requests the server received, in order: each URI
// exactly, and the bearer token as its only Authorization.
func (e *env) wantRequests(t *testing.T, uris ...string) {
	t.Helper()
	got := e.requests()
	if len(got) != len(uris) {
		var seen []string
		for _, r := range got {
			seen = append(seen, r.uri)
		}
		t.Fatalf("requests = %q, want %q", seen, uris)
	}
	for i, r := range got {
		if r.uri != uris[i] {
			t.Errorf("request %d = %q, want %q", i, r.uri, uris[i])
		}
		if r.auth != "Bearer "+testToken {
			t.Errorf("request %d: Authorization header is not the bearer token", i)
		}
	}
}

// cached returns the TMDB rows in provider_cache, as key → expires_at.
func (e *env) cached(t *testing.T) map[string]string {
	t.Helper()
	rows, err := e.d.Read.QueryContext(context.Background(), `SELECT key, expires_at FROM provider_cache WHERE provider = 'tmdb'`)
	if err != nil {
		t.Fatalf("read provider_cache: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var key, expires string
		if err := rows.Scan(&key, &expires); err != nil {
			t.Fatalf("scan provider_cache: %v", err)
		}
		out[key] = expires
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read provider_cache: %v", err)
	}
	return out
}

// wantField reports a field that isn't the expected value.
func wantField[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func wantGenres(t *testing.T, got, want []Genre) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("Genres = %+v, want %+v", got, want)
	}
}

func wantEpisodeIDs(t *testing.T, eps []Episode, want []int) {
	t.Helper()
	var got []int
	for _, ep := range eps {
		got = append(got, ep.TMDBID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("episode IDs = %v, want %v", got, want)
	}
}

func wantFindCounts(t *testing.T, res FindResults, movies, shows, episodes int) {
	t.Helper()
	if len(res.Movies) != movies || len(res.Shows) != shows || len(res.Episodes) != episodes {
		t.Fatalf("Movies, Shows, Episodes = %d, %d, %d; want %d, %d, %d",
			len(res.Movies), len(res.Shows), len(res.Episodes), movies, shows, episodes)
	}
}

func TestNewPanicsWithoutToken(t *testing.T) {
	d := dbtest.New(t)
	for _, token := range []string{"", "  \t"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New with a blank token did not panic")
				}
			}()
			New(d, Options{Token: token})
		}()
	}
}

// roundTripFunc answers requests in memory. It lets a test use the default
// BaseURL without opening a socket.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultBaseURLAndHeaders(t *testing.T) {
	var sent *http.Request
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sent = r
		body, err := os.ReadFile(filepath.Join("testdata", "movie_603.json"))
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    r,
		}, nil
	})}
	c := New(dbtest.New(t), Options{Token: testToken, UserAgent: "watcht-test", HTTPClient: hc})

	m, err := c.Movie(context.Background(), 603)
	if err != nil {
		t.Fatalf("Movie(603): %v", err)
	}
	if sent == nil {
		t.Fatal("no request was sent")
	}
	wantField(t, "URL", sent.URL.String(), BaseURL+"/movie/603?append_to_response=external_ids")
	if sent.Header.Get("Authorization") != "Bearer "+testToken {
		t.Error("Authorization header is not the bearer token")
	}
	wantField(t, "User-Agent", sent.Header.Get("User-Agent"), "watcht-test")
	wantField(t, "TMDBID", m.TMDBID, 603)
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	t.Run("trims the query and keeps movies and shows", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.Search(ctx, "  Matrix  ", 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		e.wantRequests(t, "/3/search/multi?include_adult=false&page=1&query=Matrix")
		wantField(t, "Page", res.Page, 1)
		wantField(t, "TotalPages", res.TotalPages, 7)
		wantField(t, "TotalResults", res.TotalResults, 126)
		if len(res.Results) != 5 {
			t.Fatalf("Results = %d, want 5", len(res.Results))
		}
		movie := res.Results[0]
		wantField(t, "Results[0].MediaType", movie.MediaType, "movie")
		wantField(t, "Results[0].TMDBID", movie.TMDBID, 603)
		wantField(t, "Results[0].Title", movie.Title, "The Matrix")
		wantField(t, "Results[0].Date", movie.Date, "1999-03-31")
		wantField(t, "Results[0].VoteAverage", movie.VoteAverage, 8.3)
		wantField(t, "Results[0].PosterPath", movie.PosterPath, "/dXNAPwY7VrqMAo51EKhhCJfaGb5.jpg")
		// A show takes its title from name and its date from first_air_date.
		show := res.Results[1]
		wantField(t, "Results[1].MediaType", show.MediaType, "tv")
		wantField(t, "Results[1].TMDBID", show.TMDBID, 711)
		wantField(t, "Results[1].Title", show.Title, "Threat Matrix")
		wantField(t, "Results[1].Date", show.Date, "2003-09-18")
	})
	t.Run("people are dropped and the rest keep their order", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.Search(ctx, "Harrison Ford", 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		e.wantRequests(t, "/3/search/multi?include_adult=false&page=1&query=Harrison+Ford")
		// TMDB's totals count the person, which Results leaves out.
		wantField(t, "TotalResults", res.TotalResults, 10)
		var got []string
		for _, r := range res.Results {
			got = append(got, fmt.Sprintf("%s %d", r.MediaType, r.TMDBID))
		}
		if want := []string{"movie 1755490", "tv 325342", "movie 1357745", "movie 1199447"}; !slices.Equal(got, want) {
			t.Fatalf("Results = %v, want %v", got, want)
		}
		show := res.Results[1]
		wantField(t, "Results[1].Title", show.Title, "Icons Unearthed: Harrison Ford")
		wantField(t, "Results[1].OriginalTitle", show.OriginalTitle, "Icons Unearthed: Harrison Ford")
		wantField(t, "Results[1].Date", show.Date, "2026-06-12")
	})
	t.Run("a page below 1 is page 1", func(t *testing.T) {
		for _, page := range []int{0, -3} {
			e := newEnv(t)
			if _, err := e.client.Search(ctx, "Matrix", page); err != nil {
				t.Fatalf("Search page %d: %v", page, err)
			}
			e.wantRequests(t, "/3/search/multi?include_adult=false&page=1&query=Matrix")
		}
	})
	t.Run("no results", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.Search(ctx, "zzqxjvnonexistenttitle", 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		e.wantRequests(t, "/3/search/multi?include_adult=false&page=1&query=zzqxjvnonexistenttitle")
		wantField(t, "TotalResults", res.TotalResults, 0)
		wantField(t, "TotalPages", res.TotalPages, 1)
		wantField(t, "len(Results)", len(res.Results), 0)
	})
}

func TestMovie(t *testing.T) {
	e := newEnv(t)
	m, err := e.client.Movie(context.Background(), 603)
	if err != nil {
		t.Fatalf("Movie(603): %v", err)
	}
	e.wantRequests(t, "/3/movie/603?append_to_response=external_ids")
	wantField(t, "TMDBID", m.TMDBID, 603)
	wantField(t, "IMDbID", m.IMDbID, "tt0133093")
	wantField(t, "Title", m.Title, "The Matrix")
	wantField(t, "OriginalTitle", m.OriginalTitle, "The Matrix")
	wantField(t, "OriginalLanguage", m.OriginalLanguage, "en")
	wantField(t, "Overview", m.Overview, "Set in the 22nd century, The Matrix tells the story of a computer hacker who joins a group of underground insurgents fighting the vast and powerful computers who now rule the earth.")
	wantField(t, "Tagline", m.Tagline, "Believe the unbelievable.")
	wantField(t, "ReleaseDate", m.ReleaseDate, "1999-03-31")
	wantField(t, "Status", m.Status, "Released")
	wantField(t, "Runtime", m.Runtime, 136)
	wantField(t, "PosterPath", m.PosterPath, "/dXNAPwY7VrqMAo51EKhhCJfaGb5.jpg")
	wantField(t, "BackdropPath", m.BackdropPath, "/tlm8UkiQsitc8rSuIAscQDCnP8d.jpg")
	wantField(t, "VoteAverage", m.VoteAverage, 8.262)
	wantField(t, "VoteCount", m.VoteCount, 29114)
	wantGenres(t, m.Genres, []Genre{{ID: 28, Name: "Action"}, {ID: 878, Name: "Science Fiction"}})
}

func TestShow(t *testing.T) {
	ctx := context.Background()
	t.Run("ended show", func(t *testing.T) {
		e := newEnv(t)
		s, err := e.client.Show(ctx, 1396)
		if err != nil {
			t.Fatalf("Show(1396): %v", err)
		}
		e.wantRequests(t, "/3/tv/1396?append_to_response=external_ids")
		wantField(t, "TMDBID", s.TMDBID, 1396)
		wantField(t, "IMDbID", s.IMDbID, "tt0903747")
		wantField(t, "TVDBID", s.TVDBID, 81189)
		wantField(t, "Name", s.Name, "Breaking Bad")
		wantField(t, "OriginalName", s.OriginalName, "Breaking Bad")
		wantField(t, "Overview", s.Overview, "Walter White, a New Mexico chemistry teacher, is diagnosed with Stage III cancer and given a prognosis of only two years left to live. He becomes filled with a sense of fearlessness and an unrelenting desire to secure his family's financial future at any cost as he enters the dangerous world of drugs and crime.")
		wantField(t, "Tagline", s.Tagline, "Change the equation.")
		wantField(t, "PosterPath", s.PosterPath, "/anFx9aTOOYqgS3v7x3R84Kz67ly.jpg")
		wantField(t, "BackdropPath", s.BackdropPath, "/tsRy63Mu5cu8etL1X7ZLyf7UP1M.jpg")
		wantField(t, "Status", s.Status, "Ended")
		wantField(t, "Ended", s.Ended, true)
		wantField(t, "InProduction", s.InProduction, false)
		wantField(t, "NumberOfSeasons", s.NumberOfSeasons, 5)
		wantField(t, "NumberOfEpisodes", s.NumberOfEpisodes, 62)
		wantField(t, "FirstAirDate", s.FirstAirDate, "2008-01-20")
		wantField(t, "LastAirDate", s.LastAirDate, "2013-09-29")
		wantField(t, "VoteAverage", s.VoteAverage, 8.959)
		wantField(t, "VoteCount", s.VoteCount, 19001)
		wantGenres(t, s.Genres, []Genre{{ID: 18, Name: "Drama"}, {ID: 80, Name: "Crime"}})
		if len(s.Seasons) != 6 {
			t.Fatalf("Seasons = %d, want 6", len(s.Seasons))
		}
		wantField(t, "Seasons[0]", s.Seasons[0], SeasonSummary{
			TMDBID: 3577, SeasonNumber: 0, EpisodeCount: 9, Name: "Specials",
			AirDate: "2009-02-17", PosterPath: "/4RaIvSQgHHHPfaI1jFqaMR8UJWM.jpg",
		})
		wantField(t, "Seasons[5]", s.Seasons[5], SeasonSummary{
			TMDBID: 3578, SeasonNumber: 5, EpisodeCount: 16, Name: "Season 5",
			AirDate: "2012-07-15", PosterPath: "/r3z70vunihrAkjILQKWHX0G2xzO.jpg",
		})
	})
	t.Run("returning show is not ended", func(t *testing.T) {
		e := newEnv(t)
		s, err := e.client.Show(ctx, 94997)
		if err != nil {
			t.Fatalf("Show(94997): %v", err)
		}
		e.wantRequests(t, "/3/tv/94997?append_to_response=external_ids")
		wantField(t, "IMDbID", s.IMDbID, "tt11198330")
		wantField(t, "TVDBID", s.TVDBID, 371572)
		wantField(t, "Status", s.Status, "Returning Series")
		wantField(t, "Ended", s.Ended, false)
		wantField(t, "InProduction", s.InProduction, true)
		wantField(t, "NumberOfSeasons", s.NumberOfSeasons, 3)
		wantField(t, "NumberOfEpisodes", s.NumberOfEpisodes, 26)
		wantField(t, "LastAirDate", s.LastAirDate, "2026-08-09")
		wantField(t, "len(Seasons)", len(s.Seasons), 4)
	})
	t.Run("canceled show is ended", func(t *testing.T) {
		e := newEnv(t)
		s, err := e.client.Show(ctx, 1437)
		if err != nil {
			t.Fatalf("Show(1437): %v", err)
		}
		e.wantRequests(t, "/3/tv/1437?append_to_response=external_ids")
		wantField(t, "Name", s.Name, "Firefly")
		wantField(t, "IMDbID", s.IMDbID, "tt0303461")
		wantField(t, "TVDBID", s.TVDBID, 78874)
		wantField(t, "Status", s.Status, "Canceled")
		wantField(t, "Ended", s.Ended, true)
		wantField(t, "InProduction", s.InProduction, false)
	})
}

// TestShowEndedRule checks the status rule on the converter, with inline values.
// Ended is true for "Ended" and "Canceled" only, whatever in_production says, so
// each status is checked with both values.
func TestShowEndedRule(t *testing.T) {
	tests := []struct {
		status string
		ended  bool
	}{
		{"Ended", true},
		{"Canceled", true},
		{"Returning Series", false},
		{"In Production", false},
		{"Planned", false},
		{"Pilot", false},
	}
	for _, tt := range tests {
		for _, inProduction := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s in_production=%t", tt.status, inProduction), func(t *testing.T) {
				s := showFromWire(wireShow{Status: tt.status, InProduction: inProduction})
				wantField(t, "Ended", s.Ended, tt.ended)
			})
		}
	}
}

func TestSeason(t *testing.T) {
	ctx := context.Background()
	t.Run("season 1 episodes carry the show and TMDB IDs", func(t *testing.T) {
		e := newEnv(t)
		s, err := e.client.Season(ctx, 1396, 1)
		if err != nil {
			t.Fatalf("Season(1396, 1): %v", err)
		}
		e.wantRequests(t, "/3/tv/1396/season/1")
		wantField(t, "TMDBID", s.TMDBID, 3572)
		wantField(t, "ShowTMDBID", s.ShowTMDBID, 1396)
		wantField(t, "SeasonNumber", s.SeasonNumber, 1)
		wantField(t, "Name", s.Name, "Season 1")
		wantField(t, "Overview", s.Overview, "High school chemistry teacher Walter White's life is suddenly transformed by a dire medical diagnosis. Street-savvy former student Jesse Pinkman \"teaches\" Walter a new trade.")
		wantField(t, "AirDate", s.AirDate, "2008-01-20")
		wantField(t, "PosterPath", s.PosterPath, "/1BP4xYv9ZG4ZVHkL7ocOziBbSYH.jpg")
		wantEpisodeIDs(t, s.Episodes, []int{62085, 62086, 62087, 62088, 62089, 62090, 62091})
		for _, ep := range s.Episodes {
			// Season responses carry no external IDs, so those stay empty.
			if ep.ShowTMDBID != 1396 || ep.SeasonNumber != 1 || ep.IMDbID != "" || ep.TVDBID != 0 {
				t.Errorf("episode %d: show %d, season %d, IMDb %q, TVDB %d",
					ep.TMDBID, ep.ShowTMDBID, ep.SeasonNumber, ep.IMDbID, ep.TVDBID)
			}
		}
		if len(s.Episodes) != 7 {
			t.Fatalf("Episodes = %d, want 7", len(s.Episodes))
		}
		first := s.Episodes[0]
		wantField(t, "Episodes[0].EpisodeNumber", first.EpisodeNumber, 1)
		wantField(t, "Episodes[0].Name", first.Name, "Pilot")
		wantField(t, "Episodes[0].Overview", first.Overview, "When an unassuming high school chemistry teacher discovers he has a rare form of lung cancer, he decides to team up with a former student and create a top of the line crystal meth in a used RV, to provide for his family once he is gone.")
		wantField(t, "Episodes[0].AirDate", first.AirDate, "2008-01-20")
		wantField(t, "Episodes[0].Runtime", first.Runtime, 59)
		wantField(t, "Episodes[0].VoteAverage", first.VoteAverage, 8.523)
		wantField(t, "Episodes[0].StillPath", first.StillPath, "/ydlY3iPfeOAvu8gVqrxPoMvzNCn.jpg")
		last := s.Episodes[6]
		wantField(t, "Episodes[6].EpisodeNumber", last.EpisodeNumber, 7)
		wantField(t, "Episodes[6].Name", last.Name, "A No Rough Stuff Type Deal")
	})
	t.Run("season 0 holds the specials in TMDB's order", func(t *testing.T) {
		e := newEnv(t)
		s, err := e.client.Season(ctx, 1396, 0)
		if err != nil {
			t.Fatalf("Season(1396, 0): %v", err)
		}
		e.wantRequests(t, "/3/tv/1396/season/0")
		wantField(t, "TMDBID", s.TMDBID, 3577)
		wantField(t, "SeasonNumber", s.SeasonNumber, 0)
		wantField(t, "Name", s.Name, "Specials")
		wantEpisodeIDs(t, s.Episodes, []int{62131, 62133, 62132, 62135, 62134, 62136, 1836320, 1836324, 1836322})
		if len(s.Episodes) != 9 {
			t.Fatalf("Episodes = %d, want 9", len(s.Episodes))
		}
		for _, ep := range s.Episodes {
			if ep.ShowTMDBID != 1396 || ep.SeasonNumber != 0 {
				t.Errorf("episode %d: show %d, season %d", ep.TMDBID, ep.ShowTMDBID, ep.SeasonNumber)
			}
		}
		// TMDB sends null for an unknown air date; it becomes "".
		wantField(t, "Episodes[5].Name", s.Episodes[5].Name, "Jesse Pinkman Evidence Tape")
		wantField(t, "Episodes[5].AirDate", s.Episodes[5].AirDate, "")
		wantField(t, "Episodes[8].AirDate", s.Episodes[8].AirDate, "2013-11-26")
	})
}

func TestEpisode(t *testing.T) {
	e := newEnv(t)
	ep, err := e.client.Episode(context.Background(), 1396, 1, 1)
	if err != nil {
		t.Fatalf("Episode(1396, 1, 1): %v", err)
	}
	e.wantRequests(t, "/3/tv/1396/season/1/episode/1?append_to_response=external_ids")
	wantField(t, "TMDBID", ep.TMDBID, 62085)
	wantField(t, "ShowTMDBID", ep.ShowTMDBID, 1396)
	wantField(t, "SeasonNumber", ep.SeasonNumber, 1)
	wantField(t, "EpisodeNumber", ep.EpisodeNumber, 1)
	wantField(t, "Name", ep.Name, "Pilot")
	wantField(t, "Overview", ep.Overview, "When an unassuming high school chemistry teacher discovers he has a rare form of lung cancer, he decides to team up with a former student and create a top of the line crystal meth in a used RV, to provide for his family once he is gone.")
	wantField(t, "AirDate", ep.AirDate, "2008-01-20")
	wantField(t, "StillPath", ep.StillPath, "/ydlY3iPfeOAvu8gVqrxPoMvzNCn.jpg")
	wantField(t, "Runtime", ep.Runtime, 59)
	wantField(t, "VoteAverage", ep.VoteAverage, 8.523)
	wantField(t, "IMDbID", ep.IMDbID, "tt0959621")
	wantField(t, "TVDBID", ep.TVDBID, 349232)
}

// TestSeasonAndEpisodeTakeIDsFromRequest checks that the show and season come
// from the request, so an episode whose show_id or season_number is missing or
// different doesn't change them. The bodies are inline because the recorded
// responses all carry the IDs the request asked for.
func TestSeasonAndEpisodeTakeIDsFromRequest(t *testing.T) {
	ctx := context.Background()
	t.Run("season", func(t *testing.T) {
		// Episode 1 names another show and season; episode 2 names none.
		const body = `{"id": 3572, "name": "Season 1", "episodes": [
			{"id": 62085, "episode_number": 1, "season_number": 9, "show_id": 2},
			{"id": 62086, "episode_number": 2}]}`
		e := newEnvWith(t, map[string]route{"/3/tv/1396/season/1": inline(body)})
		s, err := e.client.Season(ctx, 1396, 1)
		if err != nil {
			t.Fatalf("Season(1396, 1): %v", err)
		}
		e.wantRequests(t, "/3/tv/1396/season/1")
		wantField(t, "ShowTMDBID", s.ShowTMDBID, 1396)
		wantField(t, "SeasonNumber", s.SeasonNumber, 1)
		wantEpisodeIDs(t, s.Episodes, []int{62085, 62086})
		for _, ep := range s.Episodes {
			if ep.ShowTMDBID != 1396 || ep.SeasonNumber != 1 {
				t.Errorf("episode %d: show %d, season %d; want 1396, 1", ep.TMDBID, ep.ShowTMDBID, ep.SeasonNumber)
			}
		}
	})
	t.Run("episode", func(t *testing.T) {
		const uri = "/3/tv/1396/season/1/episode/1?append_to_response=external_ids"
		const body = `{"id": 62085, "episode_number": 1, "season_number": 9, "show_id": 2}`
		e := newEnvWith(t, map[string]route{uri: inline(body)})
		ep, err := e.client.Episode(ctx, 1396, 1, 1)
		if err != nil {
			t.Fatalf("Episode(1396, 1, 1): %v", err)
		}
		e.wantRequests(t, uri)
		wantField(t, "ShowTMDBID", ep.ShowTMDBID, 1396)
		wantField(t, "SeasonNumber", ep.SeasonNumber, 1)
		wantField(t, "EpisodeNumber", ep.EpisodeNumber, 1)
	})
}

func TestFind(t *testing.T) {
	ctx := context.Background()
	t.Run("IMDb ID of a show", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.FindByIMDbID(ctx, "tt0903747")
		if err != nil {
			t.Fatalf("FindByIMDbID(tt0903747): %v", err)
		}
		e.wantRequests(t, "/3/find/tt0903747?external_source=imdb_id")
		wantFindCounts(t, res, 0, 1, 0)
		show := res.Shows[0]
		wantField(t, "MediaType", show.MediaType, "tv")
		wantField(t, "TMDBID", show.TMDBID, 1396)
		wantField(t, "Title", show.Title, "Breaking Bad")
		wantField(t, "Date", show.Date, "2008-01-20")
		wantField(t, "VoteAverage", show.VoteAverage, 8.959)
	})
	t.Run("IMDb ID of a movie", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.FindByIMDbID(ctx, "tt0133093")
		if err != nil {
			t.Fatalf("FindByIMDbID(tt0133093): %v", err)
		}
		e.wantRequests(t, "/3/find/tt0133093?external_source=imdb_id")
		wantFindCounts(t, res, 1, 0, 0)
		movie := res.Movies[0]
		wantField(t, "MediaType", movie.MediaType, "movie")
		wantField(t, "TMDBID", movie.TMDBID, 603)
		wantField(t, "Title", movie.Title, "The Matrix")
		wantField(t, "Date", movie.Date, "1999-03-31")
		wantField(t, "VoteAverage", movie.VoteAverage, 8.262)
	})
	t.Run("TVDB ID", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.FindByTVDBID(ctx, 81189)
		if err != nil {
			t.Fatalf("FindByTVDBID(81189): %v", err)
		}
		e.wantRequests(t, "/3/find/81189?external_source=tvdb_id")
		wantFindCounts(t, res, 0, 1, 0)
		wantField(t, "Shows[0].TMDBID", res.Shows[0].TMDBID, 1396)
	})
	t.Run("TVDB ID of an episode", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.FindByTVDBID(ctx, 349232)
		if err != nil {
			t.Fatalf("FindByTVDBID(349232): %v", err)
		}
		e.wantRequests(t, "/3/find/349232?external_source=tvdb_id")
		wantFindCounts(t, res, 0, 0, 1)
		ep := res.Episodes[0]
		wantField(t, "TMDBID", ep.TMDBID, 62085)
		wantField(t, "ShowTMDBID", ep.ShowTMDBID, 1396)
		wantField(t, "SeasonNumber", ep.SeasonNumber, 1)
		wantField(t, "EpisodeNumber", ep.EpisodeNumber, 1)
		wantField(t, "Name", ep.Name, "Pilot")
		wantField(t, "AirDate", ep.AirDate, "2008-01-20")
		wantField(t, "Runtime", ep.Runtime, 59)
		wantField(t, "VoteAverage", ep.VoteAverage, 8.523)
	})
	t.Run("no match is not an error", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.client.FindByIMDbID(ctx, "tt0000000")
		if err != nil {
			t.Fatalf("FindByIMDbID(tt0000000): %v", err)
		}
		e.wantRequests(t, "/3/find/tt0000000?external_source=imdb_id")
		wantFindCounts(t, res, 0, 0, 0)
	})
}

// TestNotFoundMatchesErrNotFound checks that a 404 matches providers.ErrNotFound
// through every method, so a caller can tell a missing title from a failure.
func TestNotFoundMatchesErrNotFound(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		uri  string
		call func(*Client) error
	}{
		{"search", "/3/search/multi?include_adult=false&page=1&query=nosuchtitle",
			func(c *Client) error { _, err := c.Search(ctx, "nosuchtitle", 1); return err }},
		{"movie", "/3/movie/999999999?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Movie(ctx, 999999999); return err }},
		{"show", "/3/tv/999999999?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Show(ctx, 999999999); return err }},
		{"season", "/3/tv/999999999/season/1",
			func(c *Client) error { _, err := c.Season(ctx, 999999999, 1); return err }},
		{"episode", "/3/tv/999999999/season/1/episode/1?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Episode(ctx, 999999999, 1, 1); return err }},
		{"find by IMDb ID", "/3/find/tt9999999?external_source=imdb_id",
			func(c *Client) error { _, err := c.FindByIMDbID(ctx, "tt9999999"); return err }},
		{"find by TVDB ID", "/3/find/999999999?external_source=tvdb_id",
			func(c *Client) error { _, err := c.FindByTVDBID(ctx, 999999999); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if err := tt.call(e.client); !errors.Is(err, providers.ErrNotFound) {
				t.Fatalf("error = %v, want one matching providers.ErrNotFound", err)
			}
			e.wantRequests(t, tt.uri)
		})
	}
}

// TestInvalidArgumentsMakeNoRequest covers each input rule: a bad argument
// returns ErrInvalidArgument and the server never hears about it.
func TestInvalidArgumentsMakeNoRequest(t *testing.T) {
	ctx := context.Background()
	findIMDb := func(id string) func(*Client) error {
		return func(c *Client) error { _, err := c.FindByIMDbID(ctx, id); return err }
	}
	type invalidCase struct {
		name string
		call func(*Client) error
	}
	tests := []invalidCase{
		{"movie ID zero", func(c *Client) error { _, err := c.Movie(ctx, 0); return err }},
		{"movie ID negative", func(c *Client) error { _, err := c.Movie(ctx, -1); return err }},
		{"show ID zero", func(c *Client) error { _, err := c.Show(ctx, 0); return err }},
		{"season's show ID zero", func(c *Client) error { _, err := c.Season(ctx, 0, 1); return err }},
		{"season number negative", func(c *Client) error { _, err := c.Season(ctx, 1396, -1); return err }},
		{"episode's show ID zero", func(c *Client) error { _, err := c.Episode(ctx, 0, 1, 1); return err }},
		{"episode season negative", func(c *Client) error { _, err := c.Episode(ctx, 1396, -1, 1); return err }},
		{"episode number zero", func(c *Client) error { _, err := c.Episode(ctx, 1396, 1, 0); return err }},
		{"search query blank", func(c *Client) error { _, err := c.Search(ctx, " \t ", 1); return err }},
		{"search page above 500", func(c *Client) error { _, err := c.Search(ctx, "Matrix", 501); return err }},
		{"TVDB ID zero", func(c *Client) error { _, err := c.FindByTVDBID(ctx, 0); return err }},
		{"TVDB ID negative", func(c *Client) error { _, err := c.FindByTVDBID(ctx, -81189); return err }},
	}
	// IMDb IDs go into a path, so only tt and 7 to 10 digits may get through.
	for _, id := range []string{
		"",
		"0133093",
		"tt013309",
		"tt12345678901",
		"TT0133093",
		" tt0133093",
		"tt0133093 ",
		"tt0133093\n",
		"tt013309a",
		"tt0133093/../../account",
		"tt0133093?x=1",
	} {
		tests = append(tests, invalidCase{fmt.Sprintf("IMDb ID %q", id), findIMDb(id)})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if err := tt.call(e.client); !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("error = %v, want ErrInvalidArgument", err)
			}
			if n := len(e.requests()); n != 0 {
				t.Errorf("made %d requests, want none", n)
			}
		})
	}
}

// TestTokenAndQueryStayOutOfPathsAndErrors checks that the token never shows
// up in an error or a request URI, and that search text stays in the query,
// never the path.
func TestTokenAndQueryStayOutOfPathsAndErrors(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	_, err := e.client.Movie(ctx, 999999999)
	if err == nil {
		t.Fatal("Movie(999999999): no error")
	}
	wantNoSecrets(t, err, testToken)

	// No route answers this search, so the server returns a 500, which the
	// client doesn't retry. The text has characters that would change a path.
	const search = "secret title?x=y#z"
	_, err = e.client.Search(ctx, search, 1)
	if err == nil {
		t.Fatal("Search with no recorded response: no error")
	}
	wantNoSecrets(t, err, testToken, "secret")

	// The exact URIs show the path is the endpoint alone, with the search text
	// escaped into the query. wantRequests also checks the bearer token.
	e.wantRequests(t,
		"/3/movie/999999999?append_to_response=external_ids",
		"/3/search/multi?include_adult=false&page=1&query=secret+title%3Fx%3Dy%23z")
}

// wantNoSecrets fails when err's text contains one of the secrets. It doesn't
// print the secrets, so a failure doesn't leak them into test output.
func wantNoSecrets(t *testing.T, err error, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(err.Error(), s) {
			t.Error("error message contains a secret")
		}
	}
}

// TestValidDate checks that only a real day in YYYY-MM-DD form is kept. A date
// that doesn't exist, a date in another shape, and a date with a time all become
// "".
func TestValidDate(t *testing.T) {
	tests := []struct{ in, want string }{
		{"2008-01-20", "2008-01-20"},
		{"2013-00-00", ""},
		{"2013-13-01", ""},
		{"2013-02-30", ""},
		{"2013-1-1", ""},
		{"2013-01-01T00:00:00Z", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := validDate(tt.in); got != tt.want {
			t.Errorf("validDate(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestTitlesComeFromTheirOwnFields checks where each title comes from, with
// values that differ. The recorded responses mostly give a title and its
// original title the same value, so they can't tell the fields apart. The values
// are copied from real TMDB search results.
func TestTitlesComeFromTheirOwnFields(t *testing.T) {
	movie := searchResult("movie", wireSearchItem{Title: "The Matrix: Generation", OriginalTitle: "Matrix génération"})
	wantField(t, "search movie Title", movie.Title, "The Matrix: Generation")
	wantField(t, "search movie OriginalTitle", movie.OriginalTitle, "Matrix génération")
	show := searchResult("tv", wireSearchItem{Name: "Breaking Bad Fortune Teller", OriginalName: "绝命卦师"})
	wantField(t, "search show Title", show.Title, "Breaking Bad Fortune Teller")
	wantField(t, "search show OriginalTitle", show.OriginalTitle, "绝命卦师")
	m := movieFromWire(wireMovie{Title: "The Matrix: Generation", OriginalTitle: "Matrix génération"})
	wantField(t, "movie Title", m.Title, "The Matrix: Generation")
	wantField(t, "movie OriginalTitle", m.OriginalTitle, "Matrix génération")
	s := showFromWire(wireShow{Name: "Breaking Bad Fortune Teller", OriginalName: "绝命卦师"})
	wantField(t, "show Name", s.Name, "Breaking Bad Fortune Teller")
	wantField(t, "show OriginalName", s.OriginalName, "绝命卦师")
}

// TestTTLs checks each method's cache lifetime. The first call stores the
// response expiring one lifetime later; a call inside the lifetime is served
// from the cache; a call at the expiry asks TMDB again. The lifetimes are
// written out here, not read from the package constants, so a wrong constant
// fails the test.
func TestTTLs(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		ttl  time.Duration
		uri  string
		call func(*Client) error
	}{
		{"search", 24 * time.Hour, "/3/search/multi?include_adult=false&page=1&query=Matrix",
			func(c *Client) error { _, err := c.Search(ctx, "Matrix", 1); return err }},
		{"movie", 7 * 24 * time.Hour, "/3/movie/603?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Movie(ctx, 603); return err }},
		{"ended show", 24 * time.Hour, "/3/tv/1396?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Show(ctx, 1396); return err }},
		{"returning show", 24 * time.Hour, "/3/tv/94997?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Show(ctx, 94997); return err }},
		{"season", 24 * time.Hour, "/3/tv/1396/season/1",
			func(c *Client) error { _, err := c.Season(ctx, 1396, 1); return err }},
		{"episode", 24 * time.Hour, "/3/tv/1396/season/1/episode/1?append_to_response=external_ids",
			func(c *Client) error { _, err := c.Episode(ctx, 1396, 1, 1); return err }},
		{"find by IMDb ID", 30 * 24 * time.Hour, "/3/find/tt0903747?external_source=imdb_id",
			func(c *Client) error { _, err := c.FindByIMDbID(ctx, "tt0903747"); return err }},
		{"find by TVDB ID", 30 * 24 * time.Hour, "/3/find/81189?external_source=tvdb_id",
			func(c *Client) error { _, err := c.FindByTVDBID(ctx, 81189); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			// The cache key is the path and query, which is the URI without BaseURL's /3.
			key := strings.TrimPrefix(tt.uri, "/3")
			call := func() {
				t.Helper()
				if err := tt.call(e.client); err != nil {
					t.Fatalf("call: %v", err)
				}
			}

			call()
			e.wantRequests(t, tt.uri)
			wantExpiry := db.Timestamp(testNow.Add(tt.ttl))
			if got := e.cached(t); len(got) != 1 || got[key] != wantExpiry {
				t.Fatalf("provider_cache = %v, want one row %q expiring %s", got, key, wantExpiry)
			}

			call() // within the lifetime: no request
			e.wantRequests(t, tt.uri)

			e.clock.Advance(tt.ttl - time.Second)
			call() // one second before expiry: still no request
			e.wantRequests(t, tt.uri)

			e.clock.Advance(time.Second)
			call() // at expiry: asked again
			e.wantRequests(t, tt.uri, tt.uri)
		})
	}
}
