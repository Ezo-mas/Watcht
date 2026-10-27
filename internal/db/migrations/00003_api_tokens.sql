-- +goose Up
-- API tokens for scripts and, later, media-server webhooks. Like sessions,
-- only the SHA-256 of each token is stored. A token acts as its user on the
-- data API; account management needs a browser session.
-- AUTOINCREMENT, like users: a revoked token's ID is never reused, so
-- revoking from a stale list can't hit a newer token.
CREATE TABLE api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- A label the user chose, e.g. "jellyfin". Checked in internal/auth
    -- (1-64 characters, no control characters).
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    UNIQUE (user_id, name)
) STRICT;
