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
- Covered means an accepted, non-stale link to a test of any kind; `no-test` applies only when
  there is none (owner answer 9, AFU-V1-166).
- A flow with no linked E2E test gets a `no-e2e-proof` record that never blocks (owner answer 15,
  AFU-V1-167). The record is per flow, not per variation.

## Open decisions

- Settled (decided 2026-11-09 under R42, N1): the codes are additive under
  `application-flow-gaps/1`. `unassigned`, `stale-link`, `dangling-link` and `no-e2e-proof` are new
  values of the existing code field; the profile text gains one sentence: a reader MUST ignore a
  code it does not know (AFU-V1-155). Earlier wording, superseded by N1: "The proposal leaves
  'additive value versus new version' to the lane's first spec commit. Not decided by the owner."
- Settled (decided 2026-11-09 under R42, N2): a link record does not state the kind. It keeps its
  present members (no `kind`); the report reads the kind from the test inventory by the canonical
  `test_id`, with `unit` as the default and the kind source reported (`affected-plan-kinds`
  decision 2). A link whose `test_id` is not in the inventory is `dangling-link` (AFU-V1-166).
  Earlier wording, superseded by N2: "This change reads the kind and does not define it. Not
  decided here."

## Failure modes

- Inventory absent or stale: the report says so with the reason and exit code 3 (distinct from a
  clean 0); no `unassigned` list is presented as complete.
- `.corvint/flows.json` missing, malformed or with an unknown profile: typed failure, exit 2.
- More records than the bound: stable order, truncation marker, totals.
- Ratchet enabled without a base: typed failure, exit 2.

## Acceptance evidence

- Fixtures: all linked; one unassigned; one stale link; one dangling link; inventory stale; a
  variation covered by a unit test only; a flow with no E2E link (`no-e2e-proof`, exit 0). Go tests
  named for V1-5255 on the command and the MCP tool.
- A release smoke test running `corvint flows gaps` with `corvint-docs` absent from `PATH` (G5).
- NOT_RUN at the time of writing: every test of the current `flowrep` code.

## Rollback

Revert the change's commits; the codes and IDs are retired, not reused. No stored data is written.
The ratchet is off by default, so turning it off needs no migration.
