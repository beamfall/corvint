# MCP Tool Table V1

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `MTT-V1`
Intent status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5447)
Delivery status: experimental; partially delivered
Authoritative inputs: change folder `docs/specs/openspec/changes/mcp-tool-table/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/mcp-tool-table/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-DOCS-001, PROD-DOCS-010, PROD-TASK-007, PROD-TEST-009),
`docs/specs/mcp-multi-root-v0.md` (MMR-V0), `docs/specs/capability-catalog-v0.md` (CAP-V0-8800) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`.

## Agent digest
- Claim: One table fixes every MCP tool; MCP-SERVER.md is generated from it; corvint-docs mcp serves docs tools, corvint mcp testing and flow tools.
- Status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5447) / experimental; partially delivered
- Exists: the table in `internal/capcatalog/mcp.go` with its rules, the generated `docs/MCP-SERVER.md` and its doc gate, the drift and parity tests, `corvint-docs mcp` (`internal/mcp/docsserver`), the narrowed `corvint mcp`, the renamed flow tools, and the `affected` select-all for `CORVINT_SELECTION` and a stale binary (tasks 2.1 to 2.7, 2.9).
- Blocked on: the removal batch that takes `corvint-mcp` and `corvint-test-validity-mcp` off the release list (MTT-V1-010, task 2.8); the plan row of task 3.1.
- Read next: Requirements; Superseded requirements; Delivery and traceability; Rollback.

## User and boundary

An agent author or MCP client configurer needs one place that says which Corvint MCP tools exist,
which server serves each, what each takes and returns, and which command line gives the same answer.
Owner decision R30 (2026-11-09) puts the documentation tools on one `corvint-docs mcp` server, keeps
the testing tools on `corvint mcp`, and requires one table that fixes every tool name and schema.
Owner decision R17 (2026-11-09) removes the Tasks MCP server from 1.0. This spec is the canonical
home of MTT-V1-001 to MTT-V1-011, copied from the change's spec delta. Decisions, failure modes and
open questions stay in the change's `design.md`, which owns them. Each delivery is recorded in one
`docs/build-log/YYYY-MM-DD-<slug>.md` entry.

Non-goals (from the change): no Tasks MCP server or tasks tools (R17); no new tool behaviour beyond
the renames and the server move, so tool outputs keep their profiles (the behaviour DMF-V1-009 to
DMF-V1-011 require is built by the change `docs-mcp-hardening` and only recorded in this table); no
network transport, stdio only; the `how` tool's existence is decided by the change `how-rewrite`,
and this change only hosts its row.

Trust boundary: both servers are local, read-only stdio processes bound to declared repository
roots, as before. The table is authored Go data reviewed like code; generated or inferred text never
adds a row. Every mismatch between the table and what a server registers fails a test, and every
unknown or unsafe selection input widens the `affected` answer to all tests, never narrows it.

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text.

- `MTT-V1-001`: (accepted automatically under G14, decision 0596; V1-5447) One table fixes every MCP tool

One authored table MUST list every MCP tool Corvint serves exactly once, with its name, server,
description, input schema reference, output profile, typed error codes, availability and CLI
equivalent. Each row MUST also carry a description for every input property. The description
column is what DMF-V1-008 (change `docs-mcp-hardening`) checks, so one table and one drift check
enforce it. The typed-error column MUST list the DMF-V1-010 members `server-busy` and `cancelled`
for every tool. Tool names MUST be unique across both servers. (owner decision R30 (2026-11-09);
PROD-DOCS-001; docs-mcp verdicts, suggested amendments)

The docs rows are re-described, not pruned: after `corvint.docs_recommend_tests` is removed, the
19 remaining `corvint.docs_*` rows stay. A docs row MAY be removed before the freeze only when a
fixture test shows that the tool's answer is a strict subset of another tool's answer for the
same input; any other removal waits for 1.1 with one release of notice. (decided 2026-11-09
under R42, B9)

The table MUST also hold one row for `corvint.docs_how` on the `corvint-docs mcp` server. It
answers a how-to question from the application map, keeps the `site-knowledge-how/0` output
profile of `corvint-docs how --json`, and lands with change `how-rewrite` (HOW-V1-011). (decided
2026-11-09 under R42, B10)

**Scenario: A duplicate name**
- GIVEN two rows named `corvint.status`, one per server
- WHEN the table test runs
- THEN it fails and names the duplicate

**Scenario: Two rows share a description**
- GIVEN two `corvint.docs_*` rows with the same description, or a row whose input property has
  no description
- WHEN the table test runs
- THEN it fails and names the rows or the property

**Scenario: A row without the shared error codes**
- GIVEN a row whose typed-error column lacks `server-busy` or `cancelled`
- WHEN the table test runs
- THEN it fails and names the row and the missing code

- `MTT-V1-002`: (accepted automatically under G14, decision 0596; V1-5447) `docs/MCP-SERVER.md` is generated from the table

`docs/MCP-SERVER.md` MUST be generated from the table, and a staleness check in `doc-gates` MUST fail
when the file differs from the generator output. (owner decision R30 (2026-11-09))

**Scenario: A hand edit**
- GIVEN a hand edit to a tool description in `docs/MCP-SERVER.md`
- WHEN `make doc-gates` runs
- THEN the staleness check fails

- `MTT-V1-003`: (accepted automatically under G14, decision 0596; V1-5447) The servers register exactly the table's tools

Each server MUST register exactly the tools and input schemas its table rows name. A test MUST fail on
a registered tool without a row or a row without a registered tool. (owner decision R30 (2026-11-09))

**Scenario: An unlisted tool**
- GIVEN a new tool registered on `corvint mcp` with no table row
- WHEN the drift test runs
- THEN it fails and names the tool

- `MTT-V1-004`: (accepted automatically under G14, decision 0596; V1-5447) `corvint-docs mcp` serves the documentation tools

`corvint-docs mcp` MUST serve the `docs_*` corpus tools, `docs_draft`, `docs_consume` and `help` over
stdio, and MUST work with only the documentation binaries installed. (owner decision R30
(2026-11-09); PROD-DOCS-001; PROD-INT-004)

**Scenario: Documentation alone**
- GIVEN a machine with only `corvint-docs` installed
- WHEN an MCP client starts `corvint-docs mcp` and lists tools
- THEN the documentation tools are listed and answer on a fixture repository

- `MTT-V1-005`: (accepted automatically under G14, decision 0596; V1-5447) `corvint mcp` keeps only testing and flow-report tools

`corvint mcp` MUST serve only `impact`, `status`, `affected`, `test_evidence` (labelled preview),
the flow report tools and `help`; `test_validity` is removed with the G24 batch and a call to it
returns the unknown-tool error (decided 2026-11-09 under R42, M12). A request for the removed
`docs` or `corpus` profile MUST return a typed refusal that names `corvint-docs mcp`. (owner
decision R30 (2026-11-09))

Earlier wording, superseded by M12: "MUST serve only `impact`, `status`, `affected`,
`test_evidence`, `test_validity`, the flow report tools and `help`."

**Scenario: A call to the removed tool**
- GIVEN `corvint mcp` started after the G24 batch
- WHEN a client lists tools and then calls `test_validity`
- THEN the list has no `test_validity` tool, `test_evidence` is labelled preview, and the call
  returns the unknown-tool error

**Scenario: An old docs profile**
- GIVEN a client configured with `corvint mcp --profile docs`
- WHEN it starts the server
- THEN the server refuses with a typed reason naming `corvint-docs mcp`

- `MTT-V1-006`: (accepted automatically under G14, decision 0596; V1-5447) No Tasks MCP tools

Neither server MAY serve a Tasks tool. The table MUST carry a "no MCP tool" row stating that Tasks is
used through its CLI. (owner decision R17 (2026-11-09); PROD-TASK-007)

**Scenario: Tool list**
- GIVEN both servers started
- WHEN their tool lists are read
- THEN no tool name contains `tasks`, and the table's Tasks row has server `none`

- `MTT-V1-007`: (accepted automatically under G14, decision 0596; V1-5447) One naming convention

Every tool name MUST follow `corvint.<area>_<verb>` with exactly one dot. Renamed tools MUST list
their old names in the row's `replaces` column. (owner decision R30 (2026-11-09))

**Scenario: A flow tool**
- GIVEN the flow gaps tool
- WHEN the table is read
- THEN its name is `corvint.flows_gaps` and `replaces` lists `corvint.flows.gaps`

- `MTT-V1-008`: (accepted automatically under G14, decision 0596; V1-5447) `CORVINT_SELECTION` in the `affected` tool

The `affected` tool MUST read `CORVINT_SELECTION` with the CLI grammar, including `full:<runner>`. An
unknown value MUST select all tests and return a typed reason, never a narrowed plan. (owner decision
R13 (2026-11-09); PROD-TEST-013)

**Scenario: An unknown value**
- GIVEN `CORVINT_SELECTION=fastest` in the server environment
- WHEN a client calls the `affected` tool
- THEN every test is selected and the result carries the unknown-value reason

- `MTT-V1-009`: (accepted automatically under G14, decision 0596; V1-5447) Same answer as the command line

For every row with a CLI equivalent, the tool MUST return the same records as that command on the same
inputs. (PROD-DOCS-010; PROD-TEST-009) Two stated exceptions apply to the `affected` tool, where
the command exits 2 and the tool selects all tests with a reason: an unknown `CORVINT_SELECTION`
value (MTT-V1-008) and a stale binary (MTT-V1-011; decided 2026-11-09 under R42, M1).

**Scenario: Affected parity**
- GIVEN a fixture repository and a base commit
- WHEN the `affected` tool and `corvint affected --base FULL_SHA` run
- THEN both return the same selections

- `MTT-V1-010`: (accepted automatically under G14, decision 0596; V1-5447) Retired MCP binaries leave the release list

`corvint-mcp` and `corvint-test-validity-mcp` MUST leave `tools/release-binary/binaries.json` in the
removal batch that retires them. (owner decision R14 (2026-11-09); owner decision R30 (2026-11-09))

**Scenario: Release list after the batch**
- GIVEN the removal batch merged
- WHEN the release binaries check runs
- THEN neither `corvint-mcp` nor `corvint-test-validity-mcp` is listed

- `MTT-V1-011`: (accepted automatically under G14, decision 0596; V1-5447) The `affected` tool on a stale binary selects all tests

On a stale binary (the running binary is older than the source) the `affected` tool MUST return a
successful tool result holding a select-all plan with reason `self-binary-stale` and a one-line
rebuild hint, and MUST NOT return a narrowed plan or a tool error. The CLI and CI keep exit 2. This
follows the pattern of owner decision R13 (2026-11-09) and MTT-V1-008, and is a stated exception to
MTT-V1-009. (decided 2026-11-09 under R42, M1; PROD-TEST-009)

**Scenario: Stale binary through MCP**
- GIVEN a `corvint mcp` server whose binary is older than the source
- WHEN a client calls the `affected` tool
- THEN the call succeeds, every test is selected, the reason is `self-binary-stale`, and the
  result carries a one-line rebuild hint

**Scenario: Stale binary on the command line**
- GIVEN the same stale binary
- WHEN `corvint affected --base FULL_SHA` runs
- THEN it exits 2, as before

Failure mode: the tool returns a narrowed plan or a tool error on a stale binary; an agent then
runs too few tests, retries or skips tests. Acceptance: one Go test of the `affected` tool on a
stale binary (select-all, reason, hint, no tool error) and one of the CLI exit code. Rollback:
revert the commit; the tool then has no stated stale-binary rule and the ID is retired, not reused.

## Superseded requirements

This spec supersedes, for 1.0, the parts of these earlier requirements that put the documentation
tools or the test-validity tool on `corvint mcp`, or that fix the old flow tool names. Their IDs are
not reused and their text stays in their own specs as history.

- MMR-V0-012 (superseded in part): the `test-validity`, `docs` and `corpus` tool groups of `corvint mcp --tool-profile`
  are superseded by MTT-V1-005; the `flows` code profile and the `test-evidence` and `affected`
  groups stay.
- MMR-V0-013 (superseded in part): serving the `test-validity` and `docs` groups through `corvint mcp` is superseded by
  MTT-V1-004 and MTT-V1-005; the docs tools keep the same `internal/mcp/toolgroup` code on
  `corvint-docs mcp`.
- MMR-V0-014 and MMR-V0-017 (superseded in part): the `corpus` group and `--corpus-artifact` move from `corvint mcp`
  to `corvint-docs mcp` (MTT-V1-004); their artifact, refusal and root-pinning rules hold there
  unchanged.
- MMR-V0-016 (superseded in part): whether the `corvint-docs-mcp` alias binary stays for one release is an open owner
  question (change `design.md`); its usage line now names `corvint-docs mcp` as the unified
  equivalent.
- AFU-V1-034 (superseded in part): the six `corvint.flows.<x>` names are replaced by `corvint.flows_<x>` (MTT-V1-007);
  the tools' inputs and outputs are unchanged.
- CAP-V0-8800 (superseded in part): the offline MCP inventory is now the table of MTT-V1-001, with server, schema,
  error and availability columns.

## Delivery and traceability

| Requirement | Task | Implementation | Evidence |
|---|---|---|---|
| MTT-V1-001 | 2.1 | `internal/capcatalog/mcp.go` (`MCPTools`, `CheckMCPTools`) | `TestMTTV1001TableRules`, `TestMTTV1001TableRulesNameTheBrokenRow`, `TestCAPV08800MCPToolsReachable` |
| MTT-V1-002 | 2.2 | `tools/mcp-server-doc`, `make mcp-server-doc`, doc gate `mcp-server-doc-check` | `TestMTTV1002StalenessCheckFailsOnAHandEdit`, `TestMTTV1002DocumentCarriesEveryRow` |
| MTT-V1-003 | 2.3 | `internal/mcp/cli/tooltable_drift_test.go` (`tableDrift`) | `TestMTTV1003CoreServerRegistersExactlyTheTable`, `TestMTTV1003TestValidityServerRegistersExactlyTheTable`, `TestMTTV1003DriftNamesTheTool` |
| MTT-V1-004 | 2.4 | `internal/mcp/docsserver`, `corvint-docs mcp` (`cmd/corvint-docs/main.go`, `help.go`) | `TestMTTV1004DocsServerToolList`, `TestMTTV1004DocsServerAnswers`, `TestMTTV1004DocsServerRefusesArguments`, `TestMTTV1004CorvintDocsMCPAnswersOnAFixture`, `TestMTTV1004CorvintDocsMCPHelp` |
| MTT-V1-005 | 2.5 | `internal/mcp/cli/groups.go` (`removedGroups`), `main.go`, `unavailable.go`; `internal/mcp/toolgroup/test_evidence.go` (preview label) | `TestMTTV1005RemovedGroupsRefuseWithATypedReason`, `TestMTTV1005TestValidityIsAnUnknownTool`, `TestMTTV1003CoreServerRegistersExactlyTheTable` |
| MTT-V1-006 | 2.1, 2.8 | the `corvint-tasks` no-MCP-tool row and the Tasks-name rule in `internal/capcatalog/mcp.go` | `TestMTTV1001TableRules`, `TestMTTV1001TableRulesNameTheBrokenRow/tasks_tool_on_a_server`, `TestMTTV1006CoreServerServesNoTasksTool`, `TestMTTV1006DocsServerServesNoTasksTool` |
| MTT-V1-007 | 2.5 | `internal/mcp/bridge/flows.go` (renamed constants), `internal/capcatalog/mcp.go` (`toolName`, `Replaces`) | `TestMTTV1007OneNamingConvention`, `TestMTTV1001TableRulesNameTheBrokenRow/two_dots` |
| MTT-V1-008 | 2.6 | `internal/selectionswitch` (grammar), `internal/mcp/toolgroup/affected.go` (`selectAll`, `affected-select-all/1`) | `TestMTTV1008Grammar`, `TestMTTV1008AffectedToolReadsTheKillSwitch`, `TestMTTV1008UnknownSelectionThroughTheServer` |
| MTT-V1-009 | 2.7 | table column `ParityTest` (a binary name and a test name) and its rule in `internal/capcatalog/mcp.go`; `MissingParityTests` in `tools/mcp-server-doc` (doc gate) | `TestMTTV1009HelpToolMatchesCLI`, `TestMTTV1009ImpactToolMatchesCLI`, `TestMTTV1009TestEvidenceToolMatchesCLI`, `TestV15254MCPAffectedMatchesCLI` (cmd/corvint); `TestMTTV1009FlowsToolsMatchCLI`, `TestMTTV1009DocsConsumeToolMatchesCLI`, `TestDocsToolVerbAnswersInOneEnvelope`, `TestCorpusTypedCLIMCPEndToEnd` (cmd/corvint-docs); `TestMTTV1009MissingParityTestIsNamed` |
| MTT-V1-010 | 2.8 | not built here: `corvint-mcp` and `corvint-test-validity-mcp` leave `tools/release-binary/binaries.json` in their removal batch | NOT_PRODUCED until that batch |
| MTT-V1-011 | 2.9 | `internal/mcp/toolgroup/affected.go` (`AffectedStaleCode`, select-all on the planner's refusal); `cmd/corvint/mcp.go` help | `TestMTTV1011AffectedToolOnAStaleBinarySelectsAll`; `TestV15254MCPAffectedStaleBinaryDegrades` (tool select-all and CLI exit 2 on one stale binary); `TestStaleBinaryRefusesThePlanUnlessAccepted_AFPV09730` |

## Rollback

Restore the `docs` and `corpus` profiles in `internal/mcp/cli`; the table and generated document stay
valid with those rows moved back to `corvint mcp`. Restore the `corvint.flows.<x>` names through
`replaces` aliases. Restore removed binaries to `tools/release-binary/binaries.json` in a revert of
the removal batch. Reverting every commit of the change removes the table columns, the generated
document and `corvint-docs mcp`; the MTT-V1 IDs are then retired, not reused.
