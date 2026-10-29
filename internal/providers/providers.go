// Package providers is the HTTP client that every metadata provider (TMDB,
// MDBList, ...) calls through. It applies a timeout to each attempt, a rate
// limit per provider, retries for failures that may pass, and a response
// cache in provider_cache. When a provider is down, a cached response past
// its expiry is still served.
//
// Provider adapters live in subpackages (providers/tmdb, ...). They turn
// responses into plain structs and never write to the database themselves.
// See docs/agent-guides/backend.md ("Adding a provider or importer").
//
// API keys go in Options.Header, never in the URL: a key in the query would
// be stored in provider_cache as part of the cache key, and proxies and
// access logs record URLs.
package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// ErrNotFound matches (through errors.Is) a 404 from a provider.
var ErrNotFound = errors.New("not found")

const (
	defaultRatePerSecond = 20
	defaultBurst         = 20
	defaultTimeout       = 15 * time.Second

	// maxAttempts counts the first try: a request is made at most three times.
	maxAttempts = 3
	// maxRetryAfter is the longest Retry-After wait accepted. A longer one
	// fails the call instead of holding a job or a page open.
	maxRetryAfter = 30 * time.Second
	// maxBodyBytes bounds a response body. A larger one is an error, so a
	// truncated body is never cached.
	maxBodyBytes = 8 << 20
)

// Options configures a Client. Zero values mean the defaults.
type Options struct {
	// Provider names the provider in the cache, logs, and errors, such as
	// "tmdb". Required.
	Provider string
	// BaseURL is the API root, such as https://api.themoviedb.org/3. It must
	// be an absolute http or https URL with no credentials, query, or
	// fragment. Required.
	BaseURL string
	// Header is added to every request. Send API keys here.
	Header http.Header
	// RatePerSecond and Burst bound the requests the provider receives,
	// retries included. Zero means 20 and 20.
	RatePerSecond float64
	Burst         int
	// Timeout bounds each attempt, including reading its body. Zero means
	// 15 seconds.
	Timeout time.Duration
	// UserAgent is sent with every request, such as "watcht/1.2.3". Empty
	// keeps Go's default.
	UserAgent string
	// Now is the clock for cache expiry and Retry-After dates (not rate
	// limiting, which always uses the real clock). Nil means time.Now.
	Now func() time.Time
	// Logger receives request and cache logs. Nil discards them.
	Logger *slog.Logger
	// HTTPClient replaces the default client, for tests. Redirects are never
	// followed, whatever it does: a redirect to another host would resend
	// the key in Header there.
	HTTPClient *http.Client
}

// Client makes a provider's HTTP calls. It's safe for concurrent use.
type Client struct {
	provider   string
	baseURL    string
	header     http.Header
	userAgent  string
	timeout    time.Duration
	httpClient *http.Client
	now        func() time.Time
	logger     *slog.Logger
	limiter    *rateLimiter
	read       *sqlc.Queries
	write      *sqlc.Queries

	// sleep waits for a rate-limit token or before a retry, and returns
	// ctx's error if ctx ends first. Tests replace it to record waits
	// instead of sleeping.
	sleep func(ctx context.Context, d time.Duration) error
}

// New returns a Client for one provider, caching in d. It panics on options
// that can't work, such as no provider name or a BaseURL with credentials:
// those are wiring bugs.
func New(d *db.DB, opts Options) *Client {
	if opts.Provider == "" {
		panic("providers: New without a Provider name")
	}
	u, err := url.Parse(opts.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		panic(fmt.Sprintf("providers: %s BaseURL must be an absolute http or https URL", opts.Provider))
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		panic(fmt.Sprintf("providers: %s BaseURL must not have credentials, a query, or a fragment", opts.Provider))
	}

	c := &Client{
		provider:  opts.Provider,
		baseURL:   strings.TrimRight(opts.BaseURL, "/"),
		header:    opts.Header.Clone(),
		userAgent: opts.UserAgent,
		timeout:   opts.Timeout,
		now:       opts.Now,
		logger:    opts.Logger,
		read:      sqlc.New(d.Read),
		write:     sqlc.New(d.Write),
		sleep:     sleepCtx,
	}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}

	// Copy the caller's client so the redirect policy below never changes
	// it. Redirects are returned as responses, not followed.
	hc := &http.Client{}
	if opts.HTTPClient != nil {
		cp := *opts.HTTPClient
		hc = &cp
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c.httpClient = hc

	rate := opts.RatePerSecond
	if rate <= 0 {
		rate = defaultRatePerSecond
	}
	burst := opts.Burst
	if burst <= 0 {
		burst = defaultBurst
	}
	// The limiter always runs on the real clock: Now may be a test's fixed
	// clock, which would never refill the bucket.
	c.limiter = newRateLimiter(rate, burst, time.Now)
	return c
}

// StatusError is a response that isn't a usable 200. Its message names the
// provider and the path, never the query, headers, or body.
type StatusError struct {
	Provider   string
	Path       string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: GET %s returned HTTP %d", e.Provider, e.Path, e.StatusCode)
}

// Unwrap makes a 404 match ErrNotFound.
func (e *StatusError) Unwrap() error {
	if e.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	return nil
}

// transportError is a request that got no response: a dial failure, a
// timeout, or a connection cut off. It's retried, and a stale cache entry
// can stand in for it.
type transportError struct {
	provider string
	// url is the request URL without its query.
	url string
	err error
}

func (e *transportError) Error() string {
	return fmt.Sprintf("%s: GET %s: %v", e.provider, e.url, e.err)
}

func (e *transportError) Unwrap() error { return e.err }

// unwrapURLError returns the cause of a *url.Error. Its message carries the
// full URL, query included, so it must never be shown or logged as is.
func unwrapURLError(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}

// sleepCtx waits for d, or returns ctx's error if ctx ends first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
