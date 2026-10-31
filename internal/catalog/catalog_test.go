package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// testNow is a whole second, like every stored timestamp.
var testNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// clock is a clock the test moves by hand. The service calls it from the
// test's goroutine, so it needs no lock.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

// seasonKey names a season by its show's TMDB ID and its number.
type seasonKey struct{ show, number int }

// fakeMetadata answers TMDB calls from maps. A missing entry is a 404, as the
// tmdb package reports it. Each call first checks that no write transaction
// is open, since a fetch must not hold the one write connection.
type fakeMetadata struct {
	d       *db.DB
	movies  map[int]tmdb.Movie
	shows   map[int]tmdb.Show
	seasons map[seasonKey]tmdb.Season
	// seasonErr makes the fetch of a season fail with the error.
	seasonErr map[seasonKey]error
	// calls lists the season fetches in order, as "show/number".
	calls []string
}

func newFakeMetadata(d *db.DB) *fakeMetadata {
	return &fakeMetadata{
		d:         d,
		movies:    map[int]tmdb.Movie{},
		shows:     map[int]tmdb.Show{},
		seasons:   map[seasonKey]tmdb.Season{},
		seasonErr: map[seasonKey]error{},
	}
}

// writeFree fails if a write transaction holds the write connection. It asks
// for that connection with a short deadline, so it can only time out while
// another transaction holds it.
func (f *fakeMetadata) writeFree(method string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx, err := f.d.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: a write transaction is open during a TMDB call: %w", method, err)
	}
	return tx.Rollback()
}

func (f *fakeMetadata) Movie(ctx context.Context, id int) (tmdb.Movie, error) {
	if err := f.writeFree("Movie"); err != nil {
		return tmdb.Movie{}, err
	}
	m, ok := f.movies[id]
	if !ok {
		return tmdb.Movie{}, fmt.Errorf("movie %d: %w", id, providers.ErrNotFound)
	}
	return m, nil
}

func (f *fakeMetadata) Show(ctx context.Context, id int) (tmdb.Show, error) {
	if err := f.writeFree("Show"); err != nil {
		return tmdb.Show{}, err
	}
	s, ok := f.shows[id]
	if !ok {
		return tmdb.Show{}, fmt.Errorf("show %d: %w", id, providers.ErrNotFound)
	}
	return s, nil
}

func (f *fakeMetadata) Season(ctx context.Context, showID, seasonNumber int) (tmdb.Season, error) {
	if err := f.writeFree("Season"); err != nil {
		return tmdb.Season{}, err
	}
	k := seasonKey{showID, seasonNumber}
	f.calls = append(f.calls, fmt.Sprintf("%d/%d", showID, seasonNumber))
	if err := f.seasonErr[k]; err != nil {
		return tmdb.Season{}, err
	}
	s, ok := f.seasons[k]
	if !ok {
		return tmdb.Season{}, fmt.Errorf("season %d of show %d: %w", seasonNumber, showID, providers.ErrNotFound)
	}
	return s, nil
}

// env is a Service on a fresh database, with a fake TMDB and a clock the test
// moves.
type env struct {
	d   *db.DB
	s   *Service
	md  *fakeMetadata
	clk *clock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d := dbtest.New(t)
	clk := &clock{now: testNow}
	md := newFakeMetadata(d)
	return &env{d: d, s: New(d, md, Options{Now: clk.Now}), md: md, clk: clk}
}

// at moves the clock to the given number of hours after testNow.
func (e *env) at(hours int) {
	e.clk.now = testNow.Add(time.Duration(hours) * time.Hour)
}

// int64Of returns the single integer a query returns. Tests use it for what
// the API doesn't expose: internal IDs and counts.
func (e *env) int64Of(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := e.d.Read.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// textOf returns the single text value a query returns. NULL has Valid false.
func (e *env) textOf(t *testing.T, query string, args ...any) sql.NullString {
	t.Helper()
	var s sql.NullString
	if err := e.d.Read.QueryRowContext(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return s
}

// episodeIDs maps each stored episode's TMDB ID to its internal ID.
func (e *env) episodeIDs(t *testing.T) map[int]int64 {
	t.Helper()
	rows, err := e.d.Read.QueryContext(context.Background(), "SELECT tmdb_id, id FROM episodes")
	if err != nil {
		t.Fatalf("read episode IDs: %v", err)
	}
	defer func() { _ = rows.Close() }()
	ids := map[int]int64{}
	for rows.Next() {
		var tmdbID int
		var id int64
		if err := rows.Scan(&tmdbID, &id); err != nil {
			t.Fatalf("scan episode ID: %v", err)
		}
		ids[tmdbID] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read episode IDs: %v", err)
	}
	return ids
}

// snapshot describes every stored episode by TMDB IDs and numbers, for
// comparing two end states. Internal IDs depend on insertion order, so they
// are left out.
func (e *env) snapshot(t *testing.T) string {
	t.Helper()
	rows, err := e.d.Read.QueryContext(context.Background(), `
		SELECT episodes.tmdb_id, seasons.tmdb_id, episodes.season_number,
			episodes.episode_number, episodes.missing_since IS NULL
		FROM episodes
		JOIN seasons ON seasons.id = episodes.season_id
		ORDER BY episodes.tmdb_id`)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var episode, season, seasonNumber, number, present int64
		if err := rows.Scan(&episode, &season, &seasonNumber, &number, &present); err != nil {
			t.Fatalf("scan snapshot: %v", err)
		}
		fmt.Fprintf(&b, "episode %d: season %d (number %d), episode %d, present %v\n",
			episode, season, seasonNumber, number, present == 1)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return b.String()
}

// tmdbIDs returns the TMDB IDs of episodes, in order.
func tmdbIDs(eps []Episode) []int {
	ids := make([]int, 0, len(eps))
	for _, e := range eps {
		ids = append(ids, e.TMDBID)
	}
	return ids
}

// seasonNumbers returns the number of each season, in order.
func seasonNumbers(seasons []Season) []int {
	nums := make([]int, 0, len(seasons))
	for _, s := range seasons {
		nums = append(nums, s.SeasonNumber)
	}
	return nums
}

// testMovie is The Matrix, as the tmdb package returns it.
func testMovie() tmdb.Movie {
	return tmdb.Movie{
		TMDBID:           603,
		IMDbID:           "tt0133093",
		Title:            "The Matrix",
		OriginalTitle:    "The Matrix",
		OriginalLanguage: "en",
		Overview:         "A computer hacker learns the truth about reality.",
		Tagline:          "Welcome to the Real World.",
		ReleaseDate:      "1999-03-31",
		Status:           "Released",
		PosterPath:       "/poster.jpg",
		BackdropPath:     "/backdrop.jpg",
		Runtime:          136,
		Genres: []tmdb.Genre{
			{ID: 28, Name: "Action"},
			{ID: 878, Name: "Science Fiction"},
		},
		VoteAverage: 8.2,
		VoteCount:   29114,
	}
}

// testShow is Breaking Bad, with no seasons. A test sets the list it needs.
func testShow() tmdb.Show {
	return tmdb.Show{
		TMDBID:           1396,
		IMDbID:           "tt0903747",
		TVDBID:           81189,
		Name:             "Breaking Bad",
		OriginalName:     "Breaking Bad",
		OriginalLanguage: "en",
		Overview:         "A chemistry teacher turns to making methamphetamine.",
		Tagline:          "Change the equation.",
		FirstAirDate:     "2008-01-20",
		LastAirDate:      "2013-09-29",
		Status:           "Ended",
		PosterPath:       "/show-poster.jpg",
		BackdropPath:     "/show-backdrop.jpg",
		Genres: []tmdb.Genre{
			{ID: 18, Name: "Drama"},
			{ID: 80, Name: "Crime"},
		},
		VoteAverage: 8.9,
		VoteCount:   12000,
	}
}

// summary is a season as its show's list gives it.
func summary(tmdbID, number, episodes int) tmdb.SeasonSummary {
	return tmdb.SeasonSummary{
		TMDBID:       tmdbID,
		SeasonNumber: number,
		EpisodeCount: episodes,
		Name:         fmt.Sprintf("Season %d", number),
		AirDate:      "2008-01-20",
		PosterPath:   "/season.jpg",
	}
}

// testEpisode is episode n of a season, identified by its TMDB ID. Its season
// is set by testSeason, as the tmdb package does.
func testEpisode(tmdbID, n int) tmdb.Episode {
	return tmdb.Episode{
		TMDBID:        tmdbID,
		ShowTMDBID:    1396,
		EpisodeNumber: n,
		Name:          fmt.Sprintf("Episode %d", tmdbID),
		Overview:      fmt.Sprintf("About episode %d", tmdbID),
		AirDate:       "2008-01-20",
		StillPath:     "/still.jpg",
		Runtime:       47,
		VoteAverage:   8.5,
	}
}

// testSeason is a season of show 1396, as its own response gives it.
func testSeason(tmdbID, number int, episodes ...tmdb.Episode) tmdb.Season {
	for i := range episodes {
		episodes[i].ShowTMDBID = 1396
		episodes[i].SeasonNumber = number
	}
	return tmdb.Season{
		TMDBID:       tmdbID,
		ShowTMDBID:   1396,
		SeasonNumber: number,
		Name:         fmt.Sprintf("Season %d", number),
		Overview:     fmt.Sprintf("About season %d", number),
		AirDate:      "2008-01-20",
		PosterPath:   "/season.jpg",
		Episodes:     episodes,
	}
}

// storeShow stores show 1396 with the given season list.
func (e *env) storeShow(t *testing.T, seasons ...tmdb.SeasonSummary) {
	t.Helper()
	show := testShow()
	show.Seasons = seasons
	if err := e.s.UpsertShow(context.Background(), show); err != nil {
		t.Fatalf("UpsertShow: %v", err)
	}
}

// upsertSeason stores a season of show 1396.
func (e *env) upsertSeason(t *testing.T, season tmdb.Season) {
	t.Helper()
	if err := e.s.UpsertSeason(context.Background(), 1396, season); err != nil {
		t.Fatalf("UpsertSeason(%d): %v", season.TMDBID, err)
	}
}

// otherShowID is a second show, for the tests that need one besides 1396.
const otherShowID = 1399

// storeOtherShow stores show 1399 with the given season list.
func (e *env) storeOtherShow(t *testing.T, seasons ...tmdb.SeasonSummary) {
	t.Helper()
	show := testShow()
	show.TMDBID = otherShowID
	show.Name = "Another Show"
	show.Seasons = seasons
	if err := e.s.UpsertShow(context.Background(), show); err != nil {
		t.Fatalf("UpsertShow(%d): %v", otherShowID, err)
	}
}

// upsertOtherSeason stores a season of show 1399. Its episodes are set to that
// show too, as the tmdb package sets them.
func (e *env) upsertOtherSeason(t *testing.T, season tmdb.Season) {
	t.Helper()
	season.ShowTMDBID = otherShowID
	for i := range season.Episodes {
		season.Episodes[i].ShowTMDBID = otherShowID
	}
	if err := e.s.UpsertSeason(context.Background(), otherShowID, season); err != nil {
		t.Fatalf("UpsertSeason(%d) of show %d: %v", season.TMDBID, otherShowID, err)
	}
}

// missingTMDBIDs returns the TMDB IDs, in order, of the rows in table that have
// missing_since set. table is "seasons" or "episodes".
func (e *env) missingTMDBIDs(t *testing.T, table string) []int {
	t.Helper()
	rows, err := e.d.Read.QueryContext(context.Background(),
		"SELECT tmdb_id FROM "+table+" WHERE missing_since IS NOT NULL ORDER BY tmdb_id")
	if err != nil {
		t.Fatalf("read missing %s: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan missing %s: %v", table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read missing %s: %v", table, err)
	}
	return ids
}

// rowCounts returns the number of rows in each catalog table.
func (e *env) rowCounts(t *testing.T) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for _, table := range []string{"genres", "movies", "movie_genres", "shows", "show_genres", "seasons", "episodes"} {
		counts[table] = e.int64Of(t, "SELECT count(*) FROM "+table)
	}
	return counts
}
