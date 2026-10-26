-- +goose Up
-- Sign-in: passwords, admin flag, and sessions. API tokens arrive with
-- their own migration.

-- NULL password_hash means the account can't sign in with a password (for
-- OIDC-only accounts later). The format is checked by internal/auth.
ALTER TABLE users ADD COLUMN password_hash TEXT
    CHECK (password_hash IS NULL OR password_hash GLOB '$argon2id$*');
ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0
    CHECK (is_admin IN (0, 1));

-- A session is a random token in a cookie; only its SHA-256 is stored, so
-- a leaked database or backup can't be used to sign in.
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    expires_at TEXT NOT NULL
        CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at))
) STRICT;

CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);

