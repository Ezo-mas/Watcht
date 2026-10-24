// Package config loads watcht's runtime configuration from environment
// variables.
package config

import (
	"fmt"
	"net"
	"os"
)

// Config holds the settings the server needs at startup. Fields are added
// as milestones need them; see the configuration table in
// docs/rewrite-plan.md.
type Config struct {
	// Listen is the TCP address the HTTP server binds, e.g. ":8080".
	Listen string
}

const defaultListen = ":8080"

// Load reads the configuration from the environment and validates it.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{Listen: defaultListen}
	if v, ok := lookup("WATCHT_LISTEN"); ok && v != "" {
		cfg.Listen = v
	}
	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return Config{}, fmt.Errorf("WATCHT_LISTEN %q is not a host:port address: %w", cfg.Listen, err)
	}
	return cfg, nil
}
