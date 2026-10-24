package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

const healthcheckTimeout = 3 * time.Second

// healthClient never uses a proxy: the check targets this machine.
var healthClient = &http.Client{Transport: &http.Transport{Proxy: nil}}

// checkHealth calls /healthz on the server listening at listen (the
// WATCHT_LISTEN value) and returns an error unless it answers 200.
func checkHealth(ctx context.Context, listen string) error {
	url, err := healthURL(listen)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return nil
}

// healthURL turns a listen address into a URL on the local machine. A
// wildcard host (":8080", "0.0.0.0:8080", "[::]:8080") becomes 127.0.0.1:
// Go's wildcard listeners accept IPv4, and some containers have no IPv6
// loopback.
func healthURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("parse listen address %q: %w", listen, err)
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz", nil
}
