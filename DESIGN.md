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
  by hairlines. Where the poster is how you recognise a thing (history,
  and later the watchlist and dashboard), use poster cards in a grid.
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
| `watched` | `#4f7a3c` | `#8fb07c` | Watched state: episode checks, the "Watched N times" mark, season progress bars |
| `watched-hover` | `#3f6531` | `#76985f` | A hovered, focused, or open watched check on a poster card (a darker sage) |
| `listed` | `#2b6f86` | `#8cc7d9` | On the watchlist (M4 token: defined, not used yet) |
| `tmdb-logo` | `#000000` | `#ffffff` | Fill of TMDB's logo mask only (see Footer and attribution) |
| `strip` | `#14201a` | `#223428` | The poster card's action bar: dark in both themes, lifted off the canvas in dark |

**The screen.** The title hero is dark in both themes, so artwork and the
title over it look the same either way: `screen` `#0e1712`, `on-screen`
`#f1f5f1`, `on-screen-muted` `#c3d0c5`, `on-screen-accent` `#eb8f55`.
These don't switch with the theme. In dark the hero fades into the canvas;
in light it ends in a hard edge that the poster overlaps. Nothing
focusable sits on the screen (the focus ring isn't tuned for it). The
poster card's bar (`strip`) uses the same `on-screen*` colours; its
controls are the exception and draw their ring in `on-screen-accent`
(`focus-visible:outline-on-screen-accent`), inside the bar's 4 px padding.

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
| on-screen / on-screen-muted / on-screen-accent on strip | 15.2 / 10.5 / 6.9 | 12.0 / 8.3 / 5.4 |
| watched fill on strip (UI); surface (the check) on watched | 3.3; 4.9 | 5.5; 6.9 |
| watched-hover on strip; surface (the check) on watched-hover | 2.5; 6.6 | 4.0; 5.1 |

`watched-hover` is a state change only: in light it's under 3:1 against
the strip, but the check icon on it stays at 6.6 and keyboard focus adds
the `on-screen-accent` ring, so the state never depends on the fill.

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
| History day heading | 20 px (18 on phones): weekday 700, date 400; the day's total 15 px (14) `muted` | `text-lg sm:text-xl` |
| Sub-heading (season, panel) | 16 px, 600 | `text-base font-semibold` |
| Episode title | 15 px, 600; code (`S01E02`) 700 in `accent-text` | `text-[0.9375rem]` |
| Poster card title | 15 px, 600, one line (ellipsis); code 700 in `accent-text` | `text-[0.9375rem] truncate` |
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
- **Action column** (`actions` snippet): the classic right-hand column,
  16 rem on `lg` (sticky, like the poster), a full-width row under the
  facts below `lg`. A flexible third grid row takes its extra height, so
  the overview never moves when it grows. If the user's watches fail to
  load, the page still shows; the column has the error and "Try again". Today: the mark split button,
  feedback notes, and (movies) the user's watches; watchlist and rating
  join it in slice 3. Never add buttons that do nothing.
- **Movie watches:** under the mark button, a heading that states the
  state ("Not watched yet", "Watched once", "Watched 3 times", with a
  sage check), then the newest 5 watches (date and time, a source badge
  when not manual, × to remove via `InlineConfirm`) and "Show all N
  watches".
- **Season header:** the season's "x of y watched" (listed episodes only)
  with a 64 px sage progress bar, then a quiet "Mark season watched".

## Artwork (`Artwork`)

Always the `*_url` fields from the API (they change when the image cache
is on); never build TMDB URLs. `referrerpolicy="no-referrer"`;
`loading="lazy"` below the fold.

| Kind | Ratio | Source size | Shown at |
| --- | --- | --- | --- |
| Poster | 2:3 | `poster_url` (w342) | 96 / 136 / 176 px on title pages; 150–175 px in poster cards; 40 px in the overlay; 56–64 px on the search page |
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
- **Placeholders** (controls whose feature comes later, so the layout
  is settled now): shown, never faked. They look inert: `muted` (on the
  strip, `on-screen-muted` at 40%; elsewhere `muted` with the control at
  60% opacity), no hover change, a not-allowed cursor. They stay
  focusable with `aria-disabled="true"`, a name for what they will do,
  and a tooltip plus accessible description saying "Coming in a later
  update". No made-up data (counts, options) in or next to them. Used by
  the poster card's watchlist button, the card menu's "Remove all plays",
  and the history toolbar.
- **Mark split button** (`MarkButton`): the main part marks it watched
  just now in one click (check icon + label: "Mark as watched", "Mark
  season watched", "Mark show watched"); the chevron opens the
  when-choice. `primary` (40 px) in the action column, `quiet` (32 px)
  for a section. Busy shows a spinner in the main part. When it can't be
  used, both parts are disabled and the reason is shown under it in
  `muted` 12 px, which is also their `aria-describedby` ("Some episodes
  are still being fetched from TMDB…", "Nothing in this season has aired
  yet."). Season and show marks are off while episodes are still being
  fetched (the server refuses them with `episodes_pending`). Result
  notes go under it as `Notice`s: "Added to your history: …", "Already
  in your history: …" (a repeat within 3 hours merges; not an error),
  "Added 8 episodes to your history.", and for 0 added "Nothing added:
  every aired episode is already watched." or "Nothing added: nothing
  has aired yet."
- **When-choice** (`WhenMenu`, its options in `WhenChoices`, which the
  history card's menu shares): a disclosure like the user menu, under its
  trigger, 288 px (never wider than the screen less 2 rem). "When did you
  watch it?", then "Just now", "Release date" / "Air date" / "Air dates"
  with the date or the reason it's off on the right ("No date on TMDB",
  "Not out yet"; off options stay focusable with `aria-disabled`), and
  "Other date…", which opens a `datetime-local` field (max now) with the
  time zone named under it and "Mark watched". Arrow keys move between
  options, Esc closes and returns focus. Release and air dates follow the
  server's rule: out from noon UTC that day. Options are 40 px on phones.
  In the history card's menu an episode's "Air date" shows no date: the
  history entries don't carry one, so the server checks it and its
  refusal ("no release date", "not out yet") is the message.

## Rows

- **Episode row** (`EpisodeRow`): still | code (`S01E02`) + title, badges
  (air date, runtime, plays), overview (3 lines on phones) | watched
  check. Rows are
  separated by `divide-line`, no cards. Headings: season `h3`, episode
  `h4`.
- **Episode watched check** (with `onmark`): a 32 px box (40 px on
  phones) joined to a when-choice chevron (stacked on phones), on every
  listed episode. Unwatched: an outlined button with a faint check that
  turns sage on hover; one click marks it watched just now, then focus
  moves to the chevron. Watched: a sage-filled check that is *not* a
  button (a stray click can't undo a watch; watches are removed on
  /history), plus a quiet "N plays" badge; the chevron still adds
  another watch. Unaired episodes (no air date, or not out yet) can
  still be marked just now or at another date (maintainer decision); the
  outline "Airs …" or "No air date" badge says they haven't aired, and
  only the menu's "Air date" is off. The row shows its own result or
  error in small text under the overview. While the watched episodes
  can't be loaded, rows have no check (and the season no progress), so
  nothing looks unwatched by mistake; the action column says so with
  "Try again".
- **Search result** (`SearchResultRow`): poster | title + year, badges
  (type in accent, TMDB score), original title if different; the search
  page adds a 2-line overview. Hover is `raised`; the keyboard-chosen row
  in the overlay is a 15% accent tint with an accent ring.
- Links to title pages use `data-sveltekit-preload-data="tap"`: opening
  a title fetches and stores it, so hovering mustn't.

## Poster cards (`PosterCard`)

After classic Trakt's history grid. Used by `/history`; the watchlist
(slice 3) and dashboard (slice 5) reuse it.

```
┌──────────┐
│[Plex]    │  poster 2:3; source chip top left (not for manual marks)
│  poster  │
│          │
├──────────┤
│[✓][≡]♥67%│  bar on `strip`: actions left, TMDB rating right
└──────────┘
S04E05 Mercy Mi…   caption, centred, one line: 15 px 600, code in accent-text
3 Nov 2026, 02:18  12 px `muted`, tabular, one line
```

- **Grid:** a `ul`, 2 columns below `sm` (`gap-x-3 gap-y-5`), then
  `repeat(auto-fill, minmax(10.5rem, 1fr))` (`gap-x-4 gap-y-6`): 6 columns
  of 192 px in the 1280 px column, 166 px each on a 375 px phone.
  Left-aligned; a short day leaves the rest of the row empty.
- **Same height.** The title and the date are one line each, cut with an
  ellipsis; the full title is in the title line's `title` tooltip and in
  the link's accessible name, so every card in a row lines up.
- **One link per card.** The caption is the link; its `::after` stretches
  over the whole card, so the poster opens it too. The bar sits above it
  (`z-10`), so its buttons are never inside the link. Keyboard focus on
  the link rings the whole card (`focus`, offset 4). `isolate` keeps the
  card's z-index under sticky headings.
- **Poster:** `Artwork`, lazy; the placeholder names the title. Episodes
  use the show's poster, then the episode still (cropped to 2:3).
- **Bar** (`strip`, 4 px padding, joined to the poster): 40 px buttons on
  phones, 32 px from `sm`. Today's contents:
  - Watched check (`WatchedCheck`): a filled `watched` square with a check
    (`text-surface`). Hover, keyboard focus, and an open menu turn it
    `watched-hover`; it stays a check. Its name is "Breaking Bad S01E02:
    watched. History options".
  - Watchlist: shown greyed out (`on-screen-muted` at 40%, `aria-disabled`,
    not-allowed cursor, the reason in its name and tooltip) until slice 3
    builds it; then it fills `listed` when on the list.
  - Rating: a filled heart in `on-screen-accent` and TMDB's vote average
    as a percentage (`tmdbPercent`, 12 px 600 `on-screen`), labelled "TMDB
    rating 67%" for screen readers. Display only; hidden with no votes.
- **Source chip:** hero-chip style (`screen` at 80%, 20% white border,
  11 px 600 `on-screen`), hidden from screen readers; the caption carries
  "recorded by Plex" as visually hidden text instead.

### The watched check's tooltip and menu

Both are native popovers (the Popover API), so they sit in the top layer:
no card, sticky heading, or overflow clips or covers them. They are
placed in script next to the check (`placeNear` in `#lib/anchor.js`):
inside the viewport with an 8 px margin, and they follow it on scroll and
resize. Never put display utilities on a popover element itself (they
would override the hidden state); style an inner wrapper.

- **Tooltip** (`role="tooltip"`, the check's `aria-describedby`): "Watched
  on **19 Aug 2026**" (the day in the browser's zone), then the runtime
  ("1h 36m") when known, and under a hairline, 11 px, the zone times are
  shown in ("Browser time zone: Europe/Vilnius", the `zoneNote` prop;
  slice 4's setting replaces it). 12 px, `canvas` on `ink` (inverted, so dark in
  light and light in dark), `rounded-md`, a shadow; above the check, left
  edges lined up, below it when there's no room. It shows on mouse hover
  (not touch) and keyboard focus, stays while the pointer moves onto it,
  and Esc hides it. Focus coming back from the menu doesn't reopen it.
- **Menu** (a non-modal `role="dialog"`, `popover="auto"`, opened by the
  check through `popovertarget`): 256 px on `surface`, `line` border,
  `rounded-lg`, the menus' shadow; under the check, above it when there's
  no room below, and once open it keeps that side, shifting to fit as it
  grows. Contents:
  - "Remove from history?" (14 px 600) and a × close button.
  - "Only this play" in `danger` with a trash icon: removes this watch.
  - "Remove all plays", trash icon, `muted` with "Coming in a later
    update" under it: a placeholder (`aria-disabled`, described by that
    line) until the endpoint exists.
  - "Keep it" with a ×: closes.
  - A hairline, then "Add another play" (plus in a circle), a disclosure
    that opens the when-choice (`WhenChoices`, the same options as the
    mark buttons) inside the menu.
  - Errors appear at its foot as an error `Notice`; the menu stays open.
- **Keyboard:** focus goes to the dialog itself on open (not "Only this
  play", so a second Enter can't remove); the arrow keys move between the
  items, Tab works as usual. Esc, Keep it, ×, or a click outside close it
  and return focus to the check (not when the click focused something
  else). Focus leaving the menu closes it. Only one menu is open at a
  time (the browser closes the other `auto` popover).
- **Results:** removing focuses the next card's check (the heading when
  nothing is left); adding closes the menu and returns focus to the check.
  The note goes to the page (see History).

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
- **History** (`/history`): the full column. No visible page title or
  intro (the nav says where you are): a visually hidden `h1` "History"
  (also the focus target when the list empties), then the toolbar, then
  a section per day, so the day headings carry the top of the page.
  - **Toolbar**, after Trakt's, every control a placeholder (see
    Placeholders) until the filter backend: on the left a type dropdown
    "All ▾" and a date range (calendar ▾); on the right sort "Watched
    date ▾" (an icon on phones), sort direction, source (plug ▾), and
    fade/hide (eye ▾). Filter-button shape, 40 px tall on phones, 32 px
    from `sm`; a `role="group"` row that wraps rather than overflowing
    (one row at 375). No item or play counts until they can be real.
  - The day heading row is the weekday in bold then the full date,
    always with the year ("**Tuesday** 3 November 2026", 20 px, 18 on
    phones; never "Today", as in the reference), and on the right the
    day's watch time with a clock icon (`dayTotal`: "45m", "1h 45m", "2h", summed from the loaded entries'
    runtimes; the tooltip and screen-reader text add the number of watches
    and how many have no runtime; with none known it shows "3 watches"
    without the clock). A hairline under it; it's sticky under the header,
    on `canvas`. Under it a grid of poster cards (see Poster cards). Days
    are the browser's time zone until slice 4's setting; the watched
    check's tooltip says which zone. More pages load as you scroll (an
    IntersectionObserver 800 px ahead; a day split across pages merges and
    its total grows); focus never moves for that. "Load more" (quiet) stays
    as the keyboard and no-observer fallback and focuses the first new
    card. A failed page shows the error and "Try again" instead of
    retrying. At the end, "That's the start of your history." The cards'
    menus report in a success note fixed at the bottom of the window
    (centred, max 28 rem, on `surface` with a shadow; announced, and it
    clears after 8 s), since the card may be far down the page: "Removed
    …", "Added to your history: …" (the new card appears in its place by
    time; one older than everything loaded while more pages remain arrives
    as you scroll, and the note says so), or "Already counted: …" when the
    play is within 3 hours of an existing one and the server merged it.
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
  (initial, name, chevron). Page links (History now, Watchlist in M4)
  sit after the wordmark as text links; the current one is `ink` and
  carries the wordmark's accent bar on the header's bottom edge. Search
  sits on the right.
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
  search button, and a 40 px menu button with the account links. History
  is listed first in that menu on phones only, until the tab bar.

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
