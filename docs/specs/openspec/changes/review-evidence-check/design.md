# Design: `review-evidence-check`

## Decisions

- `review` is advisory in every consumer unless that consumer opts in, and is never required by a
  release-blocking gate or report (owner decision R4, 2026-11-09; review docs F3). Blocking consumers
  read `affected` plus the receipt.
- The agent end-of-turn report does not consume `review` (change `agent-packages`, contradiction 9).
- A missing docs input is `NOT_OBSERVED` and does not block `CLEAN`; `stale-docs` findings are
  advisory (R4, F3). This keeps answer 13: a testing-only install can reach `CLEAN`.
- MCP exposure follows the table of change `mcp-tool-table` (owner decision R30; RVE-V1-010).

## Failure modes

- No counted receipt: every selected test is `selected-not-run`, and the input list says why.
- Receipt for another tree: not counted, listed in `inputs` with reason `tree-mismatch`.
- Affected plan `FULL` or `UNKNOWN`: every test of the full set must have a passing outcome, or the
  verdict is `INCOMPLETE`.
- Missing ticket input: that finding kind is `NOT_OBSERVED` and the verdict is at best
  `INCOMPLETE`. Missing docs input or corvint-docs absent: docs is `NOT_OBSERVED` and does not block
  `CLEAN` (RVE-V1-005; earlier text grouped docs with the ticket and said "never `CLEAN`").
- A consumer is configured to require `review` in a release-blocking gate: refused by RVE-V1-009;
  the gate must read `affected` plus the receipt instead.
- Malformed input: typed refusal, exit 2.

## Acceptance evidence

- Fixtures, one per finding kind plus a clean case and each `INCOMPLETE` cause; Go tests per
  requirement.
- An Action dry run rendering the comment from a fixture body (with change `github-action`).
- NOT_RUN at the time of writing: everything.

## Rollback

Revert the change's commits; the guidance-based review returns from the reverted diff. A Tasks gate
that opted in to `review` can drop the opt-in key from queue policy. IDs are retired, not reused.

## Open questions for the owner

- Acceptance criteria have no IDs. Tie evidence to a criterion by its position plus
  `acceptanceRevision`, or add criterion IDs to the ticket schema (a Tasks change)?
