# Testing guide — canonical verification commands and caveats

The required checks for "done" are in the root `AGENTS.md` ("Definition of
done"). This guide is the full command reference.

## Commands (from the repository root)

```bash
make check         # everything CI runs except Docker; the definition of done
make check-go      # go vet, golangci-lint, go test
make check-web     # svelte-check, prettier, eslint, vitest, vite build
make smoke         # full build, then start the binary and check it
make build         # frontend, then bin/watcht with it embedded
make docker-smoke  # build the image, run it, wait for healthy, smoke-test it
make run           # build and run on :8080
make dev           # Go on :8090 and Vite on :5173 with hot reload
make help          # list all targets
```

`make generate` regenerates `internal/db/sqlc` after a migration or query
change; `make check` fails if it's stale. It needs a C compiler (sqlc uses
CGO; the binary doesn't). CI runs
`check-go`, `check-web`, `smoke`, and the Docker build with
`scripts/docker-smoke.sh` as separate jobs. `make check` runs all but the
Docker job, which needs Docker locally: run `make docker-smoke` for
Docker changes.

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

- **Database tests** use `dbtest.New(t)` (`internal/db/dbtest`), which
  creates a temporary SQLite file with the production pragmas and all
  migrations applied, and closes it when the test ends. Never open a
  database another way in tests, and never mock the database. Tests of
  migration mechanics themselves (`internal/db`) use `db.Open` with
  in-memory `fstest` migrations.
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
