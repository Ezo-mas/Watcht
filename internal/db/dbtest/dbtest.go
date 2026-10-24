// Package dbtest gives tests a real, migrated SQLite database.
//
// Tests that touch the database use New instead of opening SQLite
// themselves, so they get the same pragmas (foreign keys, WAL, query-only
// reads) as production. Never mock the database.
package dbtest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Ezo-mas/watcht/internal/db"
)

// New returns a fresh database in a temporary directory with all embedded
// migrations applied. It is closed when the test ends.
func New(t testing.TB) *db.DB {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "watcht.db"))
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Errorf("dbtest: close: %v", err)
		}
	})
	if err := d.Migrate(ctx, db.MigrateOptions{}); err != nil {
		t.Fatalf("dbtest: migrate: %v", err)
	}
	return d
}
