package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type withoutStaleKey struct{}

// WithoutStale disables expired-cache fallback for calls made with the returned
// context. Fresh cache entries are still used. Background metadata writes use
// this so an outage cannot make old data appear freshly fetched.
func WithoutStale(ctx context.Context) context.Context {
	return context.WithValue(ctx, withoutStaleKey{}, true)
}

// GetJSON fetches BaseURL+path with query and decodes the JSON body into out,
// which should be a fresh value. ttl is how long a 200 response is cached;
// zero turns the cache off for the call (no read, no write, and no stale
// fallback).
//
// The cache key is the path followed by the query, encoded with its keys
// sorted, so it reads well in the sqlite3 CLI. A fresh entry is served with
// no request. When the request fails after its retries (a transport error or
// a 5xx), an expired entry is served instead, with a warning, unless ctx was
// wrapped with WithoutStale.
//
// Errors: a 404 matches ErrNotFound, and other failures are a *StatusError or
// a transport error. None of them include the query, headers, or body.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, ttl time.Duration, out any) error {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#") {
		// The path isn't echoed: a query pushed into it could hold a key.
		return fmt.Errorf("%s: path must start with / and have no query; pass query values as query", c.provider)
	}
	key := path
	if q := query.Encode(); q != "" {
		key += "?" + q
	}

	var stale *cacheRow
	if ttl > 0 {
		row, found, err := c.readCache(ctx, key)
		switch {
		case err != nil:
			c.logger.Warn("provider cache read failed; requesting without the cache", "provider", c.provider, "path", path, "err", err)
		case found && c.now().Before(row.expiresAt):
			derr := json.Unmarshal(row.body, out)
			if derr == nil {
				c.logger.Debug("provider cache hit", "provider", c.provider, "path", path)
				return nil
			}
			c.logger.Warn("cached response doesn't decode; requesting again", "provider", c.provider, "path", path, "err", c.decodeError(path, derr))
			// The failed decode may have filled part of out, and a field the
			// fresh body leaves out (or sends as null) would keep that value.
			if v := reflect.ValueOf(out); v.Kind() == reflect.Pointer && !v.IsNil() {
				v.Elem().SetZero()
			}
		case found:
			stale = &row
		}
		c.logger.Debug("provider cache miss", "provider", c.provider, "path", path, "expired", stale != nil)
	}

	body, err := c.fetch(ctx, path, key)
	if err != nil {
		if stale != nil && staleOK(err) && ctx.Value(withoutStaleKey{}) != true {
			c.logger.Warn("provider unavailable; serving a cached response past its expiry", "provider", c.provider, "path", path, "err", err)
			if err := json.Unmarshal(stale.body, out); err != nil {
				return c.decodeError(path, err)
			}
			return nil
		}
		return err
	}
	if !json.Valid(body) {
		// No decoder message: its syntax errors quote a byte of the body.
		return fmt.Errorf("%s: GET %s: response is not valid JSON", c.provider, path)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return c.decodeError(path, err)
	}
	// Cached only once it decodes: an entry the adapter can't read would fail
	// on every hit.
	if ttl > 0 {
		if err := c.writeCache(ctx, key, body, ttl); err != nil {
			c.logger.Warn("provider cache write failed; the response is not cached", "provider", c.provider, "path", path, "err", err)
		}
	}
	return nil
}

// fetch makes the request, retrying as the policy allows, and returns the body
// of the 200 response. Its errors are a *StatusError, a *transportError, or a
// plain error (the context ended, or the body was too large).
func (c *Client) fetch(ctx context.Context, path, key string) ([]byte, error) {
	for n := 1; ; n++ {
		// Every attempt, retries included, takes a rate-limit token first. A
		// wait the context cuts short gives its token back.
		if wait := c.limiter.reserve(); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				c.limiter.refund()
				return nil, fmt.Errorf("%s: GET %s: %w", c.provider, path, err)
			}
		}
		body, retry, wait, err := c.attempt(ctx, path, key, n)
		if err == nil {
			return body, nil
		}
		if !retry || n == maxAttempts {
			return nil, err
		}
		c.logger.Warn("provider request failed; retrying", "provider", c.provider, "path", path, "attempt", n, "wait", wait, "err", err)
		if wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return nil, fmt.Errorf("%s: GET %s: %w", c.provider, path, err)
			}
		}
	}
}

// attempt makes one request. On failure, retry says whether another attempt
// may succeed, and wait is the pause before it.
func (c *Client) attempt(ctx context.Context, path, key string, n int) ([]byte, bool, time.Duration, error) {
	start := time.Now()
	res, err := c.send(ctx, path, key)
	c.logger.Debug("provider request", "provider", c.provider, "path", path, "status", res.status, "attempt", n, "duration", time.Since(start))
	if err == nil && res.status == http.StatusOK {
		return res.body, false, 0, nil
	}
	if ctx.Err() != nil {
		return nil, false, 0, fmt.Errorf("%s: GET %s: %w", c.provider, path, ctx.Err())
	}
	if err != nil {
		if _, ok := errors.AsType[*transportError](err); ok {
			return nil, true, backoff(n), err
		}
		return nil, false, 0, err
	}
	switch res.status {
	case http.StatusTooManyRequests:
		serr := c.statusError(path, res.status)
		wait, ok := parseRetryAfter(res.retryAfter, c.now())
		switch {
		case !ok:
			return nil, true, backoff(n), serr
		case wait > maxRetryAfter:
			return nil, false, 0, serr
		default:
			return nil, true, wait, serr
		}
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return nil, true, backoff(n), c.statusError(path, res.status)
	default:
		return nil, false, 0, c.statusError(path, res.status)
	}
}

// response is what one request returned. body is set only for a 200.
type response struct {
	status     int
	retryAfter string
	body       []byte
}

// send makes one request under the per-attempt timeout, which also covers
// reading the body. A failure to get a response is a *transportError; a body
// over maxBodyBytes is a plain error.
func (c *Client) send(ctx context.Context, path, key string) (response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+key, nil)
	if err != nil {
		return response{}, fmt.Errorf("%s: GET %s: %w", c.provider, path, unwrapURLError(err))
	}
	req.Header = c.header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return response{}, c.transportFailure(path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	res := response{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After")}
	if resp.StatusCode != http.StatusOK {
		return res, nil
	}
	// Read one byte past the limit, so an over-size body is detected and
	// never cached truncated.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return response{}, c.transportFailure(path, err)
	}
	if len(body) > maxBodyBytes {
		return response{}, fmt.Errorf("%s: GET %s: response is larger than %d bytes", c.provider, path, maxBodyBytes)
	}
	res.body = body
	return res, nil
}

// transportFailure reports a request that got no response. The URL in it
// has no query, so it's safe to show.
func (c *Client) transportFailure(path string, err error) error {
	return &transportError{provider: c.provider, url: c.baseURL + path, err: unwrapURLError(err)}
}

func (c *Client) statusError(path string, status int) error {
	return &StatusError{Provider: c.provider, Path: path, StatusCode: status}
}

// decodeError reports a valid JSON body that doesn't fit out. It names the
// field and the Go type, never json's text: a type error can quote a number
// from the body, such as one that overflows.
func (c *Client) decodeError(path string, err error) error {
	ute, ok := errors.AsType[*json.UnmarshalTypeError](err)
	switch {
	case ok && ute.Field != "":
		return fmt.Errorf("%s: GET %s: response field %s is not a %s", c.provider, path, ute.Field, ute.Type)
	case ok:
		return fmt.Errorf("%s: GET %s: response is not a %s", c.provider, path, ute.Type)
	default:
		return fmt.Errorf("%s: GET %s: response doesn't match the expected shape", c.provider, path)
	}
}

// staleOK reports whether an expired cache entry may stand in for a failed
// fetch: the provider was unreachable or failed on its side. A 4xx is about
// the request, so it's never covered.
func staleOK(err error) bool {
	if se, ok := errors.AsType[*StatusError](err); ok {
		return se.StatusCode >= http.StatusInternalServerError
	}
	_, ok := errors.AsType[*transportError](err)
	return ok
}

// backoff is the pause after attempt n fails, before the next one: half a
// second, then a second.
func backoff(n int) time.Duration {
	if n <= 1 {
		return 500 * time.Millisecond
	}
	return time.Second
}

// parseRetryAfter reads a Retry-After value, which is a number of seconds or
// an HTTP date. ok is false when the value is empty or unreadable. A number
// too large for a Duration counts as the largest one, so it's over any cap.
func parseRetryAfter(v string, now time.Time) (wait time.Duration, ok bool) {
	if v == "" {
		return 0, false
	}
	if strings.Trim(v, "0123456789") == "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs > int64(math.MaxInt64/time.Second) {
			return math.MaxInt64, true
		}
		return time.Duration(secs) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	return max(t.Sub(now), 0), true
}
