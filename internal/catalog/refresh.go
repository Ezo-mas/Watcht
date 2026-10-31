package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers"
)

const (
	// JobSyncShow fetches a show and its seasons in the background.
	JobSyncShow = "catalog_sync_show"
	// JobRefresh refreshes stale, recently viewed catalog items.
	JobRefresh = "catalog_refresh"
	// Bound each run to the five-minute timeout at 20 requests/s; a show can
	// require about 30 requests.
	refreshBatch     = 50
	refreshShowBatch = 10
)

// showNotFound distinguishes a deleted show from a season that disappeared
// during a sync; only a show-level 404 makes its sync job permanent.
type showNotFound struct{ error }

func (e *showNotFound) Unwrap() error { return e.error }
func isShowNotFound(err error) bool {
	var missing *showNotFound
	return errors.As(err, &missing)
}

type syncShowPayload struct {
	TMDBID int `json:"tmdb_id"`
}

// MarkMovieViewed records a detail-page visit, writing at most once an hour.
func (s *Service) MarkMovieViewed(ctx context.Context, tmdbID int) error {
	now := s.now()
	n, err := s.write.MarkMovieViewed(ctx, sqlc.MarkMovieViewedParams{
		Now: nullString(db.Timestamp(now)), TmdbID: int64(tmdbID), Cutoff: nullString(db.Timestamp(now.Add(-time.Hour))),
	})
	if err != nil {
		return fmt.Errorf("mark movie viewed: %w", err)
	}
	if n == 0 {
		_, err = s.read.GetMovie(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
	}
	return err
}

// MarkShowViewed records a detail-page visit, writing at most once an hour.
func (s *Service) MarkShowViewed(ctx context.Context, tmdbID int) error {
	now := s.now()
	n, err := s.write.MarkShowViewed(ctx, sqlc.MarkShowViewedParams{
		Now: nullString(db.Timestamp(now)), TmdbID: int64(tmdbID), Cutoff: nullString(db.Timestamp(now.Add(-time.Hour))),
	})
	if err != nil {
		return fmt.Errorf("mark show viewed: %w", err)
	}
	if n == 0 {
		_, err = s.read.GetShowID(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
	}
	return err
}

// HandleSyncShow fetches a queued show's episodes.
func (s *Service) HandleSyncShow(ctx context.Context, payload json.RawMessage) error {
	var p syncShowPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.TMDBID <= 0 {
		return jobs.Permanent(errors.New("invalid show sync payload"))
	}
	err := s.SyncShow(providers.WithoutStale(ctx), p.TMDBID)
	if isShowNotFound(err) {
		return jobs.Permanent(err)
	}
	return err
}

// Refresh updates stale titles whose detail pages were recently opened.
func (s *Service) Refresh(ctx context.Context) error {
	ctx = providers.WithoutStale(ctx)
	now := s.now()
	viewed := nullString(db.Timestamp(now.Add(-30 * 24 * time.Hour)))
	movies, err := s.read.ListRefreshMovies(ctx, sqlc.ListRefreshMoviesParams{
		ViewedCutoff: viewed, MetadataCutoff: db.Timestamp(now.Add(-7 * 24 * time.Hour)), BatchSize: refreshBatch,
	})
	if err != nil {
		return err
	}
	shows, err := s.read.ListRefreshShows(ctx, sqlc.ListRefreshShowsParams{
		ViewedCutoff: viewed, EndedCutoff: db.Timestamp(now.Add(-7 * 24 * time.Hour)),
		OngoingCutoff: db.Timestamp(now.Add(-24 * time.Hour)), BatchSize: refreshShowBatch,
	})
	if err != nil {
		return err
	}
	var movieCount, showCount, notFound int
	defer func() {
		if movieCount+showCount+notFound > 0 {
			s.logger.Info("refreshed catalog", "movies", movieCount, "shows", showCount, "not_found", notFound)
		}
	}()
	for _, id := range movies {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := s.FetchMovie(ctx, int(id))
		missing := errors.Is(err, providers.ErrNotFound)
		if missing {
			// Keep deleted titles, but advance their age so they cannot starve the
			// oldest-first batch by being selected on every run.
			s.logger.Warn("TMDB movie deleted", "tmdb_id", id)
			err = s.write.TouchMovieMetadata(ctx, sqlc.TouchMovieMetadataParams{Now: db.Timestamp(now), TmdbID: id})
		}
		if err != nil {
			return err
		}
		if missing {
			notFound++
		} else {
			movieCount++
		}
	}
	for _, id := range shows {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := s.SyncShow(ctx, int(id))
		missing := isShowNotFound(err)
		if missing {
			// As for movies, retain the row without starving later refreshes.
			s.logger.Warn("TMDB show deleted", "tmdb_id", id)
			err = s.write.TouchShowMetadata(ctx, sqlc.TouchShowMetadataParams{Now: db.Timestamp(now), TmdbID: id})
		}
		if err != nil {
			return err
		}
		if missing {
			notFound++
		} else {
			showCount++
		}
	}
	return ctx.Err()
}
