package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Backup writes a consistent snapshot of the database to dest with
// VACUUM INTO. It is safe while the server is running: the snapshot is a
// single read transaction. Copying the live file is not safe in WAL mode.
// dest must not exist.
func (d *DB) Backup(ctx context.Context, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	// Backups hold the same private data as the database. Creating the file
	// first, exclusively and 0600, means it is never readable by others and
	// never replaces an existing file; SQLite fills an existing empty file.
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // dest is operator input (CLI) or built from WATCHT_DATA_DIR, never a request
	if err != nil {
		return fmt.Errorf("backup %s: %w", dest, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("backup %s: %w", dest, err)
	}
	// The read pool is query_only, which also blocks VACUUM INTO, so this
	// uses the write connection.
	if _, err := d.Write.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		// Don't leave an empty or partial file that looks like a snapshot.
		_ = os.Remove(dest)
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	return nil
}

// BackupPath returns a new backup file path in dir, named
// <prefix>-<UTC timestamp>.db, e.g. manual-20261024T183000Z.db.
func BackupPath(dir, prefix string, now time.Time) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s.db", prefix, now.UTC().Format("20060102T150405Z")))
}
