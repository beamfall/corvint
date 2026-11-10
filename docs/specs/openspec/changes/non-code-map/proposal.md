# Change proposal: non-code changes run everything unless a proven map entry narrows them

Change name: `non-code-map`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TEST-002, PROD-TEST-008 and PROD-TEST-012 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/non-code-map-v1.md`, a new spec with its own prefix `NCM-V1`
Requirement IDs this change adds: NCM-V1-001 to NCM-V1-009
Owner answers this change implements: G2, G3, G15b, G16 (`owner-answers-2026-11-09.md`), owner
decision R31 (2026-11-09) and owner decision R32 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/non-code-map/spec.md`.

## Why

Configuration, data, schema and asset files reach tests through reads that no import graph shows.
The owner chose "full by default, proven map" (G15b): a changed non-code file selects every test
that might read it unless an accepted map entry narrows it; recordings may only propose entries; an
entry narrows only after an injected data-file fault proves it is caught; and a fixed list of paths
is never narrowed. G16 adds that a change to Corvint configuration or to the narrowing map runs
everything. Owner decision R31 (2026-11-09) narrows G16: only Corvint configuration files under
`.corvint/` force a full run; an edit of the flow file runs the tests linked to the changed flows;
doc-only edits and the two private ledgers add nothing. Owner decision R32 (2026-11-09) adds that a
change no runner owns, or a removed or unavailable runner, runs all tests with a named reason.

What exists today, verified in `integ` at 9ec3a29d0 (all Go-selection specific, Corvint-shaped):

- `.corvint/non-source-paths.json` (profile `corvint-non-source-paths/0`, entries `{path, reason}`)
  is read by `internal/gateselect/readers.go:217-243` with unknown members rejected, and by
  `internal/liveverify/affected/non_source_paths.go`.
- `internal/liveverify/affected/select.go:987-1005` (`changesCode`): only the classes
  `TASK_STORE_PROJECTION`, `DECLARED_DOCUMENTATION` and `DECLARED_RUNTIME_READERS` count as data;
  every other dirty path counts as possible code (AFP-V0-818, AFP-V0-602, AFP-V0-987).
- `internal/liveverify/affected/driver.go:35-43`: a plan of declared data alone still selects make
  targets and workflow jobs that invoke computed names.
- Further per-package read declarations exist: `.corvint/runtime-readers.json`
  (`internal/liveverify/affected/runtime_readers.go:17`), `.corvint/test-read-scopes.json`,
  `test-read-scope-literals.json`, `test-read-scope-trees.json`, `record-readers.json`,
  `existence-readers.json` and `documentation-paths.json`. These are hand-written declarations with
  a reason string; none carries fault-injection proof, acceptance by a named person, or a pinned
  commit.
- No equivalent exists for Vitest, Jest, Playwright, pytest, RSpec or Minitest repositories
  (NOT_VERIFIED beyond a search of `internal/liveverify/affected/` and the provider packages).

## What Changes

- One map file per repository (proposed name `.corvint/data-map.json`, profile
  `corvint-data-map/0`). Each entry names a path or glob, the tests that read it, the cited evidence,
  who accepted it and when, the pinned commit, and the fault case that proved it.
- A command to propose entries from recorded reads (proposed: `corvint affected --propose-data-map`
  writing to stdout only; read commands stay read-only under invariant 4).
- The planner treats every non-code path without a proven entry as reaching every test of every
  runner in the repository, with a printed reason. Declared documentation paths keep their existing
  `DECLARED_DOCUMENTATION` class and add no test (R31).
- The full-run rule for `.corvint/` (NCM-V1-006) names the excluded paths: the flow file and its
  link records select the tests linked to the changed flows, and the two private ledgers
  (`self-observations.jsonl`, `unplanned-reads.jsonl`, AGENTS.md invariant 4) add nothing (R31).

## Capabilities

### New Capabilities

- `non-code-map`: proven, accepted narrowing for non-code files.

### Modified Capabilities

- `affected-plan`: non-code handling for all seven runners.
- `fault-injection-harness` (FIH-V1): its non-code faults prove map entries.

## Impact

- New declaration file and schema; planner change for non-Go runners. Existing Corvint-specific
  declarations keep their meaning where NCM-V1-001 does not override them: declared documentation
  still adds no test. Earlier wording said they "continue to work unchanged", which contradicted
  NCM-V1-001 for non-documentation classes; whether they later fold into the map is an open
  question for the owner.

## Non-goals

- No automatic acceptance of recorded reads (G15b).
- No narrowing of the paths in NCM-V1-005, ever.
- No change to Corvint's own existing read declarations in this change.
- Kind labels (config, data, schema, asset) are for reporting only and never change selection.
