# Tech Debt
Serious, confirmed, deferred issues only. See `AGENTS.md` for logging rules.

Entry format:

### Short title
- **Where:** `path` (`function`)
- **Problem:** What is wrong
- **Impact:** What breaks, and when
- **Evidence:** How it was confirmed
- **Logged:** YYYY-MM-DD

## Open

### User IDs can be reused after a delete
- **Where:** `internal/db/migrations/00001_users.sql` (`users.id INTEGER PRIMARY KEY`, no `AUTOINCREMENT`)
- **Problem:** SQLite gives a new row `max(id)+1`, so deleting the newest account frees its ID for the next one.
- **Impact:** Today it's contained: every table holding a user ID has a foreign key with `ON DELETE CASCADE`, and in-flight requests are re-checked by session token hash or password hash (M2 slice 3). It breaks if a future table or a JSON payload (M3 job payloads, import runs) stores a user ID without a cascading foreign key: work queued by a deleted user would land on whoever gets the ID next. An admin acting from a stale list could also delete or reset the new account. Adding `AUTOINCREMENT` to `users` is a one-line baseline change before v0.1.0 (dev databases recreated), and a table rebuild after.
- **Evidence:** M2 slice 3 review; `TestDeletedAdminCantActThroughAReusedID` and `TestLoginRacingDeleteAndIDReuse` (`internal/auth/accounts_test.go`) reproduce the reuse.
- **Logged:** 2026-10-26
