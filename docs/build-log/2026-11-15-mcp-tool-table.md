# V1-5447: MCP tool table delivery; PROD-DOCS-001 delivered; MCP binary retirement held (MTT-V1-001..011)

Delivery record for the OpenSpec change `docs/specs/openspec/changes/mcp-tool-table/` (lane
`mt1115`, base `babe97abb110feb735fef9098e0e91765c102e46`). The canonical spec is
`docs/specs/mcp-tool-table-v1.md`. It serves PROD-DOCS-001 under owner decision R30 (2026-11-09) and
the no-Tasks-server rule of R17. Tasks 2.1 to 2.8 landed in earlier lanes; this entry covers 2.9,
3.1 and 3.2 and gives the requirement-to-test trace for the whole change.

## Delivered

- One table (`internal/capcatalog/mcp.go`) fixes every tool name, server, schema, output profile,
  error list, availability and CLI equivalent. `docs/MCP-SERVER.md` is generated from it and
  checked by a doc gate, and a drift test compares it with each server's real `tools/list`.
- `corvint-docs mcp` (`internal/mcp/docsserver`, `cmd/corvint-docs`) serves the `docs_*` tools,
  `docs_draft`, `docs_consume` and `help`. `corvint mcp` serves only testing and flow-report tools.
  The `docs` and `corpus` profiles refuse with a typed reason naming `corvint-docs mcp`, and
  `test_validity` is now an unknown tool.
- New in this lane: `TestMTTV1004CorvintDocsMCPLeavesTheRepositoryUnchanged` (`cmd/corvint-docs`)
  runs a `corvint-docs mcp` session (`tools/list`, `docs_get`, `docs_search`, `help`) on a Git
  fixture. It shows HEAD, refs and `git status --ignored --untracked-files=all` unchanged, which is
  the "`git status` shows no change" clause of the PROD-DOCS-001 scenario. Size bounds come from
  the shared transport (`internal/mcp/server`, `protocol.MaxMessageBytes`).
- PROD-DOCS-001 is marked `State: DELIVERED` in `docs/specs/PRODUCT-PLAN-1.0.md`, with a
  `Delivered:` line, the R30 row and the state counts (Documentation 1/10/2, total 1/44/32). This is
  the first `DELIVERED` row in the plan. Under the plan's own definition, it means code and tests
  for the whole statement are on `origin/main`, not validated on outside repositories.

## Held: retiring `corvint-mcp` and `corvint-docs-mcp` (MTT-V1-010, task 2.9)

`corvint mcp` already serves the same server from the core binary (`cmd/corvint-mcp/main.go` calls
the same `cli.ServeProfiled`), and `corvint-docs mcp` serves the docs tools. Removing the two
binaries still needs an owner decision, for these reasons:

- PROD-INT-006 (accepted, decision 0596; "Binaries per product", owner decision R14) ships
  `corvint` and `corvint-mcp` for testing and `corvint-docs` and `corvint-docs-mcp` for
  documentation.
- `AGENTS.md` "Product binaries" names both as product binaries. Decision 0594 "Keeps" both.
- Users would have to change their install path: `docs/INSTALL.md` installs both archives, and the
  `corvint-update` core set updates them together.
- MTT-V1-010 covers `corvint-mcp` and `corvint-test-validity-mcp` only, and only "in the removal
  batch that retires them", and no batch has been decided for `corvint-mcp`. Keeping
  `corvint-docs-mcp` as an alias is an open question in the change's `design.md`.

`binaries.json` and the cmd packages are unchanged. Task 2.9 stays unticked. Because tasks are
ticked in order (OSC-V1-005), 3.1 and 3.2 stay unticked as well, with their proof recorded in
`tasks.md`. The no-Tasks-tool half of 2.9 (MTT-V1-006) is delivered and tested.

## Requirement-to-test trace

| Requirement | Evidence |
|---|---|
| MTT-V1-001 | `TestMTTV1001TableRules`, `TestMTTV1001TableRulesNameTheBrokenRow`, `TestCAPV08800MCPToolsReachable`, `TestMTTV1001AffectedErrorsAreAdvertised` |
| MTT-V1-002 | `TestMTTV1002StalenessCheckFailsOnAHandEdit`, `TestMTTV1002DocumentCarriesEveryRow` |
| MTT-V1-003 | `TestMTTV1003CoreServerRegistersExactlyTheTable`, `TestMTTV1003TestValidityServerRegistersExactlyTheTable`, `TestMTTV1003DriftNamesTheTool` |
| MTT-V1-004 | `TestMTTV1004DocsServerToolList`, `TestMTTV1004DocsServerAnswers`, `TestMTTV1004DocsServerRefusesArguments`, `TestMTTV1004CorvintDocsMCPAnswersOnAFixture`, `TestMTTV1004CorvintDocsMCPLeavesTheRepositoryUnchanged`, `TestMTTV1004CorvintDocsMCPHelp` |
| MTT-V1-005 | `TestMTTV1005RemovedGroupsRefuseWithATypedReason`, `TestMTTV1005RemovedGroupMatchesOnlyFlagPosition`, `TestMTTV1005TestValidityIsAnUnknownTool` |
| MTT-V1-006 | `TestMTTV1006CoreServerServesNoTasksTool`, `TestMTTV1006DocsServerServesNoTasksTool`, `TestMTTV1001TableRulesNameTheBrokenRow/tasks_tool_on_a_server` |
| MTT-V1-007 | `TestMTTV1007OneNamingConvention`, `TestMTTV1007OldFlowsNameNamesTheNewTool` |
| MTT-V1-008 | `TestMTTV1008Grammar`, `TestMTTV1008AffectedToolReadsTheKillSwitch`, `TestMTTV1008UnknownSelectionThroughTheServer` and the rest listed in the spec |
| MTT-V1-009 | one parity test per row with a CLI equivalent, named in the table's `ParityTest` column and checked by the doc gate |
| MTT-V1-010 | NOT_PRODUCED: held for the owner decision above |
| MTT-V1-011 | `TestMTTV1011AffectedToolOnAStaleBinarySelectsAll`, `TestV15254MCPAffectedStaleBinaryDegrades` |

## Limits

- Not done here: completing the ticket through the Tasks gate (left to the orchestrator), and live
  qualification with an MCP client (NOT_RUN).
