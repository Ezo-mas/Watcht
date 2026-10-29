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
   API token (`Authorization: Bearer wt_...`; other `Authorization`
   headers are ignored). Account and token
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

## Adding a job kind

1. Register a handler in `cmd/watcht` before the queue starts:
   `q.Register("refresh_movie", func(ctx context.Context, payload json.RawMessage) error { ... })`.
   A kind is a stable string: renaming one leaves old queued rows without a
   handler, so they fail as an unknown kind.
2. Enqueue it from the domain package that owns the work with
   `q.Enqueue(ctx, kind, payload, jobs.EnqueueOptions{})`. The payload is
   marshalled as JSON (`nil` stores `{}`); `run_at` defaults to now,
   `max_attempts` to 5, and a `unique_key` makes a second enqueue a no-op
   while one is queued or running.
3. Return `jobs.Permanent(err)` for a failure another attempt can't fix.
   Any other error retries with backoff until `max_attempts`; a panic is
   recovered as an error.
4. For periodic work, call `q.Every(kind, interval)`: it runs at startup and
   every interval, and uses the kind as its `unique_key`. Intervals, not
   cron.
5. The context carries the per-job timeout and is cancelled on shutdown, so
   the handler must return promptly then. Handlers can run more than once
   (a crash after the work but before it's marked done reruns them), so make
   them idempotent, and never log the payload.
6. A handler's error is logged and stored in `last_error`, so it must not
   contain a secret or the payload. Provider requests send API keys in a
   header, never the URL: a `*url.Error` includes the full URL.
7. Test against a migrated temp database (`dbtest.New`) and drive the
   queue's clock instead of sleeping.

## Adding a provider or importer

1. Implement the relevant interface (MetadataProvider, ListProvider,
   SyncSource, or Importer) in its own package under `internal/providers/`
   or `internal/importers/`.
2. Make HTTP calls through the shared provider client in `internal/providers`,
   never `net/http` directly. It gives each attempt a timeout, rate-limits the
   provider, retries what may pass, and caches responses in `provider_cache`.
   Build one client per provider in `cmd/watcht`:
   `providers.New(d, providers.Options{Provider: "tmdb", BaseURL: ..., Header: ..., UserAgent: ...})`,
   then call `client.GetJSON(ctx, "/movie/603", query, ttl, &out)`. The
   defaults are 20 requests a second with a burst of 20, and a 15-second
   timeout per attempt; set `RatePerSecond`, `Burst`, or `Timeout` only when
   the provider's limits differ.
   - **Keys go in `Options.Header`, never in the URL or the path.** The query
     is part of the cache key, so a key there is stored in the database, and
     URLs end up in proxy and access logs. The client refuses a path with `?`
     or `#`.
   - **User input goes in `query`**, which the client encodes. Never put it in
     the path.
   - **`ttl` is how long a 200 response is cached.** Pick it from how fast the
     data changes: the TMDB cache lifetimes in `docs/rewrite-plan.md` (M3) are
     the model. Use `0` only for data that must be live. It turns caching off
     for the call, including the stale fallback below.
   - **Stale fallback:** when the provider is unreachable or returns a 5xx
     after the retries, an expired cache entry is served with a warning. A 4xx
     is never covered by it.
   - **Errors:** a 404 matches `errors.Is(err, providers.ErrNotFound)`. Other
     failures are a `*providers.StatusError` (status and path) or a transport
     error. Neither includes the query, headers, or body, so they're safe to
     log and to show.
   - Redirects are not followed: the key header would be sent to the new host.
     A 3xx comes back as a `StatusError`.
   - Pass a fresh `out` each call. A decode that fails part way can leave it
     partly filled.
   - Cache rows are pruned daily by the `provider_cache_prune` job, which is
     shared by every provider: a new provider adds no job.
3. Return plain structs; let `catalog` and `tracking` store them.
4. Record real responses into `testdata/` and test against them. Strip
   personal data from fixtures before committing. The client's own tests use
   an `httptest` server on loopback; nothing in the suite reaches the network.
5. Add attribution if the provider's terms require it.
