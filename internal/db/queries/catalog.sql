-- The catalog's writes and reads. internal/catalog owns the rules (what is
-- stored, when an item goes missing, the episode identity rule); these are
-- the single statements.
--
-- A list of TMDB IDs is passed as a JSON array, because an empty IN list
-- can't be written in SQL. An empty array makes NOT IN match every row.

-- UpsertGenre stores a TMDB genre and returns its internal ID.
-- name: UpsertGenre :one
INSERT INTO genres (tmdb_id, name)
VALUES (sqlc.arg(tmdb_id), sqlc.arg(name))
ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name
RETURNING id;

-- ClearMovieGenres removes a movie's genre links before the new set is added.
-- name: ClearMovieGenres :exec
DELETE FROM movie_genres WHERE movie_id = sqlc.arg(movie_id);

-- AddMovieGenre links a movie to a genre. A genre TMDB lists twice is linked
-- once.
-- name: AddMovieGenre :exec
INSERT INTO movie_genres (movie_id, genre_id)
VALUES (sqlc.arg(movie_id), sqlc.arg(genre_id))
ON CONFLICT DO NOTHING;

-- ClearShowGenres removes a show's genre links before the new set is added.
-- name: ClearShowGenres :exec
DELETE FROM show_genres WHERE show_id = sqlc.arg(show_id);

-- AddShowGenre links a show to a genre. A genre TMDB lists twice is linked
-- once.
-- name: AddShowGenre :exec
INSERT INTO show_genres (show_id, genre_id)
VALUES (sqlc.arg(show_id), sqlc.arg(genre_id))
ON CONFLICT DO NOTHING;

-- UpsertMovie stores a movie by its TMDB ID and returns its internal ID.
-- created_at is kept from the first store.
-- name: UpsertMovie :one
INSERT INTO movies (
    tmdb_id, imdb_id, title, original_title, original_language, overview,
    tagline, status, poster_path, backdrop_path, release_date, runtime,
    vote_average, vote_count, created_at, metadata_updated_at
) VALUES (
    sqlc.arg(tmdb_id),
    sqlc.narg(imdb_id),
    sqlc.arg(title),
    sqlc.arg(original_title),
    sqlc.arg(original_language),
    sqlc.arg(overview),
    sqlc.arg(tagline),
    sqlc.arg(status),
    sqlc.arg(poster_path),
    sqlc.arg(backdrop_path),
    sqlc.narg(release_date),
    sqlc.narg(runtime),
    sqlc.arg(vote_average),
    sqlc.arg(vote_count),
    sqlc.arg(created_at),
    sqlc.arg(metadata_updated_at)
)
ON CONFLICT (tmdb_id) DO UPDATE SET
    imdb_id = excluded.imdb_id,
    title = excluded.title,
    original_title = excluded.original_title,
    original_language = excluded.original_language,
    overview = excluded.overview,
    tagline = excluded.tagline,
    status = excluded.status,
    poster_path = excluded.poster_path,
    backdrop_path = excluded.backdrop_path,
    release_date = excluded.release_date,
    runtime = excluded.runtime,
    vote_average = excluded.vote_average,
    vote_count = excluded.vote_count,
    metadata_updated_at = excluded.metadata_updated_at
RETURNING id;

-- UpsertShow stores a show by its TMDB ID and returns its internal ID.
-- created_at is kept from the first store. The seasons are stored separately.
-- name: UpsertShow :one
INSERT INTO shows (
    tmdb_id, imdb_id, tvdb_id, name, original_name, original_language,
    overview, tagline, status, poster_path, backdrop_path, first_air_date,
    last_air_date, in_production, vote_average, vote_count, created_at,
    metadata_updated_at
) VALUES (
    sqlc.arg(tmdb_id),
    sqlc.narg(imdb_id),
    sqlc.narg(tvdb_id),
    sqlc.arg(name),
    sqlc.arg(original_name),
    sqlc.arg(original_language),
    sqlc.arg(overview),
    sqlc.arg(tagline),
    sqlc.arg(status),
    sqlc.arg(poster_path),
    sqlc.arg(backdrop_path),
    sqlc.narg(first_air_date),
    sqlc.narg(last_air_date),
    sqlc.arg(in_production),
    sqlc.arg(vote_average),
    sqlc.arg(vote_count),
    sqlc.arg(created_at),
    sqlc.arg(metadata_updated_at)
)
ON CONFLICT (tmdb_id) DO UPDATE SET
    imdb_id = excluded.imdb_id,
    tvdb_id = excluded.tvdb_id,
    name = excluded.name,
    original_name = excluded.original_name,
    original_language = excluded.original_language,
    overview = excluded.overview,
    tagline = excluded.tagline,
    status = excluded.status,
    poster_path = excluded.poster_path,
    backdrop_path = excluded.backdrop_path,
    first_air_date = excluded.first_air_date,
    last_air_date = excluded.last_air_date,
    in_production = excluded.in_production,
    vote_average = excluded.vote_average,
    vote_count = excluded.vote_count,
    metadata_updated_at = excluded.metadata_updated_at
RETURNING id;

-- GetShowID returns the internal ID of a stored show, or no rows.
-- name: GetShowID :one
SELECT id FROM shows WHERE tmdb_id = sqlc.arg(tmdb_id);

-- UpsertSeasonSummary stores a season from its show's season list. That list
-- has no overview, so an overview stored earlier is kept. A season listed
-- again is no longer missing.
-- name: UpsertSeasonSummary :exec
INSERT INTO seasons (
    show_id, tmdb_id, season_number, name, poster_path, air_date,
    episode_count, created_at
) VALUES (
    sqlc.arg(show_id),
    sqlc.arg(tmdb_id),
    sqlc.arg(season_number),
    sqlc.arg(name),
    sqlc.arg(poster_path),
    sqlc.narg(air_date),
    sqlc.arg(episode_count),
    sqlc.arg(created_at)
)
ON CONFLICT (tmdb_id) DO UPDATE SET
    show_id = excluded.show_id,
    season_number = excluded.season_number,
    name = excluded.name,
    poster_path = excluded.poster_path,
    air_date = excluded.air_date,
    episode_count = excluded.episode_count,
    missing_since = NULL;

-- UpsertSeason stores a season from its own response, with its overview, and
-- records that its episode list was stored at episodes_updated_at. It returns
-- the season's internal ID.
-- name: UpsertSeason :one
INSERT INTO seasons (
    show_id, tmdb_id, season_number, name, overview, poster_path, air_date,
    episodes_updated_at, created_at
) VALUES (
    sqlc.arg(show_id),
    sqlc.arg(tmdb_id),
    sqlc.arg(season_number),
    sqlc.arg(name),
    sqlc.arg(overview),
    sqlc.arg(poster_path),
    sqlc.narg(air_date),
    CAST(sqlc.arg(episodes_updated_at) AS TEXT),
    sqlc.arg(created_at)
)
ON CONFLICT (tmdb_id) DO UPDATE SET
    show_id = excluded.show_id,
    season_number = excluded.season_number,
    name = excluded.name,
    overview = excluded.overview,
    poster_path = excluded.poster_path,
    air_date = excluded.air_date,
    episodes_updated_at = excluded.episodes_updated_at,
    missing_since = NULL
RETURNING id;

-- MarkSeasonsMissing sets missing_since on a show's seasons that aren't in
-- tmdb_ids. COALESCE keeps the time of a season that was already missing.
-- name: MarkSeasonsMissing :exec
UPDATE seasons
SET missing_since = COALESCE(missing_since, CAST(sqlc.arg(now) AS TEXT))
WHERE show_id = sqlc.arg(show_id)
    AND tmdb_id NOT IN (SELECT value FROM json_each(CAST(sqlc.arg(tmdb_ids) AS TEXT)));

-- UpsertEpisode stores an episode by its TMDB episode ID. An episode stored
-- under another season (or show) is moved to this one, so its internal ID
-- stays the same. The season response has no external IDs, so imdb_id and
-- tvdb_id are left as they are.
-- name: UpsertEpisode :exec
INSERT INTO episodes (
    show_id, season_id, tmdb_id, season_number, episode_number, name,
    overview, still_path, air_date, runtime, vote_average, created_at,
    metadata_updated_at
) VALUES (
    sqlc.arg(show_id),
    sqlc.arg(season_id),
    sqlc.arg(tmdb_id),
    sqlc.arg(season_number),
    sqlc.arg(episode_number),
    sqlc.arg(name),
    sqlc.arg(overview),
    sqlc.arg(still_path),
    sqlc.narg(air_date),
    sqlc.narg(runtime),
    sqlc.arg(vote_average),
    sqlc.arg(created_at),
    sqlc.arg(metadata_updated_at)
)
ON CONFLICT (tmdb_id) DO UPDATE SET
    show_id = excluded.show_id,
    season_id = excluded.season_id,
    season_number = excluded.season_number,
    episode_number = excluded.episode_number,
    name = excluded.name,
    overview = excluded.overview,
    still_path = excluded.still_path,
    air_date = excluded.air_date,
    runtime = excluded.runtime,
    vote_average = excluded.vote_average,
    metadata_updated_at = excluded.metadata_updated_at,
    missing_since = NULL;

-- MarkEpisodesMissing sets missing_since on the episodes linked to a season
-- that aren't in tmdb_ids. Run it after the season's episodes are stored, so
-- the episodes in the response are already linked to the season.
-- name: MarkEpisodesMissing :exec
UPDATE episodes
SET missing_since = COALESCE(missing_since, CAST(sqlc.arg(now) AS TEXT))
WHERE season_id = sqlc.arg(season_id)
    AND tmdb_id NOT IN (SELECT value FROM json_each(CAST(sqlc.arg(tmdb_ids) AS TEXT)));

-- GetMovie returns a stored movie by TMDB ID, or no rows.
-- name: GetMovie :one
SELECT
    id, tmdb_id, imdb_id, title, original_title, original_language, overview,
    tagline, status, poster_path, backdrop_path, release_date, runtime,
    vote_average, vote_count, metadata_updated_at
FROM movies
WHERE tmdb_id = sqlc.arg(tmdb_id);

-- ListMovieGenres returns a movie's genres, by name.
-- name: ListMovieGenres :many
SELECT genres.tmdb_id, genres.name
FROM movie_genres
JOIN genres ON genres.id = movie_genres.genre_id
WHERE movie_genres.movie_id = sqlc.arg(movie_id)
ORDER BY genres.name, genres.tmdb_id;

-- GetShow returns a stored show by TMDB ID, or no rows.
-- name: GetShow :one
SELECT
    id, tmdb_id, imdb_id, tvdb_id, name, original_name, original_language,
    overview, tagline, status, poster_path, backdrop_path, first_air_date,
    last_air_date, in_production, vote_average, vote_count,
    metadata_updated_at
FROM shows
WHERE tmdb_id = sqlc.arg(tmdb_id);

-- ListShowGenres returns a show's genres, by name.
-- name: ListShowGenres :many
SELECT genres.tmdb_id, genres.name
FROM show_genres
JOIN genres ON genres.id = show_genres.genre_id
WHERE show_genres.show_id = sqlc.arg(show_id)
ORDER BY genres.name, genres.tmdb_id;

-- ListShowSeasons returns a show's seasons that TMDB still lists, by number.
-- Numbers can repeat only if TMDB's list does, so the TMDB ID breaks ties.
-- name: ListShowSeasons :many
SELECT tmdb_id, season_number, name, overview, poster_path, air_date, episode_count
FROM seasons
WHERE show_id = sqlc.arg(show_id) AND missing_since IS NULL
ORDER BY season_number, tmdb_id;

-- GetVisibleSeasonID returns the internal ID of the season a show lists with
-- this number. If two listed seasons share the number, it's the one with the
-- lowest TMDB season ID.
-- name: GetVisibleSeasonID :one
SELECT seasons.id
FROM seasons
JOIN shows ON shows.id = seasons.show_id
WHERE shows.tmdb_id = sqlc.arg(show_tmdb_id)
    AND seasons.season_number = sqlc.arg(season_number)
    AND seasons.missing_since IS NULL
ORDER BY seasons.tmdb_id
LIMIT 1;

-- ListSeasonEpisodes returns a season's episodes that TMDB still lists, by
-- episode number. Numbers can repeat while a season is being renumbered, so
-- the TMDB ID breaks ties.
-- name: ListSeasonEpisodes :many
SELECT
    tmdb_id, season_number, episode_number, name, overview, still_path,
    air_date, runtime, vote_average, imdb_id, tvdb_id
FROM episodes
WHERE season_id = sqlc.arg(season_id) AND missing_since IS NULL
ORDER BY episode_number, tmdb_id;
