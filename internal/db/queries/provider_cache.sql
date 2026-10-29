-- The provider response cache. internal/providers owns the rules (what is
-- stored, expiry, stale fallback, pruning); these are the single statements.

-- GetProviderCache returns the row whether or not it has expired: the caller
-- compares expires_at with its clock, because an expired row is still used
-- when the provider is down.
-- name: GetProviderCache :one
SELECT response, expires_at
FROM provider_cache
WHERE provider = sqlc.arg(provider) AND key = sqlc.arg(key);

-- UpsertProviderCache stores a fresh response, replacing any earlier row.
-- name: UpsertProviderCache :exec
INSERT INTO provider_cache (provider, key, response, fetched_at, expires_at)
VALUES (
    sqlc.arg(provider),
    sqlc.arg(key),
    sqlc.arg(response),
    sqlc.arg(fetched_at),
    sqlc.arg(expires_at)
)
ON CONFLICT (provider, key) DO UPDATE SET
    response = excluded.response,
    fetched_at = excluded.fetched_at,
    expires_at = excluded.expires_at;

-- DeleteExpiredProviderCache deletes rows that expired before cutoff and
-- returns how many, for logging.
-- name: DeleteExpiredProviderCache :execrows
DELETE FROM provider_cache WHERE expires_at < sqlc.arg(cutoff);
