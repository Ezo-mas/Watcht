-- +goose Up
ALTER TABLE movies ADD COLUMN last_viewed_at TEXT
    CHECK (last_viewed_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_viewed_at));
ALTER TABLE shows ADD COLUMN last_viewed_at TEXT
    CHECK (last_viewed_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_viewed_at));

-- Restrict refresh scans to titles whose detail pages were recently opened.
CREATE INDEX movies_last_viewed_at ON movies (last_viewed_at);
CREATE INDEX shows_last_viewed_at ON shows (last_viewed_at);

-- A listed season can still return 404; only its own response clears this mark.
ALTER TABLE seasons ADD COLUMN not_found_since TEXT
    CHECK (not_found_since IS strftime('%Y-%m-%dT%H:%M:%SZ', not_found_since));
