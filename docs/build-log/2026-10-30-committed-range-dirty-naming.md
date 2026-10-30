# Committed range paths keep the dirty naming (V1-2073)

Date: 2026-10-30. Ticket: V1-2073. Spec: `docs/specs/affected-plan-v0.md` AFP-V0-010.

## Question

`corvint affected --base BASE` on a clean worktree lists committed range paths under `plan.dirty`
with reason `UNOWNED_DIRTY_PATH`. The V1-1894 closeout saw `.corvint/test-read-scope-literals.json`,
`.github/workflows/ci.yml` and `Makefile` reported this way, which widened batch-drift to every
drift test. Is this a planner defect, and should committed paths get a distinct label?

## Finding

Not a classification defect. `compileAffectedGraph` unions the worktree status with `RangePaths`
before `Select`, which sees only paths, so a committed path is classified exactly as the same
uncommitted change. The three paths are unowned on their merits: no unit lists them, `Makefile`
and `.github/workflows/` are build/CI driver inputs AFP-V0-601 rule (3) never attributes, and the
read-scope-literals record is read only by `tools/test-read-scopes-check`, not by a graph unit.
Treating them as owned because they are committed would narrow the plan with no evidence of their
readers (AGENTS.md invariant 2). The drift widening comes from those paths' own changes, not from
their commit state.

## Decision

Keep `plan.dirty` and `UNOWNED_DIRTY_PATH` for committed paths; no rename, so no wire change under
the affected-plan compatibility rules. "Dirty" means changed relative to the comparison point;
`range.paths` already separates the committed contribution. Recorded in AFP-V0-010.

## Evidence

`TestCommittedRangePathsClassifyExactlyAsWorktreePaths_V1_2073` (internal/liveverify/affected):
the V1-1894 shape on a clean worktree, the same change soft-reset to uncommitted (identical paths,
byte-identical plan), and an uncommitted path over the range (still dirty, unowned, absent from the
range paths).

## Rollback

Remove the AFP-V0-010 paragraph and the test; behaviour is unchanged by this entry.
