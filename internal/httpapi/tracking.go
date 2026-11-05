package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/tracking"
)

// Tracking is the watch history and manual marking used by the API.
type Tracking interface {
	MarkMovie(context.Context, int64, int, tracking.When) (tracking.Result, error)
	MarkEpisode(context.Context, int64, int, tracking.When) (tracking.Result, error)
	MarkSeason(context.Context, int64, int, int, tracking.When) (int, error)
	MarkShow(context.Context, int64, int, tracking.When) (int, error)
	Remove(context.Context, int64, int64) error
	History(context.Context, int64, tracking.Cursor, int) ([]tracking.HistoryEntry, error)
	MovieWatches(context.Context, int64, int) ([]tracking.Watch, error)
	ShowWatched(context.Context, int64, int) ([]tracking.EpisodeWatched, error)
}

type trackingHandlers struct {
	auth     *authHandlers
	tracking Tracking
}

func (h *trackingHandlers) register(api *apiMux) {
	api.handle("POST /api/v1/movies/{id}/watches", h.auth.withUser(h.markMovie))
	api.handle("POST /api/v1/episodes/{id}/watches", h.auth.withUser(h.markEpisode))
	api.handle("POST /api/v1/shows/{id}/seasons/{number}/watches", h.auth.withUser(h.markSeason))
	api.handle("POST /api/v1/shows/{id}/watches", h.auth.withUser(h.markShow))
	api.handle("DELETE /api/v1/watches/{id}", h.auth.withUser(h.remove))
	api.handle("GET /api/v1/movies/{id}/watches", h.auth.withUser(h.movieWatches))
	api.handle("GET /api/v1/shows/{id}/watched", h.auth.withUser(h.showWatched))
	api.handle("GET /api/v1/history", h.auth.withUser(h.history))
}

func invalidTrackingRequest(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_request", "invalid tracking request")
}

func decodeWhen(w http.ResponseWriter, r *http.Request) (tracking.When, bool) {
	body := bufio.NewReader(r.Body)
	if _, err := body.Peek(1); errors.Is(err, io.EOF) {
		return tracking.When{}, true
	}
	r.Body = struct {
		io.Reader
		io.Closer
	}{body, r.Body}
	var req *struct {
		WatchedAt   json.RawMessage `json:"watched_at"`
		ReleaseDate json.RawMessage `json:"release_date"`
	}
	// Keep decodeJSON's limits, but don't expose decoder errors or input.
	rec := &discardRecorder{header: http.Header{}, status: http.StatusOK}
	if !decodeJSON(rec, r, &req) {
		switch rec.status {
		case http.StatusUnsupportedMediaType:
			writeError(w, rec.status, "unsupported_media_type", "send a JSON body with Content-Type: application/json")
		case http.StatusRequestEntityTooLarge:
			writeError(w, rec.status, "too_large", "the request body is too large")
		default:
			// Not decodeJSON's message, which can quote the input.
			writeError(w, http.StatusBadRequest, "invalid_json", "the request body isn't valid JSON for this endpoint")
		}
		return tracking.When{}, false
	}
	var when tracking.When
	if req == nil || (len(req.WatchedAt) != 0 && len(req.ReleaseDate) != 0) {
		invalidTrackingRequest(w)
		return when, false
	}
	if len(req.WatchedAt) != 0 {
		var raw string
		if json.Unmarshal(req.WatchedAt, &raw) != nil {
			invalidTrackingRequest(w)
			return when, false
		}
		at, err := time.Parse(time.RFC3339, raw)
		if err != nil || at.IsZero() {
			invalidTrackingRequest(w)
			return when, false
		}
		when.At = at
	}
	// release_date is true or absent: false would read as "now", which an
	// omitted time already means.
	if len(req.ReleaseDate) != 0 {
		if json.Unmarshal(req.ReleaseDate, &when.ReleaseDate) != nil || !when.ReleaseDate {
			invalidTrackingRequest(w)
			return when, false
		}
	}
	return when, true
}

func (h *trackingHandlers) markMovie(w http.ResponseWriter, r *http.Request, u auth.User) {
	h.markOne(w, r, u, h.tracking.MarkMovie)
}

func (h *trackingHandlers) markEpisode(w http.ResponseWriter, r *http.Request, u auth.User) {
	h.markOne(w, r, u, h.tracking.MarkEpisode)
}

func (h *trackingHandlers) markOne(w http.ResponseWriter, r *http.Request, u auth.User, mark func(context.Context, int64, int, tracking.When) (tracking.Result, error)) {
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	when, ok := decodeWhen(w, r)
	if !ok {
		return
	}
	result, err := mark(r.Context(), u.ID, id, when)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var merged *int64
	if result.MergedInto != 0 {
		merged = &result.MergedInto
	}
	writeJSON(w, http.StatusCreated, struct {
		Watch      watchJSON `json:"watch"`
		MergedInto *int64    `json:"merged_into"`
	}{watchResponse(tracking.Watch{ID: result.ID, WatchedAt: result.WatchedAt, WatchedOn: result.WatchedOn, Source: tracking.SourceManual}), merged})
}

func (h *trackingHandlers) markSeason(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	number, ok := catalogPathNumber(w, r, "number", 0)
	if !ok {
		return
	}
	when, ok := decodeWhen(w, r)
	if !ok {
		return
	}
	added, err := h.tracking.MarkSeason(r.Context(), u.ID, id, number, when)
	h.writeAdded(w, r, added, err)
}

func (h *trackingHandlers) markShow(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	when, ok := decodeWhen(w, r)
	if !ok {
		return
	}
	added, err := h.tracking.MarkShow(r.Context(), u.ID, id, when)
	h.writeAdded(w, r, added, err)
}

func (h *trackingHandlers) writeAdded(w http.ResponseWriter, r *http.Request, added int, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Added int `json:"added"`
	}{added})
}

func (h *trackingHandlers) remove(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := pathID(w, r, "no such watch")
	if !ok {
		return
	}
	if err := h.tracking.Remove(r.Context(), u.ID, id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *trackingHandlers) movieWatches(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	watches, err := h.tracking.MovieWatches(r.Context(), u.ID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]watchJSON, 0, len(watches))
	for _, v := range watches {
		out = append(out, watchResponse(v))
	}
	writeJSON(w, http.StatusOK, struct {
		Watches []watchJSON `json:"watches"`
	}{out})
}

func (h *trackingHandlers) showWatched(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := catalogPathNumber(w, r, "id", 1)
	if !ok {
		return
	}
	episodes, err := h.tracking.ShowWatched(r.Context(), u.ID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]episodeWatchedJSON, 0, len(episodes))
	for _, v := range episodes {
		out = append(out, episodeWatchedJSON{v.TMDBID, v.Plays, v.LastWatchedAt.UTC().Format(time.RFC3339), optional(v.LastWatchedOn)})
	}
	writeJSON(w, http.StatusOK, struct {
		Episodes []episodeWatchedJSON `json:"episodes"`
	}{out})
}

func (h *trackingHandlers) history(w http.ResponseWriter, r *http.Request, u auth.User) {
	limit := 50
	query := r.URL.Query()
	if query.Has("limit") {
		n, err := strconv.Atoi(query.Get("limit"))
		if err != nil || n < 1 || n > tracking.MaxHistoryLimit {
			invalidTrackingRequest(w)
			return
		}
		limit = n
	}
	var cursor tracking.Cursor
	if query.Has("cursor") {
		seconds, id, ok := strings.Cut(query.Get("cursor"), "_")
		at, errAt := strconv.ParseInt(seconds, 10, 64)
		watchID, errID := positiveDecimal(id)
		if !ok || errAt != nil || errID != nil || at < -62135596800 || at > 253402300799 {
			invalidTrackingRequest(w)
			return
		}
		cursor = tracking.Cursor{WatchedAt: time.Unix(at, 0).UTC(), ID: watchID}
	}
	entries, err := h.tracking.History(r.Context(), u.ID, cursor, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]historyEntryJSON, 0, len(entries))
	for _, e := range entries {
		item := historyEntryJSON{watchJSON: watchResponse(e.Watch)}
		if v := e.Movie; v != nil {
			item.Movie = &historyMovieJSON{v.TMDBID, v.Title, v.PosterPath, tmdbImage("w342", v.PosterPath), v.ReleaseDate, v.Runtime, v.VoteAverage}
		}
		if v := e.Episode; v != nil {
			item.Episode = &historyEpisodeJSON{v.TMDBID, v.SeasonNumber, v.EpisodeNumber, v.Name, v.StillPath, tmdbImage("w300", v.StillPath), v.AirDate, v.Runtime, v.VoteAverage, historyShowJSON{v.ShowTMDBID, v.ShowName, v.ShowPosterPath, tmdbImage("w342", v.ShowPosterPath)}}
		}
		out = append(out, item)
	}
	var next *string
	if len(entries) == limit {
		last := entries[len(entries)-1]
		value := strconv.FormatInt(last.WatchedAt.Unix(), 10) + "_" + strconv.FormatInt(last.ID, 10)
		next = &value
	}
	writeJSON(w, http.StatusOK, struct {
		Entries    []historyEntryJSON `json:"entries"`
		NextCursor *string            `json:"next_cursor"`
	}{out, next})
}

func positiveDecimal(raw string) (int64, error) {
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, tracking.ErrInvalid
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, tracking.ErrInvalid
	}
	return n, nil
}

func (h *trackingHandlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		return
	}
	switch {
	case errors.Is(err, tracking.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no such watch or catalog item")
	case errors.Is(err, tracking.ErrInvalid):
		invalidTrackingRequest(w)
	case errors.Is(err, tracking.ErrNoReleaseDate):
		writeError(w, http.StatusUnprocessableEntity, "no_release_date", "the item has no release date")
	case errors.Is(err, tracking.ErrEpisodesPending):
		writeError(w, http.StatusConflict, "episodes_pending", "the episodes are still being fetched; try again shortly, or mark the fetched seasons one at a time")
	case errors.Is(err, tracking.ErrNotReleased):
		writeError(w, http.StatusUnprocessableEntity, "not_released", "the item has not been released")
	default:
		h.auth.internalError(w, "tracking request", err)
	}
}

// watchJSON is a watch. watched_on is its date (YYYY-MM-DD) when it has no
// time, as for a mark at the release date, and null otherwise; watched_at is
// then noon UTC on that date and only orders it.
type watchJSON struct {
	ID        int64           `json:"id"`
	WatchedAt string          `json:"watched_at"`
	WatchedOn *string         `json:"watched_on"`
	Source    tracking.Source `json:"source"`
}

func watchResponse(v tracking.Watch) watchJSON {
	return watchJSON{v.ID, v.WatchedAt.UTC().Format(time.RFC3339), optional(v.WatchedOn), v.Source}
}

// optional is s, or nil when it's empty.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type episodeWatchedJSON struct {
	TMDBID        int     `json:"tmdb_id"`
	Plays         int     `json:"plays"`
	LastWatchedAt string  `json:"last_watched_at"`
	LastWatchedOn *string `json:"last_watched_on"`
}

type historyEntryJSON struct {
	watchJSON
	Movie   *historyMovieJSON   `json:"movie"`
	Episode *historyEpisodeJSON `json:"episode"`
}

type historyMovieJSON struct {
	TMDBID      int     `json:"tmdb_id"`
	Title       string  `json:"title"`
	PosterPath  string  `json:"poster_path"`
	PosterURL   string  `json:"poster_url"`
	ReleaseDate string  `json:"release_date"`
	Runtime     int     `json:"runtime"`
	VoteAverage float64 `json:"vote_average"`
}

type historyShowJSON struct {
	TMDBID     int    `json:"tmdb_id"`
	Name       string `json:"name"`
	PosterPath string `json:"poster_path"`
	PosterURL  string `json:"poster_url"`
}

type historyEpisodeJSON struct {
	TMDBID        int             `json:"tmdb_id"`
	SeasonNumber  int             `json:"season_number"`
	EpisodeNumber int             `json:"episode_number"`
	Name          string          `json:"name"`
	StillPath     string          `json:"still_path"`
	StillURL      string          `json:"still_url"`
	AirDate       string          `json:"air_date"`
	Runtime       int             `json:"runtime"`
	VoteAverage   float64         `json:"vote_average"`
	Show          historyShowJSON `json:"show"`
}
