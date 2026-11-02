// Command fake-tmdb serves watcht's recorded TMDB responses on a loopback
// address, so watcht can run without TMDB or a network connection, for
// `make dev` and visual checks:
//
//	go run ./scripts/fake-tmdb
//	WATCHT_TMDB_BASE_URL=http://127.0.0.1:8098/3 WATCHT_TMDB_API_KEY=dev watcht serve
//
// A request the fixtures cover gets its recorded response. Anything else gets
// TMDB's 404 body. The API key isn't checked. Run it from the repository root,
// or pass -fixtures. It's for development only.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListen   = "127.0.0.1:8098"
	defaultFixtures = "internal/providers/tmdb/testdata"
)

// notFoundFixture is TMDB's 404 body, sent for any request without a route.
const notFoundFixture = "movie_not_found.json"

// pathFixtures maps a request path to its recorded response. The query is
// ignored, so append_to_response doesn't change which fixture applies.
var pathFixtures = map[string]string{
	"/3/movie/603":                  "movie_603.json",
	"/3/tv/1396":                    "tv_1396.json",
	"/3/tv/94997":                   "tv_94997.json",
	"/3/tv/1437":                    "tv_1437.json",
	"/3/tv/1396/season/0":           "tv_1396_season_0.json",
	"/3/tv/1396/season/1":           "tv_1396_season_1.json",
	"/3/tv/1396/season/1/episode/1": "tv_1396_season_1_episode_1.json",
}

// findFixtures maps "external_source/ID" to a recorded find response. Find
// reads the query, because a number alone doesn't say whether it's a TVDB ID.
var findFixtures = map[string]string{
	"imdb_id/tt0133093": "find_imdb_tt0133093.json",
	"imdb_id/tt0903747": "find_imdb_tt0903747.json",
	"imdb_id/tt0000000": "find_none.json",
	"tvdb_id/81189":     "find_tvdb_81189.json",
	"tvdb_id/349232":    "find_tvdb_349232.json",
}

// searchFixture picks the multi search response by query. "harrison ford" is a
// person among movies and shows, a query containing "matrix" gets the Matrix
// results, and anything else gets none. Page 2 and later are always empty.
func searchFixture(q url.Values) string {
	if page, err := strconv.Atoi(q.Get("page")); err == nil && page > 1 {
		return "search_multi_empty.json"
	}
	query := strings.ToLower(strings.TrimSpace(q.Get("query")))
	switch {
	case query == "harrison ford":
		return "search_multi_person.json"
	case strings.Contains(query, "matrix"):
		return "search_multi.json"
	default:
		return "search_multi_empty.json"
	}
}

// fixtureFor names the recorded response for a request, or "" when there is none.
func fixtureFor(r *http.Request) string {
	if r.URL.Path == "/3/search/multi" {
		return searchFixture(r.URL.Query())
	}
	if id, ok := strings.CutPrefix(r.URL.Path, "/3/find/"); ok {
		return findFixtures[r.URL.Query().Get("external_source")+"/"+id]
	}
	return pathFixtures[r.URL.Path]
}

// handler answers from the fixture files. It reads each file when it's
// requested, so an edited fixture is served on the next request.
type handler struct {
	fixtures fs.FS
	logger   *slog.Logger
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	name := fixtureFor(r)
	if name == "" {
		status, name = http.StatusNotFound, notFoundFixture
	}
	body, err := fs.ReadFile(h.fixtures, name)
	if err != nil {
		h.logger.Error("fixture unreadable", "file", name, "err", err)
		status, body = http.StatusInternalServerError, []byte(`{"success":false,"status_message":"fixture unreadable"}`)
	}
	h.logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body) //nolint:gosec // G705: the body is one recorded fixture file (the request only picks which), sent as application/json
}

// checkLoopback refuses an address that isn't on the loopback interface, so
// the fake can't be reached from the network. The host must be an IP on
// loopback, or localhost.
func checkLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("-listen %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-listen %q must be a loopback address such as 127.0.0.1:8098", addr)
	}
	return nil
}

func main() {
	listen := flag.String("listen", defaultListen, "loopback address to serve on")
	fixtures := flag.String("fixtures", defaultFixtures, "directory of recorded TMDB responses")
	flag.Parse()
	if err := run(*listen, *fixtures, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fake-tmdb:", err)
		os.Exit(1)
	}
}

// run serves the fixtures in dir on listen until the process stops.
func run(listen, dir string, stderr io.Writer) error {
	if err := checkLoopback(listen); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("fixtures: %w (run from the repository root, or pass -fixtures)", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("fixtures %q is not a directory", dir)
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	logger.Info("serving recorded TMDB responses", "base_url", "http://"+listen+"/3", "fixtures", dir)
	srv := &http.Server{
		Addr:              listen,
		Handler:           handler{fixtures: os.DirFS(dir), logger: logger},
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
