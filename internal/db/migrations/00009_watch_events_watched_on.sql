-- +goose Up
-- A watch whose source gave only a calendar date (a mark at the release or
-- air date; later, date-only imports such as Letterboxd) keeps that date in
-- watched_on, and is shown and grouped by it in every time zone. watched_at
-- is then noon UTC on the date, only so the watch sorts and dedupes with
-- timed ones. NULL for a watch with a real time.
ALTER TABLE watch_events ADD COLUMN watched_on TEXT
    CHECK (watched_on IS NULL
        OR (watched_on IS date(watched_on) AND watched_at = watched_on || 'T12:00:00Z'));

-- Before this column, a release or air date mark was stored only as noon UTC
-- on the date. Mark those as dates: manual watches at exactly noon UTC on
-- the item's own release or air date. A manual watch picked at exactly
-- 12:00:00 UTC on release day is the only other match, and is marked too.
UPDATE watch_events SET watched_on = substr(watched_at, 1, 10)
WHERE source = 'manual'
    AND substr(watched_at, 11) = 'T12:00:00Z'
    AND (substr(watched_at, 1, 10) = (SELECT release_date FROM movies WHERE movies.id = watch_events.movie_id)
        OR substr(watched_at, 1, 10) = (SELECT air_date FROM episodes WHERE episodes.id = watch_events.episode_id));
