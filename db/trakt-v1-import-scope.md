# Trakt v1 Import Scope (Base)

This base schema intentionally covers only the files needed for MVP tracking: profile, watchlist, watch history, progress snapshots, and ratings.

## Selected input files

- user-profile.json
- lists-watchlist.json
- watched-history-1.json, watched-history-2.json, watched-history-3.json
- watched-movies.json
- watched-shows.json
- ratings-movies.json
- ratings-shows.json
- ratings-episodes.json

## File-to-table mapping

- user-profile.json
  - app_users (core identity and Trakt ids)
- lists-watchlist.json
  - movies / shows (upsert by external ids)
  - watchlist_items
- watched-history-*.json
  - movies / shows / episodes (upsert by external ids)
  - watch_events
- watched-movies.json
  - movies (upsert)
  - progress_movies
- watched-shows.json
  - shows / episodes (upsert)
  - progress_shows
  - progress_episodes
- ratings-*.json
  - movies / shows / episodes (upsert)
  - ratings

## Import order

1. Create or map app_users from user-profile.json.
2. Upsert movies and shows from watchlist/history/progress/rating payloads.
3. Upsert episodes from watched-history and watched-shows.
4. Import watchlist_items.
5. Import watch_events (preserve source_history_id for idempotency).
6. Import progress_movies, progress_shows, progress_episodes.
7. Import ratings last so latest values overwrite older duplicates.

## Out of scope for this base pass

- comments, likes, followers/following, hidden items
- custom lists other than watchlist
- notes payloads and activity logs
- full Trakt import/export parity edge cases

## Notes

- Ratings are currently modeled as integer 1-10 to align with Trakt export values.
- JSONB raw columns preserve source data for troubleshooting and future mapping expansion.
- This schema is Postgres-first and ready for migration tooling in a later step.
