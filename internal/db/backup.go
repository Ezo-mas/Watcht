package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Backup writes a consistent snapshot of the database to dest with
// VACUUM INTO. It is safe while the server is running: the snapshot is a
// single read transaction. Copying the live file is not safe in WAL mode.
// dest must not exist.
func (d *DB) Backup(ctx context.Context, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("backup %s: %w", dest, fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("backup %s: %w", dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	// VACUUM can't run inside a transaction and only reads here, but it
	// goes through the write pool so it never races a migration.
	if _, err := d.Write.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	// Same reasoning as the database file: backups hold private data.
	if err := os.Chmod(dest, 0o600); err != nil {
		return fmt.Errorf("restrict permissions on %s: %w", dest, err)
	}
	return nil
}

// BackupPath returns a new backup file path in dir, named
// <prefix>-<UTC timestamp>.db, e.g. manual-20261024T183000Z.db.
func BackupPath(dir, prefix string, now time.Time) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s.db", prefix, now.UTC().Format("20060102T150405Z")))
}
