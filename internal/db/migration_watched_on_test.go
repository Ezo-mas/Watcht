package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"
)

// Migration 00009 marks the release and air date watches stored before it
// (manual, at noon UTC on the item's own date) as dates, and nothing else.
func TestMigrationBackfillsWatchedOn(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()

	upTo8 := fstest.MapFS{}
	entries, err := fs.ReadDir(Migrations(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name < "00009" && name != "README.md" {
			data, err := fs.ReadFile(Migrations(), name)
			if err != nil {
				t.Fatal(err)
			}
			upTo8[name] = &fstest.MapFile{Data: data}
		}
	}
	if err := d.Migrate(ctx, MigrateOptions{Migrations: upTo8}); err != nil {
		t.Fatalf("migrate to 8: %v", err)
	}

	const ts = "2026-11-03T20:00:00Z"
	seed := []string{
		`INSERT INTO users (id, username, created_at) VALUES (1, 'alice', '` + ts + `')`,
		`INSERT INTO movies (id, tmdb_id, title, release_date, created_at, metadata_updated_at) VALUES (1, 603, 'M', '1999-03-30', '` + ts + `', '` + ts + `')`,
		`INSERT INTO shows (id, tmdb_id, name, in_production, created_at, metadata_updated_at) VALUES (1, 1396, 'S', 0, '` + ts + `', '` + ts + `')`,
		`INSERT INTO seasons (id, show_id, tmdb_id, season_number, created_at) VALUES (1, 1, 3572, 1, '` + ts + `')`,
		`INSERT INTO episodes (id, show_id, season_id, tmdb_id, season_number, episode_number, air_date, created_at, metadata_updated_at) VALUES (1, 1, 1, 62085, 1, 1, '2008-01-20', '` + ts + `', '` + ts + `')`,
	}
	for _, q := range seed {
		if _, err := d.Write.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	watches := []struct {
		id                  int
		movie, episode      any
		at, source, eventID any
		want                string
	}{
		{1, 1, nil, "1999-03-30T12:00:00Z", "manual", nil, "1999-03-30"}, // release date mark
		{2, nil, 1, "2008-01-20T12:00:00Z", "manual", nil, "2008-01-20"}, // air date mark
		{3, 1, nil, "1999-03-30T12:00:01Z", "manual", nil, ""},           // not exactly noon
		{4, 1, nil, "2020-01-01T12:00:00Z", "manual", nil, ""},           // noon, another day
		{5, nil, 1, "2008-01-20T12:00:00Z", "plex", "px", ""},            // not manual
	}
	for _, w := range watches {
		if _, err := d.Write.ExecContext(ctx, `INSERT INTO watch_events (id, user_id, movie_id, episode_id, watched_at, source, source_event_id, created_at) VALUES (?, 1, ?, ?, ?, ?, ?, ?)`,
			w.id, w.movie, w.episode, w.at, w.source, w.eventID, ts); err != nil {
			t.Fatalf("watch %d: %v", w.id, err)
		}
	}

	if err := d.Migrate(ctx, MigrateOptions{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, w := range watches {
		var on sql.NullString
		if err := d.Read.QueryRowContext(ctx, `SELECT watched_on FROM watch_events WHERE id = ?`, w.id).Scan(&on); err != nil {
			t.Fatal(err)
		}
		if on.String != w.want {
			t.Errorf("watch %d (%s): watched_on = %q, want %q", w.id, w.at, on.String, w.want)
		}
	}
}
