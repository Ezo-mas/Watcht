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
- **Trakt's structure, watcht's spacing.** Title pages follow classic
  Trakt's layout (poster column, ratings band, facts, action column,
  sections), but not its density: keep watcht's gaps, padding, line
  heights, and section spacing, or roomier ones. Never tighten spacing to
  match a reference (maintainer decision, 2026-11-06).
- **The accent means "look here".** Burnt orange marks the current thing
  (the current season or episode in a picker, active filter, the primary
  button), episode codes,
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
| `ok` | `#3d6b2c` | `#8fb07c` | Inline success notes (forms) |
| `watched` | `#4f7a3c` | `#8fb07c` | Watched state: episode checks, the "Watched N times" mark, season progress bars |
| `watched-hover` | `#3f6531` | `#76985f` | A hovered, focused, or open watched check on a poster card (a darker sage) |
| `watched-part` | `#2c4428` | `#435941` | A partly watched check on the strip (a season card): a sage tint fill |
| `on-watched-part` | `#a3c48f` | `#b5d3a1` | The check icon on `watched-part`: a light sage, since the strip is dark in both themes |
| `listed` | `#2b6f86` | `#8cc7d9` | On the watchlist (M4 token: defined, not used yet) |
| `tmdb-logo` | `#000000` | `#ffffff` | Fill of TMDB's logo mask only (see Footer and attribution) |
| `strip` | `#14201a` | `#223428` | The poster card's action bar: dark in both themes, lifted off the canvas in dark |

**The screen.** The title hero is dark in both themes, so artwork and the
title over it look the same either way: `screen` `#0e1712`, `on-screen`
`#f1f5f1`, `on-screen-muted` `#c3d0c5`, `on-screen-accent` `#eb8f55`.
These don't switch with the theme. In dark the hero fades into the canvas;
in light it ends in a hard edge that the poster overlaps. The theme's
focus ring isn't tuned for the screen, so anything focusable on it (the
breadcrumb pill's links on season and episode pages) draws its ring in
`on-screen-accent` (`focus-visible:outline-on-screen-accent`), inside its
pill. The cards' action bar (`strip`, see Poster cards) uses the same
`on-screen*` colours, and its controls draw the same ring inset (2 px at
−4 px, with a 2 px `strip` inset ring round it), so the card's edges never
clip it and it reads on a sage check too. The hero's arrows draw it inset
in their circle.

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
| on-screen on the breadcrumb pill (`screen` at 70%), over pure white | 6.2 | same |
| Hero over a pure white backdrop (the bottom fade), sampled at 1440 (390): title (on-screen) at its top edge / base; year (on-screen-muted); score (on-screen-accent); TMDB mark (on-screen) | 5.2 / 7.2 (5.2); 3.6 (3.4); 4.4 (4.3); 10.2 | same |
| canvas on ink (the `strong` badge) | 14.8 | 15.5 |
| watched fill on strip (UI); surface (the check) on watched | 3.3; 4.9 | 5.5; 6.9 |
| watched-hover on strip; surface (the check) on watched-hover | 2.5; 6.6 | 4.0; 5.1 |
| Card check, not watched: on-screen-muted (the check) on strip | 10.5 | 8.3 |
| Card check, partly watched: on-watched-part (the check) on watched-part; on strip; watched-part fill on strip | 5.5; 8.7; 1.6 | 4.7; 8.0; 1.7 |
| Card check, all watched: watched fill on strip; surface (the check) on watched | 3.3; 4.9 | 5.5; 6.9 |
| Card check, menu open: on-screen on the lift (`on-screen` 15% over strip) | 9.8 | 7.7 |

The card check's three states (not watched, partly, all; see Poster
cards) are told apart by the check itself, which keeps 3:1 or more on
whatever is behind it in every state: a muted check on the bar, a light
sage check on a sage tint, a surface check on solid sage. The partly
watched tint is under 3:1 against the strip (1.6 / 1.7) and doesn't need
to carry the state alone: its check is sage where the unwatched one is
grey, the progress bar sits right above it, and the caption says "2 of 7
watched". Partly and all watched differ in fill (2.1 / 3.2 between the
two sages) and in the check's colour.

`watched-hover` is a state change only: in light it's under 3:1 against
the strip, but the check icon on it stays at 6.6 and keyboard focus adds
the `on-screen-accent` ring, so the state never depends on the fill. The
ring on a check is drawn inside it with a `strip` edge round it, so the
ring always has the strip (6.9 / 5.4) on one side, whatever the sage.

The hero's title, year, and score are all large text (24 px and up, or
20 px bold), so AA there is 3:1. The fade behind them was measured over
pure white, the worst case: the year, the faintest, keeps 3.6 at the top
of the title line (3.4 on a 390 px phone), and everything is darker below
it. Don't lighten the
fade (65% `screen` at the title block's top, 90% at the hero's bottom).

Translucent surfaces (the search overlay) were measured in the browser
over the darkest content (the hero): muted text stays above 4.5. The
header is `surface` at 85%: computed over pure black (light theme) and
pure white (dark theme) underneath, its `muted` links stay at 4.6 / 4.5,
so don't lower that opacity.

## Type

System stack only (`--font-sans`). One family; hierarchy comes from size
and weight. Numbers that line up (codes, dates, runtimes, counts) use
`tabular-nums`.

| Role | Size / weight | Tailwind |
| --- | --- | --- |
| Title hero | 24 / 30 / 36 px, 700; year 400 in `on-screen-muted` | `text-2xl sm:text-3xl lg:text-4xl font-bold` |
| Page heading | 20 px, 600 | `text-xl font-semibold tracking-tight` |
| Section heading | 18 px, 600, no hairline; with a count, the count first ("7 Seasons", "22 Episodes") | `text-lg font-semibold` |
| History day heading | 20 px (18 on phones): weekday 700, date 400; the day's total 15 px (14) `muted` | `text-lg sm:text-xl` |
| Sub-heading (season, panel) | 16 px, 600 | `text-base font-semibold` |
| Episode title | 15 px, 600; code (`S01E02`) 700 in `accent-text`. 16 px in a season's episode list | `text-[0.9375rem]` |
| Episode page title | The title hero, with the code first in `on-screen-accent`, joined by a no-break space | as Title hero |
| Poster card title | 15 px, 600, one line (ellipsis); code 700 in `accent-text` | `text-[0.9375rem] truncate` |
| Overview | 15 px, relaxed leading, max 72ch | `text-[0.9375rem] leading-relaxed` |
| Body, rows | 14 px | `text-sm` |
| Meta, hints, footer | 12 px | `text-xs` |
| Fact labels, section list, picker label | 11 px, 600, uppercase, wide tracking, `muted` | `text-[0.6875rem] font-semibold uppercase tracking-wider` |
| Badges | 11 px, 600 | see Badge |

Uppercase is only for the classic fact labels, the section list, and the
picker strip's label. Keep
running text under ~75 characters a line (`max-w-[72ch]`).

## Spacing and layout

- 4 px grid (Tailwind's scale). Dense by default: rows `py-1.5`–`py-3`,
  gaps `gap-1`–`gap-4`; sections `mt-8`, a title page's sections under
  the overview `mt-14`. Density comes from showing more, not from
  squeezing it: these are minimums.
- **Column:** `max-w-7xl` (1280 px) centred, `px-4` / `sm:px-6`. The
  header, footer, and title pages share it, so their left edges line up.
- Ordinary pages render inside the layout's padded column. Title pages
  (`/movies/[id]`, `/shows/[id]`, `/shows/[id]/seasons/[n]`,
  `/shows/[id]/seasons/all`, and `/shows/[id]/seasons/[n]/episodes/[e]`)
  are full-bleed and lay out their own column; the `(app)` layout decides
  by route ID.
- Breakpoints: phone below `sm` (640), tablet `sm`–`lg`, desktop `lg`
  (1024) up. Check 390, 1280, and 1440 wide.

## Title page anatomy (`TitlePage`)

After classic Trakt's movie page (structure only; see Principles). Movies,
shows, seasons, and episodes all use it; season and episode pages add a
breadcrumb pill above the title and a picker strip under the hero (see
Season and episode pages).

```
┌──────────── header (glass; the hero shows through it) ─────────────┐
├──────────────────────── backdrop (screen) ─────────────────────────┤
│ ┌──────┐                                                           │
│ │      │   Title 1999                                              │
├─│poster│───────────────────────────────────────────────────────────┤
│ │ 2:3  │   [TMDB] 8.3                                              │ ratings band
├─│      │───────────────────────────────────────────────────────────┤
│ │      │   LABEL value  LABEL value …   (Facts)  │[✓ 2 plays    | + ]│
│ └──────┘   Tagline, italic, muted               │[  6 Nov, 22:31|  ]│
│ OVERVIEW →  Overview, max 72ch                  │[Add to watchlist ]│
│ STATS    →                                      │[Rate it          ]│
│ PEOPLE   →                                      │[Where to watch   ]│
│ …           Stats / People / Videos (movie)                         │
│ [IMDb][TMDB] as wide as the overview and actions together           │
│                                                                     │
│ If you like The Matrix, check out…   the last section: full width,  │
│ [poster][poster][poster][poster]…    under the poster column too    │
└───────────────────────────────────────────────────────────────────┘
```

- **One grid** for hero and body: `[poster column | main | actions]`.
  Poster column 7 rem (phone), 10 rem (`sm`), 12 rem (`lg`), 14 rem
  (`xl`); action column 16 rem (`lg`), 18 rem (`xl`); gaps 16 / 24 / 32 /
  40 px. The title and band sit in the main column (and over the actions
  on `lg`), so they line up with the facts, and the poster overlaps the
  hero by 5 / 7 / 10 / 11 rem without touching the title. Pages with
  breadcrumbs measure the title-and-ratings block and align the poster
  with its top, including when an episode title wraps.
- **Hero:** starts under the translucent header (the layout pulls title
  pages up by the header's height). Tall, after classic Trakt: at least
  24 rem on phones, 32 rem from `sm`, and from `lg` 42 rem or 78% of the
  window, whichever is less (672 px at 1440×900, 562 px at 1280×720); a
  long title makes it taller. The backdrop (`object-cover` at 50% 25%) is
  in full colour: no side scrim, no dimming or blur over the picture
  (maintainer decision, 2026-11-09). Only the title and the ratings band
  sit on a short bottom fade: `screen` from 90% at the bottom to 65% at
  the top of the breadcrumb or title, then a 64 px (96 from `sm`) tail to
  nothing above it. The fade wraps the title block, so a title that wraps
  to three lines is still on it. Without a backdrop: shorter (14 / 16 /
  18 rem), a faint ember glow on `screen`.
- **Arrows** (season, All episodes, and episode pages): the previous and
  next page as round links on the hero's sides, after classic Trakt:
  40 px (44 from `sm`), `screen` at 60% with a light blur and a 20%
  `on-screen` border (85% on hover), a chevron in `on-screen`, 16 / 24 px
  from the window's edges, centred in the picture above the title. Named
  and titled for where they go ("Previous episode: S01E02 Cat's in the
  Bag...", "Next season: Season 2", "Next: All episodes"). None past the
  ends (no wrapping). Seasons follow the strip (1, 2, …, Specials, All);
  episodes follow their season's list and stop at its ends, since the
  next season's episode numbers aren't known without asking for it.
- **Ratings band** (`RatingsBand`): a full-width row along the bottom
  of the hero, on the hero's fade (no fill or blur of its own, no
  hairline), 56 px (64 from `sm`). One entry per source, and only its mark (`SourceMark`,
  screen tone) and score (20 px bold `on-screen-accent`): no "/ 10" and
  no visible vote count. The votes are in the entry's tooltip ("29,114
  votes on TMDB"), and screen readers hear "TMDB rating 8.3 out of 10,
  from 29,114 votes" (maintainer decision, 2026-11-06). TMDB only until
  M8 adds MDBList's sources, which join as more entries; no greyed-out or
  empty slots. No band without votes. Nothing on it is focusable (no
  "All stats" link; the section list goes there). Runtime, status, and
  season count are facts, not hero chips. An episode has a score but no
  vote count: its tooltip is "TMDB rating 7.8", and screen readers hear
  "TMDB rating 7.8 out of 10". Seasons show the mean of positive
  episode scores, to one decimal; no band when none are rated.
- **Poster column** (`aside`): the poster (`rounded-md`, a soft shadow),
  the section list, and the link icons. From `sm` it spans every row of
  the body but the last section's and is sticky, so it stays in view down
  the page until that section starts: 4 rem from the top, or, when it's
  taller than the window (a short laptop screen), stuck by its bottom
  edge 16 px above the window's, so the list and links stay reachable.
  The sticky `aside` sits in a cell of its own that spans those rows (a
  sticky grid item is held by the whole grid in Chrome, not its area), so
  it scrolls away where the last section starts under it.
- **Section list** (`SectionNav`, two or more sections): uppercase
  11 px rows with hairlines and a right arrow. A section with a count
  carries it first in its label, singular for one ("7 Seasons", "22
  Episodes"; `countLabel`), as its heading does; there's no separate
  count slot. Links scroll on the same page (smooth unless reduced
  motion; `scroll-padding-top` clears the header). Scroll-spy: the
  section whose top has passed a third of the way down the window is
  current (`aria-current="true"`): `ink`, the arrow in `accent-text`, and
  a 2 px accent bar left of the row; at the bottom of the page, the last
  section showing. A section that was just picked stays current while the
  page scrolls to it (a short last section can't reach the line) until
  the reader scrolls again. Phones: the column shows only the poster and
  link icons; the list becomes a strip of filter-style links (40 px,
  current one with the accent border and tint) that scrolls sideways,
  just above the sections, and only on pages that have them.
- **Link icons** (`ExternalLinks`): the title on IMDb and TMDB (when TMDB
  knows the IMDb ID), as 40 px square tiles (`surface`, `line` border,
  `rounded-md`; hover `raised` and `line-strong`) that wrap under the
  section list (under the poster on phones). Each opens a new tab; its
  name is "The Matrix on IMDb (opens in a new tab)", the site in the
  tooltip. The marks (`SourceMark`) are watcht's own text marks, not the
  sites' logos: "IMDb" in a filled `ink` box, "TMDB" in an outlined pill.
  TMDB's logo is far too wide to read at this size and stays on the About
  page (see Footer and attribution). IDs are not facts.
- **Facts** (`Facts`): label/value pairs that wrap across the main
  column, under the band. Empty values are dropped. Movie: Released,
  Runtime, Status, Genres, Original title (when different), Language.
  Show: Aired, Status, Episodes (regular seasons), Genres, Original
  title, Language. Season: Premiered, Episodes, Runtime (the sum of the
  episodes' known runtimes), Genres (the show's). Episode: Aired (Airs
  when it hasn't), Runtime. A visually hidden "Overview" `h2` heads them
  (the list's first target).
- Under the facts, in the main column: tagline (italic, `muted`), then
  the overview (max 72ch). A season without an overview shows the
  show's.
- **Action column** (`actions` snippet): the classic right-hand column on
  `lg`, beside the facts and overview; under the overview below `lg`
  (beside the sticky poster column on tablets). Not sticky. A flexible
  grid row takes up its extra height, so the overview never moves when it
  grows; the sections below start under whichever is taller. Movies and
  episodes have the plays column, shows and seasons the progress column
  (both below). Results are toasts (see Toasts).
- **Movie and episode action column** (`PlaysActions`), after classic
  Trakt's play button. One split button states and changes the watched
  state, then the actions still to come; results are toasts (see
  Toasts). No list of watches under it (maintainer decision, 2026-11-06);
  the plays are in its menu. An episode's plays come from its own
  watches, by its TMDB episode ID.
  - *Not watched:* the mark split button, "Mark as watched" (marks it
    just now) with a + that opens the when-choice (see Mark split
    button).
  - *Watched* (`PlaysButton`): the same split shape, 48 px, filled
    `watched` with `surface` text (4.9 light, 6.9 dark; `watched-hover`
    on hover and while its menu is open). The main part has the check, the
    count ("2 plays", 14 px 600) and the latest play (`formatWatch`, 12 px)
    on a second line; its name is "2 plays, latest 6 Nov 2026, 22:31. Your
    plays of The Matrix". It opens the plays menu (below). The + (48 px
    square, a `surface` 30% divider) opens the when-choice to add another
    play; it stays enabled while a play is added, so focus isn't lost.
  - When one button replaces the other, focus follows: the first play
    moves it to the plays button, removing the last moves it to "Mark as
    watched".
  - If the plays fail to load, the page still shows: "Mark as watched",
    and the error with "Try again".
  - Under it, placeholders (`PlaceholderActions`): full-width quiet
    buttons "Add to watchlist" and "Rate it" (M4 slice 3) and "Where to
    watch" (backlog), 40 px, 8 px apart, each with an icon. Nothing for
    features that aren't planned (favourites, lists, comments).
- **Plays menu** (`PlaysButton`, on the shared `MenuPopover` shell; see
  The watched check's tooltip and menu for its behaviour): 288 px, no
  header row (the button already shows the count; maintainer decision,
  2026-11-06). The dialog is named "Plays of The Matrix" and takes focus
  itself on open, as the history menu does. It starts with every play of
  the movie, newest first (the API returns them all, unpaged): date and
  time, a source badge when not manual, and a compact trash-and-"Remove"
  button ("Remove the play from …"). Past about five rows the list scrolls inside the menu
  (17 rem tall on phones, 14 rem from `sm`, never more than the window
  less 13 rem; `overscroll-contain`), so the menu never grows past the
  window; focus moving to a row (arrow keys, a confirm) scrolls it into
  view. The trash opens an `InlineConfirm` under the row ("Remove the play
  from … from your history?", "Remove play" / "Cancel"); Esc inside the
  confirm cancels just the confirm, and anywhere else in the menu closes
  it. No undo: a play from Plex, Jellyfin, or an import can't be
  re-created through the manual mark. After a removal the menu stays open
  and focus moves to the next play's trash button. Fixed under the list
  (not in it), above a hairline: "Remove all plays" (a destructive action with `InlineConfirm`) and "View all plays" (a link to `/history`, with
  "Your whole history; filters come later" under it, since history can't
  be filtered to one title yet). After another divider, "Add another play"
  with a plus-circle opens the same when-choices as the +, with the same
  busy state. No "Keep it" and no ×: Esc, a click or
  tap outside, focus leaving, or the button itself close it.
- **Show and season action column**, after classic Trakt's progress
  button (maintainer decision, 2026-11-09):
  - *Nothing watched* (or the watched episodes can't be loaded): the mark
    split button with a +, "Mark show watched" or "Mark season watched".
    The show's adds a `muted` 12 px note: "Adds every aired episode you
    haven't watched yet, except specials."
  - *Something watched* (`ProgressButton`): PlaysButton's split shape,
    size, and colours (at least 48 px, `watched` with `surface` text). The
    main part has the check, "**57%** watched" (the percentage 20 px 700,
    the word 14 px 600) and a second line, "76 of 133 episodes · 78 plays"
    (12 px, tabular; it wraps rather than cut, and the + grows with it).
    Its name is "57% watched: 76 of 133 episodes, 78 plays. Your history
    of Breaking Bad". It opens a `MenuPopover` (288 px, named "Your
    history of …"), after Trakt's: "Watch remaining" (a check; "Marks
    the rest watched just now" under it: the +'s "Just now", closing the
    menu first, so focus is back on the button while it runs; off, with
    the +'s reason under it, while episodes are being fetched), "View
    history" (a link to `/history`, "Your whole history; filters come
    later" under it), then above a hairline the placeholders "Rewatch",
    "Drop" (the plan's Backlog), and the confirmed "Remove all plays" action. The + marks the
    rest: the when-choice, then the same show or season mark and toasts. A 4 px sage bar (`watched` on a
    `line` track, `rounded-full`, decoration) sits 8 px under the button.
  - Counts: a show counts its regular seasons only (specials are marked
    on their own); the total is their TMDB episode counts, watched the
    watched episodes in them (never more than a season's count), plays
    every play of those. A season counts its listed episodes once they're
    in, and the show's counts for its number while they're still being
    fetched. The percentage rounds down, shows at least 1% once anything
    is watched, and 100% only when everything is.
  - While episodes are being fetched, the + (or both parts of the mark
    button) is off with the reason under it; the main part still opens
    the menu. The first mark moves focus from the mark button to the
    progress button.
  - Then the placeholders (`PlaceholderActions`), as on the movie.

### Season and episode pages

Routes with numbers: `/shows/{id}/seasons/{n}` and
`/shows/{id}/seasons/{n}/episodes/{e}` (maintainer decision, 2026-11-09),
and every episode at `/shows/{id}/seasons/all` (a route of its own, so
"all" never reaches the number check).
The show loads once in their shared layout, so moving between seasons or
episodes doesn't fetch it again. The episode is found by its number in
its season's stored episodes; everything after that (plays, marks) uses
its TMDB episode ID. A season the show doesn't list, or an episode its
season doesn't list, is a 404 ("Season not found", "Episode not
found"). Its error page links to the show, and an episode's to its
season too, since those still load (history links to episodes TMDB has
dropped land there). A show that can't be loaded is "Title not found"
on its season and episode pages too. The show page's old `?season=N`
links redirect to the season page.

```
┌──────────────────────── backdrop (the show's) ─────────────────────┐
│ ┌──────┐  (Breaking Bad / Season 1 | Season finale)  breadcrumb   │
│ │poster│  S01E07 A No Rough Stuff Type Deal 2008                  │
├─│season│───────────────────────────────────────────────────────────┤
│ │or the│  [TMDB] 8.4                     (episodes with a score)   │
├─│show's│───────────────────────────────────────────────────────────┤
│ └──────┘  EPISODE [1][2][3]…[7]  picker strip (main + actions)     │
│           AIRED … RUNTIME …                     │[progress / plays]│
│           Overview                              │[placeholders    ]│
└───────────────────────────────────────────────────────────────────┘
```

- **Poster:** the season's, else the show's. **Backdrop:** the show's on
  a season page; on an episode page the episode's still at full size
  (`still_large_url`), else the show's.
  **Title:** the season's name ("Season 1") with the years its episodes
  aired ("2008–2009"), or the episode's code and name with its air year.
  The TMDB link is the season's or episode's own page there (no IMDb:
  no season page there, and no episode IMDb ID in the API).
- **Breadcrumb pill** (`Breadcrumbs`), on the hero above the title: the
  show (and on episodes the season) as links in a `rounded-full` pill,
  `screen` at 70% with a light blur and a 20% `on-screen` border,
  14 px `on-screen` text, "/" between them in `on-screen-muted`; 40 px
  tall on phones, 32 px from `sm`. A long show name is cut with an
  ellipsis first. On an episode that opens or closes something, a flag
  is joined on the end, inverted (`on-screen` fill, `screen` text, 13 px
  600): "Series premiere", "Season premiere", "Season finale", or "Series
  finale". On phones, where the hero's column is narrow, the flag wraps
  under the pill as its own pill.
- **Picker strip** (`PickerStrip`), the first row under the hero, as wide
  as the facts and actions together (on lg): the label ("Season",
  "Episode") in the fact-label style, then the siblings as links in a
  row that scrolls sideways when it doesn't fit (a thin scrollbar,
  padding inside so focus rings aren't clipped; 12 px under the chips,
  taken back out of the layout with a negative margin, so the scrollbar,
  an overlay one too, runs under the chips and never over them). Chips are 40 px on
  phones, 32 px from `sm`, `rounded-md`, `line` border on `surface`; the
  current one is accent-filled with `aria-current="page"`; a season whose
  episodes are still loading has a dashed border. Seasons: 1, 2, …,
  Specials, All (screen readers hear "Season 2", and "All episodes" for
  All; the tooltip is the episode count, or "Every episode, specials
  included"). Episodes: 1 … n (the tooltip is the episode's title). The
  current chip is scrolled into view inside the strip (not the page) on
  load. Picking one keeps the scroll position and focus
  (`data-sveltekit-reset="false"`), like switching tabs. On phones the
  poster spans the strip and the facts, so both sit beside it.
- **Milestones** (`episodeMilestone`, conservative so a badge is never
  wrong): the first episode of season 1 is the series premiere, the
  first of a later season its season premiere; the last listed episode of
  a season is its finale once every episode of it has aired and the
  season can't still grow (a later season exists, or the show isn't in
  production); for the last season of a show no longer in production,
  that's the series finale. Specials have none.
- **Sections:** a season has Overview, its episodes ("7 Episodes"),
  People, and Videos; an episode Overview, People, and Videos.
- **All episodes** (`/shows/{id}/seasons/all`): every episode the show
  lists, specials included, from one request (`GET /shows/{id}/episodes`;
  maintainer decision). Title "All episodes" with the show's years
  ("2008–2013"), the show's backdrop and poster, the show in the
  breadcrumb, the season strip with All current, and an arrow back to
  Specials (or the last season). Facts: Episodes and Runtime (the sum of
  their known runtimes). The show's action column (`ShowActions`, as on
  the show page). Sections: Overview and "16 Episodes": the episode list
  in the strip's order (regular seasons by number, then specials), each
  row linking to its own season's episode page, specials coded "S00E03".
  The list is the page's last section, so it runs the full width under
  the poster column. While any season is still being fetched it's the
  pending state and polls (`pollShowEpisodes`), with the show's own
  count in the heading meanwhile.
  - *Long shows* (General Hospital lists about 15,000 episodes): the list
    renders a chunk at a time (`nextLimit` in `#lib/chunk.js`): the first
    100 rows, then 100 more each time the reader comes within 1,500 px of
    its end (an IntersectionObserver), so the page opens with a small DOM.
    Under the rows, a quiet "Show 100 more" button ("Show the last 12"
    at the end; 40 px on phones) and "Showing 100 of 15,012 episodes"
    in `muted`, for the keyboard and for browsers without the observer;
    the button moves focus to the first new episode's title. The count
    in the heading, the section list, and the facts is always the whole
    show's. Per row, everything is a lookup: watched episodes by ID (a
    Map), milestones from each season's first and last episode worked
    out once (`seasonEnds`). The browser's find-in-page only sees the
    rows shown so far.
- **Pending:** if the season's episodes are still being fetched, the
  season page shows the pending state in Episodes and polls (see
  States; `EpisodesFetching`); the episode page shows the code and the fetching line, polls,
  then loads again to find the episode.

### Sections under the overview (`below` and `last` snippets)

Movies have Stats, People, Videos, and Recommendations. Shows have
Recent episodes, People, Seasons, Stats, Videos, and Recommendations;
seasons Episodes, People, and Videos; episodes People and Videos; All
episodes only its Episodes. They span the main and action columns on
`lg`, beside the poster column. The last one (`last`: Recommendations on
movies and shows, Videos on seasons and episodes, the episode list on All
episodes) spans the full column instead, under the poster column too,
after Trakt's "If you like…" row; the poster column stops sticking where
it starts. The section list still scrolls to it and marks it current.
Phones are unchanged (everything is full width there), and the phone
section strip sits above the first section, whichever snippet that's in.
Each is a `TitleSection`: an 18 px heading with no
hairline under it (unlike Seasons; maintainer decision, 2026-11-06), a
filter on the right, a `muted` line saying what's coming, then the
content. Until their data exists they are placeholders (see
Placeholders): static shapes on `raised` in the real layout, with no
names, counts, or scores that aren't real.

- **Stats** (`StatsSection`), three parts:
  - *Other sites' scores* ("second opinions", after Trakt), straight under
    the Stats heading with no sub-heading of their own; the list is named
    "Ratings from other sites" for screen readers. A row per source: mark
    tile, name and vote count,
    the score in a `raised` box, a bar (`accent` on a `raised` track, the
    score out of 10). TMDB's row is real (its vote average and count);
    then "More sites' scores are coming in a later update." No difference
    column until watcht has a score of its own to compare with.
  - *Household ratings* (a 16 px `h3`): ten empty bar slots on one baseline, labelled
    1–10 with a heart; then four figures with a 2 px `line` top rule:
    Average, Ratings, Most given, Liked it (rated 7 or higher), each an
    empty value shape (named "Not available yet" for screen readers) and
    a caption.
  - *Household activity* (a 16 px `h3`): tiles (`surface`, `line` border) for Watchers,
    Plays, and Watchlists. Trakt's favourites, lists, and comments tiles
    are left out: comments are a non-goal (social features) and the others
    aren't planned.
- **People** (`PeopleSection`): a "Cast ▾" filter placeholder and a row
  of 2:3 person tiles (a person glyph, name and role bars).
- **Videos** (`VideosSection`): a "Trailers ▾" filter placeholder and a
  row of 16:9 tiles (a play glyph, a title bar).
- **Recommendations** (`RecommendationsSection`): "If you like
  The Matrix, check out…" and a row of 2:3 poster shapes on the poster
  card grid.
- **Recent episodes** (`RecentEpisodesSection`, shows): "Up next" (one
  card) and "Recently aired" (two), each a 16 px `h3`, on a three-column
  grid on lg (two columns from `sm`, with one recently aired card; on
  phones each block is a two-column row). Placeholders until slice 4's
  show progress fills them (maintainer decision): the episode card's
  shape (a 16:9 `raised` still with a monitor glyph, the dark `strip`
  bar with faint button and score shapes, two caption bars) and "Your
  next episode and the latest ones to air come with show progress in a
  later update."
- **Seasons** (`SeasonGrid`, shows): headed "7 Seasons" (regular
  seasons only; "Seasons" when there are only specials), with the list
  controls on the right. The poster card grid (see Poster cards), regular
  seasons in order and specials last, one per number. Each card is the
  season's poster (else the show's), a 4 px progress bar along its
  bottom edge, the bar, and a caption: the name, then "2 of 7 watched"
  once any is watched, else "7 episodes". Its tooltip names the show and
  the season ("Breaking Bad – Season 1", or the season's own name). It
  opens the season page; episodes are listed only there.
  - *The bar's check* shows how much is watched and marks the season
    (`markSeason`): its aired episodes not watched yet, so on a partly
    watched season it marks the rest. Three states (`checkState`; see
    Poster cards): not watched (a muted check), partly watched (a sage
    tint), all watched (solid sage). After Trakt's season cards there's
    no chevron: the check itself opens the when-choice (`WhenMenu` with
    the check as its trigger, `align="auto"`, see When-choice), named
    "Mark Season 1 watched…" or "Mark the rest of Season 1 watched…"
    (`aria-haspopup="dialog"`, `aria-expanded`), so a mark is two clicks,
    or Enter twice (focus starts on "Just now"). Esc closes it and
    returns focus to the check. Off (`aria-disabled`, faded by colour so
    the focus ring stays full strength, the reason as its tooltip and
    description, and it doesn't open) while its episodes are being
    fetched or before it starts airing (`seasonMark`). A partly watched
    season leads with "Watch remaining" (just now), then Air dates and
    Other date, followed by confirmed Remove all plays. Every episode
    watched: a sage button ("Remove or add plays of Season 1…") opening
    the shared "Remove from history?" menu: Remove all plays, Keep it,
    divider, Watch again (currently disabled with an explanation: the existing
    season mark only adds unwatched episodes; rewatch behavior needs a maintainer decision). While a mark runs the check spins (`aria-busy`). Results
    are toasts named for the season ("Season 1: Added 7 episodes to your
    history."); the check keeps focus after a mark. No check while the watched
    episodes can't be loaded. Then the watchlist placeholder.
- **List controls** (`ListControls`, over the season grid and every
  episode list), after Trakt's: "Sort: Number ▾" joined to a direction
  button, a "Filter by title" field ("Filter by name" for seasons), and a
  hide-watched toggle (eye ▾), all placeholders (see Placeholders). The
  field looks like a quiet input (`field`, `line` border, a magnifier)
  and is `readonly` with `aria-disabled`, named for what it will do
  ("Filter episodes by title"). A `role="group"` row; on phones it takes
  the full width under the heading, the field flexible and "Sort:"
  dropped (the button's name keeps it).
- **Episodes** (seasons, All episodes): headed with the count ("7
  Episodes"), the list controls on the right, then the episode list (see
  Rows).
- Rows of tiles (`PlaceholderRow`) fill the width and keep only their
  first row (`auto-fill` columns, the rest clipped), so a row is full at
  every width.

## Artwork (`Artwork`)

Always the `*_url` fields from the API (they change when the image cache
is on); never build TMDB URLs. `referrerpolicy="no-referrer"`;
`loading="lazy"` below the fold.

| Kind | Ratio | Source size | Shown at |
| --- | --- | --- | --- |
| Poster | 2:3 | `poster_url` (w342) | 112 / 160 / 192 / 224 px on title pages; 150–175 px in poster cards; 40 px in the overlay; 56–64 px on the search page |
| Still | 16:9 | `still_url` (w300) | Episode cards: full width (phone), 208 / 240 px |
| Large still | 16:9 | `still_large_url` (full size) | The episode page's hero |
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
  - `strong` (`ink` with `canvas` text, inverted in both themes): an
    episode that opens or closes a season or the series ("Season
    finale"; see Milestones).
- Filters and pickers are buttons or links, 32 px tall (40 on phones),
  `rounded-md`: `border-line` + `bg-surface`; the current season or
  episode in a picker strip is an accent fill; pressed filter is an
  accent border with a 15% accent tint; a season whose episodes are
  still loading has a dashed border.

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
  the poster card's watchlist button, the history toolbar, the action columns'
  watchlist, rating, and where-to-watch buttons, the episode and season
  cards' watchlist button, the progress menu's "Rewatch" and "Drop", the list controls over the season grid and episode
  lists, and the title page's section filters
  (`PlaceholderFilter`, described by the section's "coming" line).
  Content that comes later is shown as static shapes on `raised` (no
  pulse, which means loading) with one `muted` line saying what's coming.
- **Mark split button** (`MarkButton`): the main part marks it watched
  just now in one click (check icon + label: "Mark as watched", "Mark
  season watched", "Mark show watched"); the chevron opens the
  when-choice. In the action columns (movie, show, season, episode) the
  chevron is a + instead (`menuIcon="plus"`, as on `PlaysButton` and
  `ProgressButton`), which turns 45° into an × while the menu is open
  (200 ms; instant with reduced motion). `primary` (40 px) in the action
  column, `quiet` (32 px) for a section. Busy shows a spinner in the main part (`aria-busy`);
  both parts stay enabled while a mark runs, so keyboard focus isn't
  dropped, and a second mark is ignored. When it can't be used, both
  parts are disabled and the reason is shown under it in
  `muted` 12 px, which is also their `aria-describedby` ("Some episodes
  are still being fetched from TMDB…", "Nothing in this season has aired
  yet."). Season and show marks are off while episodes are still being
  fetched (the server refuses them with `episodes_pending`). Results are
  toasts (see Toasts): success "Added to your history: …", "Season 1:
  Added 8 episodes to your history."; info "Already in your history: …"
  (a repeat within 3 hours merges; not an error) and for 0 added
  "Nothing added: every aired episode is already watched." or "Nothing
  added: nothing has aired yet."; a failed mark is an error toast.
- **When-choice** (`WhenMenu`, its options in `WhenChoices`, which the
  history card's menu shares): a disclosure like the user menu, under its
  trigger, 288 px (never wider than the screen less 2 rem), lined up with
  the trigger's left or right edge, or (`align="auto"`, on grid cards)
  with its left edge shifted just enough to stay 16 px inside the
  window. "When did you
  watch it?", then "Just now", "Release date" / "Air date" / "Air dates"
  with the date or the reason it's off on the right ("No date on TMDB",
  "Not out yet"; off options stay focusable with `aria-disabled`), and
  "Other date…", which opens a `datetime-local` field (max now) with the
  time zone named under it and "Mark watched". Arrow keys move between
  options, Esc closes and returns focus. Release and air dates follow the
  server's rule: out from noon UTC that day. Options are 40 px on phones.
  A watch marked at the release or air date shows that date with no time,
  the same in every zone (`formatWatch`). On cards the trigger is the
  check itself (the `trigger` snippet), whose name alone ("Mark Season 1
  watched…") doesn't say a choice follows, so it's marked
  `aria-haspopup="dialog"` and the panel is a `role="dialog"` named "When
  did you watch it?" (`popup`); the action columns' chevron and + keep
  the plain disclosure.

## Rows

- **Episode row** (`EpisodeRow`, a season's episode list), after classic
  Trakt's season page: an episode card on the left, the text on the
  right; rows separated by `divide-line`, `py-4`. Card 13 rem (`sm`),
  15 rem (`lg`); on phones it's full width above the text. Headings: the
  section `h2` "Episodes", each episode an `h3`.
  - *Card:* the still (16:9, lazy, a link to the episode page that's
    hidden from the keyboard and screen readers, since the title is the
    same link), and the cards' `strip` bar joined under it (see Poster
    cards: no padding, the check flush in the bottom-left corner): the
    watched check, the watchlist placeholder, and on the right TMDB's
    score as the heart and percentage (`tmdbPercent`), when there is
    one. No chevron (after Trakt's cards). Controls are the bar's full
    height and square, 40 px on phones and 32 px from `sm`.
  - *Watched check* (without the watched episodes loaded, there's none),
    two states: unwatched, a check in `on-screen-muted` on the bar that
    fills sage on hover. It opens the when-choice itself ("Mark S01E02
    “Name” watched…", `aria-haspopup="dialog"`), opening to the right
    (lined up with the check's left edge); Esc returns focus to the check.
    Once the mark is in, the check turns sage and keeps focus. Watched,
    the sage-filled check opens "Remove from history?": "Only this play"
    removes the newest play fetched when chosen; "Remove all plays" opens an `InlineConfirm`; "Keep it" closes. After a
    divider, "Add another play" opens the when-choices in the same popup.
    Esc closes and returns focus to the check. A quiet "N plays" badge
    shows the count.
    Unaired episodes (no air date, or not out yet) can still be marked
    just now or at another date (maintainer decision); the outline
    "Airs …" or "No air date" badge says they haven't aired, and only the
    menu's "Air date" is off. Its result or error is a toast that names
    the episode ("Added S01E02 to your history: …", "S01E02: …"), since
    it floats away from the row.
  - *Text:* the code (`S01E02`, 700 `accent-text`) and title, 16 px 600,
    a link to the episode page (underline on hover); badges (air date
    `accent`, or `outline` when upcoming; runtime; plays; a `strong`
    premiere or finale badge); the overview, 14 px `muted`, relaxed,
    three lines on phones and four from `sm` (the full text is on the
    episode page).
  - While the watched episodes can't be loaded, rows have no check (and
    the season no progress), so nothing looks unwatched by mistake; the
    action column says so with "Try again".
- **Search result** (`SearchResultRow`): poster | title + year, badges
  (type in accent, TMDB score), original title if different; the search
  page adds a 2-line overview. Hover is `raised`; the keyboard-chosen row
  in the overlay is a 15% accent tint with an accent ring.
- Links to title pages use `data-sveltekit-preload-data="tap"`: opening
  a title fetches and stores it, so hovering mustn't.

## Poster cards (`PosterCard`)

After classic Trakt's history grid. Used by `/history` and a show's
Seasons; the watchlist (slice 3) and dashboard (slice 5) reuse it. The
episode cards (Rows) and the Recent episodes placeholders share its bar.

```
┌──────────┐
│[Plex]    │  poster 2:3; source chip top left (not for manual marks)
│  poster  │
│          │
├──────────┤
│✓ ≡   ♥67%│  bar on `strip`: the check flush in the corner, rating right
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
- **Bar** (`strip`, joined to the poster; `stripBar` and the control
  classes in `#lib/strip.js`), after Trakt's (maintainer decision,
  2026-11-09): no padding and no inner tiles. It's as tall as its
  controls, 40 px on phones and 32 px from `sm`, its bottom corners the
  card's (`rounded-b-sm`). Every control fills its full height with
  square corners, directly on the bar; the watched check sits flush
  against the card's left and bottom edges, its bottom-left corner
  rounded to match. Focus rings are drawn inset (see The screen). No
  chevron or other menu button on any card's bar (after Trakt's cards;
  maintainer decision, 2026-11-10): the check is the only way in to
  marking, and opens its own menu. Today's contents:
  - The check, one look everywhere (`stripCheck` in `#lib/strip.js`),
    by state (`checkState` in `#lib/watched.js` where progress applies;
    episode cards use only the first and last):

    | State | Look | What it does |
    | --- | --- | --- |
    | Not watched | a check in `on-screen-muted` on the bar | opens the when-choice |
    | Partly watched (a season) | `watched-part` fill, the check in `on-watched-part` | opens Watch remaining, date choices, and confirmed Remove all plays |
    | All watched | solid `watched`, the check in `surface` | a button opening the shared removal menu (Remove all plays with confirmation, Keep it, and another watch; episode cards also offer Only this play) (`watched-hover` on hover and while open) |

    A not or partly watched check fills `watched` on hover (what it's
    about to do); while its menu is open it's a neutral lift (`on-screen`
    at 15%, the check in `on-screen`), so it never looks watched before
    it is. Off, it's faded by colour (40%, or the tint at 50%), not by
    opacity, so its focus ring stays full strength. Contrast of each state
    is in the palette's table.
  - History's check (`WatchedCheck`): a filled `watched` square with a check
    (`text-surface`). Hover, keyboard focus, and an open menu turn it
    `watched-hover`; it stays a check. Its name is "Breaking Bad S01E02:
    watched. History options", and it opens the history menu (below).
  - Watchlist (`StripWatchlist`): shown greyed out (`on-screen-muted` at
    40%, `aria-disabled`, not-allowed cursor, the reason in its name and
    tooltip) until slice 3 builds it; then it fills `listed` when on the
    list.
  - Rating (`StripRating`): a filled heart in `on-screen-accent` and
    TMDB's vote average as a percentage (`tmdbPercent`, 12 px 600
    `on-screen`), 8 px from the right edge, labelled "TMDB rating 67%"
    for screen readers. Display only; hidden with no votes.
- **A menu in the bar** (the check's when-choice, a disclosure that isn't
  in the top layer) lifts its card over its neighbours while open
  (`relative isolate`, `z-20` while a descendant is expanded, still under the sticky header), so the next card can't cover
  it.
- **Tooltip** (`tooltip`): the link's `title`, when the caption alone
  isn't the full name (a season card's "Breaking Bad – Season 1").
- **Progress** (`progress`, 0 to 1; a show's season cards): a 4 px bar
  joined under the poster, `watched` on a `line` track, decoration (the
  caption says it in words), partly filled for a partly watched season
  and full for a finished one, as on Trakt's season cards. Above the bar
  when a card has both.
- **Source chip:** the screen's chip style (`screen` at 80%, 20% white border,
  11 px 600 `on-screen`), hidden from screen readers; the caption carries
  "recorded by Plex" as visually hidden text instead.

### The watched check's tooltip and menu

Both are native popovers (the Popover API), so they sit in the top layer:
no card, sticky heading, or overflow clips or covers them. The menu's
shell is `MenuPopover`, which the movie's plays menu shares: placement,
focus on open and return, arrow keys over `data-item` rows, closing when
focus leaves, an optional title and × header (the history menu has one;
without it the dialog is named by its `label`), an error `Notice` at its
foot, and
the shared rows (`menuItem`, the trash icon, "Keep it") as snippets.
`RemoveAllPlays` supplies the destructive row and its `InlineConfirm` for
history, plays, progress, and card menus. They are
placed in script next to the check (`placeNear` in `#lib/anchor.js`):
inside the viewport with an 8 px margin and never over the sticky header
(it's marked `data-app-header`; `coveredTop()` reads its bottom edge), and
they follow it on scroll and resize while it's in view. Once the anchor
has left the view (`outOfView`: scrolled up under the header, or off the
window), the menu closes and the tooltip hides, rather than being held at
the window's edge over the header; focus that was in the menu goes back
to the anchor without scrolling. Never put display utilities on a popover element itself (they
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
  - "Remove all plays", trash icon, `danger`: opens an `InlineConfirm`
    naming the title and warning that every play will be permanently
    removed. Cancel returns focus to the row; errors stay in the confirm.
    Success refreshes history/progress and toasts "Removed 3 plays of …".
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
  The result is a toast (see Toasts); a failure stays in the open menu.

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
  "Fetching episodes from TMDB…", over skeleton rows in the episode
  card's shape; poll every 3 s for up to 2 minutes (`pollSeason` in
  `#lib/season-poll.js`, on `#lib/poll.js`), then "TMDB hasn't sent this
  season's episodes yet." with "Check again". Once they're in, the show
  reloads too, so the picker and the marks are up to date.
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
    menus report in toasts (see Toasts): "Removed …", "Added to your
    history: …" (the new card appears in its place by time; one older than
    everything loaded while more pages remain arrives as you scroll, and
    the toast says so), or, as info, "Already counted: …" when the play is
    within 3 hours of an existing one and the server merged it.
- **Empty:** say what's empty and what to do ("No shows or movies match
  “x”. Check the spelling, or try the original title.").
- **Error:** an error that belongs to a control or state stays inline: a
  `Notice tone="error"` with the mapped message (`errorMessage(e,
  catalogMessages)`) and a quiet retry button for a failed load, the
  disabled reason under a button, a field's error, or an error inside an
  open menu or confirm. A failed action with no menu open is an error
  toast (see Toasts). Page
  load errors render `(app)/+error.svelte` inside the header and footer:
  status, a plain heading ("Title not found"), the message, and a way on
  (Search, dashboard, or Try again; a missing season or episode also
  links to its show, and a missing episode to its season).

## Toasts (`Toaster`, `#lib/toast.svelte.js`)

Action results float as toasts and time out, the same everywhere
(maintainer decision, 2026-11-06), instead of notes under the control.

- **One Toaster**, in the `(app)` layout. Call `toast.success(msg)`,
  `toast.info(msg)`, or `toast.error(msg)` from anywhere.
- **Where:** fixed under the header (8 px below it). From `sm`, top right,
  352 px wide, its right edge on the column's (`max(1.5rem, (100vw −
  80rem) / 2 + 1.5rem)`); on phones, full width less 8 px a side (the
  planned tab bar takes the bottom). Clicks pass through the gaps.
- **Stack:** newest on top, at most 3 (older ones drop off). Errors stack
  above the rest, since they stay. The same toast again (a repeated
  action) replaces the shown one, back on top with a fresh timeout,
  rather than stacking copies.
- **Timeout:** success and info go after 5 s, held while hovered or
  focused (it runs again only when it's neither), then run for the time
  they had left. Errors stay until dismissed. Each has a × ("Dismiss", 40
  px on phones, 32 px from `sm`); dismissing from the keyboard moves focus
  to the next toast's × (not with the mouse, which would hold that one).
- **Per session:** the store is cleared on sign-out and whenever the
  signed-in pages mount or unmount (an expired session sending you to
  /login), so the next person on a shared device never sees them.
- **Look:** `surface`, `rounded-lg`, the menus' shadow, `line` border
  (`danger` at 40% for errors), 14 px `ink` text, and a 16 px icon: a
  sage (`watched`) check in a circle for success, `muted` "i" for info,
  `danger` "!" for errors.
- **Tone:** success when something changed; info when nothing went wrong
  but nothing changed ("Already in your history: …", "Nothing added: …");
  error when it failed. A toast names what it's about when it floats away
  from where it came from ("S01E02 …", "Season 1: …").
- **Above menus:** popovers sit in the top layer, where z-index doesn't
  reach, so the stack is itself a `popover="manual"`, always open. When a
  menu (or a non-modal dialog) opens, the stack is raised (hidden and
  shown again, which makes it the newest top-layer element): at once if
  toasts are showing, otherwise just before the next toast is inserted,
  and only if that menu is still open. Usually nothing is open, so the
  live regions are never hidden; when they are, it's before the new text
  arrives, so it's still announced. Focus on a toast's × is put back after
  a raise. Tooltips don't raise it. A modal dialog (the search overlay)
  is never covered: everything outside it is inert, so toasts there
  couldn't be dismissed; they wait under it.
- **Accessibility:** two live regions are always mounted, so the first
  toast is announced: `role="alert"` for errors, `role="status"`
  (`aria-live="polite"`) for the rest. Toasts never take focus; the page
  keeps its own focus moves (the next card's check after a removal on
  /history, the plays button after a first play). Enter and exit are a
  short 8 px fly (160 / 120 ms) and the stack slides (`flip`), all off
  with reduced motion (checked in script, since Svelte's transitions
  don't follow the CSS rule).
- **What stays inline (`Notice`):** errors that belong to a control or a
  state (a failed load with "Try again", a disabled reason, a field's
  error, an error inside an open menu or confirm) and form results whose
  content must stay in place (a new API token to copy, a changed
  password, admin user changes on their pages).

## Navigation

- **Desktop:** sticky 48 px header, glass: `surface` at 85% with
  `backdrop-blur-xl` and `backdrop-saturate-150`, with no hairline under
  it (maintainer decision, 2026-11-06). Pages scroll under it; title pages
  start their hero under it. It
  holds the wordmark (dashboard),
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
  globally (layout.css); smooth anchor scrolling (the section list) only
  without it.

## Accessibility checklist

AA contrast in both themes (tables above); visible `:focus-visible` ring
(2 px `focus`, offset 2); semantic landmarks and headings in order; labels
on every input; `aria-label` on icon-only buttons; status text in
`role="status"`; keyboard paths for search (/, arrows, Enter, Esc) and
the menu; 40 px touch targets on phones; no horizontal scrolling at
390 px.
