package tracking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// RemoveMovieWatches deletes every play of a stored movie for the user.
func (s *Service) RemoveMovieWatches(ctx context.Context, userID int64, tmdbID int) (int, error) {
	var removed int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		movie, err := q.MovieToMark(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find movie: %w", err)
		}
		removed, err = q.DeleteMovieWatches(ctx, sqlc.DeleteMovieWatchesParams{UserID: userID, MovieID: nullID(movie.ID)})
		if err != nil {
			return fmt.Errorf("delete movie watches: %w", err)
		}
		return nil
	})
	return int(removed), err
}

// RemoveEpisodeWatches deletes every play of an episode by its TMDB ID.
func (s *Service) RemoveEpisodeWatches(ctx context.Context, userID int64, tmdbID int) (int, error) {
	var removed int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		episode, err := q.EpisodeToMark(ctx, int64(tmdbID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find episode: %w", err)
		}
		removed, err = q.DeleteEpisodeWatches(ctx, sqlc.DeleteEpisodeWatchesParams{UserID: userID, EpisodeID: nullID(episode.ID)})
		if err != nil {
			return fmt.Errorf("delete episode watches: %w", err)
		}
		return nil
	})
	return int(removed), err
}

// RemoveSeasonWatches resolves the listed season as MarkSeason does, then
// deletes all its plays, including episodes no longer listed by metadata.
func (s *Service) RemoveSeasonWatches(ctx context.Context, userID int64, showTMDBID, seasonNumber int) (int, error) {
	var removed int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		season, err := q.VisibleSeasonToMark(ctx, sqlc.VisibleSeasonToMarkParams{ShowTmdbID: int64(showTMDBID), SeasonNumber: int64(seasonNumber)})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find season: %w", err)
		}
		removed, err = q.DeleteSeasonWatches(ctx, sqlc.DeleteSeasonWatchesParams{UserID: userID, SeasonID: season.ID})
		if err != nil {
			return fmt.Errorf("delete season watches: %w", err)
		}
		return nil
	})
	return int(removed), err
}

// RemoveShowWatches deletes all plays of a show, including specials and
// episodes or seasons no longer listed by metadata.
func (s *Service) RemoveShowWatches(ctx context.Context, userID int64, showTMDBID int) (int, error) {
	var removed int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		showID, err := q.ShowToMark(ctx, int64(showTMDBID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find show: %w", err)
		}
		removed, err = q.DeleteShowWatches(ctx, sqlc.DeleteShowWatchesParams{UserID: userID, ShowID: showID})
		if err != nil {
			return fmt.Errorf("delete show watches: %w", err)
		}
		return nil
	})
	return int(removed), err
}
