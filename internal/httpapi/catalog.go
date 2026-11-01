package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// CatalogSearch is the TMDB search used by the API.
type CatalogSearch interface {
	Search(context.Context, string, int) (tmdb.SearchResults, error)
}

// Catalog is the stored catalog and the operations used by detail pages.
type Catalog interface {
	Movie(context.Context, int) (catalog.Movie, error)
	Show(context.Context, int) (catalog.Show, error)
	Season(context.Context, int, int) (catalog.Season, []catalog.Episode, error)
	FetchMovie(context.Context, int) error
	FetchShow(context.Context, int) error
	MarkMovieViewed(context.Context, int) error
	MarkShowViewed(context.Context, int) error
}

type catalogHandlers struct {
	auth    *authHandlers
	catalog Catalog
	search  CatalogSearch
}

func (h *catalogHandlers) register(api *apiMux) {
	api.handle("GET /api/v1/search", h.auth.withUser(h.searchTitles))
	api.handle("GET /api/v1/movies/{id}", h.auth.withUser(h.movie))
	api.handle("GET /api/v1/shows/{id}", h.auth.withUser(h.show))
	api.handle("GET /api/v1/shows/{id}/seasons/{number}", h.auth.withUser(h.season))
}

func (h *catalogHandlers) configured(w http.ResponseWriter) bool {
	if h.catalog == nil || h.search == nil {
		writeError(w, http.StatusServiceUnavailable, "tmdb_not_configured", "ask the admin to set WATCHT_TMDB_API_KEY")
		return false
	}
	return true
}

func (h *catalogHandlers) searchTitles(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if !h.configured(w) {
		return
	}
	q := strings.Join(strings.Fields(r.URL.Query().Get("q")), " ")
	if q == "" || utf8.RuneCountInString(q) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_query", "query must contain 1 to 100 characters")
		return
	}
	page := 1
	if r.URL.Query().Has("page") {
		var err error
		page, err = strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 || page > 500 {
			writeError(w, http.StatusBadRequest, "invalid_page", "page must be an integer from 1 to 500")
			return
		}
	}
	result, err := h.search.Search(r.Context(), q, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	results := make([]searchResultJSON, 0, len(result.Results))
	for _, item := range result.Results {
		mediaType := item.MediaType
		if mediaType == "tv" {
			mediaType = "show"
		}
		results = append(results, searchResultJSON{
			MediaType:     mediaType,
			TMDBID:        item.TMDBID,
			Title:         item.Title,
			OriginalTitle: item.OriginalTitle,
			Overview:      item.Overview,
			Date:          item.Date,
			PosterPath:    item.PosterPath,
			VoteAverage:   item.VoteAverage,
		})
	}
	writeJSON(w, http.StatusOK, searchJSON{
		Page:         result.Page,
		TotalPages:   result.TotalPages,
		TotalResults: result.TotalResults,
		Results:      results,
	})
}

// catalogPathNumber bounds path integers to TMDB's signed 32-bit numbers.
func catalogPathNumber(w http.ResponseWriter, r *http.Request, key string, minimum int64) (int, bool) {
	raw := r.PathValue(key)
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < minimum || strings.HasPrefix(raw, "+") {
		writeError(w, http.StatusNotFound, "not_found", "no such catalog item")
		return 0, false
	}
	return int(n), true
}

func (h *catalogHandlers) movie(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if !h.configured(w) {
		return
	}
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	movie, err := h.catalog.Movie(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		err = h.catalog.FetchMovie(r.Context(), id)
		if err == nil {
			movie, err = h.catalog.Movie(r.Context(), id)
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.catalog.MarkMovieViewed(r.Context(), id); err != nil && r.Context().Err() == nil {
		h.auth.logger.Warn("mark movie viewed", "err", err)
	}
	writeJSON(w, http.StatusOK, struct {
		Movie movieJSON `json:"movie"`
	}{Movie: movieResponse(movie)})
}

func (h *catalogHandlers) show(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if !h.configured(w) {
		return
	}
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	show, err := h.catalog.Show(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		err = h.catalog.FetchShow(r.Context(), id)
		if err == nil {
			show, err = h.catalog.Show(r.Context(), id)
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.catalog.MarkShowViewed(r.Context(), id); err != nil && r.Context().Err() == nil {
		h.auth.logger.Warn("mark show viewed", "err", err)
	}
	writeJSON(w, http.StatusOK, struct {
		Show showJSON `json:"show"`
	}{Show: showResponse(show)})
}

func (h *catalogHandlers) season(w http.ResponseWriter, r *http.Request, _ auth.User) {
	if !h.configured(w) {
		return
	}
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	number, ok := catalogPathNumber(w, r, "number", 0)
	if !ok {
		return
	}
	season, episodes, err := h.catalog.Season(r.Context(), id, number)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.catalog.MarkShowViewed(r.Context(), id); err != nil && r.Context().Err() == nil {
		h.auth.logger.Warn("mark show viewed", "err", err)
	}
	var result []episodeJSON
	if !season.EpisodesPending {
		result = make([]episodeJSON, 0, len(episodes))
		for _, e := range episodes {
			result = append(result, episodeResponse(e))
		}
	}
	writeJSON(w, http.StatusOK, seasonJSON{
		ShowTMDBID:      id,
		Season:          seasonResponse(season),
		EpisodesPending: season.EpisodesPending,
		Episodes:        result,
	})
}

func (h *catalogHandlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		return
	}
	switch {
	case errors.Is(err, catalog.ErrNotFound), errors.Is(err, providers.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no such catalog item")
	case errors.Is(err, tmdb.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid catalog request")
	case providers.Unavailable(err):
		// The provider client's errors carry the provider, path, status, and
		// a transport cause or body description, never the query, headers,
		// or body, so they are safe to log.
		h.auth.logger.Error("catalog provider unavailable", "err", err)
		writeError(w, http.StatusBadGateway, "provider_unavailable", "metadata provider is unavailable; try again later")
	default:
		h.auth.internalError(w, "catalog request", err)
	}
}

type genreJSON struct {
	TMDBID int    `json:"tmdb_id"`
	Name   string `json:"name"`
}

func genreResponse(v catalog.Genre) genreJSON {
	return genreJSON{
		TMDBID: v.TMDBID,
		Name:   v.Name,
	}
}

type movieJSON struct {
	TMDBID           int         `json:"tmdb_id"`
	IMDbID           string      `json:"imdb_id"`
	Title            string      `json:"title"`
	OriginalTitle    string      `json:"original_title"`
	OriginalLanguage string      `json:"original_language"`
	Overview         string      `json:"overview"`
	Tagline          string      `json:"tagline"`
	Status           string      `json:"status"`
	PosterPath       string      `json:"poster_path"`
	BackdropPath     string      `json:"backdrop_path"`
	ReleaseDate      string      `json:"release_date"`
	Runtime          int         `json:"runtime"`
	Genres           []genreJSON `json:"genres"`
	VoteAverage      float64     `json:"vote_average"`
	VoteCount        int         `json:"vote_count"`
}

func movieResponse(v catalog.Movie) movieJSON {
	genres := make([]genreJSON, 0, len(v.Genres))
	for _, item := range v.Genres {
		genres = append(genres, genreResponse(item))
	}
	return movieJSON{
		TMDBID:           v.TMDBID,
		IMDbID:           v.IMDbID,
		Title:            v.Title,
		OriginalTitle:    v.OriginalTitle,
		OriginalLanguage: v.OriginalLanguage,
		Overview:         v.Overview,
		Tagline:          v.Tagline,
		Status:           v.Status,
		PosterPath:       v.PosterPath,
		BackdropPath:     v.BackdropPath,
		ReleaseDate:      v.ReleaseDate,
		Runtime:          v.Runtime,
		Genres:           genres,
		VoteAverage:      v.VoteAverage,
		VoteCount:        v.VoteCount,
	}
}

type showJSON struct {
	TMDBID           int              `json:"tmdb_id"`
	IMDbID           string           `json:"imdb_id"`
	TVDBID           int              `json:"tvdb_id"`
	Name             string           `json:"name"`
	OriginalName     string           `json:"original_name"`
	OriginalLanguage string           `json:"original_language"`
	Overview         string           `json:"overview"`
	Tagline          string           `json:"tagline"`
	Status           string           `json:"status"`
	PosterPath       string           `json:"poster_path"`
	BackdropPath     string           `json:"backdrop_path"`
	FirstAirDate     string           `json:"first_air_date"`
	LastAirDate      string           `json:"last_air_date"`
	InProduction     bool             `json:"in_production"`
	Genres           []genreJSON      `json:"genres"`
	Seasons          []seasonJSONItem `json:"seasons"`
	VoteAverage      float64          `json:"vote_average"`
	VoteCount        int              `json:"vote_count"`
}

func showResponse(v catalog.Show) showJSON {
	genres := make([]genreJSON, 0, len(v.Genres))
	for _, item := range v.Genres {
		genres = append(genres, genreResponse(item))
	}
	seasons := make([]seasonJSONItem, 0, len(v.Seasons))
	for _, item := range v.Seasons {
		seasons = append(seasons, seasonResponse(item))
	}
	return showJSON{
		TMDBID:           v.TMDBID,
		IMDbID:           v.IMDbID,
		TVDBID:           v.TVDBID,
		Name:             v.Name,
		OriginalName:     v.OriginalName,
		OriginalLanguage: v.OriginalLanguage,
		Overview:         v.Overview,
		Tagline:          v.Tagline,
		Status:           v.Status,
		PosterPath:       v.PosterPath,
		BackdropPath:     v.BackdropPath,
		FirstAirDate:     v.FirstAirDate,
		LastAirDate:      v.LastAirDate,
		InProduction:     v.InProduction,
		Genres:           genres,
		Seasons:          seasons,
		VoteAverage:      v.VoteAverage,
		VoteCount:        v.VoteCount,
	}
}

type seasonJSONItem struct {
	TMDBID          int    `json:"tmdb_id"`
	SeasonNumber    int    `json:"season_number"`
	Name            string `json:"name"`
	Overview        string `json:"overview"`
	PosterPath      string `json:"poster_path"`
	AirDate         string `json:"air_date"`
	EpisodeCount    int    `json:"episode_count"`
	EpisodesPending bool   `json:"episodes_pending"`
}

func seasonResponse(v catalog.Season) seasonJSONItem {
	return seasonJSONItem{
		TMDBID:          v.TMDBID,
		SeasonNumber:    v.SeasonNumber,
		Name:            v.Name,
		Overview:        v.Overview,
		PosterPath:      v.PosterPath,
		AirDate:         v.AirDate,
		EpisodeCount:    v.EpisodeCount,
		EpisodesPending: v.EpisodesPending,
	}
}

type episodeJSON struct {
	TMDBID        int     `json:"tmdb_id"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	StillPath     string  `json:"still_path"`
	AirDate       string  `json:"air_date"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
}

func episodeResponse(v catalog.Episode) episodeJSON {
	return episodeJSON{
		TMDBID:        v.TMDBID,
		SeasonNumber:  v.SeasonNumber,
		EpisodeNumber: v.EpisodeNumber,
		Name:          v.Name,
		Overview:      v.Overview,
		StillPath:     v.StillPath,
		AirDate:       v.AirDate,
		Runtime:       v.Runtime,
		VoteAverage:   v.VoteAverage,
	}
}

type searchResultJSON struct {
	MediaType     string  `json:"media_type"`
	TMDBID        int     `json:"tmdb_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	Date          string  `json:"date"`
	PosterPath    string  `json:"poster_path"`
	VoteAverage   float64 `json:"vote_average"`
}
type searchJSON struct {
	Page         int                `json:"page"`
	TotalPages   int                `json:"total_pages"`
	TotalResults int                `json:"total_results"`
	Results      []searchResultJSON `json:"results"`
}
type seasonJSON struct {
	ShowTMDBID      int            `json:"show_tmdb_id"`
	Season          seasonJSONItem `json:"season"`
	EpisodesPending bool           `json:"episodes_pending"`
	Episodes        []episodeJSON  `json:"episodes"`
}
