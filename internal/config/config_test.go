package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		env         map[string]string
		wantListen  string
		wantDataDir string
		wantErr     bool
	}{
		{name: "defaults", env: map[string]string{}, wantListen: ":8080", wantDataDir: "/data"},
		{name: "empty uses defaults", env: map[string]string{"WATCHT_LISTEN": "", "WATCHT_DATA_DIR": ""}, wantListen: ":8080", wantDataDir: "/data"},
		{name: "custom port", env: map[string]string{"WATCHT_LISTEN": ":9090"}, wantListen: ":9090", wantDataDir: "/data"},
		{name: "host and port", env: map[string]string{"WATCHT_LISTEN": "127.0.0.1:8099"}, wantListen: "127.0.0.1:8099", wantDataDir: "/data"},
		{name: "relative data dir becomes absolute", env: map[string]string{"WATCHT_DATA_DIR": "data"}, wantListen: ":8080", wantDataDir: filepath.Join(cwd, "data")},
		{name: "missing port", env: map[string]string{"WATCHT_LISTEN": "localhost"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			cfg, err := load(lookup)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("load() = %+v, want error", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if cfg.Listen != tt.wantListen {
				t.Errorf("Listen = %q, want %q", cfg.Listen, tt.wantListen)
			}
			if cfg.DataDir != tt.wantDataDir {
				t.Errorf("DataDir = %q, want %q", cfg.DataDir, tt.wantDataDir)
			}
		})
	}
}

func TestPaths(t *testing.T) {
	cfg := Config{DataDir: "/data"}
	if got := cfg.DatabasePath(); got != filepath.Join("/data", "watcht.db") {
		t.Errorf("DatabasePath = %q", got)
	}
	if got := cfg.BackupDir(); got != filepath.Join("/data", "backups") {
		t.Errorf("BackupDir = %q", got)
	}
}

func TestEnsureDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	if err := (Config{DataDir: dir}).EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("data dir has leftovers: %v", entries)
	}
}

func TestEnsureDataDirNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("directory permissions aren't enforced for root or on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := (Config{DataDir: dir}).EnsureDataDir()
	if err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("EnsureDataDir on a read-only dir = %v, want a 'not writable' error", err)
	}
}
