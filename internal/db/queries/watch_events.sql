-- Watch history. internal/tracking owns the rules (source idempotency,
-- dedupe, merging); these are the single statements. No other package
-- writes to watch_events.

-- WatchEventBySource finds an event already recorded from a source, so a
-- repeated webhook or re-import returns it instead of adding a row.
-- name: WatchEventBySource :one
SELECT id, merged_into_id FROM watch_events
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
