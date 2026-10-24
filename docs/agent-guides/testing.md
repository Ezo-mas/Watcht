# Testing guide — canonical verification commands and caveats

The required checks for "done" are in the root `AGENTS.md` ("Definition of
done"). This guide is the full command reference. The commands exist from
milestone M0 on.

## Commands (from the repository root)

```bash
make check       # everything CI runs; the definition of done
make test        # go test ./... and the web tests
make lint        # go vet, golangci-lint, web lint
make generate    # sqlc generate; commit the result
make build       # web build, then the Go binary with it embedded
make dev         # Go server on :8080 and Vite on :5173
```

While iterating:

```bash
go test ./internal/tracking/...          # one package
go test ./internal/tracking/ -run Dedupe # one test
```

Go and its tools come from the versions pinned in `go.mod` (`go tool sqlc`,
`go tool goose`). Don't install them globally.

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
