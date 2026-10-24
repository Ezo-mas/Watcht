---
paths:
  - "internal/db/**"
  - "internal/tracking/**"
  - "internal/auth/**"
  - "internal/jobs/**"
  - "internal/httpapi/**"
---

# Load-bearing code

Read `docs/agent-guides/architecture.md` before changing this code, not
after. Its invariants are easy to break by accident: one write connection
through `internal/db`, migrations backed up first and forward-only after
v0.1.0, generated sqlc code never hand-edited, UTC storage, TMDB episode
identity, watch events only through `internal/tracking`, hashed secrets,
and one API error shape.

Changes under `internal/auth/`, webhook authentication, or request logging
need a security review: see `SECURITY.md`.
