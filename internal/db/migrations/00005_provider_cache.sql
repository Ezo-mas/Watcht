-- +goose Up
-- Cached provider responses (TMDB, MDBList, ...), keyed by provider and the
-- request path plus its sorted query. Rows past expires_at stay for a while
-- so the client can serve them when a provider is down; a daily job prunes
-- rows that expired long ago. See internal/providers.
CREATE TABLE provider_cache (
    provider TEXT NOT NULL,
    key TEXT NOT NULL,
    response TEXT NOT NULL
        CHECK (json_valid(response)),
    fetched_at TEXT NOT NULL
        CHECK (fetched_at IS strftime('%Y-%m-%dT%H:%M:%SZ', fetched_at)),
    expires_at TEXT NOT NULL
        CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at)),
    PRIMARY KEY (provider, key)
) STRICT;

-- The prune job deletes by expiry.
CREATE INDEX provider_cache_expires_at ON provider_cache (expires_at);
