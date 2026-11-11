# V1-5434: compatibility-promise stopped at the CMP-V1-003 count (7 of 22 surfaces tested)

Change `docs/specs/openspec/changes/compatibility-promise/`, ticket V1-5434, base
`87c57df657cbe6fe132f2cc8ed31b37824991ac0`, branch `claude/cmp`. Owner decisions D10 and D10a
(2026-11-09, under R42) are recorded in decision 0596. The spec
`docs/specs/compatibility-promise-v1.md` (CMP-V1-001 to CMP-V1-006) was created from the change's
spec delta (task 1.2).

## Outcome: stop rule of CMP-V1-003

CMP-V1-003 says: "If fewer than half of these surfaces have a contract test, the lane MUST stop
and report that to the owner before it writes the table." Design failure mode: "the lane stops and
reports to the owner before it writes the table. This is the condition the decision named as one
that would change it."

Counted at HEAD of this branch, **7 of the 22 CMP-V1-003 surfaces have a contract test and 15 have
none**. That is fewer than half, so `docs/COMPATIBILITY.md`, its generator and its check were
**not written** (tasks 2.2 to 2.4 are not started). This entry is the report to the owner. The
decision it needs: keep the narrow promise and ship a 1.0 table with 7 stable rows (which
overrides the stop rule), add contract tests before the table, wait for the changes that create
the missing surfaces, or narrow CMP-V1-003.

How the count changes with how the surfaces are split and what counts as a test:

| Counting | With a test | Of | Fewer than half? |
|---|---:|---:|---|
| Strict, 22 surfaces (the table below) | 7 | 22 | yes |
| MCP tool names counted per server (3 rows) | 9 | 24 | yes |
| Also counting row 13 (`test-run-evidence/0`, a different format) and row 3 (`corvint` exit codes, partial) | 11 | 24 | yes |
| Also counting row 12 (the test-run receipt's struct round-trip, weak) | 12 | 24 | no: exactly half, but only by counting a partial, a weak and a wrong-format test, which CMP-V1-002 does not admit as covering the surface |
| The five numbered items of CMP-V1-003, each counted only when fully covered | 1 (item 5) | 5 | yes |

## How it was measured

Read-only. Where each surface is documented and which test pins its names was found by searching
the code (`git grep`, excluding `.agent-evidence` and `.corvint/changes`). Then each named test
function was confirmed to exist (`grep -n '^func NAME('` in the package's `_test.go` files), and
each label was confirmed as a string literal in non-test Go code. A test counts as a contract test
only when it fails if a name the surface promises is removed or renamed: a golden or snapshot
comparison, an explicit field-name assertion, or a closed decoder tested against fixed fixtures.
A test that only round-trips a Go struct does not count, because renaming a field in the struct
still passes. Every golden below can be regenerated deliberately (`make capability-catalog`, the
golden update variables), so these tests catch silent drift, not intended breaks.

## The surface list (task 2.1)

| # | Surface (CMP-V1-003 item) | Label read from code | Contract test, or gap |
|---:|---|---|---|
| 1 | `corvint` verbs (1) | `corvint-command-tree/0` (`internal/clitree/clitree.go`, the snapshot schema) | `TestCAPV0045CatalogLinksCurrentTreeSnapshots` in `./cmd/corvint` (`help --tree` against `internal/capcatalog/trees/corvint.json`); also `TestCAPV0047RepositoryCommandTreesMatchDispatch` in `./tools/capabilities-check` |
| 2 | `corvint` flags (1) | same | same tests: the snapshot records every node's flags. Gap: the snapshot comes from help text, so a flag removed from the parser but left in help is not caught (UNVERIFIED whether anything else catches it) |
| 3 | `corvint` exit codes (1) | none | **gap (partial)**: no exit-code table exists. `TestCoreVerbsEmitTheFrozenProfiles` and `TestCoreRefusalsKeepTheFrozenEnvelope` in `./cmd/corvint` pin exit 0 and exit 2 for init, adopt, index, context, impact, affected and cem; `TestCLIErrorContracts_ACXV0859` in `./conformance/cli-errors-v0` pins exit 2 for an unknown verb; other verbs have none. `docs/AUTOMATION.md` documents exits only for `affected --base` and `impact --base` |
| 4 | `corvint-docs` verbs (1) | `corvint-command-tree/0` | `TestCAPV07700ShippedHelp` in `./conformance/cli-errors-v0` (builds the binaries and compares `help --json` with `internal/capcatalog/trees/corvint-docs.json`) |
| 5 | `corvint-docs` flags (1) | same | same test |
| 6 | `corvint-docs` exit codes (1) | none | **gap**: no table; only one-off tests such as `TestDraftStdoutWriteFailureIsOneEnvelope` in `./cmd/corvint-docs` |
| 7 | `corvint-tasks` verbs (1) | none | **gap**: Tasks ships from its own repository (decision 0594); no `cmd/corvint-tasks` here |
| 8 | `corvint-tasks` flags (1) | none | **gap**: as row 7 |
| 9 | `corvint-tasks` exit codes (1) | none | **gap**: as row 7 |
| 10 | Flow file `.corvint/flows.json` (2) | `corvint-flows/0` (`internal/appflows/flowplan.go`); no `/1` exists, change `flow-schema-version` not started | `TestAPKV0001OnePlanForEveryKind_V15251` and `TestAPKV0004FlowEvidenceFailsClosed_V15251` in `./internal/appflows` (closed decoder over fixed fixtures; an extra member is refused) |
| 11 | Plan (2) | `affected-plan/1` (`cmd/corvint/affected.go`; `affected-plan/0` is still read) | `TestCoreVerbsEmitTheFrozenProfiles` in `./cmd/corvint` (goldens under `cmd/corvint/testdata/core-freeze/affected-*.json`, compared member by member and by type) |
| 12 | Test-run receipt (2) | `corvint-test-run/0` (`internal/sessiontest/manifest.go`); UNVERIFIED that this is the receipt the spec means (plan row O48 is open) | **gap (weak)**: `TestSTRV0018ManifestStrictRoundTrip` in `./internal/sessiontest` round-trips the Go struct only |
| 13 | Run evidence (2) | `test-run-evidence/0` (`internal/appflows/runevidence.go`); the run-evidence store of change `run-evidence` does not exist | **gap**: `TestAFUV1RunEvidenceClosedSchema` in `./internal/appflows` names the fields of `test-run-evidence/0`, but that is the flow-coverage evidence record, not the run-evidence format of change `run-evidence`, which is not started. Counted as a gap in the strict count |
| 14 | Ticket schema v1 (2) | none in this repository (`taskman-ticket/1` is a proposed name in change `tasks-conformance`) | **gap**: no format, no shared fixture |
| 15 | Receipt reference v1 (2) | none (`taskman-receipt-ref/1` proposed) | **gap**: as row 14 |
| 16 | Documented files under `.corvint/` (3) | none | **gap**: no single document lists them; `./tools/declaration-compat-check` checks this repository's planner declarations against CI pins, not the documented user file set |
| 17 | Files in the Tasks store (3) | none | **gap (weak)**: `TestOnlyProjection` in `./internal/taskprojection` pins path recognition only, not schemas; the layout is owned by corvint-tasks |
| 18 | Action inputs (4) | none | **gap**: no `action.yml` in this repository; change `github-action` not started |
| 19 | Action outputs (4) | none | **gap**: as row 18 |
| 20 | Action major tag (4) | none | **gap**: as row 18 |
| 21 | Corvint version range per agent package (4) | none | **gap**: `integrations/claude-code`, `integrations/codex` and `extensions/vscode` declare no Corvint range; change `agent-packages` not started |
| 22 | MCP tool names in the tool table (5) | `MCPTools()` inventory (`internal/capcatalog/mcp.go`); the tool table of change `mcp-tool-table` does not exist yet | `TestAFUV1034FlowsToolProfile` and `TestMMRV0012GroupsToolsListGolden` in `./internal/mcp/cli` (tools/list byte goldens) for `corvint-mcp`; `TestServeToolsListAndCallRoundTripsDocsDraftAndConsume` in `./cmd/corvint-docs-mcp`; `TestToolCatalogueIsExactlyOneReadOnlyTool` in `./conformance/mcp-test-validity-v0` |

With a test (7): rows 1, 2, 4, 5, 10, 11, 22. Without (15): rows 3, 6, 7, 8, 9, 12, 13, 14, 15, 16,
17, 18, 19, 20, 21.

## Labels of the stable formats (CMP-V1-003 item 2)

Read from non-test Go code at this head: flow file `corvint-flows/0`; plan `affected-plan/1`
(`affected-plan/0` still read); test-run receipt `corvint-test-run/0` (UNVERIFIED as the meant
receipt); run evidence: no format yet (`test-run-evidence/0` is a different record); ticket schema
v1: no label; receipt reference v1: no label. So only two of the six formats have a label that
both exists and has a contract test.

CMP-V1-005 preview labels in Go code: `corvint-harness-event/0` is in `internal/gokernel/harness.go`.
`corvint-feature-map/0`, `corvint-data-map/0`, `corvint-change-report/0`, `corvint-review-check/0`
and `corvint-flow-link-proposals/0` are not in Go code yet. Their preview rows name them anyway.

## Not done, with reasons

- Tasks 2.2 to 2.4 (generator, page, rules, check and its tests): NOT_PRODUCED. The CMP-V1-003
  stop rule applies. A generator and check draft (list-driven, stability derived from a named test,
  test existence via `go/parser`, refusal of unlisted and missing surfaces, byte comparison of the
  page) exists outside the repository for the lane that resumes after the owner's decision. It is
  not committed and nothing here claims it.
- Task 3.1 (Tasks-gate completion): left to the orchestrator; the ticket stays open.
- `openspec validate compatibility-promise`: NOT_RUN, the `openspec` command is not installed on
  this host.
