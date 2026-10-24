---
name: milestone
description: Work through a watcht rewrite milestone (M0–M9 in docs/rewrite-plan.md) or a slice of one, from reading the plan to a reviewed, checked change ready for the maintainer. Use whenever the user asks to start, continue, or finish a milestone, or names a piece of work that belongs to one.
---

# Work a milestone

The maintainer reviews code rather than writing it, so every milestone
slice must end as a change that is small enough to review, passes the
checks, has had an independent review, and leaves the docs accurate.

## 1. Scope the slice

1. Read the milestone in `docs/rewrite-plan.md` and its "Done when" line.
   Read `AGENTS.md`, the guides it points to for the areas you'll touch,
   and `docs/TECH_DEBT.md`.
2. Check what's already done: `git log --oneline -20`, and look at the
   code the milestone builds on.
3. If the milestone is too big for one reviewable change (roughly more
   than a few hundred lines of non-generated code), split it into slices
   that each run and pass `make check` on their own. Tell the user the
   slices and which one you're starting.
4. If anything in the plan is ambiguous or seems wrong once you see the
   code, stop and ask. Don't silently decide something the plan covers.

## 2. Branch

Work on a branch named `m<N>-<short-name>` (for example `m1-database`),
created from an up-to-date `main`. Never work on `main` directly.

## 3. Build

- Follow the plan's stack and conventions and the AGENTS.md
  non-negotiables.
- Hand well-defined pieces (a query file and its tests, a handler, a
  component) to the `watcht-builder` agent with a precise brief. Keep
  design decisions, cross-package wiring, and anything in the
  architecture guide's invariants in the main session.
- Hand UI layout and styling to the `watcht-designer` agent.
- Write tests with the code, not after it. The rules for critical logic
  in the plan's "Testing" section are required, not optional.

## 4. Verify

1. Run `make check`. Fix failures at the cause; never weaken a test.
2. For UI changes, make sure the visual check was done (or say why not).
3. Check the milestone's "Done when" line literally: do what it says
   (build the binary, run the image, walk through setup) and note the
   result.

## 5. Review

1. Run the `watcht-reviewer` agent on the branch diff with a brief that
   names the milestone, the slice, and its goal.
2. Fix blockers. For each risk or note, either fix it or say why not.
3. If the change touches anything in `SECURITY.md`'s list, run
   `/security-review` (or say it still needs running).

## 6. Update the docs

In the same change:

- `AGENTS.md` and `docs/agent-guides/`: real commands, paths, and new
  conventions. Remove anything the change made untrue.
- `docs/rewrite-plan.md`: mark what's done, and record any decision the
  user made along the way.
- `README.md`: only when user-facing behavior or setup changed.
- `docs/TECH_DEBT.md`: confirmed, deferred, serious issues only.

## 7. Report

Don't commit or push unless the user asks. Report:

- What the slice does and how it maps to the milestone's "Done when"
- Files changed, grouped by area (generated code listed separately)
- Check results: pass, fail, or skipped, with each skip's reason
- The reviewer's verdict and what you did about each finding
- Decisions the user should look at during review, and what's next
