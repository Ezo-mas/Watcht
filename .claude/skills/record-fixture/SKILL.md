---
name: record-fixture
description: Record, trim, and anonymize provider responses (TMDB now; Jellyfin, Plex, MDBList later) into a package's testdata/ for fixture-based tests. Use before handing provider or importer work to a builder, whenever a test needs a new or refreshed fixture, or when a review finds a rule with no recorded case.
---

# Record a provider fixture

Provider tests run only against recorded responses in `testdata/` next to
the package (AGENTS.md, non-negotiable 8). A fixture is a real response,
trimmed and stripped of personal data, never written or edited by hand to
make a test pass. Fixtures are recorded by the main session *before* a
builder starts, so every builder works from the same data and nobody needs
the network or a credential.

## 1. List the cases before recording anything

The common miss is recording the obvious responses and leaving a rule in the
brief with nothing that exercises it. Go through the brief (or the code) rule
by rule and write down, for each one, a response that makes the rule
*visible*:

- **Every branch of a filter or mapping:** e.g. a search that contains the
  kind of result that gets dropped (a person in a TMDB multi search), each
  status value a rule treats differently (`Ended`, `Canceled`, `Returning
  Series`).
- **Empty and missing:** an empty result set *from the same endpoint* the
  code calls (not a similar one), nulls, absent optional objects.
- **Errors:** a 404 for each kind of resource the code fetches, if their
  bodies differ.
- **Values that tell two fields apart:** if the code picks `name` over
  `original_name`, or takes an ID from the request rather than the response,
  find a response where they differ. If no real response has that, say so:
  the builder covers it with an inline converter test, not an edited fixture.
- **Lookups that return each result type** (a TMDB `find` that returns a
  movie, a show, an episode, and nothing).

Probe candidates first (record into the scratchpad, inspect with `jq`) and
only copy the ones that show the case into `testdata/`.

## 2. Record

```bash
scripts/record-fixture.sh tmdb internal/providers/tmdb/testdata/movie_603.json /movie/603 append_to_response=external_ids
```

- The credential comes from a file (TMDB: `~/.config/watcht/tmdb-token`, the
  v4 API Read Access Token; override with `WATCHT_TMDB_TOKEN_FILE`) and goes
  only in a header. Never put a credential in a URL, a command line, or chat.
  If the file is missing, ask the user to create it (mode 600) themselves.
- Record exactly the request the code will make: same path, same
  `append_to_response`, same query parameters. A fixture served for a
  different URI than it was recorded from hides mismatches.
- Name files after the request: `tv_1396_season_1.json`,
  `find_tvdb_349232.json`, `search_multi_empty.json`.
- A new provider: add a `case` to `scripts/record-fixture.sh` with its base
  URL, credential file, and header.

## 3. Trim (drop only, never change)

Keep files small enough to review. Trimming may only *remove* data:

- Cut long arrays to the first few items: `jq '.results |= .[:5]'`. Leave
  totals and paging fields as recorded.
- Drop large sub-objects the code doesn't read: `jq 'del(.crew, .guest_stars)'`.
- Never change a value, reorder to suit a test, or add a field. If a test
  needs data the response doesn't have, record a different response.

## 4. Anonymize (media servers and personal exports)

TMDB responses hold no personal data. Jellyfin, Plex, and import files do:
user names, user and server IDs, device names, IP addresses, file paths,
access tokens. Replace each with a consistent placeholder (the same real
value always becomes the same fake one, so references still line up), using
`jq` or `sed` on the recorded file, and note each replacement in the
README. Then check: `grep` the fixture for the real token, user name, and
server name; all must return nothing.

## 5. Document and commit

- Add a row per file to the package's `testdata/README.md`: the file, the
  exact request, and what case it shows. State the recording date and every
  trim or replacement applied.
- Remove fixtures no test loads any more.
- Commit fixtures on the slice branch as their own commit
  (`Test(<pkg>): recorded <provider> fixtures`) before builders start, so
  every builder's worktree has them.
- Tell the builder which fixture shows which case, and which rules have no
  recorded case and need an inline test instead.
