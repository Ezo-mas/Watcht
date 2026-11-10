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
- **API access** only through `web/src/lib/api/`: `api()` in `client.ts`
  sends JSON with same-origin cookies and throws an `ApiError` (`status`,
  `code`, `message` from the standard error shape; status 0 when the
  server can't be reached). A 401 `unauthenticated` re-runs the session
  check, which redirects to `/login`; other 401s (a wrong password) are
  just errors. Components don't call `fetch` directly.
- **Auth routing** lives in the root `+layout.ts` (`load` calls
  `getSession` and `guard` from `web/src/lib/guard.ts`). Signed-in pages
  go under `src/routes/(app)/`. After a sign-in or sign-out, navigate with
  `goto(target, { invalidate: ['app:session'] })` so the guard runs again
  against the new session (read anything you need from the current URL
  first). Never redirect to a `?next=` value without `safeNext()`. If the
  session check can't reach the server, the root layout shows a "can't
  reach watcht" card itself: an error page would re-run the same load.
- **Page data** comes from a `+page.ts` `load` that calls `#lib/api` with
  the `fetch` it's given and declares a key with `depends('app:<name>')`
  (`app:tokens`, `app:users`); after a change, `invalidate` that key to
  reload. Admin-only pages check `session.user.is_admin` from `parent()`
  and redirect; the server refuses non-admins anyway. An error a signed-in
  page's `load` throws (`error(404, …)`) renders `(app)/+error.svelte`,
  inside the header and footer.
- **Imports and links.** `#lib/...` is a package subpath import, so it
  needs the file extension (`#lib/api/client.js`). Internal links and
  `goto` go through `resolve()` from `$app/paths` (lint enforces it): a
  leading `/` is a route ID (`resolve('/(app)')` is the dashboard),
  anything else a pathname (`resolve('history')`).
- **Types** for API responses live in `web/src/lib/api/types.ts` and must
  match the Go handlers. Update both in the same change.
- **Styling** with Tailwind, following [DESIGN.md](../../DESIGN.md) (the
  design system: tokens, type, layout, title page anatomy, states). Keep
  the dense, information-first layout; reuse existing components before
  adding new ones.
- **Images** come from the URLs the API returns. Don't hardcode TMDB image
  URLs, because the optional image cache changes them.
- **Attribution.** TMDB's full notice and logo on the About page
  (`TmdbAttribution`) and the short TMDB credit in the footer
  (`AppFooter`) must stay; see DESIGN.md.
- **Dates** come from the API as UTC RFC 3339. Format calendar dates
  (release and air dates) with `formatDate`/`formatDay`
  (`#lib/format.js`). Show a watch with `formatWatch` and group it with
  `watchDay`: a watch with `watched_on` (a mark at the release or air
  date) is that date in every zone, and its `watched_at` only orders it.
  Other watch times use `formatDateTime`, `formatTime`, and `dayKey`, in
  the browser's time zone until M4 slice 4 adds the user's setting
  (maintainer decision, 2026-11-03); pass a zone rather than formatting
  times anywhere else.

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

1. Build the fake TMDB, which serves the recorded fixtures in
   `internal/providers/tmdb/testdata/` on 127.0.0.1:8098, into a temporary
   directory, and start that binary in the background from the repository
   root, noting its PID:
   `tmp=$(mktemp -d); go build -o "$tmp/fake-tmdb" ./scripts/fake-tmdb; "$tmp/fake-tmdb" & echo $! "$tmp"`.
   Keep the statements separate as written: a trailing `&` after an `&&`
   chain backgrounds the whole chain in a subshell, so `$!` is the subshell
   and `$tmp` stays unset. Don't use `go run ... &` either: its PID is the
   `go` command, and killing it leaves the compiled server running.
2. Build the binary with the current frontend: `make build`.
3. Start it on a spare port (not 8080, which a real instance may use)
   with a throwaway data directory, pointed at the fake, in the
   background. Always use a fresh data directory with the fake: its
   responses end up in `provider_cache` and in the catalog's not-found
   marks, and would carry over into real use.
   `WATCHT_DATA_DIR=$(mktemp -d) WATCHT_LISTEN=127.0.0.1:8099 WATCHT_TMDB_API_KEY=dev WATCHT_TMDB_BASE_URL=http://127.0.0.1:8098/3 ./bin/watcht serve &`
   and note the PID (`echo $!`) and the data directory; shell state
   doesn't persist between agent commands, so `kill %1` won't work later.
   Wait until `WATCHT_LISTEN=127.0.0.1:8099 ./bin/watcht healthcheck`
   exits 0.
4. A fresh instance opens on `/setup`; to skip it, create the admin with
   `curl -H 'Content-Type: application/json' -d '{"username":"admin","password":"long enough"}' http://127.0.0.1:8099/api/v1/auth/setup`
   and sign in at `http://127.0.0.1:8099/login`.
5. Check the pages. What the fixtures cover:
   - Search: "matrix" gives 5 results (movie 603, show 711, …; page 2 is
     empty), "harrison ford" shows that people are dropped, anything else
     is empty.
   - Detail: movie 603; shows 1396 (Breaking Bad), 94997, and 1437. Other
     IDs are 404, so opening show 711 from the search shows the
     not-found state.
   - Seasons: 1396 has episodes for seasons 0 and 1. Its other seasons,
     and every season of 94997 and 1437, are 404 at the fake, so the
     background sync hides them a moment after the first open. The fake
     answers in milliseconds, so the "Fetching episodes" state is usually
     gone before you look; to see it, wrap `window.fetch` in the page to
     return `episodes_pending: true` for `/seasons/` requests for a while.
   - Posters and backdrops load from TMDB's image CDN in the browser.
6. Check light and dark at 1440x900, 1280x720, and 390x844. The browser
   tools can't switch `prefers-color-scheme`; to check dark, add a
   `<style>` that sets the dark block's variables from `layout.css` on
   `html:root`.
7. Stop both processes by PID, delete the data directory and the fake's
   temporary directory, and delete any `.playwright-mcp/` folder the
   browser tools left in the repository root.

Session cookies aren't separated by port, so two local instances on
127.0.0.1 sign each other out.

Design tokens (colours, focus ring) are defined once in
`src/routes/layout.css` with Tailwind's `@theme`, and switch with
`prefers-color-scheme`; [DESIGN.md](../../DESIGN.md) lists them with their
contrast. `src/error.html` repeats them inline because SvelteKit uses it
when the app itself can't start; keep the two in step. Shared pieces live
in `src/lib/components/`: forms and pages (`Button`, `Field`, `CardPage`,
`Panel`, `Notice`, `InlineConfirm`, `Wordmark`, `Kbd`; action results go
to `toast` from `#lib/toast.svelte.js`, which the layout's `Toaster`
shows), catalog pieces
(`Artwork`, `Badge`, `Facts`, `EpisodeRow`, a season's episode card and
text; `EpisodesFetching`, an episode list's pending state; `ListControls`,
the placeholder sort and filter row over season grids and episode
lists), title pages (`TitlePage`, with the previous and next arrows on
its hero, and `RatingsBand`, `SectionNav`, and `ExternalLinks`, whose marks are
`SourceMark`; on season and episode pages `Breadcrumbs`, the pill above
the title, and `PickerStrip`, the sibling seasons or episodes under the
hero, whose season items and arrow links come from `#lib/season-nav.js`;
the sections under the overview, `TitleSection` with
`StatsSection`, `PeopleSection`, `VideosSection`,
`RecommendationsSection`, and `RecentEpisodesSection`, placeholders built
from `PlaceholderFilter` and `PlaceholderRow`), tracking
(`MarkButton` with its `WhenMenu`, whose options are `WhenChoices`;
`PlaysActions`, the movie and episode action column, with `PlaysButton`,
its watched state and plays menu; `ProgressButton`, the show and season
action column's watched state; `ShowActions`, the show's action column
on the show and All episodes pages; `PlaceholderActions` under both;
`MenuPopover`, the popover menu shell the plays and progress menus and
`WatchedCheck` share;
`PosterCard`, the poster, action bar, and caption card that `/history`
and a show's seasons use and the watchlist and dashboard will reuse; its
bar's classes are in `#lib/strip.js`, shared with the episode card's,
and `StripWatchlist` and `StripRating` are the bar's watchlist
placeholder and TMDB score; `WatchedCheck`, the
card's watched check with its tooltip and remove-or-add menu, built on
the Popover API and placed with `placeNear` from `#lib/anchor.js`;
`/history`'s own placeholder toolbar is `HistoryToolbar` next to its page),
search
(`SearchView`, `SearchResultRow`, `SearchOverlay`), and the app frame
(`UserMenu`, `AppFooter`, `TmdbAttribution`); reuse them before adding
new ones. Destructive actions use `InlineConfirm`, never
`window.confirm`. API errors become UI text through `errorMessage()` in
`#lib/errors.js`, which maps error codes and falls back to the server's
message; catalog calls pass `catalogMessages`. A page `load` turns a failed call
into an error page with `loadError()` (`#lib/load-error.js`), or
`catalogLoadError()` on title pages, so the error page shows a useful
message instead of "Internal Error". No external fonts or CDNs: watcht may run
offline.
