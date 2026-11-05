package tracking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

var (
	// ErrNoReleaseDate means a mark at the release date was asked for an item
	// TMDB has no release or air date for.
	ErrNoReleaseDate = errors.New("no release date")
	// ErrNotReleased means a mark at the release date was asked for an item
	// that hasn't been released yet.
	ErrNotReleased = errors.New("not released yet")
	// ErrEpisodesPending means a season or show mark was asked before the
	// background sync stored the episodes it covers, so it would mark only
	// part of them.
	ErrEpisodesPending = errors.New("episodes not fetched yet")
)

// When is the time a manual mark gets: At, or each item's own release or air
// date when ReleaseDate is set. A zero When means now.
type When struct {
	At          time.Time
	ReleaseDate bool
}

// ReleaseTime is noon UTC on a release or air date: the watch time a mark at
// that date is stored at (with the date itself, see Event.DateOnly), and the
// moment the item counts as released or aired (maintainer decision,
// 2026-11-03). ok is false for an empty or invalid date.
func ReleaseTime(date string) (t time.Time, ok bool) {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return time.Time{}, false
	}
	return d.Add(12 * time.Hour), true
}

// MarkMovie records a manual watch of a stored movie, by its TMDB ID.
func (s *Service) MarkMovie(ctx context.Context, userID int64, tmdbID int, w When) (Result, error) {
	var res Result
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		m, err := q.MovieToMark(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find movie: %w", err)
		}
		at, err := s.markTime(w, m.ReleaseDate)
		if err != nil {
			return err
		}
		res, err = s.recordManual(ctx, q, Event{UserID: userID, MovieID: m.ID, WatchedAt: at, DateOnly: w.ReleaseDate})
		return err
	})
	return res, err
}

// MarkEpisode records a manual watch of a stored episode, by its TMDB
// episode ID.
func (s *Service) MarkEpisode(ctx context.Context, userID int64, tmdbID int, w When) (Result, error) {
	var res Result
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		e, err := q.EpisodeToMark(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find episode: %w", err)
		}
		at, err := s.markTime(w, e.AirDate)
		if err != nil {
			return err
		}
		res, err = s.recordManual(ctx, q, Event{UserID: userID, EpisodeID: e.ID, WatchedAt: at, DateOnly: w.ReleaseDate})
		return err
	})
	return res, err
}

// MarkSeason records a manual watch of every episode of a season that has
// aired and that the user hasn't watched, and returns how many it added.
// The season is the one the show lists with this number; ErrNotFound if the
// show or season isn't stored or is hidden, ErrEpisodesPending if its
// episodes haven't been fetched yet. Missing episodes are skipped.
func (s *Service) MarkSeason(ctx context.Context, userID int64, showTMDBID, seasonNumber int, w When) (int, error) {
	var added int
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		season, err := q.VisibleSeasonToMark(ctx, sqlc.VisibleSeasonToMarkParams{
			ShowTmdbID: int64(showTMDBID), SeasonNumber: int64(seasonNumber),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find season: %w", err)
		}
		if season.EpisodesPending {
			return ErrEpisodesPending
		}
		rows, err := q.UnwatchedSeasonEpisodes(ctx, sqlc.UnwatchedSeasonEpisodesParams{SeasonID: season.ID, UserID: userID})
		if err != nil {
			return fmt.Errorf("list episodes: %w", err)
		}
		eps := make([]episodeToMark, len(rows))
		for i, r := range rows {
			eps[i] = episodeToMark{r.ID, r.AirDate}
		}
		added, err = s.markEpisodes(ctx, q, userID, eps, w)
		return err
	})
	return added, err
}

// MarkShow is MarkSeason for every season the show lists except specials
// (season 0), which are marked one season at a time. It refuses with
// ErrEpisodesPending until every one of those seasons' episodes is stored.
func (s *Service) MarkShow(ctx context.Context, userID int64, showTMDBID int, w When) (int, error) {
	var added int
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		showID, err := q.ShowToMark(ctx, int64(showTMDBID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find show: %w", err)
		}
		pending, err := q.ShowEpisodesPending(ctx, showID)
		if err != nil {
			return fmt.Errorf("check episodes: %w", err)
		}
		if pending {
			return ErrEpisodesPending
		}
		rows, err := q.UnwatchedShowEpisodes(ctx, sqlc.UnwatchedShowEpisodesParams{ShowID: showID, UserID: userID})
		if err != nil {
			return fmt.Errorf("list episodes: %w", err)
		}
		eps := make([]episodeToMark, len(rows))
		for i, r := range rows {
			eps[i] = episodeToMark{r.ID, r.AirDate}
		}
		added, err = s.markEpisodes(ctx, q, userID, eps, w)
		return err
	})
	return added, err
}

type episodeToMark struct {
	id      int64
	airDate sql.NullString
}

// markEpisodes records a manual watch of each episode that has aired: its air
// date's ReleaseTime has passed. Episodes with no air date haven't aired.
func (s *Service) markEpisodes(ctx context.Context, q *sqlc.Queries, userID int64, eps []episodeToMark, w When) (int, error) {
	now := s.now()
	added := 0
	for _, ep := range eps {
		aired, ok := ReleaseTime(ep.airDate.String)
		if !ok || aired.After(now) {
			continue
		}
		at := w.At
		if w.ReleaseDate {
			at = aired
		}
		if at.IsZero() {
			at = now
		}
		if _, err := s.recordManual(ctx, q, Event{UserID: userID, EpisodeID: ep.id, WatchedAt: at, DateOnly: w.ReleaseDate}); err != nil {
			return 0, err
		}
		added++
	}
	return added, nil
}

// markTime resolves a When for one item with the given release or air date.
func (s *Service) markTime(w When, date sql.NullString) (time.Time, error) {
	if !w.ReleaseDate {
		if w.At.IsZero() {
			return s.now(), nil
		}
		return w.At, nil
	}
	at, ok := ReleaseTime(date.String)
	if !ok {
		return time.Time{}, ErrNoReleaseDate
	}
	if at.After(s.now()) {
		return time.Time{}, ErrNotReleased
	}
	return at, nil
}

// recordManual validates and records a manual mark inside the caller's
// transaction.
func (s *Service) recordManual(ctx context.Context, q *sqlc.Queries, e Event) (Result, error) {
	e.Source = SourceManual
	if err := s.validate(e); err != nil {
		return Result{}, err
	}
	return s.record(ctx, q, e)
}
