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
	// Watch history is private. SQLite creates files with the umask
	// (usually 0644) and gives the -wal and -shm files the database file's
	// mode, so restricting it here covers all three.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("restrict permissions on %s: %w", path, err)
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

// Ping checks that the database answers a query. It is used by /healthz.
func (d *DB) Ping(ctx context.Context) error {
	var one int
	if err := d.Read.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("ping database: %w", err)
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
