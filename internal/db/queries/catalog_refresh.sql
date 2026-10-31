-- name: MarkMovieViewed :execrows
UPDATE movies SET last_viewed_at = sqlc.arg(now)
WHERE tmdb_id = sqlc.arg(tmdb_id)
AND (last_viewed_at IS NULL OR last_viewed_at < sqlc.arg(cutoff));

-- name: MarkShowViewed :execrows
UPDATE shows SET last_viewed_at = sqlc.arg(now)
WHERE tmdb_id = sqlc.arg(tmdb_id)
AND (last_viewed_at IS NULL OR last_viewed_at < sqlc.arg(cutoff));

-- name: ShowNeedsEpisodes :one
SELECT EXISTS (SELECT 1 FROM seasons JOIN shows ON shows.id = seasons.show_id
WHERE shows.tmdb_id = sqlc.arg(tmdb_id)
AND seasons.missing_since IS NULL AND seasons.not_found_since IS NULL
AND seasons.episodes_updated_at IS NULL);

-- name: ListRefreshMovies :many
SELECT tmdb_id FROM movies
WHERE last_viewed_at >= sqlc.arg(viewed_cutoff)
AND metadata_updated_at < sqlc.arg(metadata_cutoff)
ORDER BY metadata_updated_at, tmdb_id LIMIT sqlc.arg(batch_size);

-- name: ListRefreshShows :many
SELECT tmdb_id FROM shows
WHERE last_viewed_at >= sqlc.arg(viewed_cutoff)
AND (metadata_updated_at < CASE WHEN status IN ('Ended', 'Canceled')
    THEN sqlc.arg(ended_cutoff) ELSE sqlc.arg(ongoing_cutoff) END
OR EXISTS (SELECT 1 FROM seasons WHERE seasons.show_id = shows.id
    AND missing_since IS NULL AND not_found_since IS NULL AND (episodes_updated_at IS NULL
    OR episodes_updated_at < CASE WHEN shows.status IN ('Ended', 'Canceled')
        THEN sqlc.arg(ended_cutoff) ELSE sqlc.arg(ongoing_cutoff) END)))
-- Touching a deleted show sorts it last so it cannot starve the batch.
-- With unfetched/stale seasons it is re-requested once per run while viewed.
ORDER BY metadata_updated_at, tmdb_id LIMIT sqlc.arg(batch_size);

-- name: TouchMovieMetadata :exec
UPDATE movies SET metadata_updated_at = sqlc.arg(now) WHERE tmdb_id = sqlc.arg(tmdb_id);

-- name: TouchShowMetadata :exec
UPDATE shows SET metadata_updated_at = sqlc.arg(now) WHERE tmdb_id = sqlc.arg(tmdb_id);


-- name: MarkSeasonNotFound :exec
UPDATE seasons SET not_found_since = sqlc.arg(now)
WHERE tmdb_id = sqlc.arg(tmdb_id) AND not_found_since IS NULL;
