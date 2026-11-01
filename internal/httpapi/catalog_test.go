package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

type catalogFixture struct {
	mu     sync.Mutex
	uris   []string
	status int
}

func (f *catalogFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.uris = append(f.uris, r.URL.RequestURI())
	status := f.status
	f.mu.Unlock()
	if status != 0 {
		http.Error(w, "private provider error", status)
		return
	}
	file := ""
	switch r.URL.Path {
	case "/3/search/multi":
		file = "search_multi.json"
		if r.URL.Query().Get("query") == "person" {
			file = "search_multi_person.json"
		}
		if r.URL.Query().Get("query") == "empty" {
			file = "search_multi_empty.json"
		}
	case "/3/movie/603":
		file = "movie_603.json"
	case "/3/tv/1396":
		file = "tv_1396.json"
	case "/3/tv/1396/season/0":
		file = "tv_1396_season_0.json"
	case "/3/tv/1396/season/1":
		file = "tv_1396_season_1.json"
	default:
		w.WriteHeader(http.StatusNotFound)
		file = "movie_not_found.json"
	}
	b, err := os.ReadFile("../providers/tmdb/testdata/" + file)
	if err != nil {
		http.Error(w, "fixture unreadable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
func (f *catalogFixture) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.uris) }

type catalogTestServer struct {
	*authServer
	fixture *catalogFixture
	service *catalog.Service
	client  *tmdb.Client
	cookie  *http.Cookie
}

func newCatalogServer(t *testing.T) *catalogTestServer {
	t.Helper()
	s := newAuthServer(t, authServerOptions{})
	f := &catalogFixture{}
	upstream := httptest.NewServer(f)
	t.Cleanup(upstream.Close)
	client := tmdb.New(s.db, tmdb.Options{Token: "fixture-token", BaseURL: upstream.URL + "/3", HTTPClient: upstream.Client()})
	q := jobs.New(s.db, jobs.Options{})
	service := catalog.New(s.db, client, catalog.Options{Jobs: q})
	logger := slog.New(slog.NewTextHandler(s.logs, nil))
	h, err := NewRouter(Deps{Logger: logger, Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2}), WebUI: http.NotFoundHandler(), Catalog: service, Search: client})
	if err != nil {
		t.Fatal(err)
	}
	s.h = h
	return &catalogTestServer{s, f, service, client, s.setup("admin", "long enough")}
}
func (s *catalogTestServer) get(path string) *httptest.ResponseRecorder {
	return s.do(request{method: "GET", path: path, cookie: s.cookie})
}
func catalogDecode[T any](t *testing.T, r *httptest.ResponseRecorder) T {
	t.Helper()
	if r.Code != 200 {
		t.Fatalf("GET: %d %s", r.Code, r.Body)
	}
	var v T
	if err := json.Unmarshal(r.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func catalogError(t *testing.T, r *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if r.Code != status || decodeError(t, r).Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Code, r.Body, status, code)
	}
}

func TestCatalogSearch(t *testing.T) {
	s := newCatalogServer(t)
	got := catalogDecode[searchJSON](t, s.get("/api/v1/search?q=%20matrix%20"))
	if len(got.Results) != 5 || got.Results[0].MediaType != "movie" || got.Results[1].MediaType != "show" || got.Results[0].TMDBID != 603 {
		t.Fatalf("search: %+v", got)
	}
	if s.fixture.uris[0] != "/3/search/multi?include_adult=false&page=1&query=matrix" {
		t.Fatal(s.fixture.uris)
	}
	people := catalogDecode[searchJSON](t, s.get("/api/v1/search?q=person"))
	if len(people.Results) != 4 {
		t.Fatalf("people were not dropped: %+v", people)
	}
	for _, result := range people.Results {
		if result.MediaType != "movie" && result.MediaType != "show" {
			t.Fatalf("unexpected type %q", result.MediaType)
		}
	}
	before := s.fixture.count()
	for _, q := range []string{"", "   ", strings.Repeat("界", 101)} {
		catalogError(t, s.get("/api/v1/search?q="+url.QueryEscape(q)), 400, "invalid_query")
	}
	for _, page := range []string{"", "abc", "0", "-1", "501", "1.5", "99999999999999999999999"} {
		catalogError(t, s.get("/api/v1/search?q=x&page="+page), 400, "invalid_page")
	}
	if s.fixture.count() != before {
		t.Fatal("invalid input reached TMDB")
	}
	catalogDecode[searchJSON](t, s.get("/api/v1/search?q="+url.QueryEscape(strings.Repeat("界", 100))+"&page=500"))
	empty := catalogDecode[searchJSON](t, s.get("/api/v1/search?q=empty"))
	if empty.Results == nil || len(empty.Results) != 0 {
		t.Fatal("empty results must be []")
	}
	s.fixture.mu.Lock()
	s.fixture.status = 503
	s.fixture.mu.Unlock()
	rec := s.get("/api/v1/search?q=uncached")
	catalogError(t, rec, 502, "provider_unavailable")
	if strings.Contains(rec.Body.String(), "private") {
		t.Fatal("provider error leaked")
	}
}

func TestCatalogMovieStoredFirst(t *testing.T) {
	s := newCatalogServer(t)
	type response struct {
		Movie movieJSON `json:"movie"`
	}
	rec := s.get("/api/v1/movies/603")
	assertCatalogMovieFixture(t, rec)
	first := catalogDecode[response](t, rec)
	if first.Movie.Title != "The Matrix" || first.Movie.IMDbID != "tt0133093" || first.Movie.Runtime != 136 || len(first.Movie.Genres) != 2 {
		t.Fatalf("movie: %+v", first)
	}
	// Expire provider cache as well: an unconditional fetch must hit TMDB.
	if _, err := s.db.Write.Exec("DELETE FROM provider_cache"); err != nil {
		t.Fatal(err)
	}
	before := s.fixture.count()
	catalogDecode[response](t, s.get("/api/v1/movies/603"))
	if s.fixture.count() != before {
		t.Fatal("stored movie fetched again")
	}
	var viewed string
	if err := s.db.Read.QueryRow("SELECT last_viewed_at FROM movies WHERE tmdb_id=603").Scan(&viewed); err != nil || viewed == "" {
		t.Fatalf("viewed %q: %v", viewed, err)
	}
	for _, id := range []string{"abc", "0", "-1", "+603", "2147483648", "99999999999999999999999"} {
		catalogError(t, s.get("/api/v1/movies/"+id), 404, "not_found")
	}
	if s.fixture.count() != before {
		t.Fatal("bad IDs reached TMDB")
	}
	catalogError(t, s.get("/api/v1/movies/999"), 404, "not_found")
}

func TestCatalogShowAndSeason(t *testing.T) {
	s := newCatalogServer(t)
	type response struct {
		Show showJSON `json:"show"`
	}
	rec := s.get("/api/v1/shows/1396")
	assertCatalogShowFixture(t, rec)
	show := catalogDecode[response](t, rec).Show
	if show.Name != "Breaking Bad" || len(show.Genres) == 0 || len(show.Seasons) != 6 {
		t.Fatalf("show: %+v", show)
	}
	for _, season := range show.Seasons {
		if !season.EpisodesPending {
			t.Fatal("new season not pending")
		}
	}
	var n int
	if err := s.db.Read.QueryRow("SELECT count(*) FROM jobs WHERE unique_key='catalog_sync_show:1396'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("sync jobs %d: %v", n, err)
	}
	before := s.fixture.count()
	pending := catalogDecode[seasonJSON](t, s.get("/api/v1/shows/1396/seasons/1"))
	if !pending.EpisodesPending || !pending.Season.EpisodesPending || pending.Episodes != nil {
		t.Fatalf("pending: %+v", pending)
	}
	if _, err := s.db.Write.Exec("DELETE FROM provider_cache"); err != nil {
		t.Fatal(err)
	}
	catalogDecode[response](t, s.get("/api/v1/shows/1396"))
	for _, path := range []string{"/api/v1/shows/999/seasons/1", "/api/v1/shows/1396/seasons/99", "/api/v1/shows/1396/seasons/-1", "/api/v1/shows/abc", "/api/v1/shows/2147483648", "/api/v1/shows/1396/seasons/2147483648"} {
		catalogError(t, s.get(path), 404, "not_found")
	}
	if s.fixture.count() != before {
		t.Fatal("stored show or season fetched TMDB")
	}
	if err := s.service.SyncShow(context.Background(), 1396); err != nil {
		t.Fatal(err)
	}
	ready := catalogDecode[seasonJSON](t, s.get("/api/v1/shows/1396/seasons/1"))
	if ready.EpisodesPending || ready.Season.EpisodesPending || len(ready.Episodes) != 7 || ready.Episodes[0].TMDBID == 0 {
		t.Fatalf("ready: %+v", ready)
	}
	catalogError(t, s.get("/api/v1/shows/1396/seasons/2"), 404, "not_found")
	if _, err := s.db.Write.Exec("UPDATE seasons SET missing_since=? WHERE season_number=1", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	catalogError(t, s.get("/api/v1/shows/1396/seasons/1"), 404, "not_found")
	// A fetched season can have no episodes, distinct from pending.
	season, err := s.client.Season(context.Background(), 1396, 0)
	if err != nil {
		t.Fatal(err)
	}
	season.Episodes = nil
	if err := s.service.UpsertSeason(context.Background(), 1396, season); err != nil {
		t.Fatal(err)
	}
	empty := catalogDecode[seasonJSON](t, s.get("/api/v1/shows/1396/seasons/0"))
	if empty.EpisodesPending || empty.Episodes == nil || len(empty.Episodes) != 0 {
		t.Fatal("fetched empty season must return []")
	}
}

func TestCatalogAuthenticationAndConfiguration(t *testing.T) {
	paths := []string{"/api/v1/search?q=matrix", "/api/v1/movies/603", "/api/v1/shows/1396", "/api/v1/shows/1396/seasons/1"}
	s := newCatalogServer(t)
	for _, path := range paths {
		catalogError(t, s.do(request{method: "GET", path: path}), 401, "unauthenticated")
	}
	tokenRec := s.do(request{method: "POST", path: "/api/v1/tokens", body: `{"name":"catalog","password":"long enough"}`, cookie: s.cookie})
	var token struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &token); err != nil || token.Secret == "" {
		t.Fatalf("token: %d %s", tokenRec.Code, tokenRec.Body)
	}
	for _, path := range paths {
		rec := s.do(request{method: "GET", path: path, header: map[string]string{"Authorization": "Bearer " + token.Secret}})
		if rec.Code != 200 {
			t.Fatalf("API token %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	unconfigured := newAuthServer(t, authServerOptions{})
	cookie := unconfigured.setup("admin", "long enough")
	for _, path := range paths {
		catalogError(t, unconfigured.do(request{method: "GET", path: path}), 401, "unauthenticated")
		rec := unconfigured.do(request{method: "GET", path: path, cookie: cookie})
		catalogError(t, rec, 503, "tmdb_not_configured")
		if !strings.Contains(rec.Body.String(), "WATCHT_TMDB_API_KEY") {
			t.Fatal("missing configuration advice")
		}
	}
}

type catalogSearchFunc func(context.Context, string, int) (tmdb.SearchResults, error)

func (f catalogSearchFunc) Search(ctx context.Context, q string, page int) (tmdb.SearchResults, error) {
	return f(ctx, q, page)
}

type catalogViewFailure struct{ Catalog }

func (c catalogViewFailure) MarkMovieViewed(context.Context, int) error {
	return errors.New("view write failed")
}
func (c catalogViewFailure) MarkShowViewed(context.Context, int) error {
	return errors.New("view write failed")
}

func TestCatalogErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid argument", tmdb.ErrInvalidArgument, 400, "invalid_request"},
		{"rate limited", &providers.StatusError{StatusCode: 429}, 502, "provider_unavailable"},
		{"internal", errors.New("private internal error"), 500, "internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newCatalogServer(t)
			search := catalogSearchFunc(func(context.Context, string, int) (tmdb.SearchResults, error) { return tmdb.SearchResults{}, tc.err })
			h, err := NewRouter(Deps{Logger: slog.New(slog.NewTextHandler(s.logs, nil)), Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2}), WebUI: http.NotFoundHandler(), Catalog: s.service, Search: search})
			if err != nil {
				t.Fatal(err)
			}
			s.h = h
			rec := s.get("/api/v1/search?q=x")
			catalogError(t, rec, tc.status, tc.code)
			if strings.Contains(rec.Body.String(), "private") {
				t.Fatal("raw error leaked")
			}
			if tc.status >= 500 && !strings.Contains(s.logs.String(), "level=ERROR") {
				t.Fatal("failure was not logged")
			}
			if tc.status == 502 && strings.Contains(s.logs.String(), "private") {
				t.Fatal("provider URL or cause leaked in log")
			}
		})
	}
}

func TestCatalogClientCancelled(t *testing.T) {
	s := newCatalogServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	search := catalogSearchFunc(func(context.Context, string, int) (tmdb.SearchResults, error) {
		cancel()
		return tmdb.SearchResults{}, ctx.Err()
	})
	h, err := NewRouter(Deps{Logger: slog.New(slog.NewTextHandler(s.logs, nil)), Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2}), WebUI: http.NotFoundHandler(), Catalog: s.service, Search: search})
	if err != nil {
		t.Fatal(err)
	}
	s.h = h
	rec := s.do(request{method: "GET", path: "/api/v1/search?q=x", cookie: s.cookie, ctx: ctx})
	if rec.Body.Len() != 0 || strings.Contains(s.logs.String(), "level=ERROR") {
		t.Fatalf("cancelled request: %s; logs %s", rec.Body, s.logs)
	}
}

func TestCatalogViewFailureStillReturnsPage(t *testing.T) {
	s := newCatalogServer(t)
	h, err := NewRouter(Deps{Logger: slog.New(slog.NewTextHandler(s.logs, nil)), Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2}), WebUI: http.NotFoundHandler(), Catalog: catalogViewFailure{s.service}, Search: s.client})
	if err != nil {
		t.Fatal(err)
	}
	s.h = h
	for _, path := range []string{"/api/v1/movies/603", "/api/v1/shows/1396", "/api/v1/shows/1396/seasons/1"} {
		if rec := s.get(path); rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if strings.Count(s.logs.String(), "level=WARN") != 3 {
		t.Fatalf("view failure warnings: %s", s.logs)
	}
}

func TestCatalogSearchWhitespaceCache(t *testing.T) {
	s := newCatalogServer(t)
	for _, q := range []string{"the  matrix", "the matrix", " \tthe\n matrix  "} {
		catalogDecode[searchJSON](t, s.get("/api/v1/search?q="+url.QueryEscape(q)))
	}
	if s.fixture.count() != 1 {
		t.Fatalf("equivalent queries made %d TMDB requests, want 1", s.fixture.count())
	}
	if s.fixture.uris[0] != "/3/search/multi?include_adult=false&page=1&query=the+matrix" {
		t.Fatalf("query URI: %s", s.fixture.uris[0])
	}
	catalogDecode[searchJSON](t, s.get("/api/v1/search?q="+url.QueryEscape("the"+strings.Repeat(" ", 101)+"matrix")))
	catalogDecode[searchJSON](t, s.get("/api/v1/search?q="+url.QueryEscape("The Matrix")))
	if s.fixture.count() != 2 {
		t.Fatal("query normalization must preserve case")
	}
}

func TestCatalogRealProviderFailures(t *testing.T) {
	for _, kind := range []string{"connection refused", "connection closed", "TLS untrusted", "HTML body"} {
		t.Run(kind, func(t *testing.T) {
			s := newCatalogServer(t)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "connection closed" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					_ = conn.Close()
					return
				}
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte("<html>private response</html>"))
			})
			var upstream *httptest.Server
			if kind == "TLS untrusted" {
				upstream = httptest.NewTLSServer(handler)
			} else {
				upstream = httptest.NewServer(handler)
			}
			t.Cleanup(upstream.Close)
			if kind == "connection refused" {
				upstream.Close()
			}
			logger := slog.New(slog.NewTextHandler(s.logs, nil))
			// The default client intentionally doesn't trust the TLS test server.
			client := tmdb.New(s.db, tmdb.Options{Token: "fixture-token", BaseURL: upstream.URL + "/3", Logger: logger})
			h, err := NewRouter(Deps{Logger: logger, Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2}), WebUI: http.NotFoundHandler(), Catalog: s.service, Search: client})
			if err != nil {
				t.Fatal(err)
			}
			s.h = h
			const query = "distinctive-private-search-9f83"
			rec := s.get("/api/v1/search?q=" + query)
			catalogError(t, rec, 502, "provider_unavailable")
			for _, secret := range []string{query, "private response", "fixture-token"} {
				if strings.Contains(rec.Body.String(), secret) || strings.Contains(s.logs.String(), secret) {
					t.Fatalf("%q leaked: response %s logs %s", secret, rec.Body, s.logs)
				}
			}
			if !strings.Contains(s.logs.String(), "level=ERROR") || !strings.Contains(s.logs.String(), "err=") {
				t.Fatalf("failure not logged with cause: %s", s.logs)
			}
		})
	}
}
