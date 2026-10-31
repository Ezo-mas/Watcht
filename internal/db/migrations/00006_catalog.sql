-- +goose Up
-- The catalog, shared by every user: movies, shows, their seasons and
-- episodes, and genres. The metadata comes from TMDB, and internal/catalog
-- decides what is stored and when. See the Episode identity paragraph in
-- docs/rewrite-plan.md and the catalog section of the architecture guide.
--
-- The catalog never deletes a row. Watch history will point at movies,
-- episodes, and seasons, so an item that TMDB stops listing gets
-- missing_since instead, and that is cleared if TMDB lists it again.
--
-- Nullable columns hold values TMDB can leave unknown: NULL means unknown,
-- so it can't be mistaken for a real empty value. Text that TMDB may leave
-- empty (overview, tagline, paths) is NOT NULL DEFAULT '' instead, as the
-- tmdb package returns it.
--
-- Every TMDB ID is above 0. A response with no ID decodes to 0, and storing
-- that would merge unrelated rows under one key, so the CHECKs refuse it.

-- A genre is identified by TMDB's genre ID. Its name is display data.
CREATE TABLE genres (
    id INTEGER PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE
        CHECK (tmdb_id > 0),
    name TEXT NOT NULL
) STRICT;

-- A movie is identified by its TMDB ID. imdb_id is not unique: TMDB can have
-- two entries with the same IMDb ID, and that must not stop either from being
-- stored. Imports send several matches to review (M5), so no guess is made here.
CREATE TABLE movies (
    id INTEGER PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE
        CHECK (tmdb_id > 0),
    imdb_id TEXT,
    title TEXT NOT NULL DEFAULT '',
    original_title TEXT NOT NULL DEFAULT '',
    original_language TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    tagline TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    poster_path TEXT NOT NULL DEFAULT '',
    backdrop_path TEXT NOT NULL DEFAULT '',
    -- YYYY-MM-DD. date() returns NULL for a bad value and for NULL. A value
    -- that date() changes, such as a day that doesn't exist, fails the check.
    release_date TEXT
        CHECK (release_date IS date(release_date)),
    -- Minutes. NULL when TMDB has none, which it sends as 0 or null.
    runtime INTEGER
        CHECK (runtime IS NULL OR runtime > 0),
    vote_average REAL NOT NULL DEFAULT 0
        CHECK (vote_average >= 0),
    vote_count INTEGER NOT NULL DEFAULT 0
        CHECK (vote_count >= 0),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    -- When the TMDB data in this row was last stored. created_at stays as it
    -- was at the first store.
    metadata_updated_at TEXT NOT NULL
        CHECK (metadata_updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', metadata_updated_at))
) STRICT;

CREATE INDEX movies_imdb_id ON movies (imdb_id);

-- A show is identified by its TMDB ID. Like movies, imdb_id and tvdb_id are
-- not unique: two TMDB entries can share one.
CREATE TABLE shows (
    id INTEGER PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE
        CHECK (tmdb_id > 0),
    imdb_id TEXT,
    tvdb_id INTEGER,
    name TEXT NOT NULL DEFAULT '',
    original_name TEXT NOT NULL DEFAULT '',
    original_language TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    tagline TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    poster_path TEXT NOT NULL DEFAULT '',
    backdrop_path TEXT NOT NULL DEFAULT '',
    first_air_date TEXT
        CHECK (first_air_date IS date(first_air_date)),
    last_air_date TEXT
        CHECK (last_air_date IS date(last_air_date)),
    -- 1 while TMDB says the show is still in production. Whether it has ended
    -- is in status (Ended or Canceled), which the refresh rule reads.
    in_production INTEGER NOT NULL CHECK (in_production IN (0, 1)),
    vote_average REAL NOT NULL DEFAULT 0
        CHECK (vote_average >= 0),
    vote_count INTEGER NOT NULL DEFAULT 0
        CHECK (vote_count >= 0),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    metadata_updated_at TEXT NOT NULL
        CHECK (metadata_updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', metadata_updated_at))
) STRICT;

CREATE INDEX shows_imdb_id ON shows (imdb_id);
CREATE INDEX shows_tvdb_id ON shows (tvdb_id);

-- The genres of each title. The catalog rewrites a title's set in one
-- transaction. Deleting a title removes its links; genres are kept.
CREATE TABLE movie_genres (
    movie_id INTEGER NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres (id),
    PRIMARY KEY (movie_id, genre_id)
) STRICT;

CREATE TABLE show_genres (
    show_id INTEGER NOT NULL REFERENCES shows (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres (id),
    PRIMARY KEY (show_id, genre_id)
) STRICT;

-- A season of a show. Season 0 holds the specials. Only tmdb_id is unique.
-- The season number is display data that TMDB can reshuffle, and a unique
-- index would fail halfway through a swap of two numbers, since SQLite checks
-- each row as it is updated. The index is for lookups by number.
CREATE TABLE seasons (
    id INTEGER PRIMARY KEY,
    -- No ON DELETE action: a show with seasons can't be deleted, because
    -- watch history will point at them.
    show_id INTEGER NOT NULL REFERENCES shows (id),
    tmdb_id INTEGER NOT NULL UNIQUE
        CHECK (tmdb_id > 0),
    season_number INTEGER NOT NULL CHECK (season_number >= 0),
    name TEXT NOT NULL DEFAULT '',
    -- The show's season list doesn't include the overview, so only a season
    -- response sets it.
    overview TEXT NOT NULL DEFAULT '',
    poster_path TEXT NOT NULL DEFAULT '',
    air_date TEXT
        CHECK (air_date IS date(air_date)),
    -- TMDB's episode count, from the show's season list.
    episode_count INTEGER NOT NULL DEFAULT 0
        CHECK (episode_count >= 0),
    -- When TMDB stopped listing this season. NULL while it's listed. A missing
    -- season is hidden from the catalog.
    missing_since TEXT
        CHECK (missing_since IS strftime('%Y-%m-%dT%H:%M:%SZ', missing_since)),
    -- When this season's episode list was last stored. NULL until the first
    -- time it is.
    episodes_updated_at TEXT
        CHECK (episodes_updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', episodes_updated_at)),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
) STRICT;

CREATE INDEX seasons_show_id_season_number ON seasons (show_id, season_number);

-- An episode is identified by its TMDB episode ID, never by its numbers. The
-- internal id stays with the TMDB ID even when TMDB renumbers the episode or
-- moves it to another season, so watch history never moves to a different
-- episode. season_number and episode_number are display data.
CREATE TABLE episodes (
    id INTEGER PRIMARY KEY,
    -- No ON DELETE action, for the same reason as seasons.
    show_id INTEGER NOT NULL REFERENCES shows (id),
    season_id INTEGER NOT NULL REFERENCES seasons (id),
    tmdb_id INTEGER NOT NULL UNIQUE
        CHECK (tmdb_id > 0),
    season_number INTEGER NOT NULL CHECK (season_number >= 0),
    episode_number INTEGER NOT NULL CHECK (episode_number >= 0),
    name TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    still_path TEXT NOT NULL DEFAULT '',
    air_date TEXT
        CHECK (air_date IS date(air_date)),
    -- Minutes. NULL when TMDB has none.
    runtime INTEGER
        CHECK (runtime IS NULL OR runtime > 0),
    vote_average REAL NOT NULL DEFAULT 0
        CHECK (vote_average >= 0),
    -- Only the episode lookup returns external IDs, not the season response,
    -- so a season store leaves these as they are.
    imdb_id TEXT,
    tvdb_id INTEGER,
    -- When TMDB stopped listing this episode. NULL while it's listed.
    missing_since TEXT
        CHECK (missing_since IS strftime('%Y-%m-%dT%H:%M:%SZ', missing_since)),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    metadata_updated_at TEXT NOT NULL
        CHECK (metadata_updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', metadata_updated_at))
) STRICT;

-- Not unique, for the same reason as seasons: a renumbering can swap two
-- episodes, and the second update of the pair would fail under a unique index.
CREATE INDEX episodes_season_id_episode_number ON episodes (season_id, episode_number);
CREATE INDEX episodes_show_id ON episodes (show_id);
