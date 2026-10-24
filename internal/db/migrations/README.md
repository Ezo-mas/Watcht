# Migrations

Goose SQL migrations, embedded into the binary and applied at startup by
`db.Migrate` (after a backup when the database already has data).

- Name files `NNNNN_short_name.sql` with the next number, and start each with
  `-- +goose Up`. Wrap triggers and other multi-statement blocks in
  `-- +goose StatementBegin` / `-- +goose StatementEnd`.
- Tables are `STRICT`; timestamps are UTC RFC 3339 `TEXT`; enums are `TEXT`
  with a `CHECK`.
- Add tables with the feature that needs them, not ahead of time.
- Don't use `-- +goose NO TRANSACTION` unless a statement requires it, and
  never register Go migrations that run outside a transaction: the write
  pool has one connection, so they deadlock. The connection keeps
  `foreign_keys=ON`; a migration that turns it off must turn it back on.
- Before v0.1.0 the baseline may be rewritten. From v0.1.0 on, never edit or
  delete a released file; add a new one.
