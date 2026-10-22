# Affected adapters: Python continuations and pytest edges, strict go.work, nested-module unknown

Tickets V1-2297, V1-2296, V1-0874 and V1-0922. Requirements AFP-V0-187 to AFP-V0-191 in
`docs/specs/affected-plan-v0.md`, accepted under the owner's standing auto-accept (2026-10-11).
This work is stacked on PR #1469 (`claude/affected-1021n`, AFP-V0-181 to AFP-V0-186), whose
from-import name loop AFP-V0-187 extends.

## Problems (each verified against the lane base before building)

- **V1-2297.** `scanImports` read only the first physical line of a statement, so
  `from pkg import (` followed by one name per line read none of the names; the indented
  continuation lines also looked like conditional imports.
- **V1-2296.** pytest loads `conftest.py` and the modules named by `pytest_plugins` without an
  import, so a change to either selected only tests that imported it (usually none).
- **V1-0874.** `workspaceDirectories` read `go.work` with a loose line parser while the
  nested-module check (AFP-V0-031) used the strict `readManifest`. A raw-string `use`, two
  directories on one line or an unknown directive made the loose reader guess a use set and the
  plan stayed bounded.
- **V1-0922.** A change inside an unobserved nested Go module reported `UNINDEXED_SOURCE_PATH`,
  which does not say why the file has no unit.

## Decisions

- Continuations: join lines (comments removed) until `(`/`[` balance or no trailing backslash; the
  joined statement goes through the existing `parseImportLine`.
- pytest: literal `pytest_plugins` names become absolute imports; a non-literal value raises
  `python:pytest-plugins-unresolved`. Every test gets an edge to each `conftest.py` at or above its
  directory. Plugins registered outside `.py` files stay a documented gap.
- go.work: one reader. A refusal raises `go:workspace-unparsed` and observes the root module only,
  so AFP-V0-031 keeps every nested module open. Fail-closed rather than guessing.
- V1-0922: the ticket allows a nested-module-specific unknown, and indexing nested modules is not
  root-runnable (PR #1420 design note). Chosen: a generic `Result.Unobserved` seam and a
  `LANGUAGE_FRONTIER` detail `go:unobserved-directory:<dir>:<path>`. The reason enum is unchanged,
  so the core-compat freeze table is untouched; the detail is not in the AFP-V0-028 allowlist, so
  the gate tier stays full. The digest member is `omitempty`, so graphs without unobserved
  directories keep their digests. A directory that is or contains an observed module is left out.

## Evidence

- The AFP-V0-187 to AFP-V0-190 tests failed against the pre-change source (temporary revert in
  the lane); the AFP-V0-191 tests do not compile without the new `Result.Unobserved` seam. All pass
  after: `TestAFPV0187*`, `TestAFPV0188PytestPluginsAreImports`,
  `TestAFPV0189ConftestSelectsEveryTestBelowIt`, `TestWorkspaceUseSetIsTheStrictReaders_AFPV0190`,
  `TestNestedModuleChangeIsANestedModuleUnknown_AFPV0191`,
  `TestUnobservedLeavesOutDirectoriesHoldingObservedModules_AFPV0191`,
  `TestUnobservedDirectoriesAreValidatedAndDigested_AFPV0191`.
- `TestIndependentNestedModulesCloseTheFrontier_V1_0867` now expects the AFP-V0-191 detail.
- Package runs, affected tiers and gates are listed in the PR body.

## NOT_RUN

- `make gate` and affected tier 2 (not required for scoped lane work).
- External Python corpus qualification (AFP-V0-186 remains unobserved).

## Rollback

Revert this change. Each requirement names its own narrower rollback in the spec.
