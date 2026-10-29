package providers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

// testNow is a whole second, like every stored timestamp.
var testNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// fakeClock lets a test move time without sleeping. The mutex matters because
// the server and the client read it from different goroutines.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// lockedBuffer collects log output, which the client writes from several
// goroutines in the concurrency test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// reply is one scripted response.
type reply struct {
	status     int
	retryAfter string
	body       string
}

// script answers requests with its replies in order, repeating the last one.
// It counts requests so tests can check how many attempts were made.
type script struct {
	mu      sync.Mutex
	replies []reply
	calls   int
}

// set replaces the replies and resets the count.
func (s *script) set(replies ...reply) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replies = replies
	s.calls = 0
}

func (s *script) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *script) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	rep := s.replies[min(s.calls, len(s.replies)-1)]
	s.calls++
	s.mu.Unlock()
	if rep.retryAfter != "" {
		w.Header().Set("Retry-After", rep.retryAfter)
	}
	w.WriteHeader(rep.status)
	_, _ = io.WriteString(w, rep.body)
}

// okScript is a script that always answers 200 with a payload named name.
func okScript(name string) *script {
	return &script{replies: []reply{{status: http.StatusOK, body: `{"name":"` + name + `"}`}}}
}

// env is a Client on a fresh database and a fake clock, talking to a
// loopback server. Waits are recorded instead of slept, so retries and rate
// limits cost no real time.
type env struct {
	d      *db.DB
	clock  *fakeClock
	srv    *httptest.Server
	client *Client
	logs   *lockedBuffer

	mu     sync.Mutex
	sleeps []time.Duration
}

// newEnv serves h on loopback, unless h is nil (for tests that point BaseURL
// somewhere else in tweak). tweak adjusts the options before New.
func newEnv(t *testing.T, h http.Handler, tweak func(*Options)) *env {
	t.Helper()
	e := &env{d: dbtest.New(t), clock: &fakeClock{t: testNow}, logs: &lockedBuffer{}}
	opts := Options{
		Provider:      "test",
		RatePerSecond: 1000,
		Burst:         1000,
		Now:           e.clock.Now,
		Logger:        slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	if h != nil {
		e.srv = httptest.NewServer(h)
		t.Cleanup(e.srv.Close)
		opts.BaseURL = e.srv.URL
	} else {
		// No server: for tests that never make a request. The .test name is
		// reserved, so a request would fail rather than reach anyone.
		opts.BaseURL = "http://provider.test"
	}
	if tweak != nil {
		tweak(&opts)
	}
	e.client = New(e.d, opts)
	e.client.sleep = e.recordSleep
	// The limiter runs on the real clock; on the fake one its waits are
	// exact, and recorded sleeps don't move the clock anyway.
	e.client.limiter = newRateLimiter(e.client.limiter.rate, int(e.client.limiter.burst), e.clock.Now)
	return e
}

func (e *env) recordSleep(ctx context.Context, d time.Duration) error {
	e.mu.Lock()
	e.sleeps = append(e.sleeps, d)
	e.mu.Unlock()
	return ctx.Err()
}

func (e *env) waits() []time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]time.Duration(nil), e.sleeps...)
}

// payload is the JSON shape the tests decode.
type payload struct {
	Name string `json:"name"`
}

// get calls GetJSON for /movie/603 (or path) with a background context.
func (e *env) get(path string, query url.Values, ttl time.Duration) (payload, error) {
	var out payload
	err := e.client.GetJSON(context.Background(), path, query, ttl, &out)
	return out, err
}

// cached returns the stored body and expiry for key, or ok false.
func (e *env) cached(key string) (body, expires string, ok bool) {
	err := e.d.Read.QueryRowContext(context.Background(),
		`SELECT response, expires_at FROM provider_cache WHERE provider = 'test' AND key = ?`, key).
		Scan(&body, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false
	}
	if err != nil {
		panic(err)
	}
	return body, expires, true
}

// cacheRows counts every row in provider_cache.
func (e *env) cacheRows(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.d.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM provider_cache").Scan(&n); err != nil {
		t.Fatalf("count provider_cache: %v", err)
	}
	return n
}

func TestCacheHitMissAndExpiry(t *testing.T) {
	const ttl = time.Hour
	s := &script{replies: []reply{
		{status: 200, body: `{"name":"first"}`},
		{status: 200, body: `{"name":"second"}`},
	}}
	e := newEnv(t, s, nil)

	expectName := func(want string, wantCalls int) {
		t.Helper()
		got, err := e.get("/movie/603", nil, ttl)
		if err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if got.Name != want {
			t.Errorf("name = %q, want %q", got.Name, want)
		}
		if s.count() != wantCalls {
			t.Errorf("requests = %d, want %d", s.count(), wantCalls)
		}
	}

	expectName("first", 1) // miss: fetched and stored
	expectName("first", 1) // hit: no request
	e.clock.Advance(ttl - time.Second)
	expectName("first", 1) // one second before expiry: still a hit
	e.clock.Advance(time.Second)
	expectName("second", 2) // at expires_at the entry is expired: refetched

	body, _, ok := e.cached("/movie/603")
	if !ok || body != `{"name":"second"}` {
		t.Errorf("stored body = %q, %v; want the refetched response", body, ok)
	}
}

func TestTTLZeroIsNotCached(t *testing.T) {
	const ttl = time.Hour
	s := &script{replies: []reply{{status: 200, body: `{"name":"a"}`}}}
	e := newEnv(t, s, nil)

	for range 2 {
		if _, err := e.get("/movie/603", nil, 0); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
	}
	if s.count() != 2 {
		t.Errorf("requests = %d, want 2 (no caching at ttl 0)", s.count())
	}
	if n := e.cacheRows(t); n != 0 {
		t.Errorf("cache rows = %d, want 0", n)
	}

	// ttl 0 also ignores an existing entry, expired or not, and never falls
	// back to it.
	if _, err := e.get("/movie/603", nil, ttl); err != nil {
		t.Fatalf("GetJSON with ttl: %v", err)
	}
	e.clock.Advance(ttl)
	s.set(reply{status: http.StatusServiceUnavailable})
	if _, err := e.get("/movie/603", nil, 0); err == nil {
		t.Error("ttl 0 served a stale entry, want the error")
	}
}

func TestOnlyUsableResponsesAreCached(t *testing.T) {
	tests := []struct {
		name  string
		reply reply
		want  error // checked with errors.Is when set
	}{
		{name: "invalid JSON", reply: reply{status: 200, body: `<html>`}},
		{name: "empty body", reply: reply{status: 200}},
		{name: "wrong shape", reply: reply{status: 200, body: `{"name":5}`}},
		{name: "204 is not 200", reply: reply{status: 204}},
		{name: "404", reply: reply{status: 404, body: `{"status_message":"gone"}`}, want: ErrNotFound},
		{name: "400", reply: reply{status: 400, body: `{"error":"bad"}`}},
		{name: "500 is not retried", reply: reply{status: 500}},
		{name: "503 after retries", reply: reply{status: 503}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &script{replies: []reply{tt.reply}}
			e := newEnv(t, s, nil)
			_, err := e.get("/movie/603", nil, time.Hour)
			if err == nil {
				t.Fatal("GetJSON succeeded, want an error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if n := e.cacheRows(t); n != 0 {
				t.Errorf("cache rows = %d, want 0", n)
			}
		})
	}
}

func TestCacheKeyIsPathAndSortedQuery(t *testing.T) {
	s := okScript("x")
	e := newEnv(t, s, nil)

	q1 := url.Values{}
	q1.Set("query", "Alien")
	q1.Set("language", "en-US")
	if _, err := e.get("/search/movie", q1, time.Hour); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if _, _, ok := e.cached("/search/movie?language=en-US&query=Alien"); !ok {
		t.Fatal("no cache row under the sorted key /search/movie?language=en-US&query=Alien")
	}

	// The same values, set in another order, are the same entry.
	q2 := url.Values{}
	q2.Set("language", "en-US")
	q2.Set("query", "Alien")
	if _, err := e.get("/search/movie", q2, time.Hour); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if s.count() != 1 {
		t.Errorf("requests = %d, want 1: the reordered query should hit the cache", s.count())
	}
}

func TestHeaderIsSentButNeverCachedOrReported(t *testing.T) {
	const secret = "hdr-secret-value"
	header := http.Header{}
	header.Set("X-Api-Key", secret)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"name":"keyed"}`)
	})
	e := newEnv(t, h, func(o *Options) { o.Header = header })

	got, err := e.get("/movie/603", url.Values{"language": {"en-US"}}, time.Hour)
	if err != nil {
		t.Fatalf("GetJSON: %v (was the header sent?)", err)
	}
	if got.Name != "keyed" {
		t.Errorf("name = %q, want keyed", got.Name)
	}
	if _, _, ok := e.cached("/movie/603?language=en-US"); !ok {
		t.Fatal("no cache row under /movie/603?language=en-US")
	}
	if strings.Contains(e.cachedKeys(t), secret) {
		t.Errorf("cache keys contain the header value: %s", e.cachedKeys(t))
	}
}

// cachedKeys lists every key in provider_cache, for assertions that a value
// is absent.
func (e *env) cachedKeys(t *testing.T) string {
	t.Helper()
	rows, err := e.d.Read.QueryContext(context.Background(), "SELECT provider || ' ' || key FROM provider_cache ORDER BY provider, key")
	if err != nil {
		t.Fatalf("list cache keys: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan cache key: %v", err)
		}
		b.WriteString(k + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cache keys: %v", err)
	}
	return b.String()
}

// TestErrorsAndLogsOmitHeaderAndQuery makes a transport error against a closed
// server with both a header key and a query secret, and checks that neither
// shows up in the error or in the logs.
func TestErrorsAndLogsOmitHeaderAndQuery(t *testing.T) {
	const headerSecret = "hdr-secret-value"
	const querySecret = "query-secret-value"
	closed := httptest.NewServer(http.NotFoundHandler())
	base := closed.URL
	closed.Close()

	header := http.Header{}
	header.Set("X-Api-Key", headerSecret)
	e := newEnv(t, nil, func(o *Options) {
		o.BaseURL = base
		o.Header = header
	})

	var out payload
	err := e.client.GetJSON(context.Background(), "/movie/603", url.Values{"api_key": {querySecret}}, time.Hour, &out)
	if err == nil {
		t.Fatal("GetJSON against a closed server succeeded")
	}
	if strings.Contains(err.Error(), headerSecret) || strings.Contains(err.Error(), querySecret) {
		t.Errorf("error leaks a secret: %v", err)
	}
	if !strings.Contains(err.Error(), "/movie/603") {
		t.Errorf("error = %v, want the path in it", err)
	}
	if n := len(e.waits()); n != maxAttempts-1 {
		t.Errorf("waits = %d, want %d: a transport error is retried", n, maxAttempts-1)
	}
	logs := e.logs.String()
	if strings.Contains(logs, headerSecret) || strings.Contains(logs, querySecret) {
		t.Errorf("logs leak a secret:\n%s", logs)
	}
}

func TestNotFoundMatchesErrNotFound(t *testing.T) {
	s := &script{replies: []reply{{status: 404, body: `{"status_message":"not here"}`}}}
	e := newEnv(t, s, nil)

	_, err := e.get("/movie/999", nil, time.Hour)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	se, ok := errors.AsType[*StatusError](err)
	if !ok || se.StatusCode != http.StatusNotFound || se.Path != "/movie/999" || se.Provider != "test" {
		t.Errorf("error = %#v, want a StatusError for 404 at /movie/999", err)
	}
	if s.count() != 1 || len(e.waits()) != 0 {
		t.Errorf("requests = %d, waits = %v; a 404 is not retried", s.count(), e.waits())
	}
}

// TestClientErrorsAreNotRetriedOrCoveredByStale: a 4xx is about the request,
// so it's returned at once, and an expired entry isn't served for it.
func TestClientErrorsAreNotRetriedOrCoveredByStale(t *testing.T) {
	for _, status := range []int{400, 401, 403, 429} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			s := okScript("old")
			e := newEnv(t, s, nil)
			if _, err := e.get("/movie/603", nil, time.Hour); err != nil {
				t.Fatalf("seed: %v", err)
			}
			e.clock.Advance(time.Hour)
			s.set(reply{status: status})

			_, err := e.get("/movie/603", nil, time.Hour)
			se, ok := errors.AsType[*StatusError](err)
			if !ok || se.StatusCode != status {
				t.Fatalf("error = %v, want a StatusError for %d (no stale entry for a 4xx)", err, status)
			}
			// The count restarted at set: a 429 is retried, then gives up; any
			// other 4xx is tried once.
			wantCalls := 1
			if status == 429 {
				wantCalls = maxAttempts
			}
			if s.count() != wantCalls {
				t.Errorf("requests = %d, want %d", s.count(), wantCalls)
			}
		})
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	s := &script{replies: []reply{
		{status: 429, retryAfter: "7"},
		{status: 200, body: `{"name":"later"}`},
	}}
	e := newEnv(t, s, nil)

	got, err := e.get("/movie/603", nil, 0)
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if got.Name != "later" {
		t.Errorf("name = %q, want later", got.Name)
	}
	if w := e.waits(); len(w) != 1 || w[0] != 7*time.Second {
		t.Errorf("waits = %v, want [7s]", w)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	date := testNow.Add(12 * time.Second).UTC().Format(http.TimeFormat)
	s := &script{replies: []reply{
		{status: 429, retryAfter: date},
		{status: 200, body: `{"name":"later"}`},
	}}
	e := newEnv(t, s, nil)

	if _, err := e.get("/movie/603", nil, 0); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if w := e.waits(); len(w) != 1 || w[0] != 12*time.Second {
		t.Errorf("waits = %v, want [12s]", w)
	}
}

// TestRetryAfterCap: a Retry-After at the cap is waited out, and one above it
// fails the call at once, without waiting.
func TestRetryAfterCap(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter string
		wantErr    bool
	}{
		{name: "at the cap", retryAfter: "30"},
		{name: "above the cap", retryAfter: "31", wantErr: true},
		{name: "beyond any duration", retryAfter: "99999999999999999999", wantErr: true},
		{name: "HTTP date above the cap", retryAfter: testNow.Add(time.Minute).UTC().Format(http.TimeFormat), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &script{replies: []reply{
				{status: 429, retryAfter: tt.retryAfter},
				{status: 200, body: `{"name":"later"}`},
			}}
			e := newEnv(t, s, nil)

			_, err := e.get("/movie/603", nil, 0)
			if tt.wantErr {
				se, ok := errors.AsType[*StatusError](err)
				if !ok || se.StatusCode != http.StatusTooManyRequests {
					t.Fatalf("error = %v, want a StatusError for 429", err)
				}
				if s.count() != 1 || len(e.waits()) != 0 {
					t.Errorf("requests = %d, waits = %v; want one request and no wait", s.count(), e.waits())
				}
				return
			}
			if err != nil {
				t.Fatalf("GetJSON: %v", err)
			}
			if w := e.waits(); len(w) != 1 || w[0] != maxRetryAfter {
				t.Errorf("waits = %v, want [%v]", w, maxRetryAfter)
			}
		})
	}
}

func TestServerErrorsRetriedThenSucceed(t *testing.T) {
	s := &script{replies: []reply{
		{status: 503},
		{status: 502},
		{status: 200, body: `{"name":"third"}`},
	}}
	e := newEnv(t, s, nil)

	got, err := e.get("/movie/603", nil, 0)
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if got.Name != "third" || s.count() != 3 {
		t.Errorf("name = %q after %d requests, want third after 3", got.Name, s.count())
	}
	if w := e.waits(); len(w) != 2 || w[0] != 500*time.Millisecond || w[1] != time.Second {
		t.Errorf("waits = %v, want [500ms 1s]", w)
	}
}

func TestRetriesExhaustedServeStaleEntry(t *testing.T) {
	const ttl = time.Hour
	s := okScript("old")
	e := newEnv(t, s, nil)
	if _, err := e.get("/movie/603", nil, ttl); err != nil {
		t.Fatalf("seed: %v", err)
	}
	e.clock.Advance(ttl)
	s.set(reply{status: 503})

	got, err := e.get("/movie/603", nil, ttl)
	if err != nil {
		t.Fatalf("GetJSON during an outage: %v, want the stale entry", err)
	}
	if got.Name != "old" {
		t.Errorf("name = %q, want the stale entry", got.Name)
	}
	if s.count() != maxAttempts {
		t.Errorf("requests = %d, want %d", s.count(), maxAttempts)
	}
	if !strings.Contains(e.logs.String(), "serving a cached response past its expiry") {
		t.Errorf("no stale-fallback warning in the logs:\n%s", e.logs.String())
	}
}

func TestRetriesExhaustedWithoutStaleEntryIsAnError(t *testing.T) {
	s := &script{replies: []reply{{status: 503}}}
	e := newEnv(t, s, nil)

	_, err := e.get("/movie/603", nil, time.Hour)
	se, ok := errors.AsType[*StatusError](err)
	if !ok || se.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want a StatusError for 503", err)
	}
	if s.count() != maxAttempts {
		t.Errorf("requests = %d, want %d", s.count(), maxAttempts)
	}
}

// TestTransportFailureServesStaleEntry: the provider is unreachable, not just
// failing, and the expired entry still answers.
func TestTransportFailureServesStaleEntry(t *testing.T) {
	const ttl = time.Hour
	s := okScript("old")
	e := newEnv(t, s, nil)
	if _, err := e.get("/movie/603", nil, ttl); err != nil {
		t.Fatalf("seed: %v", err)
	}
	e.clock.Advance(ttl)
	e.srv.Close()

	got, err := e.get("/movie/603", nil, ttl)
	if err != nil {
		t.Fatalf("GetJSON with the provider down: %v, want the stale entry", err)
	}
	if got.Name != "old" {
		t.Errorf("name = %q, want the stale entry", got.Name)
	}
}

// TestRedirectsAreNotFollowed: a redirect would resend the key header to
// another host, so the 3xx is returned as a failure and the target is never
// contacted.
func TestRedirectsAreNotFollowed(t *testing.T) {
	var targetHits int
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		targetHits++
		mu.Unlock()
		_, _ = io.WriteString(w, `{"name":"elsewhere"}`)
	}))
	defer target.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/x")
		w.WriteHeader(http.StatusFound)
	}))
	defer primary.Close()

	header := http.Header{}
	header.Set("X-Api-Key", "hdr-secret-value")
	e := newEnv(t, nil, func(o *Options) {
		o.BaseURL = primary.URL
		o.Header = header
	})

	_, err := e.get("/movie/603", nil, 0)
	se, ok := errors.AsType[*StatusError](err)
	if !ok || se.StatusCode != http.StatusFound {
		t.Fatalf("error = %v, want a StatusError for 302", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if targetHits != 0 {
		t.Errorf("redirect target was contacted %d times, want 0", targetHits)
	}
}

func TestResponseSizeLimit(t *testing.T) {
	// A JSON string of n bytes is n+2 bytes on the wire.
	atLimit := `"` + strings.Repeat("a", maxBodyBytes-2) + `"`
	overLimit := `"` + strings.Repeat("a", maxBodyBytes-1) + `"`
	s := &script{replies: []reply{{status: 200, body: atLimit}}}
	e := newEnv(t, s, nil)

	var got string
	if err := e.client.GetJSON(context.Background(), "/at-limit", nil, time.Hour, &got); err != nil {
		t.Fatalf("a body of exactly %d bytes: %v", maxBodyBytes, err)
	}
	if len(got) != maxBodyBytes-2 {
		t.Errorf("decoded %d bytes, want %d", len(got), maxBodyBytes-2)
	}

	s.set(reply{status: 200, body: overLimit})
	var over string
	err := e.client.GetJSON(context.Background(), "/over-limit", nil, time.Hour, &over)
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("a body one byte over the limit: error = %v, want a size error", err)
	}
	if _, _, ok := e.cached("/over-limit"); ok {
		t.Error("an over-size body was cached")
	}
	if s.count() != 1 {
		t.Errorf("requests = %d, want 1: an over-size body isn't retried", s.count())
	}
}

func TestBrokenCacheStillReturnsResponse(t *testing.T) {
	e := newEnv(t, okScript("fetched"), nil)
	if _, err := e.d.Write.ExecContext(context.Background(), "DROP TABLE provider_cache"); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	got, err := e.get("/movie/603", nil, time.Hour)
	if err != nil {
		t.Fatalf("GetJSON with a broken cache: %v, want the fetched response", err)
	}
	if got.Name != "fetched" {
		t.Errorf("name = %q, want fetched", got.Name)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, "provider cache read failed") || !strings.Contains(logs, "provider cache write failed") {
		t.Errorf("want cache failures logged:\n%s", logs)
	}
}

// TestCancelDuringRequestIsNotRetried: the caller gives up mid-request. That
// ends the call at once, with no retry and no fallback.
func TestCancelDuringRequestIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	var mu sync.Mutex
	e := newEnv(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		cancel()
		<-r.Context().Done()
	}), nil)

	var out payload
	err := e.client.GetJSON(ctx, "/movie/603", nil, time.Hour, &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("requests = %d, want 1", calls)
	}
	if len(e.waits()) != 0 {
		t.Errorf("waits = %v, want none", e.waits())
	}
}

// TestBadPathIsRejected: a path with a query would put it in an error or a
// log, so it's refused before any request.
func TestBadPathIsRejected(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a bad path reached the provider")
	})
	e := newEnv(t, h, nil)
	for _, path := range []string{"movie/603", "/movie?api_key=path-secret", "/movie#frag"} {
		var out payload
		err := e.client.GetJSON(context.Background(), path, nil, time.Hour, &out)
		if err == nil {
			t.Errorf("GetJSON(%q) succeeded, want an error", path)
		}
		if err != nil && strings.Contains(err.Error(), "path-secret") {
			t.Errorf("GetJSON(%q) error echoes the path: %v", path, err)
		}
	}
}

func TestConcurrentUseIsSafe(t *testing.T) {
	e := newEnv(t, okScript("x"), nil)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 5 {
				var out payload
				q := url.Values{"page": {strconv.Itoa(j % 3)}}
				if err := e.client.GetJSON(context.Background(), "/movie/603", q, time.Hour, &out); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestDecodeErrorOmitsBodyText: json's type error for an overflowing number
// quotes the number, so the error must name the field instead.
func TestDecodeErrorOmitsBodyText(t *testing.T) {
	s := &script{replies: []reply{{status: 200, body: `{"n": 987654}`}}}
	e := newEnv(t, s, nil)
	var out struct {
		N int8 `json:"n"`
	}
	err := e.client.GetJSON(context.Background(), "/movie/603", nil, 0, &out)
	if err == nil {
		t.Fatal("GetJSON decoded 987654 into an int8, want an error")
	}
	if strings.Contains(err.Error(), "987654") || strings.Contains(e.logs.String(), "987654") {
		t.Errorf("body text leaked into the error or the logs: %v", err)
	}
	if !strings.Contains(err.Error(), "field n") {
		t.Errorf("error = %v, want it to name the field", err)
	}
}

func TestNewPanicsOnUnusableOptions(t *testing.T) {
	d := dbtest.New(t)
	tests := []struct {
		name string
		opts Options
	}{
		{name: "no provider", opts: Options{BaseURL: "https://example.test"}},
		{name: "no scheme", opts: Options{Provider: "p", BaseURL: "example.test"}},
		{name: "not http", opts: Options{Provider: "p", BaseURL: "ftp://example.test"}},
		{name: "credentials", opts: Options{Provider: "p", BaseURL: "https://user:pw@example.test"}},
		{name: "query", opts: Options{Provider: "p", BaseURL: "https://example.test/?api_key=x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%+v) did not panic", tt.opts)
				}
			}()
			New(d, tt.opts)
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := testNow
	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{in: "", want: 0, wantOK: false},
		{in: "0", want: 0, wantOK: true},
		{in: "7", want: 7 * time.Second, wantOK: true},
		{in: "-1", wantOK: false},
		{in: "soon", wantOK: false},
		{in: now.Add(12 * time.Second).UTC().Format(http.TimeFormat), want: 12 * time.Second, wantOK: true},
		{in: now.Add(-time.Minute).UTC().Format(http.TimeFormat), want: 0, wantOK: true},
	}
	for _, tt := range tests {
		got, ok := parseRetryAfter(tt.in, now)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
	if got, ok := parseRetryAfter("99999999999999999999", now); !ok || got <= maxRetryAfter {
		t.Errorf("a number beyond any Duration = %v, %v; want above the cap", got, ok)
	}
}

func TestSleepCtxReturnsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleepCtx on a cancelled context = %v, want context.Canceled", err)
	}
	if time.Since(start) > time.Minute {
		t.Error("sleepCtx waited for a cancelled context")
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleepCtx(1ms) = %v, want nil", err)
	}
}
