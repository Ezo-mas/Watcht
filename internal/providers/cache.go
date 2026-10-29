package providers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// staleRetention is how long an entry is kept after it expires, so it can
// still stand in while the provider is down. PruneCache deletes older ones.
const staleRetention = 30 * 24 * time.Hour

// cacheRow is a stored response and the time it stops being fresh.
type cacheRow struct {
	body      []byte
	expiresAt time.Time
}

// readCache returns the entry for key, expired or not. found is false when
// there is no entry at all.
func (c *Client) readCache(ctx context.Context, key string) (cacheRow, bool, error) {
	r, err := c.read.GetProviderCache(ctx, sqlc.GetProviderCacheParams{Provider: c.provider, Key: key})
	if errors.Is(err, sql.ErrNoRows) {
		return cacheRow{}, false, nil
	}
	if err != nil {
		return cacheRow{}, false, fmt.Errorf("read provider cache: %w", err)
	}
	expires, err := db.ParseTimestamp(r.ExpiresAt)
	if err != nil {
		return cacheRow{}, false, fmt.Errorf("read provider cache: %w", err)
	}
	return cacheRow{body: []byte(r.Response), expiresAt: expires}, true, nil
}

// writeCache stores body under key, fresh for ttl from now. Timestamps are
// whole seconds, so the expiry is rounded down.
func (c *Client) writeCache(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	now := c.now()
	err := c.write.UpsertProviderCache(ctx, sqlc.UpsertProviderCacheParams{
		Provider:  c.provider,
		Key:       key,
		Response:  string(body),
		FetchedAt: db.Timestamp(now),
		ExpiresAt: db.Timestamp(now.Add(ttl)),
	})
	if err != nil {
		return fmt.Errorf("write provider cache: %w", err)
	}
	return nil
}

// PruneCache deletes the cache entries that expired more than 30 days before
// now, for every provider, and returns how many. The provider_cache_prune job
// calls it daily.
func PruneCache(ctx context.Context, d *db.DB, now time.Time) (int64, error) {
	n, err := sqlc.New(d.Write).DeleteExpiredProviderCache(ctx, db.Timestamp(now.Add(-staleRetention)))
	if err != nil {
		return 0, fmt.Errorf("prune provider cache: %w", err)
	}
	return n, nil
}
