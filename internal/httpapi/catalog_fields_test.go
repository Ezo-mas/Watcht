package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Ezo-mas/watcht/internal/catalog"
)

func assertCatalogMovieFixture(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	fixture := catalogFieldsFixture(t, "movie_603.json")
	want := catalogFixtureFields(fixture, "imdb_id title original_title original_language overview tagline status poster_path backdrop_path release_date runtime vote_average vote_count")
	want["tmdb_id"] = fixture["id"]
	want["genres"] = catalogFixtureGenres(fixture)
	want["poster_url"] = fixtureImageURL("w342", fixture["poster_path"])
	want["backdrop_url"] = fixtureImageURL("w1280", fixture["backdrop_path"])
	assertCatalogFields(t, rec, "movie", want)
}

func assertCatalogShowFixture(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	fixture := catalogFieldsFixture(t, "tv_1396.json")
	want := catalogFixtureFields(fixture, "name original_name original_language overview tagline status poster_path backdrop_path first_air_date last_air_date in_production vote_average vote_count")
	want["tmdb_id"] = fixture["id"]
	externalIDs := fixture["external_ids"].(map[string]any)
	want["imdb_id"] = externalIDs["imdb_id"]
	want["tvdb_id"] = externalIDs["tvdb_id"]
	want["genres"] = catalogFixtureGenres(fixture)
	want["poster_url"] = fixtureImageURL("w342", fixture["poster_path"])
	want["backdrop_url"] = fixtureImageURL("w1280", fixture["backdrop_path"])
	seasons := make([]map[string]any, 0)
	for _, value := range fixture["seasons"].([]any) {
		season := value.(map[string]any)
		item := catalogFixtureFields(season, "season_number name poster_path air_date episode_count")
		item["tmdb_id"] = season["id"]
		item["poster_url"] = fixtureImageURL("w342", season["poster_path"])
		// The initial show fetch stores only the season summary; the TMDB
		// adapter doesn't include its overview until the season is fetched.
		item["overview"] = ""
		item["episodes_pending"] = true
		seasons = append(seasons, item)
	}
	sort.Slice(seasons, func(i, j int) bool {
		if seasons[i]["season_number"] == seasons[j]["season_number"] {
			return seasons[i]["tmdb_id"].(float64) < seasons[j]["tmdb_id"].(float64)
		}
		return seasons[i]["season_number"].(float64) < seasons[j]["season_number"].(float64)
	})
	want["seasons"] = seasons
	assertCatalogFields(t, rec, "show", want)
}

func catalogFieldsFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../providers/tmdb/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func catalogFixtureFields(fixture map[string]any, fields string) map[string]any {
	result := make(map[string]any)
	for _, name := range strings.Fields(fields) {
		result[name] = fixture[name]
	}
	return result
}

func catalogFixtureGenres(fixture map[string]any) []map[string]any {
	genres := make([]map[string]any, 0)
	for _, value := range fixture["genres"].([]any) {
		genre := value.(map[string]any)
		genres = append(genres, map[string]any{"tmdb_id": genre["id"], "name": genre["name"]})
	}
	sort.Slice(genres, func(i, j int) bool {
		if genres[i]["name"] == genres[j]["name"] {
			return genres[i]["tmdb_id"].(float64) < genres[j]["tmdb_id"].(float64)
		}
		return genres[i]["name"].(string) < genres[j]["name"].(string)
	})
	return genres
}

// fixtureImageURL is the URL the API should return for a fixture's image
// path. It's spelled out here, not built with tmdbImage, so the size and the
// base are checked too. A null path means no image.
func fixtureImageURL(size string, path any) string {
	p, _ := path.(string)
	if p == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/" + size + p
}

func assertCatalogFields(t *testing.T, rec *httptest.ResponseRecorder, key string, want map[string]any) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("GET %s: %d %s", key, rec.Code, rec.Body)
	}
	var got any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// Compare raw JSON, including all keys, so missing fields and accidental
	// exposure of internal IDs or metadata timestamps both fail.
	var expectedBody any = want
	if key != "" {
		expectedBody = map[string]any{key: want}
	}
	data, err := json.Marshal(expectedBody)
	if err != nil {
		t.Fatal(err)
	}
	var expected any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("%s response = %s, want %s", key, rec.Body, data)
	}
}

func TestCatalogSeasonFields(t *testing.T) {
	s := newCatalogServer(t)
	catalogDecode[map[string]any](t, s.get("/api/v1/shows/1396"))
	season, err := s.client.Season(context.Background(), 1396, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.service.UpsertSeason(context.Background(), 1396, season); err != nil {
		t.Fatal(err)
	}

	fixture := catalogFieldsFixture(t, "tv_1396_season_1.json")
	wantSeason := catalogFixtureFields(fixture, "season_number name overview poster_path air_date")
	wantSeason["tmdb_id"] = fixture["id"]
	wantSeason["episodes_pending"] = false
	wantSeason["poster_url"] = fixtureImageURL("w342", fixture["poster_path"])
	show := catalogFieldsFixture(t, "tv_1396.json")
	for _, value := range show["seasons"].([]any) {
		summary := value.(map[string]any)
		if summary["season_number"] == fixture["season_number"] {
			wantSeason["episode_count"] = summary["episode_count"]
		}
	}
	episodes := catalogEpisodeFields(t, "tv_1396_season_1.json")
	if len(episodes) < 2 {
		t.Fatal("fixture must cover at least two episodes")
	}
	assertCatalogFields(t, s.get("/api/v1/shows/1396/seasons/1"), "", map[string]any{
		"show_tmdb_id":     1396,
		"season":           wantSeason,
		"episodes_pending": false,
		"episodes":         episodes,
	})
}

// catalogEpisodeFields is the JSON the API should return for each episode of a
// season fixture, in the fixture's order.
func catalogEpisodeFields(t *testing.T, name string) []map[string]any {
	t.Helper()
	fixture := catalogFieldsFixture(t, name)
	episodes := make([]map[string]any, 0)
	for _, value := range fixture["episodes"].([]any) {
		episode := value.(map[string]any)
		want := catalogFixtureFields(episode, "season_number episode_number name overview still_path air_date runtime vote_average")
		// TMDB sends null for an episode with no air date. The API sends "" for
		// any unknown string, as it does for the stored NULL.
		if want["air_date"] == nil {
			want["air_date"] = ""
		}
		want["tmdb_id"] = episode["id"]
		want["still_url"] = fixtureImageURL("w300", episode["still_path"])
		want["still_large_url"] = fixtureImageURL("original", episode["still_path"])
		episodes = append(episodes, want)
	}
	return episodes
}

// TestEpisodeStillURLs checks both still sizes. Each is "" when the episode has
// no still, and the key is present either way.
func TestEpisodeStillURLs(t *testing.T) {
	got := episodeResponse(catalog.Episode{StillPath: "/abc.jpg"})
	if got.StillURL != "https://image.tmdb.org/t/p/w300/abc.jpg" || got.StillLargeURL != "https://image.tmdb.org/t/p/original/abc.jpg" {
		t.Fatalf("still URLs = %q, %q", got.StillURL, got.StillLargeURL)
	}
	none := episodeResponse(catalog.Episode{})
	if none.StillURL != "" || none.StillLargeURL != "" {
		t.Fatalf("still URLs without a path = %q, %q; want both empty", none.StillURL, none.StillLargeURL)
	}
	data, err := json.Marshal(none)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"still_large_url":""`) {
		t.Fatalf("episode JSON = %s; want still_large_url present and empty", data)
	}
}

func TestCatalogSearchFields(t *testing.T) {
	s := newCatalogServer(t)
	fixture := catalogFieldsFixture(t, "search_multi.json")
	want := catalogFixtureFields(fixture, "page total_pages total_results")
	results := make([]map[string]any, 0)
	var movies, shows int
	for _, value := range fixture["results"].([]any) {
		result := value.(map[string]any)
		item := catalogFixtureFields(result, "overview poster_path vote_average")
		item["tmdb_id"] = result["id"]
		item["poster_url"] = fixtureImageURL("w342", result["poster_path"])
		switch result["media_type"] {
		case "movie":
			item["media_type"] = "movie"
			item["title"] = result["title"]
			item["original_title"] = result["original_title"]
			item["date"] = result["release_date"]
			movies++
		case "tv":
			item["media_type"] = "show"
			item["title"] = result["name"]
			item["original_title"] = result["original_name"]
			item["date"] = result["first_air_date"]
			shows++
		default:
			continue
		}
		results = append(results, item)
	}
	if movies == 0 || shows == 0 {
		t.Fatal("fixture must cover a movie and a show")
	}
	want["results"] = results
	assertCatalogFields(t, s.get("/api/v1/search?q=matrix"), "", want)
}

func TestTMDBImage(t *testing.T) {
	tests := []struct {
		name string
		size string
		path string
		want string
	}{
		{name: "no image", size: "w342", path: "", want: ""},
		{name: "poster", size: "w342", path: "/abc.jpg", want: "https://image.tmdb.org/t/p/w342/abc.jpg"},
		{name: "backdrop", size: "w1280", path: "/def.jpg", want: "https://image.tmdb.org/t/p/w1280/def.jpg"},
		{name: "no leading slash", size: "w342", path: "abc.jpg", want: ""},
		{name: "absolute URL", size: "w342", path: "https://example.com/abc.jpg", want: ""},
		// The host is fixed, so a path that starts with // stays on TMDB's image host.
		{name: "protocol-relative path", size: "w342", path: "//example.com/abc.jpg", want: "https://image.tmdb.org/t/p/w342//example.com/abc.jpg"},
		{name: "query", size: "w342", path: "/abc.jpg?size=big", want: ""},
		{name: "fragment", size: "w342", path: "/abc.jpg#top", want: ""},
		{name: "parent directory", size: "w342", path: "/../abc.jpg", want: ""},
		{name: "dots in a name", size: "w342", path: "/a..b.jpg", want: ""},
		{name: "backslash", size: "w342", path: `/a\b.jpg`, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tmdbImage(tt.size, tt.path); got != tt.want {
				t.Errorf("tmdbImage(%q, %q) = %q, want %q", tt.size, tt.path, got, tt.want)
			}
		})
	}
}
