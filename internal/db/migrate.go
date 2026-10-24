package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/pressly/goose/v3"
)

// The migration policy is in docs/rewrite-plan.md ("Migration policy"):
// before v0.1.0 the baseline may be rewritten; after that, released files
// are never edited.
//
//go:embed migrations
var embeddedMigrations embed.FS

// Migrations returns the embedded goose migration files.
func Migrations() fs.FS {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err) // constant path; can't fail
	}
	return sub
}

// MigrateOptions configures Migrate.
type MigrateOptions struct {
	// Migrations to apply. Nil means the embedded ones.
	Migrations fs.FS
	// BackupDir receives a snapshot before pending migrations are applied
	// to a database that already has data. Empty disables the backup.
	BackupDir string
	Logger    *slog.Logger
	// Now returns the current time, for backup file names. Nil means
	// time.Now.
	Now func() time.Time
}

// Migrate applies pending migrations. If the database is already at some
// version and migrations are pending, it first writes a VACUUM INTO
// snapshot to BackupDir, so a failed or unwanted upgrade can be rolled back
// by restoring that file and running the previous version. A brand-new
// database has nothing to back up and is migrated directly.
func (d *DB) Migrate(ctx context.Context, opts MigrateOptions) error {
	fsys := opts.Migrations
	if fsys == nil {
		fsys = Migrations()
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	p, err := goose.NewProvider(goose.DialectSQLite3, d.Write, fsys, goose.WithDisableGlobalRegistry(true))
	if errors.Is(err, goose.ErrNoMigrations) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	pending, err := p.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations: %w", err)
	}
	if !pending {
		return nil
	}
	current, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	var backup string
	if current > 0 && opts.BackupDir != "" {
		backup = BackupPath(opts.BackupDir, fmt.Sprintf("pre-migrate-v%d", current), now())
		if err := d.Backup(ctx, backup); err != nil {
			return fmt.Errorf("back up before migrating: %w", err)
		}
		logger.Info("backed up database before migrating", "path", backup, "version", current)
	}

	results, err := p.Up(ctx)
	for _, r := range results {
		if r.Error == nil {
			logger.Info("applied migration", "version", r.Source.Version, "file", r.Source.Path, "duration", r.Duration)
		}
	}
	if err != nil {
		if backup != "" {
			return fmt.Errorf("migrate from version %d (restore %s to roll back): %w", current, backup, err)
		}
		return fmt.Errorf("migrate from version %d: %w", current, err)
	}
	return nil
}
