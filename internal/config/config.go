// Package config loads watcht's runtime configuration from environment
// variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the settings the server needs at startup. Fields are added
// as milestones need them; see the configuration table in
// docs/rewrite-plan.md.
type Config struct {
	// Listen is the TCP address the HTTP server binds, e.g. ":8080".
	Listen string
	// DataDir holds the database and backups (WATCHT_DATA_DIR).
	DataDir string
	// BaseURL is the public URL watcht is reached at (WATCHT_BASE_URL). Only
	// the scheme and host are kept. Nil when unset.
	BaseURL *url.URL
	// TrustedProxies are the reverse proxies whose X-Forwarded-For and
	// X-Forwarded-Proto headers are believed (WATCHT_TRUSTED_PROXIES). Nil
	// when unset.
	TrustedProxies []netip.Prefix
	// TMDBToken is TMDB's API Read Access Token (WATCHT_TMDB_API_KEY), with
	// surrounding space removed. Empty when unset: the server still starts,
	// and TMDB features report that they aren't configured.
	TMDBToken string
	// TMDBBaseURL replaces TMDB's API root (WATCHT_TMDB_BASE_URL), for
	// development and tests only: scripts/fake-tmdb serves recorded responses
	// there. Empty means TMDB's own API. It has no trailing slash.
	TMDBBaseURL string
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
	if v, ok := lookup("WATCHT_BASE_URL"); ok && v != "" {
		u, err := parseBaseURL(v)
		if err != nil {
			return Config{}, err
		}
		cfg.BaseURL = u
	}
	if v, ok := lookup("WATCHT_TRUSTED_PROXIES"); ok && v != "" {
		proxies, err := parseTrustedProxies(v)
		if err != nil {
			return Config{}, err
		}
		cfg.TrustedProxies = proxies
	}
	if v, ok := lookup("WATCHT_TMDB_API_KEY"); ok {
		cfg.TMDBToken = strings.TrimSpace(v)
	}
	if v, ok := lookup("WATCHT_TMDB_BASE_URL"); ok && v != "" {
		u, err := parseTMDBBaseURL(v)
		if err != nil {
			return Config{}, err
		}
		cfg.TMDBBaseURL = u
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

// parseBaseURL accepts an http or https origin, optionally with a port and a
// trailing "/", and keeps only the scheme and host.
func parseBaseURL(v string) (*url.URL, error) {
	u, err := url.Parse(v)
	if err != nil {
		// Not quoted: an unparsed value may hold a password, and the error
		// is logged. *url.Error repeats the whole value; keep only its reason.
		reason := err
		if ue, ok := errors.AsType[*url.Error](err); ok {
			reason = ue.Err
		}
		return nil, fmt.Errorf("WATCHT_BASE_URL is not a valid URL: %w", reason)
	}
	switch {
	// Checked first, and quoted redacted, so a password never reaches the message.
	case u.User != nil:
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must not include a username or password", u.Redacted())
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must start with http:// or https://", v)
	case u.Hostname() == "":
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must include a host, e.g. https://watcht.example.com", v)
	case strings.HasSuffix(u.Host, ":"):
		return nil, fmt.Errorf("WATCHT_BASE_URL %q has an empty port", v)
	case u.ForceQuery || u.RawQuery != "":
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must not have a query", v)
	case strings.Contains(v, "#"):
		// url.URL can't tell an empty fragment ("#") from none, so check the raw value.
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must not have a fragment", v)
	case u.Path != "" && u.Path != "/":
		return nil, fmt.Errorf("WATCHT_BASE_URL %q must not have a path: watcht is served at the root", v)
	}
	// Normalize to how browsers send Origin (lowercase, no default port),
	// since the cross-origin check compares origins as strings.
	host := strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		host = strings.TrimSuffix(host, ":"+u.Port())
	}
	return &url.URL{Scheme: u.Scheme, Host: host}, nil
}

// parseTMDBBaseURL accepts an absolute http or https URL with an optional path
// (TMDB's API is at .../3), and returns it without trailing slashes.
func parseTMDBBaseURL(v string) (string, error) {
	// Until the URL has parsed and been redacted, errors don't quote it: it
	// may hold a password, and the error is logged.
	if strings.TrimSpace(v) != v {
		// url.Parse accepts a trailing space and escapes it into the path,
		// which would turn every request into a 404.
		return "", errors.New("WATCHT_TMDB_BASE_URL must not have leading or trailing spaces")
	}
	u, err := url.Parse(v)
	if err != nil {
		// *url.Error repeats the whole value; keep only its reason.
		reason := err
		if ue, ok := errors.AsType[*url.Error](err); ok {
			reason = ue.Err
		}
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL is not a valid URL: %w", reason)
	}
	switch {
	// Checked first, and quoted redacted, so a password never reaches the message.
	case u.User != nil:
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must not include a username or password", u.Redacted())
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must start with http:// or https://", v)
	case u.Hostname() == "":
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must include a host, e.g. http://127.0.0.1:8098/3", v)
	case strings.HasSuffix(u.Host, ":"):
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q has an empty port", v)
	case u.Scheme == "http" && !isLoopbackHost(u.Hostname()):
		// The TMDB token travels in a header, so plain http is only for a
		// fake server on this machine.
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must use https unless the host is loopback (localhost, 127.0.0.1, ::1)", v)
	case u.ForceQuery || u.RawQuery != "":
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must not have a query", v)
	case strings.Contains(v, "#"):
		// url.URL can't tell an empty fragment ("#") from none, so check the raw value.
		return "", fmt.Errorf("WATCHT_TMDB_BASE_URL %q must not have a fragment", v)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// isLoopbackHost reports whether host is localhost or a loopback IP.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// parseTrustedProxies parses a comma-separated list of IPs and CIDRs. Blank
// entries are skipped.
func parseTrustedProxies(v string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, entry := range strings.Split(v, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		p, err := parseProxy(entry)
		if err != nil {
			return nil, fmt.Errorf("WATCHT_TRUSTED_PROXIES entry %q: %w", entry, err)
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

// parseProxy parses one IP or CIDR. An IPv4-mapped IPv6 address becomes IPv4,
// because that is how Go reports the client address of an IPv4 connection.
func parseProxy(entry string) (netip.Prefix, error) {
	// netip.Prefix can't hold a zone and drops it silently, so check the text.
	if strings.Contains(entry, "%") {
		return netip.Prefix{}, errors.New("must not have an IPv6 zone")
	}
	p, err := netip.ParsePrefix(entry)
	if err != nil {
		a, addrErr := netip.ParseAddr(entry)
		if addrErr != nil {
			return netip.Prefix{}, errors.New("is not an IP address or CIDR")
		}
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if p.Addr().Is4In6() {
		if p.Bits() < 96 {
			return netip.Prefix{}, errors.New("IPv4-mapped IPv6 prefixes must be /96 or longer")
		}
		p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	// Trusting everyone would let any client choose its own IP for the
	// login limit, and claim HTTPS.
	if p.Bits() == 0 {
		return netip.Prefix{}, errors.New("trusts every address; list only your reverse proxy's addresses")
	}
	return p.Masked(), nil
}

// DatabasePath is the SQLite file inside the data directory.
func (c Config) DatabasePath() string { return filepath.Join(c.DataDir, "watcht.db") }

// BackupDir is where backups are written inside the data directory.
func (c Config) BackupDir() string { return filepath.Join(c.DataDir, "backups") }

// BaseOrigin is the scheme and host of BaseURL, e.g. "https://watcht.example.com",
// or "" when BaseURL is unset.
func (c Config) BaseOrigin() string {
	if c.BaseURL == nil {
		return ""
	}
	return c.BaseURL.Scheme + "://" + c.BaseURL.Host
}

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
