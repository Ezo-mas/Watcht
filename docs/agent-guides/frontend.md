# Frontend guide — SvelteKit SPA

The frontend lives in `web/`. It's SvelteKit (Svelte 5, TypeScript) built
with `adapter-static` in SPA mode and embedded into the Go binary.

## Rules

- **SPA only.** `ssr = false` and `prerender = false` in the root layout.
  No `+page.server.ts`, `+layout.server.ts`, form actions, or server hooks:
  there is no Node server in production. Data comes from `/api/v1` through
  the API client.
- **Svelte 5 runes** (`$state`, `$derived`, `$effect`, `$props`). Don't use
  legacy `export let` props or `$:` reactive statements.
- **API access** only through `web/src/lib/api/`. It handles JSON, the
  standard error shape, and redirecting to `/login` on `401`. Components
  don't call `fetch` directly.
- **Types** for API responses live in `web/src/lib/api/types.ts` and must
  match the Go handlers. Update both in the same change.
- **Styling** with Tailwind. Keep the dense, information-first layout;
  reuse existing components before adding new ones.
- **Images** come from the URLs the API returns. Don't hardcode TMDB image
  URLs, because the optional image cache changes them.
- **Attribution.** The TMDB notice in the footer and About page must stay.
- **Dates** come from the API as UTC RFC 3339. Format them in the user's
  time zone setting, not the browser's.

## Commands (from `web/`)

```bash
pnpm install --frozen-lockfile   # permitted
pnpm dev                         # Vite on :5173, proxies /api to Go on :8080
pnpm check                       # svelte-check
pnpm lint                        # eslint + prettier --check
pnpm test                        # vitest run
pnpm build                       # output embedded by the Go build
```

`make dev` from the repository root runs the Go server and Vite together.
Don't commit `web/build/`; the Go build produces it.

## Visual checks

Layout bugs only show in a real render, so UI changes are checked in a
browser (the `watcht-designer` agent does this with the Playwright tools).
The exact commands are added in M0, once the server and Makefile exist.
They will:

1. Build the binary with the current frontend (`make build`).
2. Start it on a spare port (not 8080, which a real instance may use) with
   a throwaway data directory under `/tmp`, so no real data is touched.
3. Seed a test user and sample data, using recorded fixtures rather than
   live TMDB calls.
4. Stop the server and delete the data directory afterwards.

Until these steps exist, report visual checks as not done.
