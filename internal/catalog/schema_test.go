package catalog

import (
	"context"
	"strings"
	"testing"
)

// The CHECK constraints in migration 00006 are the last line of defence for
// the values the catalog stores. These tests send bad values straight to the
// database, since the catalog never sends them.
func TestSchemaRejectsBadValues(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		query string
		arg   any
	}{
		{"release date that doesn't exist", "UPDATE movies SET release_date = ? WHERE tmdb_id = 603", "2026-02-30"},
		{"release date out of form", "UPDATE movies SET release_date = ? WHERE tmdb_id = 603", "2026-1-5"},
		{"in_production of 2", "UPDATE shows SET in_production = ? WHERE tmdb_id = 1396", 2},
		{"negative season number", "UPDATE seasons SET season_number = ? WHERE tmdb_id = 3572", -1},
		{"negative episode number", "UPDATE episodes SET episode_number = ? WHERE tmdb_id = 101", -1},
		{"timestamp without the Z layout", "UPDATE seasons SET created_at = ? WHERE tmdb_id = 3572", "2026-10-31 12:00:00"},
		{"missing_since that isn't a timestamp", "UPDATE episodes SET missing_since = ? WHERE tmdb_id = 101", "yesterday"},
		{"genre TMDB ID of 0", "UPDATE genres SET tmdb_id = ? WHERE tmdb_id = 28", 0},
		{"movie TMDB ID of 0", "UPDATE movies SET tmdb_id = ? WHERE tmdb_id = 603", 0},
		{"show TMDB ID of 0", "UPDATE shows SET tmdb_id = ? WHERE tmdb_id = 1396", 0},
		{"season TMDB ID of 0", "UPDATE seasons SET tmdb_id = ? WHERE tmdb_id = 3572", 0},
		{"episode TMDB ID of 0", "UPDATE episodes SET tmdb_id = ? WHERE tmdb_id = 101", 0},
		{"runtime of 0", "UPDATE movies SET runtime = ? WHERE tmdb_id = 603", 0},
		{"negative runtime", "UPDATE episodes SET runtime = ? WHERE tmdb_id = 101", -1},
		{"negative movie vote count", "UPDATE movies SET vote_count = ? WHERE tmdb_id = 603", -1},
		{"negative show vote average", "UPDATE shows SET vote_average = ? WHERE tmdb_id = 1396", -0.5},
		{"negative episode count", "UPDATE seasons SET episode_count = ? WHERE tmdb_id = 3572", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each case gets its own catalog. A value that is wrongly accepted would
			// otherwise change a row that later cases look up by its TMDB ID.
			e := newSchemaEnv(t)
			_, err := e.d.Write.ExecContext(ctx, tt.query, tt.arg)
			if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Errorf("error = %v, want a CHECK constraint failure", err)
			}
		})
	}
}

// newSchemaEnv returns a catalog with show 1396, movie 603, and season 3572 with
// episode 101, for the CHECK cases to change.
func newSchemaEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	ctx := context.Background()
	e.storeShow(t, summary(3572, 1, 1))
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatalf("UpsertMovie: %v", err)
	}
	e.upsertSeason(t, testSeason(3572, 1, testEpisode(101, 1)))
	return e
}

// Deleting a movie removes its genre links, and keeps the genres.
func TestDeletingMovieKeepsGenres(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.s.UpsertMovie(ctx, testMovie()); err != nil {
		t.Fatalf("UpsertMovie: %v", err)
	}
	if _, err := e.d.Write.ExecContext(ctx, "DELETE FROM movies WHERE tmdb_id = 603"); err != nil {
		t.Fatalf("delete movie: %v", err)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM movie_genres"); n != 0 {
		t.Errorf("movie_genres rows = %d after the movie was deleted, want 0", n)
	}
	if n := e.int64Of(t, "SELECT count(*) FROM genres"); n != 2 {
		t.Errorf("genres = %d, want 2 (kept)", n)
	}
}
