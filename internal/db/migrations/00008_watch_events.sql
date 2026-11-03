-- +goose Up
-- Watch history: one row per reported watch of a movie or an episode.
-- internal/tracking is the only writer; it owns source idempotency and the
-- dedupe rule (see the Deduplication section of docs/rewrite-plan.md).
--
-- A watch reported again (by another source, or the same one with a new
-- event ID) within the dedupe window is stored with merged_into_id set to
-- the watch it repeats, so nothing is lost. merged_into_id always points at
-- a kept row (one whose own merged_into_id is NULL), never at another merged
-- one. Deleting a kept row deletes the rows merged into it.
--
-- id is AUTOINCREMENT so a removed watch's ID is never reused: the SPA holds
-- event IDs, and a stale or retried remove must not hit a newer watch.

CREATE TABLE watch_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- No ON DELETE action: the catalog never deletes a row.
    movie_id INTEGER REFERENCES movies (id),
    episode_id INTEGER REFERENCES episodes (id),
    watched_at TEXT NOT NULL
        CHECK (watched_at IS strftime('%Y-%m-%dT%H:%M:%SZ', watched_at)),
    source TEXT NOT NULL
        CHECK (source IN ('manual', 'jellyfin', 'plex', 'trakt_import', 'letterboxd_import')),
    -- The source's own ID for the event, which makes re-imports and
    -- repeated webhooks idempotent. Manual marks have none; every other
    -- source must send one.
    source_event_id TEXT,
    -- The event as the source sent it, so a rule change can be re-evaluated.
    payload TEXT
        CHECK (payload IS NULL OR json_valid(payload)),
    merged_into_id INTEGER REFERENCES watch_events (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    CHECK ((movie_id IS NULL) != (episode_id IS NULL)),
    CHECK ((source = 'manual') = (source_event_id IS NULL)),
    CHECK (source_event_id IS NULL OR source_event_id != ''),
    CHECK (merged_into_id IS NULL OR merged_into_id != id)
) STRICT;

CREATE UNIQUE INDEX watch_events_source_event
    ON watch_events (user_id, source, source_event_id)
    WHERE source_event_id IS NOT NULL;

-- The dedupe lookup, and an item's watches.
CREATE INDEX watch_events_user_movie ON watch_events (user_id, movie_id, watched_at)
    WHERE movie_id IS NOT NULL;
CREATE INDEX watch_events_user_episode ON watch_events (user_id, episode_id, watched_at)
    WHERE episode_id IS NOT NULL;

-- The history feed and stats, which count each watch once.
CREATE INDEX watch_events_user_watched_at ON watch_events (user_id, watched_at)
    WHERE merged_into_id IS NULL;

-- For the cascade from a kept row to its merged rows.
CREATE INDEX watch_events_merged_into_id ON watch_events (merged_into_id);
