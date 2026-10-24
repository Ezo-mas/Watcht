// Package config loads watcht's runtime configuration from environment
// variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Config holds the settings the server needs at startup. Fields are added
// as milestones need them; see the configuration table in
// docs/rewrite-plan.md.
type Config struct {
	// Listen is the TCP address the HTTP server binds, e.g. ":8080".
	Listen string
	// DataDir holds the database and backups (WATCHT_DATA_DIR).
	DataDir string
}

const (
	defaultListen  = ":8080"
	defaultDataDir = "/data"
)

// Load reads the configuration from the environment and validates it.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{Listen: defaultListen, DataDir: defaultDataDir}
	if v, ok := lookup("WATCHT_LISTEN"); ok && v != "" {
		cfg.Listen = v
	}
	if v, ok := lookup("WATCHT_DATA_DIR"); ok && v != "" {
		cfg.DataDir = v
	}

	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return Config{}, fmt.Errorf("WATCHT_LISTEN %q is not a host:port address: %w", cfg.Listen, err)
	}
	abs, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return Config{}, fmt.Errorf("WATCHT_DATA_DIR %q: %w", cfg.DataDir, err)
	}
	cfg.DataDir = abs
	return cfg, nil
}

// DatabasePath is the SQLite file inside the data directory.
func (c Config) DatabasePath() string { return filepath.Join(c.DataDir, "watcht.db") }

// BackupDir is where backups are written inside the data directory.
func (c Config) BackupDir() string { return filepath.Join(c.DataDir, "backups") }

// EnsureDataDir creates the data directory if needed and checks that it is
// writable, so a permission problem gives a clear message instead of an
// SQLite "unable to open database file".
func (c Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return dataDirError(c.DataDir, err)
	}
	f, err := os.CreateTemp(c.DataDir, ".write-test-*")
	if err != nil {
		return dataDirError(c.DataDir, err)
	}
	name := f.Name()
	return errors.Join(f.Close(), os.Remove(name))
}

func dataDirError(dir string, err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("data directory %s is not writable by this user (uid %d); in Docker, set `user:` to the directory's owner: %w", dir, os.Getuid(), err)
	}
	return fmt.Errorf("data directory %s: %w", dir, err)
}
