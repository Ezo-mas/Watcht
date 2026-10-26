# Backend guide — Go server

Layout and package responsibilities are in
[rewrite-plan.md](../rewrite-plan.md#repository-layout). Read
[architecture.md](architecture.md) first if you touch the database,
tracking, auth, jobs, or the API shape.

## Conventions

- **Standard library first.** `net/http` routing, `log/slog`,
  `encoding/json`, `database/sql`. New third-party modules need a reason in
  your summary.
- **Context first.** Functions that do I/O take `ctx context.Context` as
  their first argument and pass it down to queries and HTTP calls.
- **Errors.** Wrap with `fmt.Errorf("doing x: %w", err)`. Domain packages
  define sentinel errors (`ErrNotFound`, `ErrConflict`); `httpapi` maps
  them to status codes and the standard error shape. Never expose raw
  database errors to clients.
- **No package-level mutable state.** Dependencies (database handles,
  config, HTTP clients, clock) are passed in as struct fields, wired in
  `cmd/watcht`. Inject a clock wherever time matters so tests can control
  it.
- **Logging.** `slog` with structured fields. Never log tokens, cookies,
  passwords, or API keys.
- **Formatting.** `gofmt` (the format hook runs it after every edit).
  golangci-lint must pass.

## Adding a query

1. Write the SQL in `internal/db/queries/<area>.sql` with an sqlc
   annotation (`-- name: GetMovieByTMDBID :one`).
2. Run `make generate`.
3. Call the generated method from the domain package, not from `httpapi`.

## Adding a migration

1. Create the next numbered goose file in `internal/db/migrations/`.
2. Use `STRICT` tables, `TEXT` timestamps, and `CHECK` constraints for
   enums.
3. Run `make generate` and `make check`.
4. From v0.1.0 on, never edit an existing migration file.

## Adding an endpoint

1. Add the handler in `internal/httpapi/`, registered under `/api/v1`.
2. Decode and validate input in the handler; put the logic in a domain
   package.
3. Register the route on the API mux with a method and full path
   (`api.handle("POST /api/v1/auth/login", h.login)`). Unknown paths get a
   JSON 404 and wrong methods a JSON 405 with `Allow`, automatically.
   Decode bodies with `decodeJSON` (JSON only, 64 KiB, unknown fields
   rejected) and return errors with `writeError` so the shape stays
   consistent. Unexpected errors are logged and sent as a generic 500;
   never put a raw error in a response.
   Every non-GET request passes `http.CrossOriginProtection` (wired in
   `NewRouter`), so state-changing endpoints must not be GETs.
   Data API handlers use `withUser`, which accepts a session cookie or an
   API token (`Authorization: Bearer wt_...`). Account and token
   management use `withSession` (browser session only; 401 without one,
   403 `session_required` for a token) or `withAdmin` (also 403 for
   non-admins); the domain function takes the acting session and re-checks
   it inside its write transaction.
4. Add an `httptest` test against the real router and a temporary database.
   Server-wide timeouts (`internal/server`) are 30s read and 60s write.
   Long uploads or streaming responses (importers, exports) extend their
   own deadlines with `http.ResponseController`, not by raising the
   server-wide values.
5. Add or update the matching TypeScript types in `web/src/lib/api/`.

## Adding a provider or importer

1. Implement the relevant interface (MetadataProvider, ListProvider,
   SyncSource, or Importer) in its own package under `internal/providers/`
   or `internal/importers/`.
2. Make HTTP calls through the shared provider client (timeouts, rate
   limiting, cache).
3. Return plain structs; let `catalog` and `tracking` store them.
4. Record real responses into `testdata/` and test against them. Strip
   personal data from fixtures before committing.
5. Add attribution if the provider's terms require it.
