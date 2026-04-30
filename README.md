# watcht

watcht is a private-first film and TV tracking app inspired by the information density and workflow speed of classic Trakt-era interfaces.

The goal is to deliver a focused personal tracker first: watchlist, watched history, show episode progress, ratings, and dashboard stats. Social and sharing features are intentionally deferred until after the private core is stable.

## What it does today

- OAuth sign-in foundation with NextAuth v5 (Google and GitHub)
- Personal dashboard with a Trakt-v2-inspired hero, live data-backed widgets, and modular block ordering
- Dedicated profile page with persisted per-user layout preferences
- Watchlist management (list, add, remove)
- Mark watched for movies and episode-level logging for shows
- Recent watched activity feed (movies and episodes)
- PostgreSQL-first schema with Drizzle data access and Trakt import-ready model coverage

## Tech stack

- Next.js App Router + React + TypeScript
- Tailwind CSS for UI styling
- NextAuth v5 for authentication
- PostgreSQL + Drizzle ORM + pg
- Redis planned for caching/queues in later phases

## Getting started

### Prerequisites

- Node.js 20+
- pnpm 9+
- PostgreSQL instance (local or managed)

### Local setup

1. Install dependencies.
   pnpm install
2. Create environment file.
   copy .env.example .env.local
3. Fill required values in .env.local (at minimum: DATABASE_URL, NEXTAUTH_URL, NEXTAUTH_SECRET).
4. Run database migrations.
   pnpm db:migrate:dev
5. Start the dev server.
   pnpm dev

### OAuth setup

OAuth works locally once the provider app settings and env vars match.

#### Google

1. Create an OAuth client in Google Cloud Console.
2. Add this redirect URI:

   `http://localhost:3000/api/auth/callback/google`

3. Copy the client ID and client secret into `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.

#### GitHub

1. Create an OAuth App in GitHub Developer Settings.
2. Add this callback URL:

   `http://localhost:3000/api/auth/callback/github`

3. Copy the client ID and client secret into `GITHUB_ID` and `GITHUB_SECRET`.

#### Required auth env vars

- `NEXTAUTH_URL=http://localhost:3000`
- `NEXTAUTH_SECRET=<strong-random-secret>`
- `AUTH_ENABLE_LOCAL_BYPASS=true` for development-only bypass sign-in

If you only want the local bypass while testing, you can leave the OAuth provider values as placeholders. If you want Google/GitHub sign-in, replace the placeholders before using the OAuth buttons.

## Environment variables

Required for local auth and data flows:

- NEXTAUTH_URL
- NEXTAUTH_SECRET
- GOOGLE_CLIENT_ID
- GOOGLE_CLIENT_SECRET
- GITHUB_ID
- GITHUB_SECRET
- DATABASE_URL
- REDIS_URL
- TMDB_API_KEY
- JOB_SECRET (optional for /api/jobs/stats-snapshot)

OAuth callback URLs for local development:

- http://localhost:3000/api/auth/callback/google
- http://localhost:3000/api/auth/callback/github

Tip: `AUTH_ENABLE_LOCAL_BYPASS=true` is optional in development and lets you sign in without OAuth while you finish provider setup.

## Scripts

- pnpm dev - start dev server on port 3000
- pnpm build - build for production
- pnpm start - run production build
- pnpm lint - run ESLint
- pnpm typecheck - run TypeScript type check
- pnpm format - format code with Prettier
- pnpm db:generate - generate new migration from schema.ts changes
- pnpm db:migrate:dev - run pending migrations (loads .env.local)
- pnpm db:migrate - run pending migrations (requires DATABASE_URL env var)
- pnpm db:studio - open Drizzle Studio for interactive schema exploration

## Database migrations

Drizzle is the source of truth for schema in this repo. 

**Files to commit:**
- `db/schema.ts` - schema definitions (edit this to make schema changes)
- `drizzle/` - generated migration files and metadata (auto-generated, always commit)

**Workflow:**
1. Edit `db/schema.ts` to define schema changes
2. Run `pnpm db:generate` to create a new migration file in `drizzle/`
3. Commit both `db/schema.ts` and the new migration file
4. Run `pnpm db:migrate:dev` (locally) or `pnpm db:migrate` (in other environments) to apply migrations

The `drizzle/` folder contains SQL files and `_journal.json` which tracks applied migrations across environments.

## API surface (current)

- /api/auth/[...nextauth]
- /api/watchlist
- /api/watch-events
- /api/tmdb/search
- /api/jobs/stats-snapshot
- /api/ratings

## Project structure

- app/ - routes, server components, and API handlers
- components/ - UI and interactive client components
- db/ - Drizzle schema, generated migration metadata, and import-scope notes
- lib/ - auth/db/env helpers and core services
- .github/ - planning docs and CI workflow

## Quality gates

CI runs on push and pull request with:

- pnpm lint
- pnpm typecheck
- pnpm build

Workflow file: .github/workflows/ci.yml

## Roadmap focus

Near-term implementation priorities:

- Dashboard and profile layout customization UI
- Ratings submission + dashboard integration
- Search/discovery integration with TMDB data enrichment
- Stats hardening and richer dashboard slices
- Trakt import/export contract tests after DB phase expansion
