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
)

func assertCatalogMovieFixture(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	fixture := catalogFieldsFixture(t, "movie_603.json")
	want := catalogFixtureFields(fixture, "imdb_id title original_title original_language overview tagline status poster_path backdrop_path release_date runtime vote_average vote_count")
	want["tmdb_id"] = fixture["id"]
	want["genres"] = catalogFixtureGenres(fixture)
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
	seasons := make([]map[string]any, 0)
	for _, value := range fixture["seasons"].([]any) {
		season := value.(map[string]any)
		item := catalogFixtureFields(season, "season_number name poster_path air_date episode_count")
		item["tmdb_id"] = season["id"]
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
	show := catalogFieldsFixture(t, "tv_1396.json")
	for _, value := range show["seasons"].([]any) {
		summary := value.(map[string]any)
		if summary["season_number"] == fixture["season_number"] {
			wantSeason["episode_count"] = summary["episode_count"]
		}
	}
	episodes := make([]map[string]any, 0)
	for _, value := range fixture["episodes"].([]any) {
		episode := value.(map[string]any)
		want := catalogFixtureFields(episode, "season_number episode_number name overview still_path air_date runtime vote_average")
		want["tmdb_id"] = episode["id"]
		episodes = append(episodes, want)
	}
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
