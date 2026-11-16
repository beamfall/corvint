# Application Flow Status V1

Owner: Russell Lewis
Date: 2026-11-16
Requirement prefix: `AFL-V1`
Intent status: proposed (change `flows-status-report`, rewritten 2026-11-09; V1-5623)
Delivery status: not-started
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/flows-status-report/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/application-flow-status/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-TEST-010, PROD-DOCS-011, G5),
`docs/specs/run-evidence-v1.md` (RVD-V1, the run evidence store),
`docs/specs/flow-schema-version-v1.md` (FSV-V1) and the MCP tool table of change `mcp-tool-table`
(`docs/specs/openspec/changes/archive/2026-11-15-mcp-tool-table/`).

## Agent digest
- Claim: `corvint flows status` and `flows impact` run in `corvint` alone, report each flow's tests and last recorded result, and match the MCP flow tools.
- Status: proposed (change `flows-status-report`, rewritten 2026-11-09; V1-5623) / not-started
- Exists: the report logic in `internal/appflows` (`FlowStatusFrom`, `FlowImpactResults`, `FlowGaps`) and the `corvint mcp --tool-profile flows` tools; the command entry is in the `corvint-docs` command group.
- Blocked on: nothing outside the change.
- Read next: Requirements; Delivery and traceability; Rollback.

## User and boundary

A team that installs only the testing product (`corvint`) needs to see, for each application flow
and variation, which tests are attached and what each last did, from the CLI and from an agent's MCP
host (PROD-TEST-010; G5). Flow reports that join flows with test results belong to testing; flow
authoring and discovery stay in `corvint-docs` (PROD-DOCS-011). This spec is the canonical home of
AFL-V1-001 to AFL-V1-008, copied from the change's spec delta. Decisions, failure modes and
acceptance evidence stay in the change's `design.md`, which owns them. Each delivery is recorded in
one `docs/build-log/YYYY-MM-DD-<slug>.md` entry.

Non-goals (from the change): running tests or persisting results inside a report command
(invariant 4); inferring a result from history, CI logs or a test name; authoring flows or links
(`corvint-docs`).

Trust boundary: the flow file, the intents and the link records are tracked repository input read
at HEAD through the shared reader (FSV-V1-003). A result is evidence only when one of the two named
writers recorded it; any other test is `NOT_OBSERVED` (invariant 2).

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text.

- `AFL-V1-001`: (proposed 2026-11-09; V1-5623; delivery not-started) One native report per flow and variation

Source: G5, G11. `corvint flows status` MUST run with `corvint` installed alone and MUST report,
for each flow and variation in `.corvint/flows.json`, the tests tied to it by accepted, non-stale
link records, plus the `unassigned` count of the E2E inventory.

**Scenario: corvint installed alone**
- GIVEN a repository with two flows, link records and no `corvint-docs` on `PATH`
- WHEN the user runs `corvint flows status`
- THEN the report lists both flows, their variations and their linked tests

- `AFL-V1-002`: (proposed 2026-11-09; V1-5623; delivery not-started) Results only from named writers

Source: G5, G24. The report MUST take a test result only from a `corvint-test-run/0` receipt
written by `corvint test` or from a `test-run-evidence/0` file written by `flows ingest`, MUST show
`NOT_OBSERVED` for any test with neither, and MUST mark a result from another commit as stale with
its source commit.

**Scenario: One test has no recorded result**
- WHEN the receipt covers one linked test and not another
- THEN the first shows its outcome and the second shows `NOT_OBSERVED`

- `AFL-V1-003`: (proposed 2026-11-09; V1-5623; delivery not-started) The MCP tool returns the same report

Source: G5; owner decision R30 (2026-11-09). `corvint.flows.status` on `corvint mcp` MUST return the
same records as the command for the same inputs, and its tool name, input schema, output profile and
error codes MUST match its row in the table of change `mcp-tool-table`.

**Scenario: Command and tool agree**
- WHEN the command and the tool run on the same inputs
- THEN both return byte-identical report bodies

- `AFL-V1-004`: (proposed 2026-11-09; V1-5623; delivery not-started) `--base` matches `flows impact`

Source: G5, G6. With `--base FULL_SHA`, the report MUST contain exactly the flows and variations
that `corvint flows impact` reports as touched for the same base and head.

**Scenario: One flow touched**
- WHEN a change touches one route of one flow
- THEN the report and `flows impact` name the same single flow and variation

- `AFL-V1-005`: (proposed 2026-11-09; V1-5623; delivery not-started) Bounded, read-only and deterministic

Source: invariant 4. The report MUST be bounded with a stable order, truncation marker and totals,
MUST write nothing, and MUST be byte-identical for identical inputs.

**Scenario: Repeated run**
- WHEN the command runs twice on the same inputs
- THEN both outputs are byte-identical and `git status` shows no change

- `AFL-V1-006`: (proposed 2026-11-09; V1-5623; delivery not-started) Invalid input is refused whole

Source: G5, invariant 2. A malformed, oversized or unknown-profile flow file, receipt or evidence
file MUST produce a typed refusal with exit 2 and MUST NOT produce a partial report.

**Scenario: Unknown flow file version**
- WHEN `.corvint/flows.json` declares profile `corvint-flows/9`
- THEN the command exits 2 with a typed refusal naming the profile

- `AFL-V1-007`: (proposed 2026-11-09; V1-5623; delivery not-started) `flows impact` is native

Source: G5 (docs F2). `corvint flows impact --base FULL_SHA` MUST run with `corvint` installed
alone, computing the touched flows and variations and their linked tests on the shared flow model
and readers with the reach of APK-V0-010. The `corvint-docs` copy of the verb MUST be removed and
replaced by a notice naming `corvint flows impact`. The `corvint.flows.impact` tool MUST stay on
`corvint mcp` and return the same records as the command.

**Scenario: corvint installed alone**
- GIVEN a repository with `.corvint/flows.json`, link records and no `corvint-docs` on `PATH`
- WHEN the user runs `corvint flows impact --base FULL_SHA`
- THEN the touched flows, variations and linked tests are printed and the exit code is 0

**Scenario: Tool and command agree**
- WHEN `corvint.flows.impact` and the command run on the same inputs
- THEN both return byte-identical report bodies

Failure mode: the flow file has an unknown profile or the graph is over its bound: typed refusal,
exit 2, no partial answer (as AFL-V1-006). Acceptance: the two scenarios as Go tests, the release
smoke test with `corvint-docs` absent, and the APK-V0-007 comparison test. Rollback: revert the
commit; `runFlowsShim` again forwards `flows impact` to `corvint-docs`, and APK-V0-007 returns to
blocked.

- `AFL-V1-008`: (proposed 2026-11-09; V1-5623; delivery not-started) One docs command group before 1.0

Source: R42 (panel pick) B4 (docs-mcp finding 20). The three look-alike docs command families,
`docs flows generate` / `docs flows check`, `flows docs` / `flows docs maintain` and
`docs maintain`, MUST be renamed into one command group inside this change's window, so the verbs
move once. The new names MUST be recorded in this change's design before the rename commit. Each
old name MUST stay as an alias for one release that runs the same command and prints a notice
naming the new name. The rename MUST land before change `user-docs` and the command table of change
`mcp-tool-table` are written.

**Scenario: Old name still works with a notice**
- GIVEN a build with the renamed group
- WHEN the user runs `corvint-docs docs maintain --preview`
- THEN the command runs as its new name does, and stderr prints a notice naming the new name

**Scenario: Help lists one group**
- WHEN the user runs `corvint-docs --help`
- THEN the three families appear under one group, and the old names are marked as aliases

Failure mode: an alias and a new name diverge in output: the alias test fails the build. Acceptance:
the two scenarios as Go tests and a golden of the help text. Rollback: revert the rename commit;
the old names return as the only names.

## Delivery and traceability

| Requirement | Task | Implementation | Evidence |
|---|---|---|---|
| AFL-V1-001 | 3 | not started | not produced |
| AFL-V1-002 | 4 | not started | not produced |
| AFL-V1-003 | 7 | not started | not produced |
| AFL-V1-004 | 6 | not started | not produced |
| AFL-V1-005 | 3 | not started | not produced |
| AFL-V1-006 | 3 | not started | not produced |
| AFL-V1-007 | 5 | not started | not produced |
| AFL-V1-008 | 7a | not started | not produced |

## Rollback

Revert the change's commits; `runFlowsShim` again forwards `flows status`, `flows impact` and
`flows gaps` to `corvint-docs`, whose copies return from the reverted diff, and the docs command
names return to their old spellings. No stored data is written. IDs are retired, not reused.
