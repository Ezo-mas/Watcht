package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

// FetchMovie fetches a movie from TMDB and stores it.
func (s *Service) FetchMovie(ctx context.Context, tmdbID int) error {
	m, err := s.md.Movie(ctx, tmdbID)
	if err != nil {
		return err
	}
	return s.UpsertMovie(ctx, m)
}

// FetchShow fetches a show and its season list from TMDB and stores them. Its
// seasons' episodes are stored by SyncShow.
func (s *Service) FetchShow(ctx context.Context, tmdbID int) error {
	_, err := s.fetchShow(ctx, tmdbID)
	return err
}

// fetchShow is FetchShow, returning the response so SyncShow can read its
// season list.
func (s *Service) fetchShow(ctx context.Context, tmdbID int) (tmdb.Show, error) {
	show, err := s.md.Show(ctx, tmdbID)
	if err != nil {
		return tmdb.Show{}, err
	}
	if err := s.UpsertShow(ctx, show); err != nil {
		return tmdb.Show{}, err
	}
	return show, nil
}

// SyncShow fetches a show, then each season it lists (season 0 included), and
// stores them. Each season is one transaction. If a season fails, SyncShow
// stops with that error, and the seasons stored before it stay stored. A TMDB
// 404 still matches providers.ErrNotFound.
func (s *Service) SyncShow(ctx context.Context, tmdbID int) error {
	show, err := s.fetchShow(ctx, tmdbID)
	if err != nil {
		return err
	}
	for _, sum := range show.Seasons {
		season, err := s.md.Season(ctx, tmdbID, sum.SeasonNumber)
		if err != nil {
			return fmt.Errorf("sync season %d: %w", sum.SeasonNumber, err)
		}
		if err := s.UpsertSeason(ctx, tmdbID, season); err != nil {
			return fmt.Errorf("sync season %d: %w", sum.SeasonNumber, err)
		}
	}
	return nil
}

// UpsertMovie stores a movie by its TMDB ID, with its genres. The movie's
// genres become exactly m.Genres. A genre that TMDB drops is unlinked, but
// the genre row stays.
func (s *Service) UpsertMovie(ctx context.Context, m tmdb.Movie) error {
	now := db.Timestamp(s.now())
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		return upsertMovie(ctx, q, m, now)
	})
}

func upsertMovie(ctx context.Context, q *sqlc.Queries, m tmdb.Movie, now string) error {
	id, err := q.UpsertMovie(ctx, sqlc.UpsertMovieParams{
		TmdbID:            int64(m.TMDBID),
		ImdbID:            nullString(m.IMDbID),
		Title:             m.Title,
		OriginalTitle:     m.OriginalTitle,
		OriginalLanguage:  m.OriginalLanguage,
		Overview:          m.Overview,
		Tagline:           m.Tagline,
		Status:            m.Status,
		PosterPath:        m.PosterPath,
		BackdropPath:      m.BackdropPath,
		ReleaseDate:       nullString(m.ReleaseDate),
		Runtime:           nullInt(m.Runtime),
		VoteAverage:       m.VoteAverage,
		VoteCount:         int64(m.VoteCount),
		CreatedAt:         now,
		MetadataUpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("store movie %d: %w", m.TMDBID, err)
	}
	genreIDs, err := storeGenres(ctx, q, m.Genres)
	if err != nil {
		return fmt.Errorf("movie %d: %w", m.TMDBID, err)
	}
	if err := q.ClearMovieGenres(ctx, id); err != nil {
		return fmt.Errorf("clear genres of movie %d: %w", m.TMDBID, err)
	}
	for _, g := range genreIDs {
		if err := q.AddMovieGenre(ctx, sqlc.AddMovieGenreParams{MovieID: id, GenreID: g}); err != nil {
			return fmt.Errorf("link genre of movie %d: %w", m.TMDBID, err)
		}
	}
	return nil
}

// UpsertShow stores a show by its TMDB ID, with its genres and its season
// list. A season of the show's list is stored or updated by its TMDB ID. A
// season the list no longer has gets missing_since, unless it already has one.
// The overview of a stored season is kept, because the list doesn't carry it.
func (s *Service) UpsertShow(ctx context.Context, show tmdb.Show) error {
	now := db.Timestamp(s.now())
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		return upsertShow(ctx, q, show, now)
	})
}

func upsertShow(ctx context.Context, q *sqlc.Queries, show tmdb.Show, now string) error {
	id, err := q.UpsertShow(ctx, sqlc.UpsertShowParams{
		TmdbID:            int64(show.TMDBID),
		ImdbID:            nullString(show.IMDbID),
		TvdbID:            nullInt(show.TVDBID),
		Name:              show.Name,
		OriginalName:      show.OriginalName,
		OriginalLanguage:  show.OriginalLanguage,
		Overview:          show.Overview,
		Tagline:           show.Tagline,
		Status:            show.Status,
		PosterPath:        show.PosterPath,
		BackdropPath:      show.BackdropPath,
		FirstAirDate:      nullString(show.FirstAirDate),
		LastAirDate:       nullString(show.LastAirDate),
		InProduction:      boolInt(show.InProduction),
		VoteAverage:       show.VoteAverage,
		VoteCount:         int64(show.VoteCount),
		CreatedAt:         now,
		MetadataUpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("store show %d: %w", show.TMDBID, err)
	}
	genreIDs, err := storeGenres(ctx, q, show.Genres)
	if err != nil {
		return fmt.Errorf("show %d: %w", show.TMDBID, err)
	}
	if err := q.ClearShowGenres(ctx, id); err != nil {
		return fmt.Errorf("clear genres of show %d: %w", show.TMDBID, err)
	}
	for _, g := range genreIDs {
		if err := q.AddShowGenre(ctx, sqlc.AddShowGenreParams{ShowID: id, GenreID: g}); err != nil {
			return fmt.Errorf("link genre of show %d: %w", show.TMDBID, err)
		}
	}

	// All of the list is written in this one transaction, so a swap of two
	// season numbers is applied as a whole. The season_number index isn't
	// unique, so the rows can be written one at a time in any order.
	listed := make([]int64, 0, len(show.Seasons))
	for _, sum := range show.Seasons {
		err := q.UpsertSeasonSummary(ctx, sqlc.UpsertSeasonSummaryParams{
			ShowID:       id,
			TmdbID:       int64(sum.TMDBID),
			SeasonNumber: int64(sum.SeasonNumber),
			Name:         sum.Name,
			PosterPath:   sum.PosterPath,
			AirDate:      nullString(sum.AirDate),
			EpisodeCount: int64(sum.EpisodeCount),
			CreatedAt:    now,
		})
		if err != nil {
			return fmt.Errorf("store season %d of show %d: %w", sum.SeasonNumber, show.TMDBID, err)
		}
		listed = append(listed, int64(sum.TMDBID))
	}
	ids, err := idList(listed)
	if err != nil {
		return err
	}
	if err := q.MarkSeasonsMissing(ctx, sqlc.MarkSeasonsMissingParams{Now: now, ShowID: id, TmdbIds: ids}); err != nil {
		return fmt.Errorf("mark seasons of show %d missing: %w", show.TMDBID, err)
	}
	return nil
}

// UpsertSeason stores a season and its episodes, from TMDB's season response.
// showTMDBID must be a stored show, or the error matches ErrNotStored and
// nothing is written.
//
// An episode is matched by its TMDB episode ID. If it's already stored under
// another season, it's moved: its season_id and numbers change, and its
// internal ID stays. Fetching seasons in any order ends in the same state.
// Take an episode that moved from season 1 to season 0. Fetched 1 then 0:
// season 1's pass marks the episode missing (it's still linked to season 1
// and its response no longer lists it), and season 0's pass moves it and
// clears the mark. Fetched 0 then 1: season 0's pass moves the episode, so it
// isn't linked to season 1 anymore, and season 1's pass doesn't touch it.
// Either way it ends linked to season 0, with season 0's numbers and no
// missing time. Only a pass that moves an episode changes its season_id, so
// a season's pass only ever marks episodes still linked to it, and the order
// of the passes can't change where an episode ends up.
func (s *Service) UpsertSeason(ctx context.Context, showTMDBID int, season tmdb.Season) error {
	now := db.Timestamp(s.now())
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		showID, err := q.GetShowID(ctx, int64(showTMDBID))
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("show %d: %w", showTMDBID, ErrNotStored)
		}
		if err != nil {
			return fmt.Errorf("find show %d: %w", showTMDBID, err)
		}
		seasonID, err := q.UpsertSeason(ctx, sqlc.UpsertSeasonParams{
			ShowID:            showID,
			TmdbID:            int64(season.TMDBID),
			SeasonNumber:      int64(season.SeasonNumber),
			Name:              season.Name,
			Overview:          season.Overview,
			PosterPath:        season.PosterPath,
			AirDate:           nullString(season.AirDate),
			EpisodesUpdatedAt: now,
			CreatedAt:         now,
		})
		if err != nil {
			return fmt.Errorf("store season %d of show %d: %w", season.SeasonNumber, showTMDBID, err)
		}

		listed := make([]int64, 0, len(season.Episodes))
		for _, e := range season.Episodes {
			err := q.UpsertEpisode(ctx, sqlc.UpsertEpisodeParams{
				ShowID:            showID,
				SeasonID:          seasonID,
				TmdbID:            int64(e.TMDBID),
				SeasonNumber:      int64(e.SeasonNumber),
				EpisodeNumber:     int64(e.EpisodeNumber),
				Name:              e.Name,
				Overview:          e.Overview,
				StillPath:         e.StillPath,
				AirDate:           nullString(e.AirDate),
				Runtime:           nullInt(e.Runtime),
				VoteAverage:       e.VoteAverage,
				CreatedAt:         now,
				MetadataUpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("store episode %d of season %d: %w", e.TMDBID, season.SeasonNumber, err)
			}
			listed = append(listed, int64(e.TMDBID))
		}
		// Runs after the upserts, so every listed episode is already linked to
		// this season and is excluded by its TMDB ID.
		ids, err := idList(listed)
		if err != nil {
			return err
		}
		if err := q.MarkEpisodesMissing(ctx, sqlc.MarkEpisodesMissingParams{Now: now, SeasonID: seasonID, TmdbIds: ids}); err != nil {
			return fmt.Errorf("mark episodes of season %d missing: %w", season.SeasonNumber, err)
		}
		return nil
	})
}

// storeGenres stores each genre by its TMDB ID and returns the internal IDs,
// in the same order.
func storeGenres(ctx context.Context, q *sqlc.Queries, genres []tmdb.Genre) ([]int64, error) {
	ids := make([]int64, 0, len(genres))
	for _, g := range genres {
		id, err := q.UpsertGenre(ctx, sqlc.UpsertGenreParams{TmdbID: int64(g.ID), Name: g.Name})
		if err != nil {
			return nil, fmt.Errorf("store genre %d: %w", g.ID, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
