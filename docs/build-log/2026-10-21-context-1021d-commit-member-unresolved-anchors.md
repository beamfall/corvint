# Context lane 1021d: `commit` beside the tree `revision`, and unresolved-anchor disclosure

Date: 2026-10-21. Lane: context-1021d, branch claude/context-bugs-1021d (base 0106d06cd).
Tickets: V1-2127, V1-1010, V1-1015 (delivered); V1-1285 (already in flight); V1-1269 (skipped).
Requirements TCP-V0-150 and TCP-V0-152 in `docs/specs/task-context-packet-v0.md`, accepted under
the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- V1-2127: the packet's `revision` is the HEAD tree OID. Consumers that wanted the commit had to
  run Git themselves.
- V1-1010: a task naming only ticket IDs (V1-0523, V1-0465 ...) selected no governing spec, and
  the packet said nothing about why. The index holds no Tasks ticket state (TCP-V0-121), and
  4-digit ticket IDs are not requirement IDs.
- V1-1015: a task naming the struct field `ExpectedTests` did not carry `types.go`. Struct fields
  are not indexed declarations, so the symbol lane silently found nothing.
- V1-1285: a requirement ID that no tracked spec defines was dropped silently.

## Decisions

- Orchestrator decision: `commit` is a separate, optional member carrying the full HEAD commit.
  `revision` stays the tree OID. The member is additive (CCF-V1-006), and both the task packet and
  `context defs` carry it (TCP-V0-150).
- `coverage.unresolved_anchors` (TCP-V0-152) has `count` and at most 10 `samples` of
  `{anchor, reason}`, and appears only when there is at least one anchor. It covers three cases:
  (a) a requirement ID with no defining clause; (b) a ticket-shaped ID (`-NNNN`); (c) a
  camel/snake identifier with no indexed declaration. This is a disclosure, not a resolution.
  Ranking weights are unchanged.
- V1-1285 is ALREADY-IN-FLIGHT: #1427 (TCP-V0-141 worktree coverage) and #1409, both in batch
  #1430. The duplicate worktree disclosure was removed from this lane. TCP-V0-152(a) adds only the
  abstention for undefined IDs. Replay at 0106d06: `Implement STR-V0-008 and STR-V0-013` already
  routes to `docs/specs/session-test-runner-v0.md` ("defines STR-V0-008").
- V1-1269 is SKIPPED. It was replayed at 136dfa483 with two wordings. Wording 1 carries
  `tools/gate-affected-select/main.go` and the Makefile, but not `cmd/corvint/affected.go`.
  Wording 2 carries `cmd/corvint/affected.go` and gate-affected-select, but not the Makefile.
  `coverage.uncertainty` already states that 5422 code rows and 1504 documentation rows were
  omitted by the limit of 25. Closing the gap needs a ranking change, and the charter forbids that
  without a frozen-evaluation result.
- The fixture commit in `cmd/corvint` (`gitFixture`) now has pinned author and committer dates,
  so the default-wire golden can carry a stable `commit`.

## Evidence

- `TestTaskContextCarriesHeadCommitBesideTreeRevision` covers the clean and dirty cases for the
  task packet and `context defs`.
- `TestTaskContextDisclosesUnresolvedIDAnchors` covers all three anchor classes and the absence
  of the member on a plain task.
- Both tests failed without the fix: `commit` was nil, and the anchor member was missing.
- These suites pass: the focused `./internal/contextindex` run (TestTaskContext*,
  TestAnalyzerSchema*, TestLookup*, TestContextRecipe*) and the `./cmd/corvint` context tests.
- The analyzer schema was bumped to corvint-analyzer/136. The orchestrator renumbers, because
  peers hold 136 and 137.
- `TestCoreVerbsEmitTheFrozenProfiles` fails in cem_status and cem_verify only because the host Go
  is go1.27.3 and the golden pins go1.27.2. This is an environment mismatch and predates this lane.

## NOT_RUN

`make gate` (owner preference for scoped work). The full `./cmd/corvint` suite was not run;
only the focused tests were.

## Rollback

Revert the lane commits. `commit` and `unresolved_anchors` are additive, so no consumer depends on
them yet.
