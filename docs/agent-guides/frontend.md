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
pnpm dev                         # Vite on :5173, proxies /api to Go (WATCHT_DEV_API, default :8090)
pnpm check                       # svelte-check
pnpm lint                        # eslint + prettier --check
pnpm test                        # vitest run
pnpm build                       # output embedded by the Go build
```

`make dev` from the repository root runs the Go server on :8090 (not
8080, so it never talks to a real instance on the same machine) and Vite
on :5173 together; open :5173.
`make build` builds the frontend, copies `web/build/` into
`internal/webui/dist/`, and embeds it in `bin/watcht`. Neither build
directory is committed (only `dist/.gitkeep` is).

Node must match `.node-version` (24); `make` stops with a clear message on
another version. A version manager such as fnm switches automatically. In
a non-interactive shell where it doesn't, prefix commands with
`fnm exec --using=24 --`.

## Visual checks

Layout bugs only show in a real render, so UI changes are checked in a
browser (the `watcht-designer` agent does this with the Playwright tools).

1. Build the binary with the current frontend: `make build`.
2. Start it on a spare port (not 8080, which a real instance may use), in
   the background:
   `WATCHT_LISTEN=127.0.0.1:8099 ./bin/watcht serve &`
   and note the PID (`echo $!`); shell state doesn't persist between agent
   commands, so `kill %1` won't work later. Wait until
   `WATCHT_LISTEN=127.0.0.1:8099 ./bin/watcht healthcheck` exits 0.
3. Open `http://127.0.0.1:8099/` in the browser tools and check the pages.
4. Stop the server by PID and delete any `.playwright-mcp/`
   folder the browser tools left in the repository root.

From M1 on, start the server with a throwaway data directory under `/tmp`
so no real data is touched; from M3 on, seed sample data from recorded
fixtures rather than live TMDB calls. These steps will be added here when
those milestones land.
