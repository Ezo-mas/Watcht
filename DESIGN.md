# watcht design system

How watcht looks and why. Read this before building UI; extend it when a
new pattern appears (M4's dashboard, history, and watchlist follow it).
The tokens live in `web/src/routes/layout.css`; `web/src/error.html`
repeats the colours it uses. Shared components are in
`web/src/lib/components/`.

## Principles

- **Information first, in the spirit of classic Trakt.** Show a lot at a
  glance: artwork, dates, runtimes, and codes are scannable without
  opening anything. No decorative cards around every item; rows separated
  by hairlines.
- **One bold element per page.** On a title page it's the backdrop hero
  with the overlapping poster. Everything around it stays quiet.
- **The accent means "look here".** Burnt orange marks the current thing
  (selected season, active filter, the primary button), episode codes,
  external links, and a result's type. Don't use it for decoration.
- **State is obvious** (M4): watched is sage (`watched`), on the
  watchlist is blue (`listed`), and the accent stays for actions.
- **Fast and offline-safe.** System fonts only, no CDNs or icon packs.
  Icons are inline SVG paths, 1.6–2 px strokes.

## Palette: "Woodland Fire"

Pine and sage neutrals with a burnt orange accent. Both themes follow
`prefers-color-scheme`. Use the Tailwind utilities (`bg-surface`,
`text-muted`, `border-line`…), never raw hex in components.

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `canvas` | `#eef2ec` | `#0e1712` | Page background |
| `surface` | `#fbfcfa` | `#142019` | Header, panels, menus |
| `raised` | `#e2e9e0` | `#1b2a21` | Quiet badges, hover rows, image placeholders, skeletons |
| `field` | `#f5f8f3` | `#0b130e` | Inputs |
| `line` | `#d2dccf` | `#2a3d31` | Hairlines, quiet borders |
| `line-strong` | `#76877a` | `#62786a` | Input borders, hovered borders (3:1) |
| `ink` | `#14201a` | `#e7eee8` | Text |
| `muted` | `#4e6054` | `#9cae9f` | Secondary text, labels |
| `accent` | `#d4692c` | `#e07b3c` | Fills: primary button, selected chip, type and date badges |
| `accent-hover` | `#e0834e` | `#ea925a` | Hovered accent fill (lighter in both themes) |
| `on-accent` | `#1e0d03` | `#1e0d03` | Text on an accent fill |
| `accent-text` | `#9c4313` | `#eb8f55` | Accent as text: links, episode codes |
| `focus` | `#9c4313` | `#eb8f55` | Focus ring |
| `danger` / `on-danger` | `#b42318` / `#ffffff` | `#ff8f80` / `#2a0d09` | Errors, destructive actions |
| `ok` | `#3d6b2c` | `#8fb07c` | Success notes |
| `watched` | `#4f7a3c` | `#8fb07c` | Watched state (M4 token: defined, not used yet) |
| `listed` | `#2b6f86` | `#8cc7d9` | On the watchlist (M4 token: defined, not used yet) |
| `tmdb-logo` | `#000000` | `#ffffff` | Fill of TMDB's logo mask only (see Footer and attribution) |

**The screen.** The title hero is dark in both themes, so artwork and the
title over it look the same either way: `screen` `#0e1712`, `on-screen`
`#f1f5f1`, `on-screen-muted` `#c3d0c5`, `on-screen-accent` `#eb8f55`.
These don't switch with the theme. In dark the hero fades into the canvas;
in light it ends in a hard edge that the poster overlaps. Nothing
focusable sits on the screen (the focus ring isn't tuned for it).

Contrast (WCAG 2.1; AA is 4.5 for text, 3 for UI and large text):

| Pair | Light | Dark |
| --- | --- | --- |
| ink on canvas / surface | 14.8 / 16.3 | 15.5 / 14.2 |
| muted on canvas / surface / raised / field | 5.9 / 6.5 / 5.4 / 6.3 | 7.8 / 7.2 / 6.4 / 8.1 |
| on-accent on accent / accent-hover | 5.3 / 6.8 | 6.3 / 7.9 |
| accent-text on canvas / surface / raised | 5.8 / 6.3 / 5.3 | 7.5 / 6.9 / 6.1 |
| accent-text on the 15% accent tint | 5.3 | 5.5 |
| focus on canvas / surface | 5.8 / 6.3 | 7.5 / 6.9 |
| line-strong on surface (UI) | 3.7 | 3.5 |
| accent fill on surface (UI) | 3.5 | 5.7 |
| danger on surface; on-danger on danger | 6.4; 6.6 | 7.6; 8.2 |
| watched / listed on surface (UI) | 4.9 / 5.5 | 6.9 / 9.0 |
| on-screen / on-screen-muted / on-screen-accent on screen | 16.6 / 11.4 / 7.5 | same |

Translucent surfaces (the search overlay) were measured in the browser
over the darkest content (the hero): muted text stays above 4.5.

## Type

System stack only (`--font-sans`). One family; hierarchy comes from size
and weight. Numbers that line up (codes, dates, runtimes, counts) use
`tabular-nums`.

| Role | Size / weight | Tailwind |
| --- | --- | --- |
| Title hero | 24 / 30 / 36 px, 700; year 400 in `on-screen-muted` | `text-2xl sm:text-3xl lg:text-4xl font-bold` |
| Page heading | 20 px, 600 | `text-xl font-semibold tracking-tight` |
| Section heading | 18 px, 600, hairline under it | `text-lg font-semibold` |
| Sub-heading (season, panel) | 16 px, 600 | `text-base font-semibold` |
| Episode title | 15 px, 600; code 700 in `accent-text` | `text-[0.9375rem]` |
| Overview | 15 px, relaxed leading, max 72ch | `text-[0.9375rem] leading-relaxed` |
| Body, rows | 14 px | `text-sm` |
| Meta, hints, footer | 12 px | `text-xs` |
| Fact labels, section list | 11 px, 600, uppercase, wide tracking, `muted` | `text-[0.6875rem] font-semibold uppercase tracking-wider` |
| Badges | 11 px, 600 | see Badge |

Uppercase is only for the classic fact labels and the section list. Keep
running text under ~75 characters a line (`max-w-[72ch]`).

## Spacing and layout

- 4 px grid (Tailwind's scale). Dense by default: rows `py-1.5`–`py-3`,
  gaps `gap-1`–`gap-4`; sections `mt-8`.
- **Column:** `max-w-7xl` (1280 px) centred, `px-4` / `sm:px-6`. The
  header, footer, and title pages share it, so their left edges line up.
- Ordinary pages render inside the layout's padded column. Title pages
  (`/movies/[id]`, `/shows/[id]`) are full-bleed and lay out their own
  column; the `(app)` layout decides by route ID.
- Breakpoints: phone below `sm` (640), tablet `sm`–`lg`, desktop `lg`
  (1024) up. Check 390, 1280, and 1440 wide.

## Title page anatomy (`TitlePage`)

```
┌──────────────────────── backdrop (screen) ────────────────────────┐
│                                                                   │
│              Title 1999                                           │
│ ┌──────┐     [TMDB 8.3] [2h 16m] [Ended]                          │
├─│poster│───────────────────────────────────────────────────────────┤
│ │ 2:3  │     LABEL value   LABEL value   LABEL link …   (Facts)    │
│ └──────┘     Tagline, italic, muted                               │
│ OVERVIEW     Overview, max 72ch                                   │
│ SEASONS  6   Seasons: picker, season heading, episode rows        │
└───────────────────────────────────────────────────────────────────┘
```

- One grid for hero and body: `[poster column | main]`, poster column
  6 rem (phone), 8.5 rem (`sm`), 11 rem (`lg`). The title sits in the
  main column, so it lines up with the facts, and the poster overlaps the
  hero by 4 / 6 / 9 rem without touching the title.
- Hero: backdrop `object-cover` at 50% 25%, a bottom-to-top scrim to
  `screen` plus a left scrim, title and chips at the bottom. Without a
  backdrop: shorter, a faint ember glow on `screen`.
- Chips on the hero: TMDB score (label + bold accent number, vote count
  in the tooltip), then key facts (runtime or status and season count).
  Genres go in the facts, not the chips.
- Section list under the poster (`sm` up), only with two or more
  sections; in-page anchors, `scroll-padding-top` clears the header. On
  `lg` the poster column is sticky.
- Facts (`Facts`): label/value pairs that wrap across the column. Empty
  values are dropped. IMDb and TMDB IDs link out (new tab, `accent-text`).
- **Reserved for M4:** the classic right-hand action column (check in,
  add to history, watchlist, rate). Until those actions exist, the main
  column takes the width; never add buttons that do nothing.

## Artwork (`Artwork`)

Always the `*_url` fields from the API (they change when the image cache
is on); never build TMDB URLs. `referrerpolicy="no-referrer"`;
`loading="lazy"` below the fold.

| Kind | Ratio | Source size | Shown at |
| --- | --- | --- | --- |
| Poster | 2:3 | `poster_url` (w342) | 96 / 136 / 176 px on title pages; 40 px in the overlay; 56–64 px on the search page |
| Still | 16:9 | `still_url` (w300) | 120 px (phone), 208 px |
| Backdrop | cover | `backdrop_url` (w1280) | Full-width hero |

Missing or failed images keep the same box on `raised` with a film-strip
glyph in `muted`; posters big enough to read also show the title. Radius:
posters and stills `rounded-sm` (4 px), the title poster `rounded-md`
with a soft shadow.

## Badges and chips

- `Badge` (18 px tall, 11 px, `rounded-sm`):
  - `accent`: one per row at most, the thing to see first (Movie/Show,
    an aired episode's date).
  - `quiet` (`raised` + `muted`): secondary facts (runtime, TMDB score).
  - `outline`: not yet (an upcoming air date, "No air date").
- Hero chips are the screen's own style (translucent `screen`, 20% white
  border), 24 px tall.
- Filters and pickers are buttons or links, 32 px tall, `rounded-md`:
  `border-line` + `bg-surface`; selected season is an accent fill;
  pressed filter is an accent border with a 15% accent tint; a season
  whose episodes are still loading has a dashed border.

## Buttons and links

- `Button` `primary` (accent fill, 40 px), `quiet` (bordered, 32 px),
  `danger`. One primary per form or state.
- Internal links go through `resolve()`. External links: `accent-text`,
  underline on hover, `target="_blank"` with a visually hidden
  "(opens in a new tab)".
- Icon-only buttons need `aria-label` and a 40 px target on phones.

## Rows

- **Episode row** (`EpisodeRow`): still | code + title, badges (air date,
  runtime), overview (3 lines on phones). Rows are separated by `divide-line`,
  no cards. Headings: season `h3`, episode `h4`.
- **Search result** (`SearchResultRow`): poster | title + year, badges
  (type in accent, TMDB score), original title if different; the search
  page adds a 2-line overview. Hover is `raised`; the keyboard-chosen row
  in the overlay is a 15% accent tint with an accent ring.
- Links to title pages use `data-sveltekit-preload-data="tap"`: opening
  a title fetches and stores it, so hovering mustn't.

## Search overlay (`SearchOverlay`, `SearchView`)

- A native modal `<dialog>`: opened by the header's Search control or `/`
  (not while typing), closed by Esc, ×, or a click outside; focus returns
  to what opened it; the page doesn't scroll behind it.
- The backdrop blurs the app (`blur-md`) and dims it toward the canvas
  (60%); the panel is glass: `surface` at 75% with `backdrop-blur-2xl`.
  Desktop: 768 px wide, 8vh from the top. Phone: full screen.
- Input (48 px, 2 px border, accent when focused) and ×; filter tabs
  "Shows & movies / Shows / Movies" (client-side); a live result count;
  the list; "Load more results"; a key-hint footer on desktop.
- Combobox pattern: arrow keys move `aria-activedescendant`, Enter opens
  the chosen result, Enter with none chosen opens `/search?q=`.
- Searching: 300 ms after typing stops, from 2 characters, older answers
  dropped (`#lib/search.js`). `/search` is the same view as a page with a
  plain list of links; typing replaces the URL rather than adding history.

## States

- **Loading:** skeleton rows on `raised` (pulse) in the shape of the
  content, plus "Searching…" in the live count. Don't blank existing
  results while refreshing; dim them to 60%.
- **Pending (episodes still being fetched):** a line with a spinner,
  "Fetching episodes from TMDB…", over skeleton rows; poll every 3 s for
  up to 2 minutes (`#lib/poll.js`), then "TMDB hasn't sent this season's
  episodes yet." with "Check again".
- **Empty:** say what's empty and what to do ("No shows or movies match
  “x”. Check the spelling, or try the original title.").
- **Error:** `Notice tone="error"` with the mapped message
  (`errorMessage(e, catalogMessages)`) and a quiet retry button. Page
  load errors render `(app)/+error.svelte` inside the header and footer:
  status, a plain heading ("Title not found"), the message, and a way on
  (Search, dashboard, or Try again).

## Navigation

- **Desktop:** sticky 48 px header on `surface`: wordmark (dashboard),
  main nav, the Search control (field-styled, `/` hint), user menu
  (initial, name, chevron). History and Watchlist (M4) join the nav as
  text links; the current one carries the wordmark's accent bar on the
  header's bottom edge.
- **User menu:** a disclosure, not an ARIA menu: "Signed in as …",
  Settings, Users (admins), About, Sign out. Closes on Esc (focus back to
  the button), outside click, focus leaving, or navigation.
- **Phone (target, built in M4):** navigation and search at the bottom,
  page controls at the top, as modern phone apps do (maintainer decision,
  2026-11-02).
  - A bottom tab bar, always visible (it doesn't hide on scroll): Dashboard,
    Search, History, Watchlist, More (Settings, Users, About, Sign out).
    Icons with short labels, 48 px tall plus
    `env(safe-area-inset-bottom)`, on `surface` with a `line` top border;
    the current tab in the accent. Pages get bottom padding so nothing
    hides behind it. It must also clear browsers that put their own
    address bar at the bottom (Safari on iPhone, Chrome on Android).
  - Search tab: the overlay opens with its input at the bottom, above the
    keyboard, and results fill the space above it.
  - Top: a slim header with the wordmark or page title. Filters and page
    controls (season picker, Shows/Movies tabs, sort) stay at the top of
    the content they change.
  - No sidebar.
- **Phone (until M4):** M3 has only one destination (Search), so a tab bar
  would hold one button. Until then the header shows the wordmark, a 40 px
  search button, and a 40 px menu button with the account links.

## Footer and attribution

Every signed-in page ends with a hairline footer (`AppFooter`) with a
short credit, as Letterboxd does: "Film and TV data from TMDB", where TMDB
links to themoviedb.org (new tab), and "About watcht". No logo and no long
notice there.

The About page carries TMDB's full attribution (`TmdbAttribution`): the
logo, 20 px tall and linked to themoviedb.org, and the notice "This
product uses the TMDB API but is not endorsed or certified by TMDB." The
logo is TMDB's official "Alt short" file
(`web/src/lib/assets/tmdb-logo.svg`), unedited, drawn as a CSS mask filled
with `tmdb-logo`: pure black in light, pure white in dark, which TMDB's
logo rules allow. Never recolour it beyond that, or change its shape or
proportions. The footer credit and the About attribution must both stay.

## Motion

- Motion answers an action: colour transitions on hover and press, the
  chevron flipping, spinners and skeleton pulses while waiting. No
  entrance animations, no parallax.
- `prefers-reduced-motion: reduce` turns animations and transitions off
  globally (layout.css); smooth anchor scrolling only without it.

## Accessibility checklist

AA contrast in both themes (tables above); visible `:focus-visible` ring
(2 px `focus`, offset 2); semantic landmarks and headings in order; labels
on every input; `aria-label` on icon-only buttons; status text in
`role="status"`; keyboard paths for search (/, arrows, Enter, Esc) and
the menu; 40 px touch targets on phones; no horizontal scrolling at
390 px.
