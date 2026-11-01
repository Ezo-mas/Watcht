package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// The types below are what the catalog returns, not the sqlc types. They
// carry TMDB IDs, since that's what the API uses. Optional values are "" or
// 0, as in the tmdb package.

// Genre is a TMDB genre.
type Genre struct {
	TMDBID int
	Name   string
}

// Movie is a stored movie, with its genres ordered by name.
type Movie struct {
	TMDBID            int
	IMDbID            string
	Title             string
	OriginalTitle     string
	OriginalLanguage  string
	Overview          string
	Tagline           string
	Status            string
	PosterPath        string
	BackdropPath      string
	ReleaseDate       string
	Runtime           int
	Genres            []Genre
	VoteAverage       float64
	VoteCount         int
	MetadataUpdatedAt time.Time
}

// Show is a stored show, with its genres ordered by name and its seasons that
// TMDB still lists, ordered by number.
type Show struct {
	TMDBID            int
	IMDbID            string
	TVDBID            int
	Name              string
	OriginalName      string
	OriginalLanguage  string
	Overview          string
	Tagline           string
	Status            string
	PosterPath        string
	BackdropPath      string
	FirstAirDate      string
	LastAirDate       string
	InProduction      bool
	Genres            []Genre
	Seasons           []Season
	VoteAverage       float64
	VoteCount         int
	MetadataUpdatedAt time.Time
}

// Season is one season of a show. EpisodeCount is the count TMDB gives in the
// show's season list.
type Season struct {
	TMDBID          int
	SeasonNumber    int
	Name            string
	Overview        string
	PosterPath      string
	AirDate         string
	EpisodeCount    int
	EpisodesPending bool
}

// Episode is a stored episode, identified by its TMDB episode ID.
type Episode struct {
	TMDBID        int
	SeasonNumber  int
	EpisodeNumber int
	Name          string
	Overview      string
	StillPath     string
	AirDate       string
	Runtime       int
	VoteAverage   float64
	IMDbID        string
	TVDBID        int
}

// Movie returns a stored movie by its TMDB ID. It returns ErrNotFound if the
// movie isn't stored.
func (s *Service) Movie(ctx context.Context, tmdbID int) (Movie, error) {
	row, err := s.read.GetMovie(ctx, int64(tmdbID))
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNotFound
	}
	if err != nil {
		return Movie{}, fmt.Errorf("read movie %d: %w", tmdbID, err)
	}
	rows, err := s.read.ListMovieGenres(ctx, row.ID)
	if err != nil {
		return Movie{}, fmt.Errorf("read genres of movie %d: %w", tmdbID, err)
	}
	updated, err := db.ParseTimestamp(row.MetadataUpdatedAt)
	if err != nil {
		return Movie{}, fmt.Errorf("movie %d: %w", tmdbID, err)
	}
	genres := make([]Genre, 0, len(rows))
	for _, g := range rows {
		genres = append(genres, Genre{TMDBID: int(g.TmdbID), Name: g.Name})
	}
	return Movie{
		TMDBID:            int(row.TmdbID),
		IMDbID:            row.ImdbID.String,
		Title:             row.Title,
		OriginalTitle:     row.OriginalTitle,
		OriginalLanguage:  row.OriginalLanguage,
		Overview:          row.Overview,
		Tagline:           row.Tagline,
		Status:            row.Status,
		PosterPath:        row.PosterPath,
		BackdropPath:      row.BackdropPath,
		ReleaseDate:       row.ReleaseDate.String,
		Runtime:           int(row.Runtime.Int64),
		Genres:            genres,
		VoteAverage:       row.VoteAverage,
		VoteCount:         int(row.VoteCount),
		MetadataUpdatedAt: updated,
	}, nil
}

// Show returns a stored show by its TMDB ID. It returns ErrNotFound if the
// show isn't stored.
func (s *Service) Show(ctx context.Context, tmdbID int) (Show, error) {
	row, err := s.read.GetShow(ctx, int64(tmdbID))
	if errors.Is(err, sql.ErrNoRows) {
		return Show{}, ErrNotFound
	}
	if err != nil {
		return Show{}, fmt.Errorf("read show %d: %w", tmdbID, err)
	}
	genreRows, err := s.read.ListShowGenres(ctx, row.ID)
	if err != nil {
		return Show{}, fmt.Errorf("read genres of show %d: %w", tmdbID, err)
	}
	seasonRows, err := s.read.ListShowSeasons(ctx, row.ID)
	if err != nil {
		return Show{}, fmt.Errorf("read seasons of show %d: %w", tmdbID, err)
	}
	updated, err := db.ParseTimestamp(row.MetadataUpdatedAt)
	if err != nil {
		return Show{}, fmt.Errorf("show %d: %w", tmdbID, err)
	}
	genres := make([]Genre, 0, len(genreRows))
	for _, g := range genreRows {
		genres = append(genres, Genre{TMDBID: int(g.TmdbID), Name: g.Name})
	}
	seasons := make([]Season, 0, len(seasonRows))
	for _, r := range seasonRows {
		seasons = append(seasons, Season{
			TMDBID:          int(r.TmdbID),
			SeasonNumber:    int(r.SeasonNumber),
			Name:            r.Name,
			Overview:        r.Overview,
			PosterPath:      r.PosterPath,
			AirDate:         r.AirDate.String,
			EpisodeCount:    int(r.EpisodeCount),
			EpisodesPending: !r.EpisodesUpdatedAt.Valid,
		})
	}
	return Show{
		TMDBID:            int(row.TmdbID),
		IMDbID:            row.ImdbID.String,
		TVDBID:            int(row.TvdbID.Int64),
		Name:              row.Name,
		OriginalName:      row.OriginalName,
		OriginalLanguage:  row.OriginalLanguage,
		Overview:          row.Overview,
		Tagline:           row.Tagline,
		Status:            row.Status,
		PosterPath:        row.PosterPath,
		BackdropPath:      row.BackdropPath,
		FirstAirDate:      row.FirstAirDate.String,
		LastAirDate:       row.LastAirDate.String,
		InProduction:      row.InProduction == 1,
		Genres:            genres,
		Seasons:           seasons,
		VoteAverage:       row.VoteAverage,
		VoteCount:         int(row.VoteCount),
		MetadataUpdatedAt: updated,
	}, nil
}

// SeasonEpisodes returns the episodes of a show's season, by episode number.
// Only episodes TMDB still lists are returned. If two listed seasons of the
// show share the number, the one with the lowest TMDB season ID is used. It
// returns ErrNotFound if the show or season isn't stored, or the season is
// hidden because TMDB no longer lists it.
func (s *Service) SeasonEpisodes(ctx context.Context, showTMDBID, seasonNumber int) ([]Episode, error) {
	_, episodes, err := s.Season(ctx, showTMDBID, seasonNumber)
	return episodes, err
}

// Season returns a visible season and its episodes. Episodes are nil while the
// season has not been fetched, and an empty slice when a fetched season has none.
func (s *Service) Season(ctx context.Context, showTMDBID, seasonNumber int) (Season, []Episode, error) {
	row, err := s.read.GetVisibleSeason(ctx, sqlc.GetVisibleSeasonParams{
		ShowTmdbID:   int64(showTMDBID),
		SeasonNumber: int64(seasonNumber),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Season{}, nil, ErrNotFound
	}
	if err != nil {
		return Season{}, nil, fmt.Errorf("read season %d of show %d: %w", seasonNumber, showTMDBID, err)
	}
	season := Season{
		TMDBID:          int(row.TmdbID),
		SeasonNumber:    int(row.SeasonNumber),
		Name:            row.Name,
		Overview:        row.Overview,
		PosterPath:      row.PosterPath,
		AirDate:         row.AirDate.String,
		EpisodeCount:    int(row.EpisodeCount),
		EpisodesPending: !row.EpisodesUpdatedAt.Valid,
	}
	if season.EpisodesPending {
		return season, nil, nil
	}
	rows, err := s.read.ListSeasonEpisodes(ctx, row.ID)
	if err != nil {
		return Season{}, nil, fmt.Errorf("read episodes of season %d of show %d: %w", seasonNumber, showTMDBID, err)
	}
	episodes := make([]Episode, 0, len(rows))
	for _, r := range rows {
		episodes = append(episodes, Episode{
			TMDBID:        int(r.TmdbID),
			SeasonNumber:  int(r.SeasonNumber),
			EpisodeNumber: int(r.EpisodeNumber),
			Name:          r.Name,
			Overview:      r.Overview,
			StillPath:     r.StillPath,
			AirDate:       r.AirDate.String,
			Runtime:       int(r.Runtime.Int64),
			VoteAverage:   r.VoteAverage,
			IMDbID:        r.ImdbID.String,
			TVDBID:        int(r.TvdbID.Int64),
		})
	}
	return season, episodes, nil
}
