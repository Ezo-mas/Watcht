package tracking

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// MaxHistoryLimit is the most history entries one page returns.
const MaxHistoryLimit = 100

// Watch is one kept watch: merged events aren't listed, since they count as
// the watch they were merged into.
type Watch struct {
	ID        int64
	WatchedAt time.Time
	// WatchedOn is the date (YYYY-MM-DD) of a watch with no time (see
	// Event.DateOnly), or empty.
	WatchedOn string
	Source    Source
}

// HistoryEntry is a watch with what was watched. Exactly one of Movie and
// Episode is set.
type HistoryEntry struct {
	Watch
	Movie   *HistoryMovie
	Episode *HistoryEpisode
}

// HistoryMovie is the movie a history entry is for. Runtime is in minutes, 0
// when TMDB has none; VoteAverage is TMDB's, 0 to 10.
type HistoryMovie struct {
	TMDBID      int
	Title       string
	PosterPath  string
	ReleaseDate string
	Runtime     int
	VoteAverage float64
}

// HistoryEpisode is the episode a history entry is for, with its show.
// AirDate is YYYY-MM-DD, or empty; Runtime and VoteAverage are as for
// HistoryMovie.
type HistoryEpisode struct {
	TMDBID         int
	SeasonNumber   int
	EpisodeNumber  int
	Name           string
	StillPath      string
	AirDate        string
	Runtime        int
	VoteAverage    float64
	ShowTMDBID     int
	ShowName       string
	ShowPosterPath string
}

// Cursor is where a history page starts: after the entry with this time and
// ID. A Cursor with ID 0 is the first page (watch IDs start at 1; the time
// alone can't say so, since year 1 is a valid watch time).
type Cursor struct {
	WatchedAt time.Time
	ID        int64
}

// EpisodeWatched is how often a user watched one episode, and the episode's
// season (0 for specials). LastWatchedOn is the last watch's date when it has
// no time, or empty.
type EpisodeWatched struct {
	TMDBID        int
	SeasonNumber  int
	Plays         int
	LastWatchedAt time.Time
	LastWatchedOn string
}

// History returns up to limit of the user's watches after the cursor, newest
// first. limit is clamped to 1..MaxHistoryLimit.
func (s *Service) History(ctx context.Context, userID int64, after Cursor, limit int) ([]HistoryEntry, error) {
	limit = min(max(limit, 1), MaxHistoryLimit)
	p := sqlc.HistoryParams{UserID: userID, BeforeWatchedAt: "9999-12-31T23:59:59Z", BeforeID: math.MaxInt64, RowLimit: int64(limit)}
	if after.ID != 0 {
		p.BeforeWatchedAt, p.BeforeID = db.Timestamp(after.WatchedAt), after.ID
	}
	rows, err := s.read.History(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, r := range rows {
		at, err := db.ParseTimestamp(r.WatchedAt)
		if err != nil {
			return nil, err
		}
		e := HistoryEntry{Watch: Watch{ID: r.ID, WatchedAt: at, WatchedOn: r.WatchedOn.String, Source: Source(r.Source)}}
		if r.MovieTmdbID.Valid {
			e.Movie = &HistoryMovie{
				TMDBID:      int(r.MovieTmdbID.Int64),
				Title:       r.MovieTitle.String,
				PosterPath:  r.MoviePosterPath.String,
				ReleaseDate: r.MovieReleaseDate.String,
				Runtime:     int(r.MovieRuntime.Int64),
				VoteAverage: r.MovieVoteAverage.Float64,
			}
		} else {
			e.Episode = &HistoryEpisode{
				TMDBID:         int(r.EpisodeTmdbID.Int64),
				SeasonNumber:   int(r.SeasonNumber.Int64),
				EpisodeNumber:  int(r.EpisodeNumber.Int64),
				Name:           r.EpisodeName.String,
				StillPath:      r.EpisodeStillPath.String,
				AirDate:        r.EpisodeAirDate.String,
				Runtime:        int(r.EpisodeRuntime.Int64),
				VoteAverage:    r.EpisodeVoteAverage.Float64,
				ShowTMDBID:     int(r.ShowTmdbID.Int64),
				ShowName:       r.ShowName.String,
				ShowPosterPath: r.ShowPosterPath.String,
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// MovieWatches returns the user's watches of a movie, by its TMDB ID, newest
// first. A movie that isn't stored has none.
func (s *Service) MovieWatches(ctx context.Context, userID int64, tmdbID int) ([]Watch, error) {
	rows, err := s.read.MovieWatches(ctx, sqlc.MovieWatchesParams{UserID: userID, MovieTmdbID: int64(tmdbID)})
	if err != nil {
		return nil, fmt.Errorf("movie watches: %w", err)
	}
	out := make([]Watch, 0, len(rows))
	for _, r := range rows {
		at, err := db.ParseTimestamp(r.WatchedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, Watch{ID: r.ID, WatchedAt: at, WatchedOn: r.WatchedOn.String, Source: Source(r.Source)})
	}
	return out, nil
}

// EpisodeWatches returns the user's watches of an episode, by its TMDB episode
// ID, newest first. An episode that isn't stored has none.
func (s *Service) EpisodeWatches(ctx context.Context, userID int64, tmdbID int) ([]Watch, error) {
	rows, err := s.read.EpisodeWatches(ctx, sqlc.EpisodeWatchesParams{UserID: userID, EpisodeTmdbID: int64(tmdbID)})
	if err != nil {
		return nil, fmt.Errorf("episode watches: %w", err)
	}
	out := make([]Watch, 0, len(rows))
	for _, r := range rows {
		at, err := db.ParseTimestamp(r.WatchedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, Watch{ID: r.ID, WatchedAt: at, WatchedOn: r.WatchedOn.String, Source: Source(r.Source)})
	}
	return out, nil
}

// ShowWatched returns, for each episode of a show the user has watched, how
// often and when last. Episodes TMDB no longer lists are included, since the
// history is still there; the caller shows what it lists.
func (s *Service) ShowWatched(ctx context.Context, userID int64, showTMDBID int) ([]EpisodeWatched, error) {
	rows, err := s.read.ShowWatchedEpisodes(ctx, sqlc.ShowWatchedEpisodesParams{UserID: userID, ShowTmdbID: int64(showTMDBID)})
	if err != nil {
		return nil, fmt.Errorf("show watched: %w", err)
	}
	out := make([]EpisodeWatched, 0, len(rows))
	for _, r := range rows {
		at, err := db.ParseTimestamp(r.LastWatchedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, EpisodeWatched{TMDBID: int(r.TmdbID), SeasonNumber: int(r.SeasonNumber), Plays: int(r.Plays), LastWatchedAt: at, LastWatchedOn: r.LastWatchedOn.String})
	}
	return out, nil
}
