# Trakt import scope

The Trakt importer (milestone M5 in [rewrite-plan.md](rewrite-plan.md)) reads a Trakt data export and loads it into the watcht schema. It covers what's needed for tracking: watchlist, watch history, and ratings.

## Input files

- `user-profile.json`
- `lists-watchlist.json`
- `watched-history-*.json` (the export splits history across numbered files)
- `ratings-movies.json`
- `ratings-shows.json`
- `ratings-episodes.json`

`watched-movies.json` and `watched-shows.json` are not imported. They hold Trakt's progress and play counts, which watcht derives from watch history instead. The importer may read them to cross-check its derived counts and report differences.

## File-to-table mapping

- `user-profile.json`: nothing is created from it. The import runs for the signed-in watcht user; the profile is read only to show which Trakt account the export belongs to.
- `lists-watchlist.json`: `movies` / `shows` (upsert by external IDs), then `watchlist_items`
- `watched-history-*.json`: `movies` / `shows` / `episodes` (upsert by external IDs), then `watch_events` with `source = 'trakt_import'` and `source_event_id` set to Trakt's history ID
- `ratings-*.json`: `movies` / `shows` / `episodes` (upsert), then `ratings`

## Import order

1. Create an `import_runs` row.
2. Upsert movies and shows from all files, matching on TMDB ID first, then IMDb or TVDB ID.
3. Resolve episodes to TMDB episode IDs and upsert them. Trakt exports include TMDB and TVDB IDs for most episodes; anything that can't be resolved goes to the review list.
4. Import watchlist items.
5. Import watch events. The unique index on (user, source, source_event_id) makes re-running the same export a no-op.
6. Import ratings last, so the latest value wins when an item appears more than once.
7. Finish the `import_runs` row with counts, unmatched items, and errors.

## Out of scope

- Comments, likes, followers/following, hidden items
- Custom lists other than the watchlist
- Notes and activity logs
- Checkins and scrobble-in-progress state

## Notes

- Trakt ratings are integers 1–10, the same scale watcht uses.
- The raw Trakt object for each event and rating is kept in its raw payload column, for troubleshooting and future mapping changes.
- Real export files used as test fixtures must be trimmed and stripped of personal data before they're committed under `testdata/`.
