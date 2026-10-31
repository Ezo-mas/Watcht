# Tech Debt
Serious, confirmed, deferred issues only. See `AGENTS.md` for logging rules.

Entry format:

### Short title
- **Where:** `path` (`function`)
- **Problem:** What is wrong
- **Impact:** What breaks, and when
- **Evidence:** How it was confirmed
- **Logged:** YYYY-MM-DD

## Open

### A repeated season number hides one of the seasons
- **Where:** `internal/catalog` (`SyncShow`, `SeasonEpisodes`)
- **Problem:** TMDB's season endpoint is addressed by season number (`/tv/{id}/season/{n}`). If a show's season list ever has two seasons with the same number, `SyncShow` fetches `/season/n` twice and gets the same season both times, so the other season's episodes are never stored. `SeasonEpisodes(show, n)` resolves to the lower TMDB season ID, which may be the season that was never fetched.
- **Impact:** For such a show, one season shows no or stale episodes, and progress for it is wrong. It only happens when TMDB's own data repeats a number; no real case has been seen.
- **Evidence:** Found by the review of M3 slice 4a, by reading `SyncShow` against the duplicate-number rule.
- **Logged:** 2026-10-31
