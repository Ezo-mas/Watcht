-- Watch history. internal/tracking owns the rules (source idempotency,
-- dedupe, merging); these are the single statements. No other package
-- writes to watch_events.

-- WatchEventBySource finds an event already recorded from a source, so a
-- repeated webhook or re-import returns it instead of adding a row.
-- name: WatchEventBySource :one
SELECT id, merged_into_id, watched_at FROM watch_events
WHERE user_id = sqlc.arg(user_id)
  AND source = sqlc.arg(source)
  AND source_event_id = sqlc.arg(source_event_id);

-- KeptMovieWatches returns the user's kept watches of a movie with
-- watched_at in [window_start, window_end], oldest recorded first. tracking
-- picks the nearest in time.
-- name: KeptMovieWatches :many
SELECT id, watched_at FROM watch_events
WHERE user_id = sqlc.arg(user_id)
  AND movie_id = sqlc.arg(movie_id)
  AND merged_into_id IS NULL
  AND watched_at >= sqlc.arg(window_start)
  AND watched_at <= sqlc.arg(window_end)
ORDER BY id;

-- KeptEpisodeWatches is KeptMovieWatches for an episode.
-- name: KeptEpisodeWatches :many
SELECT id, watched_at FROM watch_events
WHERE user_id = sqlc.arg(user_id)
  AND episode_id = sqlc.arg(episode_id)
  AND merged_into_id IS NULL
  AND watched_at >= sqlc.arg(window_start)
  AND watched_at <= sqlc.arg(window_end)
ORDER BY id;

-- InsertWatchEvent stores one event. merged_into_id is NULL for a kept watch.
-- name: InsertWatchEvent :one
INSERT INTO watch_events (
    user_id, movie_id, episode_id, watched_at, source, source_event_id,
    payload, merged_into_id, created_at
) VALUES (
    sqlc.arg(user_id),
    sqlc.narg(movie_id),
    sqlc.narg(episode_id),
    sqlc.arg(watched_at),
    sqlc.arg(source),
    sqlc.narg(source_event_id),
    sqlc.narg(payload),
    sqlc.narg(merged_into_id),
    sqlc.arg(created_at)
)
RETURNING id;

-- MovieExists reports whether a movie is stored.
-- name: MovieExists :one
SELECT EXISTS (SELECT 1 FROM movies WHERE id = sqlc.arg(id));

-- EpisodeExists reports whether an episode is stored.
-- name: EpisodeExists :one
SELECT EXISTS (SELECT 1 FROM episodes WHERE id = sqlc.arg(id));

-- KeptWatchID returns the kept watch an event belongs to: itself, or the
-- watch it was merged into. Only the user's own events match.
-- name: KeptWatchID :one
SELECT coalesce(merged_into_id, id) FROM watch_events
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- DeleteWatch deletes a kept watch; the foreign key's ON DELETE CASCADE
-- deletes the events merged into it.
-- name: DeleteWatch :execrows
DELETE FROM watch_events
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND merged_into_id IS NULL;

-- MovieToMark returns a stored movie's internal ID and release date.
-- name: MovieToMark :one
SELECT id, release_date FROM movies WHERE tmdb_id = sqlc.arg(tmdb_id);

-- EpisodeToMark returns a stored episode's internal ID and air date.
-- name: EpisodeToMark :one
SELECT id, air_date FROM episodes WHERE tmdb_id = sqlc.arg(tmdb_id);

-- ShowToMark returns a stored show's internal ID.
-- name: ShowToMark :one
SELECT id FROM shows WHERE tmdb_id = sqlc.arg(tmdb_id);

-- VisibleSeasonToMark returns the internal ID of the season a show lists with
-- this number, by the same rule as GetVisibleSeason (lowest TMDB season ID),
-- and whether its episodes have been fetched yet.
-- name: VisibleSeasonToMark :one
SELECT seasons.id, seasons.episodes_updated_at IS NULL AS episodes_pending FROM seasons
JOIN shows ON shows.id = seasons.show_id
WHERE shows.tmdb_id = sqlc.arg(show_tmdb_id)
    AND seasons.season_number = sqlc.arg(season_number)
    AND seasons.missing_since IS NULL AND seasons.not_found_since IS NULL
ORDER BY seasons.tmdb_id
LIMIT 1;

-- UnwatchedSeasonEpisodes lists a season's listed episodes that the user has
-- no watch of, in episode order. tracking decides which have aired.
-- name: UnwatchedSeasonEpisodes :many
SELECT e.id, e.air_date FROM episodes e
WHERE e.season_id = sqlc.arg(season_id)
    AND e.missing_since IS NULL
    AND NOT EXISTS (
        SELECT 1 FROM watch_events w
        WHERE w.user_id = sqlc.arg(user_id) AND w.episode_id = e.id
    )
ORDER BY e.episode_number, e.tmdb_id;

-- ShowEpisodesPending reports whether any season UnwatchedShowEpisodes
-- covers hasn't had its episodes fetched yet.
-- name: ShowEpisodesPending :one
SELECT EXISTS (
    SELECT 1 FROM seasons s
    WHERE s.show_id = sqlc.arg(show_id)
        AND s.season_number > 0
        AND s.episodes_updated_at IS NULL
        AND s.id = (
            SELECT s2.id FROM seasons s2
            WHERE s2.show_id = s.show_id AND s2.season_number = s.season_number
                AND s2.missing_since IS NULL AND s2.not_found_since IS NULL
            ORDER BY s2.tmdb_id LIMIT 1
        )
);

-- UnwatchedShowEpisodes is UnwatchedSeasonEpisodes for every listed season
-- of a show except specials (season 0). If two listed seasons share a
-- number, only the one VisibleSeasonToMark picks counts, as on the page.
-- name: UnwatchedShowEpisodes :many
SELECT e.id, e.air_date FROM episodes e
JOIN seasons s ON s.id = e.season_id
WHERE s.show_id = sqlc.arg(show_id)
    AND s.season_number > 0
    AND s.id = (
        SELECT s2.id FROM seasons s2
        WHERE s2.show_id = s.show_id AND s2.season_number = s.season_number
            AND s2.missing_since IS NULL AND s2.not_found_since IS NULL
        ORDER BY s2.tmdb_id LIMIT 1
    )
    AND e.missing_since IS NULL
    AND NOT EXISTS (
        SELECT 1 FROM watch_events w
        WHERE w.user_id = sqlc.arg(user_id) AND w.episode_id = e.id
    )
ORDER BY s.season_number, e.episode_number, e.tmdb_id;

-- History lists a user's kept watches, newest first, with what was watched.
-- Pass the last row's watched_at and id to get the next page; the first
-- page passes a watched_at after every row and the largest id.
-- name: History :many
SELECT w.id, w.watched_at, w.source,
    m.tmdb_id AS movie_tmdb_id, m.title AS movie_title,
    m.poster_path AS movie_poster_path, m.release_date AS movie_release_date,
    m.runtime AS movie_runtime, m.vote_average AS movie_vote_average,
    e.tmdb_id AS episode_tmdb_id, e.season_number, e.episode_number,
    e.name AS episode_name, e.still_path AS episode_still_path,
    e.air_date AS episode_air_date, e.runtime AS episode_runtime, e.vote_average AS episode_vote_average,
    s.tmdb_id AS show_tmdb_id, s.name AS show_name, s.poster_path AS show_poster_path
FROM watch_events w
LEFT JOIN movies m ON m.id = w.movie_id
LEFT JOIN episodes e ON e.id = w.episode_id
LEFT JOIN shows s ON s.id = e.show_id
WHERE w.user_id = sqlc.arg(user_id)
    AND w.merged_into_id IS NULL
    AND (w.watched_at < sqlc.arg(before_watched_at)
        OR (w.watched_at = sqlc.arg(before_watched_at) AND w.id < sqlc.arg(before_id)))
ORDER BY w.watched_at DESC, w.id DESC
LIMIT sqlc.arg(row_limit);

-- MovieWatches lists a user's kept watches of a movie, newest first.
-- name: MovieWatches :many
SELECT w.id, w.watched_at, w.source FROM watch_events w
JOIN movies m ON m.id = w.movie_id
WHERE w.user_id = sqlc.arg(user_id)
    AND m.tmdb_id = sqlc.arg(movie_tmdb_id)
    AND w.merged_into_id IS NULL
ORDER BY w.watched_at DESC, w.id DESC;

-- ShowWatchedEpisodes counts a user's kept watches of each episode of a show
-- (listed or not), by TMDB episode ID.
-- name: ShowWatchedEpisodes :many
SELECT e.tmdb_id, count(*) AS plays, CAST(max(w.watched_at) AS TEXT) AS last_watched_at
FROM watch_events w
JOIN episodes e ON e.id = w.episode_id
JOIN shows s ON s.id = e.show_id
WHERE w.user_id = sqlc.arg(user_id)
    AND s.tmdb_id = sqlc.arg(show_tmdb_id)
    AND w.merged_into_id IS NULL
GROUP BY e.tmdb_id
ORDER BY e.tmdb_id;
