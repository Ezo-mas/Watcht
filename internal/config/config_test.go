package config

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

func TestLoadBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantURL string
		wantErr string
	}{
		{name: "unset", env: map[string]string{}},
		{name: "empty", env: map[string]string{"WATCHT_BASE_URL": ""}},
		{name: "https host", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com"}, wantURL: "https://watcht.example.com"},
		{name: "trailing slash is dropped", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com/"}, wantURL: "https://watcht.example.com"},
		{name: "scheme is lowercased", env: map[string]string{"WATCHT_BASE_URL": "HTTPS://watcht.example.com"}, wantURL: "https://watcht.example.com"},
		{name: "http with port", env: map[string]string{"WATCHT_BASE_URL": "http://192.168.1.10:8080"}, wantURL: "http://192.168.1.10:8080"},
		{name: "port with trailing slash", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com:8443/"}, wantURL: "https://watcht.example.com:8443"},
		// Normalized to what browsers send in Origin, which the CSRF check
		// compares as a string.
		{name: "host is lowercased", env: map[string]string{"WATCHT_BASE_URL": "https://Watcht.Example.com"}, wantURL: "https://watcht.example.com"},
		{name: "default https port is dropped", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com:443"}, wantURL: "https://watcht.example.com"},
		{name: "default http port is dropped", env: map[string]string{"WATCHT_BASE_URL": "http://watcht.lan:80/"}, wantURL: "http://watcht.lan"},
		{name: "443 on http is kept", env: map[string]string{"WATCHT_BASE_URL": "http://watcht.lan:443"}, wantURL: "http://watcht.lan:443"},
		{name: "IPv6 host", env: map[string]string{"WATCHT_BASE_URL": "http://[::1]:8080/"}, wantURL: "http://[::1]:8080"},
		{name: "no scheme", env: map[string]string{"WATCHT_BASE_URL": "watcht.example.com"}, wantErr: `WATCHT_BASE_URL "watcht.example.com" must start with http:// or https://`},
		{name: "wrong scheme", env: map[string]string{"WATCHT_BASE_URL": "ftp://watcht.example.com"}, wantErr: `WATCHT_BASE_URL "ftp://watcht.example.com" must start with http:// or https://`},
		{name: "no host", env: map[string]string{"WATCHT_BASE_URL": "https://"}, wantErr: `WATCHT_BASE_URL "https://" must include a host`},
		{name: "port without host", env: map[string]string{"WATCHT_BASE_URL": "https://:8080"}, wantErr: `WATCHT_BASE_URL "https://:8080" must include a host`},
		{name: "empty port", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com:"}, wantErr: `WATCHT_BASE_URL "https://watcht.example.com:" has an empty port`},
		{name: "non-numeric port", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com:http"}, wantErr: `WATCHT_BASE_URL "https://watcht.example.com:http" is not a valid URL`},
		{name: "username", env: map[string]string{"WATCHT_BASE_URL": "https://me@watcht.example.com"}, wantErr: `WATCHT_BASE_URL "https://me@watcht.example.com" must not include a username or password`},
		{name: "password is redacted", env: map[string]string{"WATCHT_BASE_URL": "https://me:secret@watcht.example.com"}, wantErr: `WATCHT_BASE_URL "https://me:xxxxx@watcht.example.com" must not include a username or password`},
		{name: "password is redacted before other checks", env: map[string]string{"WATCHT_BASE_URL": "ftp://me:secret@watcht.example.com"}, wantErr: `WATCHT_BASE_URL "ftp://me:xxxxx@watcht.example.com" must not include a username or password`},
		{name: "query", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com/?a=b"}, wantErr: `WATCHT_BASE_URL "https://watcht.example.com/?a=b" must not have a query`},
		{name: "empty query", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com/?"}, wantErr: `must not have a query`},
		{name: "fragment", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com/#top"}, wantErr: `WATCHT_BASE_URL "https://watcht.example.com/#top" must not have a fragment`},
		{name: "empty fragment", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com#"}, wantErr: `must not have a fragment`},
		{name: "path", env: map[string]string{"WATCHT_BASE_URL": "https://x/watcht"}, wantErr: `WATCHT_BASE_URL "https://x/watcht" must not have a path: watcht is served at the root`},
		{name: "double slash path", env: map[string]string{"WATCHT_BASE_URL": "https://watcht.example.com//"}, wantErr: `must not have a path`},
		{name: "leading space", env: map[string]string{"WATCHT_BASE_URL": " https://watcht.example.com"}, wantErr: `is not a valid URL`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			cfg, err := load(lookup)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("load() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if tt.wantURL == "" {
				if cfg.BaseURL != nil {
					t.Fatalf("BaseURL = %v, want nil", cfg.BaseURL)
				}
				return
			}
			if cfg.BaseURL == nil {
				t.Fatalf("BaseURL is nil, want %q", tt.wantURL)
			}
			if got := cfg.BaseURL.String(); got != tt.wantURL {
				t.Errorf("BaseURL = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    []string
		wantErr string
	}{
		{name: "unset", env: map[string]string{}},
		{name: "empty", env: map[string]string{"WATCHT_TRUSTED_PROXIES": ""}},
		{name: "only separators", env: map[string]string{"WATCHT_TRUSTED_PROXIES": " , ,"}},
		{name: "IPv4 address", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.1"}, want: []string{"10.0.0.1/32"}},
		{name: "IPv6 address", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "::1"}, want: []string{"::1/128"}},
		{name: "IPv4 CIDR", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.0/8"}, want: []string{"10.0.0.0/8"}},
		{name: "IPv4 CIDR host bits are masked", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.1.2.3/8"}, want: []string{"10.0.0.0/8"}},
		{name: "IPv6 CIDR host bits are masked", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "fd12::1/8"}, want: []string{"fd00::/8"}},
		{name: "list with spaces and trailing comma", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.0/8, 172.16.0.1, ::1,"}, want: []string{"10.0.0.0/8", "172.16.0.1/32", "::1/128"}},
		{name: "blank entries are skipped", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.1,,10.0.0.2"}, want: []string{"10.0.0.1/32", "10.0.0.2/32"}},
		{name: "IPv4-mapped address becomes IPv4", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "::ffff:10.0.0.1"}, want: []string{"10.0.0.1/32"}},
		{name: "IPv4-mapped CIDR becomes IPv4 CIDR", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "::ffff:10.1.2.0/120"}, want: []string{"10.1.2.0/24"}},
		{name: "IPv4-mapped CIDR host bits are masked", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "::ffff:10.1.2.3/104"}, want: []string{"10.0.0.0/8"}},
		{name: "hostname", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "proxy.local"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "proxy.local": is not an IP address or CIDR`},
		{name: "prefix length too long", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.0/33"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "10.0.0.0/33": is not an IP address or CIDR`},
		{name: "bad entry after a good one", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.0/8, nope"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "nope": is not an IP address or CIDR`},
		{name: "all IPv4", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "0.0.0.0/0"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "0.0.0.0/0": trusts every address`},
		{name: "all IPv6", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "10.0.0.1, ::/0"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "::/0": trusts every address`},
		{name: "IPv6 zone", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "fe80::1%eth0"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "fe80::1%eth0": must not have an IPv6 zone`},
		{name: "IPv6 zone in CIDR", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "fe80::%eth0/64"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "fe80::%eth0/64": must not have an IPv6 zone`},
		{name: "IPv4-mapped prefix shorter than /96", env: map[string]string{"WATCHT_TRUSTED_PROXIES": "::ffff:10.0.0.0/8"}, wantErr: `WATCHT_TRUSTED_PROXIES entry "::ffff:10.0.0.0/8": IPv4-mapped IPv6 prefixes must be /96 or longer`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			cfg, err := load(lookup)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("load() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if tt.want == nil && cfg.TrustedProxies != nil {
				t.Fatalf("TrustedProxies = %v, want nil", cfg.TrustedProxies)
			}
			var got []string
			for _, p := range cfg.TrustedProxies {
				got = append(got, p.String())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("TrustedProxies = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadTMDBToken(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "unset is allowed", env: map[string]string{}},
		{name: "empty is allowed", env: map[string]string{"WATCHT_TMDB_API_KEY": ""}},
		{name: "only spaces is empty", env: map[string]string{"WATCHT_TMDB_API_KEY": " \t "}},
		{name: "token", env: map[string]string{"WATCHT_TMDB_API_KEY": "test-read-token"}, want: "test-read-token"},
		{name: "surrounding space is trimmed", env: map[string]string{"WATCHT_TMDB_API_KEY": "  test-read-token\n"}, want: "test-read-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			cfg, err := load(lookup)
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if cfg.TMDBToken != tt.want {
				t.Errorf("TMDBToken = %q, want %q", cfg.TMDBToken, tt.want)
			}
		})
	}
}

func TestBaseOrigin(t *testing.T) {
	tests := []struct {
		name    string
		baseURL *url.URL
		want    string
	}{
		{name: "unset", want: ""},
		{name: "https host", baseURL: &url.URL{Scheme: "https", Host: "watcht.example.com"}, want: "https://watcht.example.com"},
		{name: "http host and port", baseURL: &url.URL{Scheme: "http", Host: "192.168.1.10:8080"}, want: "http://192.168.1.10:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{BaseURL: tt.baseURL}
			if got := cfg.BaseOrigin(); got != tt.want {
				t.Errorf("BaseOrigin() = %q, want %q", got, tt.want)
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
