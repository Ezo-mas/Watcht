# Testing guide — canonical verification commands and caveats

The required checks for "done" are in the root `AGENTS.md` ("Definition of
done"). This guide is the full command reference.

## Commands (from the repository root)

```bash
make check       # everything CI runs; the definition of done
make test        # go test ./...
make lint        # go vet and golangci-lint
make build       # bin/watcht
make run         # build and run on :8080
make help        # list targets
```

Planned: web lint/check/test join `make check` when the frontend lands,
and `make generate` (sqlc) arrives with the database in M1.

While iterating:

```bash
go test ./internal/httpapi/...            # one package
go test ./internal/httpapi/ -run Healthz  # one test
```

The Makefile exports `GOTOOLCHAIN=auto`, so Go uses the version pinned in
`go.mod` even where the distro defaults to a local toolchain.
golangci-lint runs through `go run` at the version in `.golangci-version`;
the first run compiles it (about a minute), later runs are cached. Don't
install Go tools globally.

## Writing tests

- **Database tests** use the test helper in `internal/db`, which creates a
  temporary SQLite file with migrations and pragmas applied. Never open a
  database another way in tests, and never mock the database.
- **HTTP tests** use `httptest` against the real router.
- **Time** is controlled through the injected clock, never `time.Sleep`.
- **Table-driven tests** for rules with many cases: dedupe windows, the
  85% watch rule, episode matching, time-zone day boundaries (include a DST
  change).
- **Provider fixtures** live in `testdata/` next to the package. They are
  recorded from real responses, trimmed, and stripped of personal data.
  Refresh a fixture by re-recording it, never by hand-editing it to make a
  test pass.
- **No network.** A test that needs the internet is a bug.

## Caveats

- **Skips are not passes.** Name every skipped test and its reason in your
  summary.
- A check failure is reported, never suppressed. Don't skip tests, weaken
  assertions, or silence errors to make verification pass; fix the cause.
- If `make check` fails because sqlc output is stale, run `make generate`
  and include the regenerated files. Don't edit generated code.
