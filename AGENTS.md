# AGENTS.md — guide for AI coding agents

Read this before editing. Detailed guides live in `docs/agent-guides/`
(`architecture.md`, `backend.md`, `frontend.md`, `testing.md`). The design
and milestones are in `docs/rewrite-plan.md`; security-sensitive areas are
listed in `SECURITY.md`; deferred issues go in `docs/TECH_DEBT.md`.

## Current state

watcht is being rewritten from Next.js + PostgreSQL to a Go server with an
embedded SvelteKit SPA and SQLite. Until milestone M0 lands, the repository
still contains the legacy Next.js app (`app/`, `components/`, `lib/`, `db/`,
`drizzle/`). It is reference material only: don't modify it, don't fix it,
and don't port its code line by line. M0 deletes it.

## Behavior rules

- **Never commit or push unless the user explicitly asks.** Leave changes
  in the working tree and report what changed.
- **Follow the plan.** `docs/rewrite-plan.md` records decisions the
  maintainer has already made (stack, schema conventions, sync rules,
  migration policy, milestones). If a task seems to need a different
  decision, stop and ask instead of deviating.
- **Ask before changing**: a migration file that has been released (from
  v0.1.0 on), the API error shape or URL versioning, the auth and session
  design, `LICENSE`, or a tool version once it has been pinned.
- **Stay in scope.** Do what was asked; don't refactor adjacent code,
  reformat files, or "improve" unrelated comments. Preserve user changes
  already in the working tree. Report other issues you spot instead of
  fixing them silently.
- **The maintainer reviews, rather than writes, most code.** Keep changes
  small enough to review, explain non-obvious decisions in the summary, and
  prefer the standard library and plain SQL over new dependencies.
- **Dependencies.** Adding a module or package named in the plan's stack
  table (`docs/rewrite-plan.md`) is fine with `go get` or `pnpm add`. Any
  other dependency: ask first, and give the reason in your summary.
- **Report test results honestly.** A skip is not a pass: name each skipped
  test and its reason. Don't weaken assertions, delete tests, or silence
  errors to make a check pass.
- If a command fails because of missing tooling (Go, Node, Corepack), say
  so and stop instead of working around it. Permitted installs beyond the
  dependency rule above: `go mod download`, and
  `pnpm install --frozen-lockfile` from `web/`. Nothing global.
- **Log serious deferred issues in `docs/TECH_DEBT.md`.** Only confirmed,
  out-of-scope issues: correctness or data integrity, security, realistic
  growth, or design flaws that make future changes much riskier. No style
  nits or speculation. Read the file first and extend an existing entry
  rather than duplicating it. Mention any entry you add in your summary.

## Definition of done

Before reporting done:

- Run `make check` from the repository root. It runs everything CI runs:
  Go vet, golangci-lint, Go tests, the sqlc diff check, and the web lint,
  type check, and tests. While iterating, run only the touched package
  (`go test ./internal/tracking/...`) and run `make check` once at the end.
- **Schema changes:** add a new migration file, run `make generate`, and
  include the regenerated sqlc code. Never hand-edit `internal/db/sqlc/`.
- **Security-sensitive changes** (the list in `SECURITY.md`): a security
  review is required. Run `/security-review` if you can; otherwise say it
  still needs running.
- **New provider or importer code:** tests run against recorded fixtures
  in `testdata/`, never the network.
- Report failures honestly, including which check failed and why. Don't
  report "done" with a failing or unrun check.
- Full command reference: [testing guide](docs/agent-guides/testing.md).

## What this project is

watcht is a self-hosted film and TV tracker for one person or a household.
It keeps watch history, ratings, watchlist, and episode progress in a single
SQLite database. TMDB supplies metadata; MDBList, Jellyfin, Plex, and file
importers plug in as providers. It ships as one Go binary (and a Docker
image) with the frontend embedded.

Request path: `browser (SvelteKit SPA) → /api/v1 (Go net/http) → domain
packages (catalog, tracking, auth) → sqlc queries → SQLite`. Webhooks and
imports go through the `jobs` table and a background worker.

## Guides: what to read for each task type

- [Architecture guide](docs/agent-guides/architecture.md): **mandatory
  reading before** touching `internal/db/`, `internal/tracking/`,
  `internal/auth/`, `internal/jobs/`, migrations, or the HTTP API shape.
- [Backend guide](docs/agent-guides/backend.md): Go layout, conventions,
  and how to add a query, migration, endpoint, or provider.
- [Frontend guide](docs/agent-guides/frontend.md): SvelteKit SPA layout and
  conventions.
- [Testing guide](docs/agent-guides/testing.md): commands, fixtures, and
  caveats.
- [SECURITY.md](SECURITY.md): changes that need a security review.

## Agents and skills

- **`milestone` skill** (`.claude/skills/milestone/`): the routine for
  working any milestone or slice of one. Use it whenever the work belongs
  to a milestone in the plan.
- **`watcht-builder`**: well-scoped implementation pieces with a clear brief.
- **`watcht-designer`**: UI layout, styling, and visual checks in `web/`.
- **`watcht-reviewer`**: independent review before reporting a milestone
  slice done.

## Non-negotiables (summary; details in architecture.md)

1. **SQLite is the only database**, opened only through `internal/db` with
   the required pragmas. All writes go through the single-connection write
   pool.
2. **One process, no new services.** No Redis, no external database, no
   sidecar. Background work uses the `jobs` table. Builds use
   `CGO_ENABLED=0`.
3. **Timestamps are stored in UTC** as RFC 3339 text. Anything grouped by
   day uses the user's time zone setting.
4. **Episodes are identified by TMDB episode ID**, never by season and
   episode number alone.
5. **Watch events are created only through `internal/tracking`**, which
   owns deduplication and the 85% watch rule. No other package inserts into
   `watch_events`.
6. **Secrets are hashed at rest and never logged**: session tokens, API
   tokens, passwords, provider API keys.
7. **The API lives under `/api/v1`** and every error uses the shape
   `{"error": {"code": "...", "message": "..."}}`.
8. **Tests never touch the network.** Providers are tested against
   fixtures in `testdata/`.
