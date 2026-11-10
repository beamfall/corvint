# Design: `feature-map-from-flows`

## Decisions

- The map is an output of the existing `corvint-docs docs flows generate`, not a new verb (settled
  without the owner, 2026-11-09).
- Only the flow file and accepted link records feed the map; no regex inference (invariant 3).
- Links count only when a person accepted them (owner decision R12, 2026-11-09).
- MCP: docs tools live on one `corvint-docs mcp` server; any feature map tool follows its row in the
  table of change `mcp-tool-table` (owner decision R30, 2026-11-09; FMP-V1-007).
- Preview: the map is never a release gate.
- The output profile is `corvint-feature-map/0`. The `/0` marks preview (review docs F13).
- The removed-verb notices last for 1.0.x and are deleted in 1.1 (review docs F13).

## Failure modes

- `.corvint/flows.json` absent: the map is empty with state `NOT_OBSERVED` and the reason; no feature
  is inferred.
- Unknown flow-file profile: typed refusal, exit 2.
- A flow with no accepted test link: listed with `untested: true`.
- A proposed, unaccepted declaration: not in the map; counted under `proposed-unaccepted`.

## Acceptance evidence

- Fixtures: two flows; untested flow; pending proposal; no flow file; unknown profile; Go tests per
  requirement.
- A test that the old verbs print the notice and exit 2.
- The removal diff with passing `go vet ./...`.
- NOT_RUN at the time of writing: everything.

## Rollback

Revert the change's commits; the old verbs and heuristics return from the reverted diff. No stored
data is written. IDs are retired, not reused.

## Open questions for the owner

- (Settled without the owner: the map is an output of the existing `docs flows generate`, not a
  new verb.)
