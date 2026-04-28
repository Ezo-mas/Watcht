# watcht

watcht is a private-first film and TV tracking app inspired by the information density and workflow speed of classic Trakt-era interfaces.

The goal is to deliver a focused personal tracker first: watchlist, watched history, show episode progress, ratings, and dashboard stats. Social and sharing features are intentionally deferred until after the private core is stable.

## What it does today

- OAuth sign-in foundation with NextAuth v5 (Google and GitHub)
- Personal dashboard with live data-backed widgets
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
3. Fill required values in .env.local.
4. Start the dev server.
   pnpm dev

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

## Scripts

- pnpm dev
- pnpm build
- pnpm start
- pnpm lint
- pnpm typecheck
- pnpm format
- pnpm db:generate
- pnpm db:migrate
- pnpm db:studio

## API surface (current)

- /api/auth/[...nextauth]
- /api/watchlist
- /api/watch-events
- /api/tmdb/search
- /api/jobs/stats-snapshot

## Project structure

- app/ - routes, server components, and API handlers
- components/ - UI and interactive client components
- db/ - SQL baseline + Drizzle schema
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

- Ratings submission + dashboard integration
- Search/discovery integration with TMDB data enrichment
- Stats hardening and richer dashboard slices
- Trakt import/export contract tests after DB phase expansion
