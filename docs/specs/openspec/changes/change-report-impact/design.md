# Design: `change-report-impact`

## Decisions

- One engine: the tests section is the affected plan; no second test-selection engine (G25).
- The flows section reuses the native `corvint flows impact` of change `flows-status-report`
  (AFL-V1-007); that verb and `corvint.flows.impact` stay (review docs F2, applied now).
- Docs come in through a saved `corvint-docs docs flows check` file only (owner answer 13).
- Preview and advisory: no release-blocking gate or report requires `impact` (owner decision R4,
  2026-11-09; CRI-V1-008).
- MCP: `corvint.impact` stays on `corvint mcp`; its row in the table of change `mcp-tool-table`
  fixes name, schema, profile and error codes (owner decision R30, 2026-11-09; CRI-V1-009).

## Failure modes

- Base not an ancestor or not a full SHA: typed refusal, exit 2.
- Affected plan `FULL` or `UNKNOWN`: the tests section says so with the plan's reasons; never an
  empty list as complete.
- Docs check file for another revision: docs section `INCOMPLETE` with the mismatch.
- Tasks store absent or unreadable: tickets and ticket-derived requirements `NOT_OBSERVED`.
- Section over its bound: stable order, `truncated: true`, `total`.

## Acceptance evidence

- Fixture repository with Go and Playwright tests, two flows, a docs check file and a scratch Tasks
  store; Go tests per requirement.
- A comparison test of `impact` against `affected` (CRI-V1-001) and against `flows impact`.
- The removal diff and a passing `go vet ./...` (CRI-V1-007).
- NOT_RUN at the time of writing: everything.

## Rollback

Revert the change's commits; the old engine and command return from the reverted diff. No stored
data is written. IDs are retired, not reused.

## Open questions for the owner

Still undecided by the owner (D21 part i); not settled by owner decisions R4 or R30.

- Decided, R42 (panel pick) C10: both commit-message IDs and claim scope, each labelled, cited
  tickets first (CRI-V1-005).
- Should `impact` default `--base` to the merge base with `origin/main` when omitted, or keep it
  required (proposed: required)?
