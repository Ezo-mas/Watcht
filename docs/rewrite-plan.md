# watcht rewrite plan

This is the plan for rewriting watcht from Next.js + PostgreSQL into a self-hosted Go + SvelteKit app backed by SQLite. It is a full rewrite: no code, ORM, or auth library is carried over. The existing app is kept only as a reference for features and UI.

## How this project is built

- **Agents write the code; the maintainer reviews it.** Changes arrive as reviewable pull requests, one milestone (or a slice of one) at a time. Code should be easy to review: small packages, plain SQL, the standard library over frameworks, and tests that show the behaviour.
- **Agent instructions live in the repo:** `AGENTS.md` at the root, detailed guides in `docs/agent-guides/`, Claude Code settings, hooks, agents (builder, designer, reviewer), and the `milestone` skill in `.claude/`, and serious deferred issues in `docs/TECH_DEBT.md`. These are kept up to date as part of each milestone, not afterwards.
- **Skills still to write:** `record-fixture` (with M3: record, trim, and anonymize a provider response) and `trakt-import-verify` (with M5: compare an import against Trakt's own counts). They're written when the code they describe exists.
- **License:** AGPL-3.0. Anyone running a modified watcht as a network service must share their changes.

## Goals

- **One binary, one data directory.** The Go server embeds the frontend and runs migrations on startup. All state lives in `/data`.
- **Runs on modest hardware** (NAS, mini PC, Raspberry Pi) next to Jellyfin/Plex, on amd64 and arm64.
- **No required third-party accounts.** Local accounts work out of the box; TMDB is the only required API key.
- **The database is the source of truth.** Providers feed data in and can be removed without losing history.
- **Easy to review.** Prefer the Go standard library and plain SQL over frameworks, so each change shows exactly what it does.

## Non-goals

- A hosted, multi-tenant service
- Social features (followers, comments, public profiles)
- Supporting databases other than SQLite
- Migrating data from the legacy Postgres database (it holds only development data; real history comes in through importers)

## Stack decisions

| Area | Choice | Why |
| --- | --- | --- |
| Language | Go (latest stable) | Single static binary, low memory, easy concurrency for webhooks and jobs |
| HTTP | `net/http` with Go 1.22+ pattern routing | No framework needed; method and path parameters are built in |
| Logging | `log/slog` | Standard library, structured |
| SQLite driver | `modernc.org/sqlite` | Pure Go, so `CGO_ENABLED=0` and trivial cross-compiles |
| Queries | `sqlc` | Write plain SQL, get type-safe Go functions generated |
| Migrations | `goose`, embedded with `go:embed` | Plain `.sql` files, applied automatically at startup |
| Passwords | `golang.org/x/crypto/argon2` (argon2id) | Current best practice for password hashing |
| OIDC (later) | `github.com/coreos/go-oidc` + `golang.org/x/oauth2` | Works with Authelia, Authentik, Keycloak, etc. |
| Frontend | SvelteKit (Svelte 5, TypeScript) with `adapter-static` in SPA mode | See below |
| Styling | Tailwind CSS | Carries over the dense UI style |
| Web tooling | pnpm, Vite, `svelte-check`, ESLint, Prettier | |
| Container | Multi-stage build to a distroless static image | Small image, no shell, runs as non-root |
| Search | SQLite FTS5 | Search your own history and watchlist without calling TMDB |

### Pinned versions

Local builds and CI must use the same tool versions:

- **Go:** the `go` and `toolchain` lines in `go.mod`, and CI reads the version from `go.mod`. Go downloads the pinned toolchain automatically, except where a distro sets `GOTOOLCHAIN=local` (Fedora does). So the Makefile exports `GOTOOLCHAIN=auto`, and every `make` target uses the pinned toolchain whatever Go is installed.
- **Go tools:** `sqlc` and `goose` pinned as `tool` directives in `go.mod` and run with `go tool sqlc` / `go tool goose`, so no global installs are needed.
- **golangci-lint:** version pinned in one place (`.golangci-version`). The Makefile runs it with `go run` at that version, and CI runs `make check`, so both use the same one.
- **Node and pnpm:** `.node-version` for Node (the major version, `24`, since distro packages and CI patch levels differ; CI reads it with `node-version-file`), `packageManager` in `web/package.json` for pnpm (Corepack locally, `pnpm/action-setup` in CI; both read that pin), and a committed `pnpm-lock.yaml` installed with `--frozen-lockfile`.
- **CI actions** pinned to exact versions.
- **Docker base images** pinned by digest.

### Why SvelteKit (SPA mode) rather than plain Svelte

Both produce static files that Go embeds, so self-hosting is identical. SvelteKit adds file-based routing, layouts, and `load` functions, which you would otherwise have to pick a separate router for. The official Svelte tutorial and docs also assume SvelteKit, which makes learning easier.

SPA mode means `ssr = false` and a fallback `index.html`. SvelteKit never runs on a server: Go serves the built files and returns `index.html` for any path that isn't `/api/*` or a static asset.

## Repository layout

```
cmd/watcht/            main.go: config, wiring, start server
internal/
  config/              env vars and flags
  db/                  open SQLite, pragmas, migrate, backup
  db/migrations/       goose .sql files (embedded)
  db/queries/          .sql query files for sqlc
  db/sqlc/             generated code (committed)
  auth/                passwords, sessions, API tokens, OIDC
  catalog/             movies, shows, episodes, ID matching
  tracking/            watch events, watchlist, ratings, progress
  jobs/                job queue worker and scheduler
  providers/
    tmdb/              metadata
    mdblist/           lists and ratings
    jellyfin/          webhooks and history import
    plex/              webhooks and history import
  importers/           Trakt export, Letterboxd CSV
  httpapi/             handlers, middleware, JSON helpers
  webui/               go:embed of the built SvelteKit app
web/                   SvelteKit project
docs/
Dockerfile
docker-compose.yml
Makefile
sqlc.yaml
```

`internal/` keeps packages private to the module. Domain packages (`catalog`, `tracking`) depend on `db/sqlc`; `httpapi` depends on the domain packages, never the other way round.

## Database design

### SQLite setup

Applied on every connection:

- `journal_mode=WAL`: readers don't block the writer
- `foreign_keys=ON`: SQLite has them off by default
- `busy_timeout=5000`: wait instead of failing when the writer is busy
- `synchronous=NORMAL`: safe with WAL and much faster

Use two `*sql.DB` handles: a write pool limited to one connection, and a read pool with several. This avoids `SQLITE_BUSY` errors entirely.

### Conventions

- Timestamps stored as `TEXT` in UTC RFC 3339 (`2026-10-24T18:30:00Z`). They stay readable in the `sqlite3` CLI and sort correctly.
- **Time zones:** everything is stored in UTC. Each user has a time zone setting (IANA name, e.g. `Europe/Vilnius`), and anything grouped by day ("watched today", streaks, stats per day or month) is computed in that zone.
- JSON stored as `TEXT` and queried with SQLite's JSON functions when needed.
- Enums are `TEXT` with a `CHECK` constraint.
- Tables are `STRICT` so SQLite enforces column types.
- Integer primary keys everywhere; user-facing IDs are the same integers (this is a single-household app, so there's no need to hide them).

### Tables

**Tables arrive with the feature that needs them** (maintainer decision, 2026-10-24): no table is created ahead of time, and each milestone adds only what its features use. The list below is the expected end state, not M1's schema.

Accounts:

- `users`: username, display name, password hash (nullable for OIDC-only users), `is_admin`, timestamps
- `sessions`: hashed session token, user, expiry, last seen, user agent
- `api_tokens`: hashed token, user, name, scopes, last used (for webhooks and scripts)
- `oidc_identities`: issuer + subject mapped to a user (added with OIDC)
- `user_settings`: time zone, plus per-user JSON for the dashboard layout (replaces the two legacy layout tables)

Catalog (shared by all users):

- `movies`, `shows`, `episodes`: core fields plus `tmdb_id`, `imdb_id`, `tvdb_id`, `trakt_id` with unique indexes.
- `seasons`: per-show season metadata (episode counts drive progress)
- `provider_cache`: provider, key, response JSON, fetched at, expires at
- `catalog_fts`: FTS5 index over titles (and original titles) of movies and shows, kept in sync by triggers

**Episode identity:** an episode's identity is its TMDB episode ID, not its season and episode number. TMDB sometimes renumbers episodes, moves them between seasons, or reorders specials (season 0) and anime, so season/episode numbers are stored as current display data and updated on metadata refresh. Watch events reference the internal episode row, which is matched by TMDB ID, so a renumbering never moves history to a different episode. Imports and media servers that only provide season/episode numbers are resolved to a TMDB episode ID at import time.

Tracking (per user):

- `watch_events`: user, movie or episode, `watched_at`, `source` (`manual`, `jellyfin`, `plex`, `trakt_import`, ...), `source_event_id`, raw payload. A unique index on (user, source, source_event_id) makes re-imports and repeated webhooks idempotent.
- `watchlist_items`: user, movie or show, rank, notes, added at. The same `CHECK` that exactly one of movie/show is set, as today.
- `ratings`: user, movie/show/episode, value 1–10, rated at. One current rating per user and item.

System:

- `jobs`: kind, payload JSON, status, run at, attempts, last error, timestamps
- `import_runs`: user, importer, status, counts, errors, started/finished at (generic replacement for `trakt_import_runs`)

### Dropped from the legacy schema

- `progress_movies`, `progress_shows`, `progress_episodes`: progress is derived from `watch_events` and `seasons` with queries. At household scale these are fast, and there's no second copy to keep in sync. Add a cache table later only if profiling shows a need.
- `stats_snapshots`: stats are computed on demand for the same reason.
- Trakt-specific columns on users: Trakt becomes an importer, not an identity.
- Community and public-profile fields (`handle`, `is_private`, `bio`, `location`, `gender`, `joined_at`, avatar URL) and the separate profile layout: watcht is self-hosted for a household, so there are no public profiles. Users have only what sign-in and the household need.

### Deduplication

A watch from Jellyfin, then the same watch from Plex, then a manual mark should count once. Rule: two events for the same user and item within a configurable window (default 3 hours) are the same watch. The first event is kept and later ones are recorded as merged, so nothing is silently lost. This lives in one function in `tracking` with thorough tests.

### Sync rules for media servers

- **What counts as a watch:** a playback event counts once playback reaches **85%** of the runtime, or the media server marks the item as played, whichever comes first. The threshold is a setting, defaulting to 85%.
- **Pauses, resumes, and stops** below the threshold are ignored. watcht only records completed watches, not in-progress playback.
- **Unmarking is not synced.** Marking an item unplayed in Jellyfin or Plex does not remove the watch in watcht. Removing history is done in watcht itself. This can be revisited later.
- Every recorded event keeps the raw payload, so a rule change can be re-evaluated against stored events.

### Migration policy

- **Before v0.1.0:** the schema may change freely. The baseline migration can be rewritten, and development databases are thrown away.
- **From v0.1.0 on:** migrations are forward-only. A released migration file is never edited or deleted; every change is a new migration.
- **Backup before migrating:** on startup, if there are pending migrations, watcht first writes a `VACUUM INTO` snapshot to `/data/backups/pre-migrate-<version>-<timestamp>.db`, then applies them. If a migration fails, startup stops and the log says which snapshot to restore.
- Downgrades are not supported. Restoring the pre-migration snapshot and running the previous version is the rollback path.

## Auth design

- **First-run setup:** if there are no users, every page redirects to `/setup`, which creates the admin account.
- **Household users:** the admin creates other accounts. No public sign-up.
- **Sessions:** a random 32-byte token in an `HttpOnly`, `SameSite=Lax` cookie (`Secure` when served over HTTPS). Only its SHA-256 hash is stored. Sliding expiry, 30 days by default.
- **CSRF:** `SameSite=Lax` plus a check that the `Origin` header matches on every non-GET request.
- **API tokens:** per-user tokens for webhooks and scripts. Stored hashed, shown once, revocable. Sent in the `Authorization` header wherever the client supports it. A token in the webhook URL is accepted only for media servers that can't send headers, and the docs explain that reverse proxy access logs will contain it.
- **No secrets in logs:** the request logger redacts the `Authorization` header, cookies, and any `token` query parameter. Provider API keys are never logged.
- **Rate limiting:** a simple in-memory limiter on login attempts.
- **OIDC (later):** optional, configured by env vars. Can auto-create users or require an existing account to link.
- **Reverse proxies:** document running behind Caddy, Traefik, or nginx, and a `WATCHT_TRUSTED_PROXIES` setting for real client IPs.

## Background jobs

- A single worker goroutine claims due jobs from the `jobs` table (`UPDATE ... RETURNING`), runs them, and records the result.
- Failed jobs retry with exponential backoff up to a max attempt count.
- A scheduler enqueues periodic jobs: metadata refresh for recently active items, provider list sync, daily database backup.
- Webhook handlers do minimal work: validate, store the raw event, enqueue a job, return `202`.

## Provider interfaces

Defined in Go as small interfaces so each integration is an adapter:

- **MetadataProvider** (TMDB): search, get movie/show/season/episode, external ID lookup
- **ListProvider** (MDBList): fetch lists and aggregated ratings
- **SyncSource** (Jellyfin, Plex): handle a webhook payload, import full play history
- **Importer** (Trakt export, Letterboxd CSV): parse a file upload into catalog upserts and watch events

All provider HTTP calls go through one client with timeouts, rate limiting, and `provider_cache`.

### Images

- **Default:** posters and backdrops are loaded directly from TMDB's image CDN. Nothing is stored locally.
- **Optional cache:** with `WATCHT_IMAGE_CACHE=true`, watcht serves images through `/api/v1/images/...` and stores them in `/data/images/`. The browser then never contacts TMDB, and images keep working if TMDB is unreachable. The cache has a size limit and evicts the least recently used files.

### Attribution

TMDB's terms require attribution. The app footer and an About page show the TMDB logo with the notice "This product uses the TMDB API but is not endorsed or certified by TMDB." The README carries the same notice. Each self-hoster uses their own TMDB API key, and the setup docs explain how to get one. Other providers (MDBList) get the same treatment when they are added, following their terms.

## HTTP API

- JSON REST under `/api/v1`. Errors use one shape: `{"error": {"code": "...", "message": "..."}}`.
- `/healthz` for container health checks.
- `/api/v1/webhooks/{provider}` for media server webhooks, authenticated by API token.
- Everything else serves the SPA.
- TypeScript types for the API are hand-written in `web/src/lib/api/` at first. If they drift, generate them from an OpenAPI spec later.

## Frontend

- SvelteKit routes: `/setup`, `/login`, `/` (dashboard), `/search`, `/movies/[id]`, `/shows/[id]`, `/history`, `/watchlist`, `/ratings`, `/settings`, `/admin/users`.
- A small `fetch` wrapper in `web/src/lib/api/` handles JSON, errors, and redirecting to `/login` on `401`.
- Port the dashboard's dense layout and block ordering from the legacy `components/` as a visual reference, rewriting them as Svelte components.

## Development workflow

- `make dev`: runs the Go server on `:8090` and Vite on `:5173`; Vite proxies `/api` to Go. Port 8090 keeps dev away from a real instance on 8080.
- `make generate`: runs `sqlc generate`.
- `make build`: builds the web app, then a Go binary that embeds it.
- `make test`: `go test ./...` and the web tests.
- `make check`: everything CI runs (lint, typecheck, tests, sqlc diff check, build, smoke test) except the Docker image build, which needs Docker and is `make docker-smoke`. This is the definition of done for agents; Docker changes also need `make docker-smoke`.

## Testing

The legacy app has no tests, and none are carried over. All tests are written fresh alongside the new code.

- **Go:** standard `testing` package, table-driven tests. Each test that needs a database gets a fresh temporary SQLite file with migrations applied. No database mocks.
- **HTTP:** handler tests through `httptest` against the real router, with a real (temporary) database.
- **Provider fixtures:** real TMDB, Jellyfin, Plex, and Trakt export payloads saved under `testdata/` in each provider package. Tests run against these files with no network access. A fixture is refreshed by re-recording it, never hand-edited to make a test pass.
- **Critical logic gets thorough tests:** dedupe, the 85% watch rule, episode identity across renumbering, time-zone day boundaries, migration backups, and auth (sessions, CSRF, token hashing).
- **Web:** Vitest for logic in `web/src/lib/`, `svelte-check` for types. Browser end-to-end tests (Playwright) are added once the core screens exist (M4).
- CI runs everything on every pull request.

## Self-host packaging

Docker is the primary distribution, since it's what Unraid, TrueNAS, Synology, Portainer, and plain compose users expect. Release binaries cover everyone else. Docker is not needed for development: `make dev` runs everything natively.

- Dockerfile: Node stage builds `web/`, Go stage builds with `CGO_ENABLED=0`, final stage is distroless static, non-root. It exists from M0 so CI catches CGO, embedding, and path problems early.
- **Health check:** distroless has no shell or `curl`, so the binary provides a `watcht healthcheck` subcommand that calls `/healthz` and exits non-zero on failure. The Dockerfile's `HEALTHCHECK` runs it.
- **File permissions:** the image runs as distroless's `nonroot` user (UID 65532) by default. Users who mount a host directory set `user: "1000:1000"` (or their own UID/GID) in compose so the data volume stays writable. linuxserver-style `PUID`/`PGID` are not supported, because they need a shell and an entrypoint script.
- Multi-arch images (`linux/amd64`, `linux/arm64`) published to GHCR from CI on tags.
- `docker-compose.yml`: one service, one `./data:/data` volume, `user:` set, `TMDB_API_KEY` env var, port 8080.
- A sample systemd unit for running the release binary without Docker.
- Deferred until someone asks: Kubernetes/Helm, Nix, AUR, and Unraid/TrueNAS community templates.
- **Backups:** don't copy the database file while watcht is running, since WAL mode can leave it inconsistent. watcht writes a consistent snapshot with `VACUUM INTO` to `/data/backups/` daily (keeping the last N) and before every migration, and `watcht backup` does the same on demand.
- Release binaries for Linux, macOS, and Windows on GitHub Releases for people not using Docker.

### Configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `WATCHT_DATA_DIR` | `/data` | Database, backups, uploads |
| `WATCHT_LISTEN` | `:8080` | Listen address |
| `WATCHT_BASE_URL` | | Public URL; needed for OIDC and secure cookies |
| `TMDB_API_KEY` | | Required |
| `MDBLIST_API_KEY` | | Optional |
| `WATCHT_TRUSTED_PROXIES` | | CIDRs of reverse proxies |
| `WATCHT_IMAGE_CACHE` | `false` | Proxy and cache TMDB images locally |
| `WATCHT_OIDC_*` | | Optional OIDC settings |

## CI

- Go: `go vet`, `golangci-lint`, `go test ./...`, and a check that `sqlc generate` leaves no diff
- Web: `svelte-check`, ESLint, `pnpm build`
- Docker build on every PR (from M0); multi-arch publish on version tags

## Milestones

Each milestone ends with something runnable and is merged to `main` before the next starts.

`main` has no working app between M0 and M4. That's accepted: the legacy app is no longer needed.

### M0: Scaffold

- Tag the last Next.js commit as `legacy-nextjs`, then remove the Next.js app, Drizzle, NextAuth, and their config and CI.
- Create the Go module, `cmd/watcht`, the SvelteKit app in SPA mode, the Makefile, and new CI, with all versions pinned.
- Update `AGENTS.md`, the agent guides, and the format hook with the real commands and paths.
- Go serves the embedded SPA and `/healthz`, and has a `watcht healthcheck` subcommand.
- Basic Dockerfile (distroless, non-root, `HEALTHCHECK`), built in CI on every PR.
- **Done when:** `make build` produces one binary that serves a "hello" page, and the Docker image does the same and reports healthy.
- **Status: done** (PRs #8, #9, #10). Both parts of "Done when" are checked on every PR: `make smoke` for the binary, and `scripts/docker-smoke.sh` for the image (CI builds it with Buildx; `make docker-smoke` runs the same check locally). Notes:
  - Base images: Node 24 and Go 1.27.3 on Debian 13 (trixie) for the build stages, `gcr.io/distroless/static-debian13:nonroot` at runtime, all pinned by digest. The image is about 18 MB.
  - The build stages run on the build machine's architecture and cross-compile Go, so the M6 multi-arch build needs no emulation.
  - The image has no `/data` volume yet; M1 adds `WATCHT_DATA_DIR` and the volume together.
  - `make check` runs everything CI runs except the Docker build, which needs Docker; that's `make docker-smoke`.

### M1: Database

- SQLite open with pragmas and read/write pools, goose migrations, sqlc setup.
- Backup-before-migrate and the `watcht backup` command (`VACUUM INTO`).
- `WATCHT_DATA_DIR` and the image's `/data` volume.
- A minimal first migration, just enough to prove the sqlc setup. Feature tables come with their milestones: accounts in M2, catalog, `provider_cache`, and `jobs` in M3, tracking tables and the FTS5 index in M4, `import_runs` in M5.
- Test helper that creates a migrated temp database.
- **Done when:** migrations apply on startup and a sample query is tested.

### M2: Auth

- Tables: `users`, `sessions`, `api_tokens`, `user_settings`.
- First-run setup, login, logout, sessions, CSRF check, login rate limit.
- Admin user management, API tokens.
- Svelte `/setup`, `/login`, and an authenticated layout.
- **Done when:** a fresh instance walks through setup and the dashboard is only reachable when signed in.

### M3: Catalog and TMDB

- Tables: `movies`, `shows`, `seasons`, `episodes`, `provider_cache`, `jobs`.
- TMDB provider with caching and rate limiting; search and detail endpoints.
- Upsert movies, shows, seasons, and episodes by external ID, with episodes anchored on TMDB episode ID.
- TMDB attribution in the footer and About page; optional image cache.
- Job worker and scheduler, with a metadata refresh job.
- **Done when:** you can search for a title and open its detail page.

### M4: Tracking parity

- Tables: `watch_events`, `watchlist_items`, `ratings`, and the `catalog_fts` index.
- Watch events with source and dedupe, episode and season marking, history feed.
- Watchlist, ratings, show progress.
- Dashboard with widgets and saved block order, with day-based stats in the user's time zone.
- Local library search with FTS5.
- **Done when:** everything the legacy app could do works in the new one.

### M5: Trakt import

Brought forward so the maintainer's real history is in watcht before v0.1, which tests the schema against real data while it can still change freely.

- Trakt export importer, building on the file mapping in `docs/trakt-import-scope.md`: watchlist, history, ratings. Progress is derived, not imported.
- Episodes resolved to TMDB episode IDs; anything unmatched goes to a review list instead of being guessed.
- Import UI with progress and an error report, tracked in `import_runs`. Re-running an import is idempotent.
- **Done when:** a real Trakt export imports cleanly and the dashboard, history, and progress match Trakt.

### M6: Packaging and first release

- Compose file, multi-arch images published to GHCR, release binaries, sample systemd unit.
- Daily backup job with retention.
- Self-hosting docs: install, file permissions, reverse proxy, backups, upgrading.
- **Done when:** `docker compose up` on a clean machine gives a working instance. Tag `v0.1.0`; the migration policy switches to forward-only.

### M7: Jellyfin

- Webhook receiver for playback events using the sync rules above, and full history import from the Jellyfin API.
- Settings page to connect a server and map Jellyfin users to watcht users.

### M8: MDBList

- List browsing and sync into the watchlist, aggregated ratings on detail pages, attribution per MDBList's terms.

### M9: Plex, Letterboxd, OIDC, export

- Plex webhooks and history import.
- Letterboxd CSV importer.
- Optional OIDC sign-in.
- Full export of a user's data as JSON (and CSV for history).

## Risks and open questions

- **ID matching:** Jellyfin, Plex, and import files don't always carry the same IDs. Fall back to TMDB lookup by IMDb/TVDB ID, and queue anything unmatched for manual review rather than guessing.
- **TMDB rate limits:** keep requests cached and refresh metadata in the background, not on page load.
- **MDBList API:** confirm current terms, limits, and endpoints before M8.
- **Scope creep:** M0–M6 is a usable v0.1. Resist starting integrations before it ships.

## Reference docs

- Go: [A Tour of Go](https://go.dev/tour/), [Effective Go](https://go.dev/doc/effective_go), and the `net/http` routing docs
- Svelte: the interactive tutorial at [svelte.dev/tutorial](https://svelte.dev/tutorial), then the SvelteKit docs on SPA mode
- [sqlc docs](https://docs.sqlc.dev/) and [goose docs](https://pressly.github.io/goose/)
