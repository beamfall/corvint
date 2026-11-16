# V1-5623: native flow status, impact and gaps reports; one docs pages group (AFL-V1-001..008)

Delivery of the OpenSpec change `docs/specs/openspec/changes/flows-status-report/` as rewritten on
2026-11-09, on lane `claude/r1flows`, base `56bfe620b7b69bd045a2748f7150057a7c2ecf03` (the head of
lane r1cleanup: origin/main `df315e350` plus its cleanup commits). It serves PROD-TEST-010 and the
PROD-DOCS-011 boundary rule (owner answer G5). V1-5257 was closed against the first version of the
change (the report inside the `corvint-docs` command group, AFU-V1-160..165); V1-5623 builds the
rewrite. The canonical spec is `docs/specs/application-flow-status-v1.md`; its intent stays
`proposed`, and delivery is `experimental`.

## What was built

- Spec (task 2). `docs/specs/application-flow-status-v1.md` holds AFL-V1-001..008 copied from the
  spec delta, with its README row and the regenerated requirement index, in the lane's first
  commit, together with the AFL-V1-008 names in the change's design.
- Native reports (tasks 3, 5; AFL-V1-001, 005, 006, 007). The handlers of `flows status`,
  `flows impact` and `flows gaps` moved, not copied, from `internal/command/flows` (docs layer) to
  the tests-layer package `internal/flowreports`, keyed in `tools/import-ratchet/areas.json`;
  `runFlowsShim` runs them in `corvint` before it would forward to `corvint-docs`. The joins stay in
  `internal/appflows`, which the shared MCP bridge also calls (see the change design). Each
  `corvint-docs flows status|impact|gaps` is a refusal naming `corvint flows VERB`.
- Result sources and granularity (task 4; AFL-V1-002). `appflows.ReadResults` reads only the two
  writers: the run evidence store, which holds every `corvint test` receipt (RVD-V1-004) and every
  recorded `flows ingest` document, then each `--evidence FILE`; `result_sources` lists every
  source and its record count. A direct `--receipt FILE` and default-receipt read was built in commit
  `a6162b50a`, then withdrawn in the closing commit: `TestSTRV0011ReceiptsHaveNoOtherReader` (STR-V0-011) admits no other
  receipt reader without an accepted requirement, and the change is still `pending-owner`. A receipt row is file level and reaches a title link of its file with
  `granularity: file`; an ingested record has `granularity: test`. A malformed, oversized or
  other-profile file refuses the whole report. `--provider FILE` adds `inventory` and `unassigned`.
- `--base` (task 6; AFL-V1-004). The report keeps exactly the flows and the variations
  `flows impact` names (before, every variation of a reached flow).
- MCP (task 7; AFL-V1-003, 007). `corvint.flows_status` takes `provider` and reads
  through `ReadResults`; the table rows of `corvint.flows_status`, `corvint.flows_impact` and
  `corvint.flows_gaps` name `corvint flows VERB` and the parity test
  `corvint TestMTTV1009NativeFlowReportToolsMatchCLI`; `docs/MCP-SERVER.md` is regenerated.
- One docs pages group (task 7a; AFL-V1-008). `corvint-docs docs pages generate|check|finalize|
  flow|flow-maintain|maintain`; `docs flows ...`, `flows docs [maintain]` and `docs maintain` are
  aliases that print one notice and run the same handler until the next release. Help, the command
  tree and `docs/CAPABILITIES.md` are regenerated.

## Evidence

- Focused Go tests, through `script/heavy.sh`, one package per command: `internal/flowreports`,
  `internal/appflows`, `internal/command/flows`, `internal/mcp/bridge`, `internal/capcatalog`,
  `cmd/corvint-docs`, `cmd/corvint` and `tools/flows-smoke`; results are in the lane handback.
  `cmd/corvint-docs` `TestPRBV0008EveryNamedDocsCommandWritesProfiles` fails identically at the
  base commit on this host (not caused by this change).
- FSV-V1-007 smoke. `go run ./tools/flows-smoke --binary` on a `corvint` built from the lane head
  printed `flows-smoke: PASS ...` and exited 0 with no other corvint executable on the smoke's
  PATH; the same command on a `corvint` built from a clean worktree of the base exited 1, each
  report failing with `not-installed` (corvint-docs NOT_INSTALLED). Outputs are kept under
  `~/lanes/r1flows-evidence/` on the build host, outside the repository. The release-workflow job
  that runs the smoke on a candidate archive is a separate change (`.github/`, not edited here).

## Limits and open items

- Intent acceptance: the change is `proposed`; only the owner accepts it.
- Task 1 (`requirementRefs` on the ticket) and the Tasks-gate half of task 8 are the
  orchestrator's; neither was observed in the build lane.
- AFL-V1-008 asked the rename to land before the command table of change `mcp-tool-table` was
  written; that change was archived on 2026-11-15, before this lane. Its table names no docs page
  command, so no row changed. Change `user-docs` is not written yet.
- The design's open questions (moving `flows ingest`; per-title Playwright outcomes in
  `corvint test` receipts) are unchanged.

## Rollback

Revert the lane's commits: `runFlowsShim` again forwards `flows status|impact|gaps` to
`corvint-docs`, whose handlers return from the reverted diff, and the docs commands return to their
old names. No stored data is written.
