// Package tmdb is the TMDB v3 metadata adapter. It sends its requests through
// the shared provider client, which caches, rate-limits, and retries them, and
// it returns plain structs rather than TMDB's JSON.
//
// The read access token goes in the Authorization header only. Image fields
// hold TMDB's paths, such as /abc.jpg, not URLs.
package tmdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/providers"
)

// BaseURL is the root of TMDB's v3 API.
const BaseURL = "https://api.themoviedb.org/3"

// ErrInvalidArgument matches (through errors.Is) an argument rejected before
// any request is made.
var ErrInvalidArgument = errors.New("tmdb: invalid argument")

// Cache lifetimes, from the M3 section of docs/rewrite-plan.md.
const (
	searchTTL  = 24 * time.Hour
	movieTTL   = 7 * 24 * time.Hour
	seasonTTL  = 24 * time.Hour
	episodeTTL = 24 * time.Hour
	findTTL    = 30 * 24 * time.Hour

	// showTTL is one day for every show, ended or not. The lifetime is chosen
	// before the response says whether the show has ended, so it can't depend
	// on the status. Slice 4's refresh rule gives ended shows the 7-day
	// lifetime from the stored status instead.
	showTTL = 24 * time.Hour
)

// maxPage is the largest page TMDB serves.
const maxPage = 500

// imdbIDPattern is the only IMDb ID shape that goes into a path.
var imdbIDPattern = regexp.MustCompile(`^tt[0-9]{7,10}$`)

// Options configures a Client. Only Token is required.
type Options struct {
	// Token is TMDB's API Read Access Token. It's sent as a bearer token and
	// never logged. New panics when it's empty.
	Token string
	// BaseURL replaces BaseURL, for tests. Empty means BaseURL.
	BaseURL string
	// UserAgent is sent with every request. Empty keeps Go's default.
	UserAgent string
	// Logger receives request and cache logs. Nil discards them.
	Logger *slog.Logger
	// HTTPClient replaces the default HTTP client, for tests.
	HTTPClient *http.Client
	// Now is the clock for cache expiry. Nil means time.Now.
	Now func() time.Time
}

// Client calls TMDB. It's safe for concurrent use.
type Client struct {
	client *providers.Client
}

// New returns a TMDB client that caches its responses in d. It panics on an
// empty Token, which is a wiring bug.
func New(d *db.DB, opts Options) *Client {
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		panic("tmdb: New without a Token")
	}
	base := opts.BaseURL
	if base == "" {
		base = BaseURL
	}
	return &Client{client: providers.New(d, providers.Options{
		Provider:   "tmdb",
		BaseURL:    base,
		Header:     http.Header{"Authorization": {"Bearer " + token}},
		UserAgent:  opts.UserAgent,
		Logger:     opts.Logger,
		HTTPClient: opts.HTTPClient,
		Now:        opts.Now,
	})}
}

// Search returns one page of TMDB's multi search, keeping only movies and
// shows. A page below 1 is page 1; above 500 is an error.
func (c *Client) Search(ctx context.Context, query string, page int) (SearchResults, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResults{}, invalid("search query is empty")
	}
	page = max(page, 1)
	if page > maxPage {
		return SearchResults{}, invalid(fmt.Sprintf("page must be 1 to %d", maxPage))
	}
	q := url.Values{
		"query":         {query},
		"page":          {strconv.Itoa(page)},
		"include_adult": {"false"},
	}
	var w wireSearch
	if err := c.client.GetJSON(ctx, "/search/multi", q, searchTTL, &w); err != nil {
		return SearchResults{}, fmt.Errorf("search: %w", err)
	}
	return searchResults(w), nil
}

// Movie returns a movie by its TMDB ID, with its IMDb ID.
func (c *Client) Movie(ctx context.Context, id int) (Movie, error) {
	if id <= 0 {
		return Movie{}, invalid("movie ID must be positive")
	}
	var w wireMovie
	if err := c.client.GetJSON(ctx, "/movie/"+strconv.Itoa(id), externalIDs(), movieTTL, &w); err != nil {
		return Movie{}, fmt.Errorf("movie %d: %w", id, err)
	}
	return movieFromWire(w), nil
}

// Show returns a show by its TMDB ID, with its IMDb and TVDB IDs and its season list.
func (c *Client) Show(ctx context.Context, id int) (Show, error) {
	if id <= 0 {
		return Show{}, invalid("show ID must be positive")
	}
	var w wireShow
	if err := c.client.GetJSON(ctx, "/tv/"+strconv.Itoa(id), externalIDs(), showTTL, &w); err != nil {
		return Show{}, fmt.Errorf("show %d: %w", id, err)
	}
	return showFromWire(w), nil
}

// Season returns one season of a show. Season 0 holds the specials.
func (c *Client) Season(ctx context.Context, showID, seasonNumber int) (Season, error) {
	if showID <= 0 {
		return Season{}, invalid("show ID must be positive")
	}
	if seasonNumber < 0 {
		return Season{}, invalid("season number can't be negative")
	}
	path := "/tv/" + strconv.Itoa(showID) + "/season/" + strconv.Itoa(seasonNumber)
	var w wireSeason
	if err := c.client.GetJSON(ctx, path, nil, seasonTTL, &w); err != nil {
		return Season{}, fmt.Errorf("season %d of show %d: %w", seasonNumber, showID, err)
	}
	return seasonFromWire(w, showID, seasonNumber), nil
}

// Episode returns one episode, with its IMDb and TVDB IDs.
func (c *Client) Episode(ctx context.Context, showID, seasonNumber, episodeNumber int) (Episode, error) {
	if showID <= 0 {
		return Episode{}, invalid("show ID must be positive")
	}
	if seasonNumber < 0 {
		return Episode{}, invalid("season number can't be negative")
	}
	if episodeNumber < 1 {
		return Episode{}, invalid("episode number must be at least 1")
	}
	path := "/tv/" + strconv.Itoa(showID) + "/season/" + strconv.Itoa(seasonNumber) +
		"/episode/" + strconv.Itoa(episodeNumber)
	var w wireEpisode
	if err := c.client.GetJSON(ctx, path, externalIDs(), episodeTTL, &w); err != nil {
		return Episode{}, fmt.Errorf("episode %d of season %d of show %d: %w", episodeNumber, seasonNumber, showID, err)
	}
	return episodeFromWire(w, showID, seasonNumber), nil
}

// FindByIMDbID looks up an IMDb ID, such as tt0133093, among movies, shows,
// and episodes.
func (c *Client) FindByIMDbID(ctx context.Context, imdbID string) (FindResults, error) {
	if !imdbIDPattern.MatchString(imdbID) {
		return FindResults{}, invalid("IMDb ID must be tt followed by 7 to 10 digits")
	}
	return c.find(ctx, "/find/"+imdbID, "imdb_id")
}

// FindByTVDBID looks up a TVDB ID among movies, shows, and episodes.
func (c *Client) FindByTVDBID(ctx context.Context, tvdbID int) (FindResults, error) {
	if tvdbID <= 0 {
		return FindResults{}, invalid("TVDB ID must be positive")
	}
	return c.find(ctx, "/find/"+strconv.Itoa(tvdbID), "tvdb_id")
}

// find fetches a find response. The external ID is already in path, checked
// by the caller.
func (c *Client) find(ctx context.Context, path, source string) (FindResults, error) {
	q := url.Values{"external_source": {source}}
	var w wireFind
	if err := c.client.GetJSON(ctx, path, q, findTTL, &w); err != nil {
		return FindResults{}, fmt.Errorf("find: %w", err)
	}
	return findResults(w), nil
}

// externalIDs asks TMDB to add the external IDs (IMDb, TVDB) to a response.
func externalIDs() url.Values {
	return url.Values{"append_to_response": {"external_ids"}}
}

// invalid is an argument error. Its message never echoes the argument, so a
// query or token can't end up in an error.
func invalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, msg)
}

// SearchResults is one page of search results. The totals are as TMDB reports
// them, so they include people, who aren't in Results.
type SearchResults struct {
	Page         int
	TotalPages   int
	TotalResults int
	Results      []SearchResult
}

// SearchResult is a movie or a show.
type SearchResult struct {
	// MediaType is "movie" or "tv".
	MediaType string
	TMDBID    int
	// Title is a movie's title or a show's name.
	Title         string
	OriginalTitle string
	Overview      string
	// Date is the release date or first air date, as YYYY-MM-DD, or "".
	Date        string
	PosterPath  string
	VoteAverage float64
}

// Genre is a TMDB genre.
type Genre struct {
	ID   int
	Name string
}

// Movie is a movie's details. Runtime is in minutes. Dates are YYYY-MM-DD, or
// "" when TMDB has none.
type Movie struct {
	TMDBID           int
	IMDbID           string
	Title            string
	OriginalTitle    string
	OriginalLanguage string
	Overview         string
	Tagline          string
	ReleaseDate      string
	Status           string
	PosterPath       string
	BackdropPath     string
	Runtime          int
	Genres           []Genre
	VoteAverage      float64
	VoteCount        int
}

// Show is a show's details. Ended is true for the statuses "Ended" and
// "Canceled" only.
type Show struct {
	TMDBID           int
	IMDbID           string
	TVDBID           int
	Name             string
	OriginalName     string
	OriginalLanguage string
	Overview         string
	Tagline          string
	FirstAirDate     string
	LastAirDate      string
	Status           string
	PosterPath       string
	BackdropPath     string
	InProduction     bool
	Ended            bool
	NumberOfSeasons  int
	NumberOfEpisodes int
	Genres           []Genre
	Seasons          []SeasonSummary
	VoteAverage      float64
	VoteCount        int
}

// SeasonSummary is a season as its show lists it. Season 0 holds specials.
type SeasonSummary struct {
	TMDBID       int
	SeasonNumber int
	EpisodeCount int
	Name         string
	AirDate      string
	PosterPath   string
}

// Season is one season and its episodes, in TMDB's order.
type Season struct {
	TMDBID       int
	ShowTMDBID   int
	SeasonNumber int
	Name         string
	Overview     string
	AirDate      string
	PosterPath   string
	Episodes     []Episode
}

// Episode is identified by TMDB's episode ID. The numbers are the ones TMDB
// has now, which can change when it renumbers. IMDbID and TVDBID are set only
// by Episode, which asks TMDB for the external IDs.
type Episode struct {
	TMDBID        int
	ShowTMDBID    int
	SeasonNumber  int
	EpisodeNumber int
	Name          string
	Overview      string
	AirDate       string
	StillPath     string
	Runtime       int
	VoteAverage   float64
	IMDbID        string
	TVDBID        int
}

// FindResults is what an external ID matches. Each list is empty when nothing
// matched.
type FindResults struct {
	Movies   []SearchResult
	Shows    []SearchResult
	Episodes []Episode
}
