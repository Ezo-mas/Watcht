// Package db opens watcht's SQLite database and runs its migrations.
//
// It is the only place that opens SQLite. A DB has two handles:
//
//   - Write: exactly one connection. Every write goes through it, so writers
//     queue in Go instead of failing with SQLITE_BUSY.
//   - Read: a small pool of query-only connections. Thanks to WAL mode,
//     readers never block the writer or each other.
//
// See docs/agent-guides/architecture.md before changing anything here.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"runtime"

	// Registers the pure-Go "sqlite" database/sql driver.
	_ "modernc.org/sqlite"
)

// DB holds the write and read handles for one database file.
type DB struct {
	// Write is the single-connection pool for all writes and for
	// transactions that read and then write.
	Write *sql.DB
	// Read is a pool of query-only connections. A write on it fails.
	Read *sql.DB

	path string
}

// Open opens (creating if needed) the database at path and applies the
// required connection settings. It does not run migrations; call Migrate.
func Open(ctx context.Context, path string) (*DB, error) {
	if err := preparePrivateFiles(path); err != nil {
		return nil, err
	}

	write, err := sql.Open("sqlite", dsn(path, writePragmas, "immediate"))
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)
	write.SetConnMaxLifetime(0)
	write.SetConnMaxIdleTime(0)

	// The first connection creates the file and switches it to WAL, which is
	// persistent, so it must happen before the read pool connects.
	if err := write.PingContext(ctx); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// SQLite reports a failed switch to WAL by returning the old mode, not
	// an error. Without WAL, readers would block the writer.
	var mode string
	if err := write.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("read journal mode: %w", err)
	}
	if mode != "wal" {
		_ = write.Close()
		return nil, fmt.Errorf("%s: journal mode is %q, want wal (is the data directory on a network filesystem?)", path, mode)
	}

	read, err := sql.Open("sqlite", dsn(path, readPragmas, ""))
	if err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	n := max(4, runtime.NumCPU())
	read.SetMaxOpenConns(n)
	read.SetMaxIdleConns(n)
	if err := read.PingContext(ctx); err != nil {
		_ = write.Close()
		_ = read.Close()
		return nil, fmt.Errorf("open %s for reading: %w", path, err)
	}

	return &DB{Write: write, Read: read, path: path}, nil
}

// Path returns the database file's path.
func (d *DB) Path() string { return d.path }

// Ping checks that the database answers a query that reads the file (the
// schema page), not just that a connection is open. It is used by /healthz.
func (d *DB) Ping(ctx context.Context) error {
	var n int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema").Scan(&n); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

// preparePrivateFiles makes sure the database and its -wal and -shm files
// are readable only by their owner, before SQLite opens them: watch history
// is private, and SQLite creates files with the umask (usually 0644). A new
// database starts as an empty 0600 file (SQLite accepts that), and SQLite
// gives new -wal/-shm files the database file's mode. Existing files, e.g.
// a restored backup, are tightened in place.
func preparePrivateFiles(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // path is built from WATCHT_DATA_DIR, never a request
	if err == nil {
		return f.Close()
	}
	if !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create %s: %w", path, err)
	}
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(name, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			if errors.Is(err, fs.ErrPermission) {
				return fmt.Errorf("%s belongs to another user (running as uid %d); in Docker, set `user:` to the data directory's owner: %w", name, os.Getuid(), err)
			}
			return fmt.Errorf("restrict permissions on %s: %w", name, err)
		}
	}
	return nil
}

// Close closes both pools. The write pool closes last so its final close
// checkpoints the WAL into the main database file.
func (d *DB) Close() error {
	return errors.Join(d.Read.Close(), d.Write.Close())
}

// Pragmas applied to every connection. journal_mode is persistent in the
// file, so only the write pool sets it.
var (
	writePragmas = []string{
		"journal_mode(WAL)",
		"foreign_keys(1)",
		"busy_timeout(5000)",
		"synchronous(NORMAL)",
	}
	readPragmas = []string{
		"foreign_keys(1)",
		"busy_timeout(5000)",
		// Enforces the invariant that writes never use the read pool.
		"query_only(1)",
	}
)

func dsn(path string, pragmas []string, txlock string) string {
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	if txlock != "" {
		// BEGIN IMMEDIATE takes the write lock up front, so a transaction
		// that reads and then writes can't fail halfway with SQLITE_BUSY.
		q.Set("_txlock", txlock)
	}
	return "file:" + url.PathEscape(path) + "?" + q.Encode()
}
