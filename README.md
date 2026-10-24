# watcht

watcht is a self-hosted film and TV tracker. You run it on your own hardware, and your watch history lives in a single SQLite database you own.

It's built for one person or a household, running on the same box as the rest of your media stack. watcht keeps your watched history, ratings, watchlist, and episode progress. External services plug in around it: metadata and lists come from providers like TMDB and MDBList, and playback history can flow in from media servers like Jellyfin and Plex.

The UI favours information density and workflow speed, in the spirit of classic Trakt-era interfaces. There is no hosted version and no social layer.

## Why watcht

- **You own the data.** Your history is in a database on your disk, not behind someone else's API or paywall.
- **Simple to run.** One binary (or one container) and one data directory. No separate database server, no Redis, no cloud account.
- **Light on resources.** A small Go server that's comfortable on a NAS or Raspberry Pi next to Jellyfin.
- **Bring your sources.** Metadata, lists, and playback sync are pluggable providers rather than hard dependencies.
- **Easy to leave other trackers.** Importers are a first-class goal, so moving your history in (and out) should be painless.

## Status

watcht is being rewritten from scratch: the original Next.js + PostgreSQL prototype is being replaced by a Go server with a SvelteKit frontend and SQLite storage. The rewrite is at its first milestone, so there's nothing usable yet.

The full plan, including architecture, schema, and milestones, is in [docs/rewrite-plan.md](docs/rewrite-plan.md).

## Planned features

First release (v0.1):

- Local accounts for household members, with first-run admin setup
- Search and metadata from TMDB
- Watch history for movies and episodes, with season marking and show progress
- Watchlist and ratings
- Dashboard with customizable widget layout and stats in your time zone
- Search across your own library
- Import from a Trakt data export
- Docker image and release binaries, with automatic daily database backups and a backup before every upgrade

After v0.1:

- Jellyfin sync via webhooks and history import (a watch counts at 85% played)
- Letterboxd CSV import
- MDBList lists and aggregated ratings
- Plex sync
- Optional OIDC sign-in (Authelia, Authentik, Keycloak, ...)
- Full data export

## Architecture

- **Go server:** the standard library HTTP server provides a JSON API, webhook receivers, and a background job worker, all in one process.
- **SvelteKit frontend:** built as a static single-page app and embedded into the Go binary.
- **SQLite storage:** WAL mode, with migrations applied automatically at startup. Queries are plain SQL compiled to type-safe Go with sqlc.
- **Providers:** TMDB, MDBList, Jellyfin, Plex, and importers are adapters behind small Go interfaces, so none of them are required to keep your data.

## Tech stack

- Go, `net/http`, `log/slog`
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- sqlc for queries, goose for migrations
- argon2id password hashing, cookie sessions, optional OIDC
- SvelteKit (Svelte 5, TypeScript) in SPA mode with `adapter-static`
- Tailwind CSS

## Self-hosting (planned)

Once the first release is out, running watcht will look like this:

```yaml
services:
  watcht:
    image: ghcr.io/ezo-mas/watcht:latest
    user: "1000:1000"
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      TMDB_API_KEY: your-tmdb-key
    restart: unless-stopped
```

Set `user` to the UID and GID that own your `./data` directory (run `id` to find yours); the container doesn't support `PUID`/`PGID` variables. The image includes a built-in health check.

Release binaries for Linux, macOS, and Windows will also be available for running without Docker, with a sample systemd unit.

All state lives in the data volume. You can take a backup at any time, even while watcht is running, with `docker compose exec watcht watcht backup`. watcht writes consistent database snapshots to `/data/backups/` daily; back up that folder rather than copying the live database file.

You'll need your own free TMDB API key; see TMDB's [API getting started guide](https://developer.themoviedb.org/docs/getting-started).

## Legacy prototype

The original Next.js app has been removed. It's kept at the git tag `legacy-nextjs` for reference and isn't maintained.

## Development

watcht is mostly written by AI coding agents and reviewed by the maintainer. Instructions for agents (and humans) are in [AGENTS.md](AGENTS.md) and [docs/agent-guides/](docs/agent-guides/). Security reporting and review rules are in [SECURITY.md](SECURITY.md).

To work on it you need Go (any recent version; the Makefile fetches the pinned 1.27 toolchain), Node 24 (ideally through a version manager that reads `.node-version`), Corepack for pnpm, and a C compiler for `make generate` (sqlc uses CGO; watcht itself doesn't). Docker is only needed for the image.

```bash
make dev           # Go on :8090 and Vite on :5173 with hot reload; open :5173
make check         # everything CI runs (lint, type checks, tests, build, smoke test)
make build         # bin/watcht with the frontend embedded
make docker-smoke  # build and test the container image
```

`make run` and `make dev` keep their data in `./data` (set `DATA_DIR` to change it).

## Attribution

Metadata and images are provided by [TMDB](https://www.themoviedb.org/). This product uses the TMDB API but is not endorsed or certified by TMDB.

## License

watcht is licensed under the [GNU Affero General Public License v3.0](LICENSE). If you run a modified version of watcht as a network service, you must make your modified source available to its users.
