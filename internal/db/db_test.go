package db

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(context.Background(), filepath.Join(t.TempDir(), "watcht.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestOpenAppliesPragmas(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()

	tests := []struct {
		pool   string
		pragma string
		want   string
	}{
		{"write", "journal_mode", "wal"},
		{"write", "foreign_keys", "1"},
		{"write", "busy_timeout", "5000"},
		{"write", "synchronous", "1"}, // NORMAL
		{"read", "journal_mode", "wal"},
		{"read", "foreign_keys", "1"},
		{"read", "busy_timeout", "5000"},
		{"read", "query_only", "1"},
		{"write", "query_only", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.pool+" "+tt.pragma, func(t *testing.T) {
			pool := d.Write
			if tt.pool == "read" {
				pool = d.Read
			}
			var got string
			if err := pool.QueryRowContext(ctx, "PRAGMA "+tt.pragma).Scan(&got); err != nil {
				t.Fatalf("PRAGMA %s: %v", tt.pragma, err)
			}
			if got != tt.want {
				t.Errorf("PRAGMA %s = %q, want %q", tt.pragma, got, tt.want)
			}
		})
	}
}

func TestReadPoolRejectsWrites(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	if _, err := d.Write.ExecContext(ctx, "CREATE TABLE t (x INTEGER) STRICT"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Read.ExecContext(ctx, "INSERT INTO t (x) VALUES (1)"); err == nil {
		t.Fatal("insert on the read pool succeeded, want an error")
	}
}

func TestConcurrentWritesDontFail(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	if _, err := d.Write.ExecContext(ctx, "CREATE TABLE t (x INTEGER) STRICT"); err != nil {
		t.Fatal(err)
	}

	const writers, each = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				// A read-then-write transaction is the case BEGIN IMMEDIATE
				// protects.
				tx, err := d.Write.BeginTx(ctx, nil)
				if err != nil {
					errs <- err
					return
				}
				var n int
				if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil {
					_ = tx.Rollback()
					errs <- err
					return
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO t (x) VALUES (?)", w*each+i); err != nil {
					_ = tx.Rollback()
					errs <- err
					return
				}
				if err := tx.Commit(); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	// Readers run alongside the writers.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				var n int
				if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent access: %v", err)
	}

	var n int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != writers*each {
		t.Errorf("rows = %d, want %d", n, writers*each)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	for _, stmt := range []string{
		"CREATE TABLE parent (id INTEGER PRIMARY KEY) STRICT",
		"CREATE TABLE child (parent_id INTEGER NOT NULL REFERENCES parent (id)) STRICT",
	} {
		if _, err := d.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Write.ExecContext(ctx, "INSERT INTO child (parent_id) VALUES (42)"); err == nil {
		t.Fatal("insert with a dangling foreign key succeeded, want an error")
	}
}

func TestPing(t *testing.T) {
	d := openTemp(t)
	if err := d.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	_ = d.Close()
	if err := d.Ping(context.Background()); err == nil {
		t.Fatal("Ping after Close = nil, want error")
	}
}

func TestDatabaseFilesArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watcht.db")
	d, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.Write.Exec("CREATE TABLE t (x INTEGER) STRICT"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Errorf("%s mode = %o, want no group or other access", filepath.Base(name), mode)
		}
	}
}

func TestPathWithSpecialCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my data #1 ?x=y")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "watcht.db")
	d, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = d.Close() }()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created at %s: %v", path, err)
	}
}

var testMigrations = fstest.MapFS{
	"00001_one.sql": {Data: []byte("-- +goose Up\nCREATE TABLE one (x INTEGER) STRICT;\n")},
	"00002_two.sql": {Data: []byte("-- +goose Up\nCREATE TABLE two (x INTEGER) STRICT;\n")},
	"README.md":     {Data: []byte("not a migration")},
}

func tableExists(t *testing.T, d *DB, name string) bool {
	t.Helper()
	var n int
	err := d.Read.QueryRowContext(context.Background(),
		"SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?", name).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func backups(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestMigrateFreshDatabaseSkipsBackup(t *testing.T) {
	d := openTemp(t)
	dir := filepath.Join(t.TempDir(), "backups")

	if err := d.Migrate(context.Background(), MigrateOptions{Migrations: testMigrations, BackupDir: dir}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !tableExists(t, d, "one") || !tableExists(t, d, "two") {
		t.Fatal("migrations were not applied")
	}
	if b := backups(t, dir); len(b) != 0 {
		t.Errorf("backups = %v, want none for a fresh database", b)
	}
}

func TestMigrateBacksUpBeforePendingMigrations(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "backups")
	now := func() time.Time { return time.Date(2026, 10, 24, 18, 30, 0, 0, time.UTC) }

	first := fstest.MapFS{"00001_one.sql": testMigrations["00001_one.sql"]}
	if err := d.Migrate(ctx, MigrateOptions{Migrations: first, BackupDir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, "INSERT INTO one (x) VALUES (7)"); err != nil {
		t.Fatal(err)
	}

	if err := d.Migrate(ctx, MigrateOptions{Migrations: testMigrations, BackupDir: dir, Now: now}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	got := backups(t, dir)
	want := "pre-migrate-v1-20261024T183000Z.db"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("backups = %v, want [%s]", got, want)
	}

	// The snapshot has the data and the old schema. Open it read-only:
	// Open would switch it to WAL and leave -wal/-shm files beside it.
	snap, err := sql.Open("sqlite", "file:"+filepath.Join(dir, want)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var x, two int
	if err := snap.QueryRowContext(ctx, "SELECT x FROM one").Scan(&x); err != nil || x != 7 {
		t.Errorf("snapshot row = %d, %v; want 7", x, err)
	}
	if err := snap.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name = 'two'").Scan(&two); err != nil || two != 0 {
		t.Errorf("snapshot has table two (%d, %v); backup ran after migrating", two, err)
	}
	_ = snap.Close()

	// Nothing pending: no new backup.
	if err := d.Migrate(ctx, MigrateOptions{Migrations: testMigrations, BackupDir: dir}); err != nil {
		t.Fatal(err)
	}
	if b := backups(t, dir); len(b) != 1 {
		t.Errorf("backups = %v, want only the first one", b)
	}
}

func TestMigrateFailureNamesBackup(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "backups")

	first := fstest.MapFS{"00001_one.sql": testMigrations["00001_one.sql"]}
	if err := d.Migrate(ctx, MigrateOptions{Migrations: first, BackupDir: dir}); err != nil {
		t.Fatal(err)
	}
	broken := fstest.MapFS{
		"00001_one.sql":    testMigrations["00001_one.sql"],
		"00002_broken.sql": {Data: []byte("-- +goose Up\nCREATE TABLE nope (;\n")},
	}
	err := d.Migrate(ctx, MigrateOptions{Migrations: broken, BackupDir: dir})
	if err == nil {
		t.Fatal("Migrate with a broken migration = nil, want error")
	}
	b := backups(t, dir)
	if len(b) != 1 || !strings.Contains(err.Error(), b[0]) {
		t.Errorf("error %q should name the backup %v", err, b)
	}
}

func TestMigrateWithoutMigrations(t *testing.T) {
	d := openTemp(t)
	empty := fstest.MapFS{"README.md": {Data: []byte("x")}}
	if err := d.Migrate(context.Background(), MigrateOptions{Migrations: empty}); err != nil {
		t.Fatalf("Migrate with no migrations = %v, want nil", err)
	}
}

func TestEmbeddedMigrationsApply(t *testing.T) {
	d := openTemp(t)
	if err := d.Migrate(context.Background(), MigrateOptions{}); err != nil {
		t.Fatalf("Migrate (embedded): %v", err)
	}
}

func TestBackupRefusesExistingFile(t *testing.T) {
	d := openTemp(t)
	dest := filepath.Join(t.TempDir(), "exists.db")
	if err := os.WriteFile(dest, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := d.Backup(context.Background(), dest)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("Backup over an existing file = %v, want fs.ErrExist", err)
	}
	if data, _ := os.ReadFile(dest); string(data) != "keep me" {
		t.Error("existing file was overwritten")
	}
}

func TestBackupIsPrivate(t *testing.T) {
	d := openTemp(t)
	dest := filepath.Join(t.TempDir(), "backups", "snap.db")
	if err := d.Backup(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{dest, filepath.Dir(dest)} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Errorf("%s mode = %o, want no group or other access", name, mode)
		}
	}
}

func TestBackupPath(t *testing.T) {
	got := BackupPath("/data/backups", "manual", time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3*3600)))
	want := filepath.Join("/data/backups", "manual-20260102T000405Z.db")
	if got != want {
		t.Errorf("BackupPath = %q, want %q", got, want)
	}
}
