# Design: `flows-unlinked-tests-report`

## Decisions

- Fresh IDs AFU-V1-155 to 159; AFU-V1-150 to 154 are retired with their integ meanings and never
  reused (owner decision R11, 2026-11-09). The experimental `flowrep` code is reworked to cite the
  new IDs.
- `flows gaps` is native in `corvint`; the `corvint-docs` copy is removed (G5).
- Only person-accepted link records count (owner decision R12, 2026-11-09).
- `corvint.flows.gaps` stays on `corvint mcp`, per its row in the table of change `mcp-tool-table`
  (owner decision R30, 2026-11-09).
- The ratchet is off by default (G11, owner answer 4).

## Failure modes

- Inventory absent or stale: the report says so with the reason and exit code 3 (distinct from a
  clean 0); no `unassigned` list is presented as complete.
- `.corvint/flows.json` missing, malformed or with an unknown profile: typed failure, exit 2.
- More records than the bound: stable order, truncation marker, totals.
- Ratchet enabled without a base: typed failure, exit 2.

## Acceptance evidence

- Fixtures: all linked; one unassigned; one stale link; one dangling link; inventory stale. Go tests
  named for V1-5255 on the command and the MCP tool.
- A release smoke test running `corvint flows gaps` with `corvint-docs` absent from `PATH` (G5).
- NOT_RUN at the time of writing: every test of the current `flowrep` code.

## Rollback

Revert the change's commits; the codes and IDs are retired, not reused. No stored data is written.
The ratchet is off by default, so turning it off needs no migration.
