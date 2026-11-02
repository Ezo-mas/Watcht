package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testFixtures is the recorded TMDB data, relative to this package. The
// routes name these files, so a renamed fixture fails the tests here.
const testFixtures = "../../internal/providers/tmdb/testdata"

func TestRoutes(t *testing.T) {
	srv := httptest.NewServer(handler{fixtures: os.DirFS(testFixtures), logger: slog.New(slog.DiscardHandler)})
	t.Cleanup(srv.Close)

	tests := []struct {
		target string
		status int
		file   string
	}{
		{"/3/search/multi?query=Harrison+Ford&page=1", http.StatusOK, "search_multi_person.json"},
		{"/3/search/multi?query=%20HARRISON%20FORD%20", http.StatusOK, "search_multi_person.json"},
		{"/3/search/multi?query=The+Matrix", http.StatusOK, "search_multi.json"},
		{"/3/search/multi?query=matrix&page=2", http.StatusOK, "search_multi_empty.json"},
		{"/3/search/multi?query=harrison+ford&page=2", http.StatusOK, "search_multi_empty.json"},
		{"/3/search/multi?query=zzqxjvnonexistenttitle", http.StatusOK, "search_multi_empty.json"},
		{"/3/movie/603?append_to_response=external_ids", http.StatusOK, "movie_603.json"},
		{"/3/tv/1396?append_to_response=external_ids", http.StatusOK, "tv_1396.json"},
		{"/3/tv/94997?append_to_response=external_ids", http.StatusOK, "tv_94997.json"},
		{"/3/tv/1437?append_to_response=external_ids", http.StatusOK, "tv_1437.json"},
		{"/3/tv/1396/season/0", http.StatusOK, "tv_1396_season_0.json"},
		{"/3/tv/1396/season/1", http.StatusOK, "tv_1396_season_1.json"},
		{"/3/tv/1396/season/1/episode/1?append_to_response=external_ids", http.StatusOK, "tv_1396_season_1_episode_1.json"},
		{"/3/find/tt0903747?external_source=imdb_id", http.StatusOK, "find_imdb_tt0903747.json"},
		{"/3/find/tt0133093?external_source=imdb_id", http.StatusOK, "find_imdb_tt0133093.json"},
		{"/3/find/tt0000000?external_source=imdb_id", http.StatusOK, "find_none.json"},
		{"/3/find/81189?external_source=tvdb_id", http.StatusOK, "find_tvdb_81189.json"},
		{"/3/find/349232?external_source=tvdb_id", http.StatusOK, "find_tvdb_349232.json"},
		// Not recorded, so TMDB's own 404 body. Seasons 2 to 5 of show 1396 are
		// listed by TMDB but not recorded.
		{"/3/movie/999999999", http.StatusNotFound, "movie_not_found.json"},
		{"/3/tv/1396/season/2", http.StatusNotFound, "movie_not_found.json"},
		{"/3/tv/1396/season/1/episode/2", http.StatusNotFound, "movie_not_found.json"},
		{"/3/find/81189?external_source=imdb_id", http.StatusNotFound, "movie_not_found.json"},
		{"/3/find/tt0903747", http.StatusNotFound, "movie_not_found.json"},
		{"/3/nothing/here", http.StatusNotFound, "movie_not_found.json"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.target)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.status)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			want, err := os.ReadFile(filepath.Join(testFixtures, tt.file))
			if err != nil {
				t.Fatalf("recorded response %s: %v", tt.file, err)
			}
			if !bytes.Equal(body, want) {
				t.Errorf("body is not %s", tt.file)
			}
		})
	}
}

func TestRequestLog(t *testing.T) {
	var logs bytes.Buffer
	h := handler{fixtures: os.DirFS(testFixtures), logger: slog.New(slog.NewTextHandler(&logs, nil))}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/3/movie/999999999?language=en", nil))
	line := logs.String()
	for _, want := range []string{"method=GET", "path=/3/movie/999999999", "status=404"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "language=en") {
		t.Errorf("log line %q includes the query", line)
	}
}

func TestCheckLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8098", "127.0.0.2:0", "[::1]:8098", "localhost:8098"} {
		if err := checkLoopback(addr); err != nil {
			t.Errorf("checkLoopback(%q) = %v, want nil", addr, err)
		}
	}
	for _, addr := range []string{":8098", "0.0.0.0:8098", "[::]:8098", "192.168.1.10:8098", "example.com:8098", "127.0.0.1", "[fe80::1%eth0]:8098"} {
		if err := checkLoopback(addr); err == nil {
			t.Errorf("checkLoopback(%q) = nil, want an error", addr)
		}
	}
}
