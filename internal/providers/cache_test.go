package providers

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

func TestUpsertReplacesEntry(t *testing.T) {
	e := newEnv(t, nil, nil)
	w := sqlc.New(e.d.Write)
	ctx := context.Background()
	ts := db.Timestamp(testNow)
	for _, body := range []string{`{"a":1}`, `{"a":2}`} {
		if err := w.UpsertProviderCache(ctx, sqlc.UpsertProviderCacheParams{
			Provider: "test", Key: "/k", Response: body, FetchedAt: ts, ExpiresAt: ts,
		}); err != nil {
			t.Fatalf("UpsertProviderCache: %v", err)
		}
	}
	if n := e.cacheRows(t); n != 1 {
		t.Errorf("cache rows = %d, want 1 after two upserts of one key", n)
	}
	if body, _, _ := e.cached("/k"); body != `{"a":2}` {
		t.Errorf("stored body = %q, want the second upsert", body)
	}
}

// TestCacheSchemaRejectsBadRows: the CHECKs hold even for a caller that skips
// this package.
func TestCacheSchemaRejectsBadRows(t *testing.T) {
	e := newEnv(t, nil, nil)
	w := sqlc.New(e.d.Write)
	ts := db.Timestamp(testNow)
	tests := []struct {
		name string
		p    sqlc.UpsertProviderCacheParams
	}{
		{name: "response is not JSON", p: sqlc.UpsertProviderCacheParams{Provider: "test", Key: "/k", Response: "{", FetchedAt: ts, ExpiresAt: ts}},
		{name: "timestamp not in the stored layout", p: sqlc.UpsertProviderCacheParams{Provider: "test", Key: "/k", Response: "{}", FetchedAt: "2026-10-29 12:00:00", ExpiresAt: ts}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.UpsertProviderCache(context.Background(), tt.p)
			if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Errorf("UpsertProviderCache error = %v, want a CHECK failure", err)
			}
		})
	}
}

// TestPruneKeepsEntriesExpiredLessThan30Days: stale fallback needs entries
// kept for 30 days past expiry, for every provider. An entry exactly 30 days
// past expiry is not "more than 30 days", so it stays.
func TestPruneKeepsEntriesExpiredLessThan30Days(t *testing.T) {
	e := newEnv(t, nil, nil)
	w := sqlc.New(e.d.Write)
	ctx := context.Background()
	const day = 24 * time.Hour
	entries := []struct {
		provider, key string
		expires       time.Time
	}{
		{"test", "/fresh", testNow.Add(day)},
		{"test", "/expired-1d", testNow.Add(-day)},
		{"test", "/expired-29d", testNow.Add(-29 * day)},
		{"test", "/expired-30d", testNow.Add(-30 * day)},
		{"test", "/expired-31d", testNow.Add(-31 * day)},
		{"other", "/expired-31d", testNow.Add(-31 * day)},
	}
	for _, en := range entries {
		if err := w.UpsertProviderCache(ctx, sqlc.UpsertProviderCacheParams{
			Provider:  en.provider,
			Key:       en.key,
			Response:  "{}",
			FetchedAt: db.Timestamp(en.expires.Add(-time.Hour)),
			ExpiresAt: db.Timestamp(en.expires),
		}); err != nil {
			t.Fatalf("UpsertProviderCache(%s %s): %v", en.provider, en.key, err)
		}
	}

	n, err := PruneCache(ctx, e.d, testNow)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if n != 2 {
		t.Errorf("PruneCache deleted %d rows, want 2 (31 days past, for each provider)", n)
	}
	want := "test /expired-1d\ntest /expired-29d\ntest /expired-30d\ntest /fresh\n"
	if got := e.cachedKeys(t); got != want {
		t.Errorf("remaining keys:\n%s\nwant:\n%s", got, want)
	}

	// Nothing else is old enough to go, so a second run deletes nothing.
	if n, err := PruneCache(ctx, e.d, testNow); err != nil || n != 0 {
		t.Errorf("second PruneCache = %d, %v; want 0, nil", n, err)
	}
}

// TestUndecodableEntryLeavesNothingInOut: when a fresh entry no longer
// decodes (an adapter's type changed across an upgrade), the refetch fills a
// zeroed out, so no value from the old entry survives a field the new body
// leaves out.
func TestUndecodableEntryLeavesNothingInOut(t *testing.T) {
	s := &script{replies: []reply{{status: http.StatusOK, body: `{"n":2}`}}}
	e := newEnv(t, s, nil)
	if err := sqlc.New(e.d.Write).UpsertProviderCache(context.Background(), sqlc.UpsertProviderCacheParams{
		Provider:  "test",
		Key:       "/movie/603",
		Response:  `{"name":"cached","n":999}`, // 999 overflows int8 after name is set
		FetchedAt: db.Timestamp(testNow),
		ExpiresAt: db.Timestamp(testNow.Add(time.Hour)),
	}); err != nil {
		t.Fatalf("UpsertProviderCache: %v", err)
	}

	var out struct {
		Name string `json:"name"`
		N    int8   `json:"n"`
	}
	if err := e.client.GetJSON(context.Background(), "/movie/603", nil, time.Hour, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out.Name != "" || out.N != 2 {
		t.Errorf("out = %+v, want only the fresh body's fields", out)
	}
	if n := s.count(); n != 1 {
		t.Errorf("requests = %d, want 1 refetch", n)
	}
}
