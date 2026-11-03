package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
	"github.com/Ezo-mas/watcht/internal/tracking"
)

type trackingTestServer struct {
	*authServer
	cookie  *http.Cookie
	service *tracking.Service
	catalog *catalog.Service
}

func newTrackingServer(t *testing.T) *trackingTestServer {
	t.Helper()
	s := newAuthServer(t, authServerOptions{})
	service := tracking.New(s.db, tracking.Options{Now: s.clock.Now})
	out := &trackingTestServer{s, nil, service, catalog.New(s.db, nil, catalog.Options{Now: s.clock.Now})}
	out.router(service)
	out.cookie = s.setup("admin", "long enough")
	ctx := context.Background()
	for id, date := range map[int]string{603: "1999-03-30", 604: "", 605: "2999-01-01"} {
		if err := out.catalog.UpsertMovie(ctx, tmdb.Movie{TMDBID: id, Title: "Movie", PosterPath: "/movie.jpg", ReleaseDate: date}); err != nil {
			t.Fatal(err)
		}
	}
	show := tmdb.Show{TMDBID: 1396, Name: "Show", PosterPath: "/show.jpg", Seasons: []tmdb.SeasonSummary{{TMDBID: 9000, SeasonNumber: 0}, {TMDBID: 9001, SeasonNumber: 1}}}
	if err := out.catalog.UpsertShow(ctx, show); err != nil {
		t.Fatal(err)
	}
	for n := 0; n <= 1; n++ {
		season := tmdb.Season{TMDBID: 9000 + n, ShowTMDBID: 1396, SeasonNumber: n, Episodes: []tmdb.Episode{{TMDBID: 62084 + n, ShowTMDBID: 1396, SeasonNumber: n, EpisodeNumber: 1, Name: "Episode", AirDate: "2008-01-20", StillPath: "/still.jpg"}}}
		if err := out.catalog.UpsertSeason(ctx, 1396, season); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func (s *trackingTestServer) router(service Tracking) {
	s.t.Helper()
	h, err := NewRouter(Deps{Logger: slog.New(slog.NewTextHandler(s.logs, nil)), Auth: auth.New(s.db, auth.Options{Argon2: cheapArgon2, Now: s.clock.Now}), WebUI: http.NotFoundHandler(), Tracking: service})
	if err != nil {
		s.t.Fatal(err)
	}
	s.h = h
}
func (s *trackingTestServer) call(method, path, body string) *httptest.ResponseRecorder {
	return s.do(request{method: method, path: "/api/v1" + path, body: body, cookie: s.cookie})
}
func trackingObject(t *testing.T, rec *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func trackingEqual(t *testing.T, got any, literal string) {
	t.Helper()
	var want any
	if err := json.Unmarshal([]byte(literal), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func (s *trackingTestServer) mark(path, body string) map[string]any {
	s.t.Helper()
	return trackingObject(s.t, s.call("POST", path, body), 201)
}

func TestTrackingSuccessShapes(t *testing.T) {
	s := newTrackingServer(t)
	movie := s.mark("/movies/603/watches", `{"watched_at":"2020-01-02T03:00:00+03:00"}`)
	id := movie["watch"].(map[string]any)["id"]
	trackingEqual(t, movie, fmt.Sprintf(`{"watch":{"id":%v,"watched_at":"2020-01-02T00:00:00Z","source":"manual"},"merged_into":null}`, id))
	merged := s.mark("/movies/603/watches", `{"watched_at":"2020-01-02T00:00:00Z"}`)
	if merged["merged_into"] != id {
		t.Fatalf("merged: %v", merged)
	}
	episode := s.mark("/episodes/62085/watches", `{"release_date":true}`)
	eid := episode["watch"].(map[string]any)["id"]
	trackingEqual(t, episode, fmt.Sprintf(`{"watch":{"id":%v,"watched_at":"2008-01-20T12:00:00Z","source":"manual"},"merged_into":null}`, eid))
	trackingEqual(t, trackingObject(t, s.call("GET", "/movies/603/watches", ""), 200), fmt.Sprintf(`{"watches":[{"id":%v,"watched_at":"2020-01-02T00:00:00Z","source":"manual"}]}`, id))
	trackingEqual(t, trackingObject(t, s.call("GET", "/shows/1396/watched", ""), 200), `{"episodes":[{"tmdb_id":62085,"plays":1,"last_watched_at":"2008-01-20T12:00:00Z"}]}`)
	history := trackingObject(t, s.call("GET", "/history", ""), 200)
	trackingEqual(t, history, fmt.Sprintf(`{"entries":[{"id":%v,"watched_at":"2020-01-02T00:00:00Z","source":"manual","movie":{"tmdb_id":603,"title":"Movie","poster_path":"/movie.jpg","poster_url":"https://image.tmdb.org/t/p/w342/movie.jpg","release_date":"1999-03-30","runtime":0,"vote_average":0},"episode":null},{"id":%v,"watched_at":"2008-01-20T12:00:00Z","source":"manual","movie":null,"episode":{"tmdb_id":62085,"season_number":1,"episode_number":1,"name":"Episode","still_path":"/still.jpg","still_url":"https://image.tmdb.org/t/p/w300/still.jpg","air_date":"2008-01-20","runtime":0,"vote_average":0,"show":{"tmdb_id":1396,"name":"Show","poster_path":"/show.jpg","poster_url":"https://image.tmdb.org/t/p/w342/show.jpg"}}}],"next_cursor":null}`, id, eid))
	s.mark("/movies/603/watches", `{"watched_at":"2021-01-01T00:00:00Z"}`)
	watches := trackingObject(t, s.call("GET", "/movies/603/watches", ""), 200)["watches"].([]any)
	if len(watches) != 2 || watches[0].(map[string]any)["watched_at"] != "2021-01-01T00:00:00Z" {
		t.Fatal(watches)
	}
	s.mark("/episodes/62085/watches", `{"watched_at":"2021-01-01T00:00:00Z"}`)
	trackingEqual(t, trackingObject(t, s.call("GET", "/shows/1396/watched", ""), 200), `{"episodes":[{"tmdb_id":62085,"plays":2,"last_watched_at":"2021-01-01T00:00:00Z"}]}`)
}

func TestTrackingBatchShapes(t *testing.T) {
	for _, path := range []string{"/shows/1396/seasons/1/watches", "/shows/1396/watches", "/shows/1396/seasons/0/watches"} {
		t.Run(path, func(t *testing.T) {
			s := newTrackingServer(t)
			trackingEqual(t, trackingObject(t, s.call("POST", path, `{}`), 200), `{"added":1}`)
			trackingEqual(t, trackingObject(t, s.call("POST", path, ""), 200), `{"added":0}`)
		})
	}
}

func TestTrackingBodyVariants(t *testing.T) {
	s := newTrackingServer(t)
	for _, path := range []string{"/movies/603/watches", "/episodes/62085/watches", "/shows/1396/seasons/1/watches", "/shows/1396/watches"} {
		for _, tc := range []struct {
			body   string
			status int
		}{
			{"", 200}, {`{}`, 200}, {`{"release_date":false}`, 200}, {`{"release_date":true}`, 200}, {`{"watched_at":"2020-01-01T03:00:00+03:00"}`, 200},
			{`{"watched_at":"2020-01-01T00:00:00Z","release_date":true}`, 400},
			{`{"watched_at":"2020-01-01T00:00:00Z","release_date":false}`, 400},
			{`{"watched_at":"private-bad-time"}`, 400}, {`{"private_unknown":true}`, 400}, {`{`, 400}, {`{} {}`, 400}, {`[]`, 400}, {`null`, 400},
			{`{"watched_at":null}`, 400}, {`{"release_date":null}`, 400}, {`{"release_date":"yes"}`, 400},
		} {
			t.Run(path+tc.body, func(t *testing.T) {
				rec := s.call("POST", path, tc.body)
				if tc.status == 400 {
					catalogError(t, rec, 400, "invalid_request")
					if strings.Contains(rec.Body.String(), "private") {
						t.Fatal("input leaked")
					}
					return
				}
				status := 200
				if strings.HasPrefix(path, "/movies/") || strings.HasPrefix(path, "/episodes/") {
					status = 201
				}
				obj := trackingObject(t, rec, status)
				if status == 201 && (tc.body == "" || tc.body == `{}` || tc.body == `{"release_date":false}`) {
					if obj["watch"].(map[string]any)["watched_at"] != s.clock.Now().Format(time.RFC3339) {
						t.Fatal(obj)
					}
				}
			})
		}
	}
	for _, path := range []string{"/movies/603/watches", "/episodes/62085/watches"} {
		catalogError(t, s.call("POST", path, `{"watched_at":"2999-01-01T00:00:00Z"}`), 400, "invalid_request")
	}
	catalogError(t, s.call("POST", "/movies/604/watches", `{"release_date":true}`), 422, "no_release_date")
	catalogError(t, s.call("POST", "/movies/605/watches", `{"release_date":true}`), 422, "not_released")
	catalogError(t, s.call("POST", "/movies/603/watches", `{"watched_at":"`+strings.Repeat("x", maxBodyBytes)+`"}`), 413, "too_large")
	catalogError(t, s.do(request{method: "POST", path: "/api/v1/movies/603/watches", body: `{}`, cookie: s.cookie, header: map[string]string{"Content-Type": "text/plain"}}), 415, "unsupported_media_type")
}

func TestTrackingMissingAndInvalidPaths(t *testing.T) {
	s := newTrackingServer(t)
	for _, path := range []string{"/movies/999/watches", "/episodes/999/watches", "/shows/999/watches", "/shows/999/seasons/1/watches", "/shows/1396/seasons/99/watches"} {
		catalogError(t, s.call("POST", path, ""), 404, "not_found")
	}
	for _, id := range []string{"abc", "0", "-1", "+1", "2147483648", "999999999999999999999"} {
		for _, route := range []struct{ method, path string }{{"POST", "/movies/%s/watches"}, {"POST", "/episodes/%s/watches"}, {"POST", "/shows/%s/watches"}, {"POST", "/shows/%s/seasons/1/watches"}, {"GET", "/movies/%s/watches"}, {"GET", "/shows/%s/watched"}} {
			catalogError(t, s.call(route.method, fmt.Sprintf(route.path, id), ""), 404, "not_found")
		}
	}
	for _, n := range []string{"abc", "-1", "+1", "2147483648"} {
		catalogError(t, s.call("POST", "/shows/1396/seasons/"+n+"/watches", ""), 404, "not_found")
	}
	for _, id := range []string{"abc", "0", "-1", "+1", "9223372036854775808", "2147483648"} {
		catalogError(t, s.call("DELETE", "/watches/"+id, ""), 404, "not_found")
	}
	for _, path := range []string{"/movies/603/watches", "/movies/999/watches"} {
		trackingEqual(t, trackingObject(t, s.call("GET", path, ""), 200), `{"watches":[]}`)
	}
	for _, path := range []string{"/shows/1396/watched", "/shows/999/watched"} {
		trackingEqual(t, trackingObject(t, s.call("GET", path, ""), 200), `{"episodes":[]}`)
	}
	trackingEqual(t, trackingObject(t, s.call("GET", "/history", ""), 200), `{"entries":[],"next_cursor":null}`)
}

func TestTrackingIsolationAndDelete(t *testing.T) {
	s := newTrackingServer(t)
	first := s.mark("/movies/603/watches", "")
	s.mark("/episodes/62085/watches", "")
	id := first["watch"].(map[string]any)["id"]
	path := fmt.Sprintf("/watches/%.0f", id)
	admin := s.cookie
	s.createAccount(admin, "member", "long enough", false)
	s.cookie = s.login("member", "long enough")
	catalogError(t, s.call("DELETE", path, ""), 404, "not_found")
	for path, want := range map[string]string{"/history": `{"entries":[],"next_cursor":null}`, "/movies/603/watches": `{"watches":[]}`, "/shows/1396/watched": `{"episodes":[]}`} {
		trackingEqual(t, trackingObject(t, s.call("GET", path, ""), 200), want)
	}
	s.mark("/movies/603/watches", "")
	s.mark("/episodes/62085/watches", "")
	s.cookie = admin
	history := trackingObject(t, s.call("GET", "/history", ""), 200)["entries"].([]any)
	if len(history) != 2 {
		t.Fatal(history)
	}
	trackingEqual(t, trackingObject(t, s.call("GET", "/movies/603/watches", ""), 200), fmt.Sprintf(`{"watches":[{"id":%v,"watched_at":%q,"source":"manual"}]}`, id, s.clock.Now().Format(time.RFC3339)))
	trackingEqual(t, trackingObject(t, s.call("GET", "/shows/1396/watched", ""), 200), fmt.Sprintf(`{"episodes":[{"tmdb_id":62085,"plays":1,"last_watched_at":%q}]}`, s.clock.Now().Format(time.RFC3339)))
	rec := s.call("DELETE", path, "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	catalogError(t, s.call("DELETE", path, ""), 404, "not_found")
}

func TestTrackingHistoryPagination(t *testing.T) {
	s := newTrackingServer(t)
	var ids []any
	for day := 1; day <= 3; day++ {
		body := fmt.Sprintf(`{"watched_at":"2020-01-0%dT00:00:00Z"}`, day)
		for _, path := range []string{"/movies/603/watches", "/episodes/62085/watches", "/episodes/62084/watches"} {
			ids = append(ids, s.mark(path, body)["watch"].(map[string]any)["id"])
		}
	}
	var got []any
	cursor := ""
	for page := 0; page < 5; page++ {
		path := "/history?limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		obj := trackingObject(t, s.call("GET", path, ""), 200)
		entries := obj["entries"].([]any)
		for _, entry := range entries {
			got = append(got, entry.(map[string]any)["id"])
		}
		if page == 4 {
			if len(entries) != 1 || obj["next_cursor"] != nil {
				t.Fatal(obj)
			}
		} else {
			if len(entries) != 2 {
				t.Fatal(obj)
			}
			last := entries[1].(map[string]any)
			at, err := time.Parse(time.RFC3339, last["watched_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			want := strconv.FormatInt(at.Unix(), 10) + fmt.Sprintf("_%.0f", last["id"])
			if obj["next_cursor"] != want {
				t.Fatalf("cursor: %v, want %s", obj, want)
			}
			cursor = want
		}
	}
	for i := range ids {
		if got[i] != ids[len(ids)-1-i] {
			t.Fatalf("history IDs %v, inserted %v", got, ids)
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "limit=", "cursor=", "cursor=abc", "cursor=1", "cursor=1_0", "cursor=1_-1", "cursor=1_2_3", "cursor=1.5_2", "cursor=999999999999999999999_1", "cursor=1_9223372036854775808", "cursor=253402300800_1", "cursor=-62135596801_1"} {
		catalogError(t, s.call("GET", "/history?"+query, ""), 400, "invalid_request")
	}
	for _, query := range []string{"", "?limit=100"} {
		if len(trackingObject(t, s.call("GET", "/history"+query, ""), 200)["entries"].([]any)) != 9 {
			t.Fatal("default or max limit")
		}
	}
}

func TestTrackingHistoryOldDatesAndEmptyImages(t *testing.T) {
	s := newTrackingServer(t)
	ctx := context.Background()
	if err := s.catalog.UpsertMovie(ctx, tmdb.Movie{TMDBID: 606, Title: "Old", ReleaseDate: "1895-12-28", Runtime: 50, VoteAverage: 6.5}); err != nil {
		t.Fatal(err)
	}
	show := tmdb.Show{TMDBID: 1397, Name: "No images", Seasons: []tmdb.SeasonSummary{{TMDBID: 9010, SeasonNumber: 1}}}
	if err := s.catalog.UpsertShow(ctx, show); err != nil {
		t.Fatal(err)
	}
	season := tmdb.Season{TMDBID: 9010, ShowTMDBID: 1397, SeasonNumber: 1, Episodes: []tmdb.Episode{{TMDBID: 62086, ShowTMDBID: 1397, SeasonNumber: 1, EpisodeNumber: 1, Name: "Old episode", AirDate: "1890-01-01"}}}
	if err := s.catalog.UpsertSeason(ctx, 1397, season); err != nil {
		t.Fatal(err)
	}
	movie := s.mark("/movies/606/watches", `{"release_date":true}`)
	episode := s.mark("/episodes/62086/watches", `{"release_date":true}`)
	movieID := movie["watch"].(map[string]any)["id"]
	episodeID := episode["watch"].(map[string]any)["id"]
	first := trackingObject(t, s.call("GET", "/history?limit=1", ""), 200)
	at := time.Date(1895, 12, 28, 12, 0, 0, 0, time.UTC)
	cursor := fmt.Sprintf("%d_%.0f", at.Unix(), movieID)
	trackingEqual(t, first, fmt.Sprintf(`{"entries":[{"id":%v,"watched_at":"1895-12-28T12:00:00Z","source":"manual","movie":{"tmdb_id":606,"title":"Old","poster_path":"","poster_url":"","release_date":"1895-12-28","runtime":50,"vote_average":6.5},"episode":null}],"next_cursor":%q}`, movieID, cursor))
	second := trackingObject(t, s.call("GET", "/history?limit=2&cursor="+cursor, ""), 200)
	trackingEqual(t, second, fmt.Sprintf(`{"entries":[{"id":%v,"watched_at":"1890-01-01T12:00:00Z","source":"manual","movie":null,"episode":{"tmdb_id":62086,"season_number":1,"episode_number":1,"name":"Old episode","still_path":"","still_url":"","air_date":"1890-01-01","runtime":0,"vote_average":0,"show":{"tmdb_id":1397,"name":"No images","poster_path":"","poster_url":""}}}],"next_cursor":null}`, episodeID))
	trackingEqual(t, trackingObject(t, s.call("GET", "/history?cursor=-62135596800_1", ""), 200), `{"entries":[],"next_cursor":null}`)
	for _, cursor := range []string{"0_1", "%2B1_1", "253402300799_1", "-62135596799_1"} {
		trackingObject(t, s.call("GET", "/history?cursor="+cursor, ""), 200)
	}
}

func TestTrackingAuthentication(t *testing.T) {
	s := newTrackingServer(t)
	routes := []struct {
		method, path string
		status       int
	}{{"POST", "/movies/603/watches", 201}, {"POST", "/episodes/62085/watches", 201}, {"POST", "/shows/1396/seasons/1/watches", 200}, {"POST", "/shows/1396/watches", 200}, {"DELETE", "/watches/1", 204}, {"GET", "/movies/603/watches", 200}, {"GET", "/shows/1396/watched", 200}, {"GET", "/history", 200}}
	token := trackingObject(t, s.do(request{method: "POST", path: "/api/v1/tokens", body: `{"name":"tracking","password":"long enough"}`, cookie: s.cookie}), 201)["secret"].(string)
	for _, route := range routes {
		catalogError(t, s.do(request{method: route.method, path: "/api/v1" + route.path}), 401, "unauthenticated")
		rec := s.do(request{method: route.method, path: "/api/v1" + route.path, header: map[string]string{"Authorization": "Bearer " + token}})
		if rec.Code != route.status {
			t.Fatalf("token %s %s: %d %s", route.method, route.path, rec.Code, rec.Body)
		}
	}
}

type trackingMovieFailure struct {
	Tracking
	mark func(context.Context, int64, int, tracking.When) (tracking.Result, error)
}

func (f trackingMovieFailure) MarkMovie(ctx context.Context, u int64, id int, w tracking.When) (tracking.Result, error) {
	return f.mark(ctx, u, id, w)
}

func TestTrackingInternalAndCancelled(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			s := newTrackingServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.router(trackingMovieFailure{Tracking: s.service, mark: func(context.Context, int64, int, tracking.When) (tracking.Result, error) {
				if cancelled {
					cancel()
					return tracking.Result{}, ctx.Err()
				}
				return tracking.Result{}, errors.New("private database failure")
			}})
			rec := s.do(request{method: "POST", path: "/api/v1/movies/603/watches", cookie: s.cookie, ctx: ctx})
			if cancelled {
				if rec.Body.Len() != 0 || strings.Contains(s.logs.String(), "level=ERROR") {
					t.Fatalf("cancel: %s %s", rec.Body, s.logs)
				}
			} else {
				catalogError(t, rec, 500, "internal")
				if strings.Contains(rec.Body.String(), "private") || !strings.Contains(s.logs.String(), "level=ERROR") {
					t.Fatalf("internal: %s %s", rec.Body, s.logs)
				}
			}
		})
	}
}

func TestTrackingEpisodesPending(t *testing.T) {
	s := newTrackingServer(t)
	// Season 2 is listed, but the background sync hasn't stored its episodes.
	show := tmdb.Show{TMDBID: 1396, Name: "Show", Seasons: []tmdb.SeasonSummary{{TMDBID: 9000, SeasonNumber: 0}, {TMDBID: 9001, SeasonNumber: 1}, {TMDBID: 9002, SeasonNumber: 2}}}
	if err := s.catalog.UpsertShow(context.Background(), show); err != nil {
		t.Fatal(err)
	}
	catalogError(t, s.call("POST", "/shows/1396/watches", ""), 409, "episodes_pending")
	catalogError(t, s.call("POST", "/shows/1396/seasons/2/watches", ""), 409, "episodes_pending")
	trackingEqual(t, trackingObject(t, s.call("POST", "/shows/1396/seasons/1/watches", ""), 200), `{"added":1}`)
}
