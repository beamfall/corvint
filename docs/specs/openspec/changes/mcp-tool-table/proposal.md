# Change proposal: one MCP tool table, a `corvint-docs mcp` server, and `corvint mcp` limited to testing tools

Change name: `mcp-tool-table`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-DOCS-001, PROD-DOCS-010, PROD-TEST-009 and PROD-TASK-007 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/mcp-tool-table-v1.md`, a new spec with prefix `MTT-V1`
Requirement IDs this change adds: MTT-V1-001 to MTT-V1-010
Requirement IDs this change modifies: none
Owner answers this change implements: owner decision R30 (2026-11-09), owner decision R17
(2026-11-09), owner decision R13 (2026-11-09) for the MCP behaviour of `CORVINT_SELECTION`, owner
decision R14 (2026-11-09) for retired provider binaries
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/mcp-tool-table/spec.md`.

## Why

Owner decision R30 (2026-11-09) puts the documentation tools on one `corvint-docs mcp` server, keeps
the testing tools on `corvint mcp`, and requires one table that fixes every tool name and schema.
Owner decision R17 (2026-11-09) removes the Tasks MCP server from 1.0. Today the tool names, servers
and schemas are spread across several Go packages and one hand-written document, three MCP binaries
ship, and the documentation tools are reachable from `corvint mcp`. PROD-DOCS-001 stays PARTIAL until
the server and table ship. Agent packages (change `agent-packages`) and `how` (change `how-rewrite`,
HOW-V1-011) need one place to read tool names from.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- `corvint mcp` is served by `cmd/corvint/mcp.go` (verb listed at `cmd/corvint/main.go` line 694).
  `internal/mcp/cli/main.go` lines 44-54 define the profiles `flows`, `test-validity`, `docs`,
  `corpus`, `test-evidence` and `affected`; lines 287-289 build the profile registries.
  `internal/mcp/cli/groups.go` imports `corpusbridge`, `docsbridge` and `testvaliditybridge`.
- `cmd/corvint-docs-mcp/main.go` lines 1-10 serve `docs_draft`, `docs_consume` and `help` and call
  themselves a "deprecated alias (MMR-V0-016)". `cmd/corvint-docs` has no `mcp` subcommand.
- Tool names are constants in several files: `corvint.impact` and `corvint.status` (bridge.go lines
  29-30); six `corvint.flows.*` tools (flows.go lines 23-28); `corvint.help` (help.go line 20); 20
  `corvint.docs_*` tools (corpusbridge.go line 147); `docs_draft` and `docs_consume` (docsbridge.go
  lines 34-35); `corvint.test_validity` (testvaliditybridge.go line 28); `corvint.affected`
  (toolgroup/affected.go line 20); `corvint.test_evidence` (toolgroup/test_evidence.go line 13).
  Two naming styles exist: `corvint.flows.gaps` and `corvint.docs_search`.
- `internal/capcatalog/mcp.go` lines 15-51 hold an `MCPTools()` inventory (CAP-V0-8800). It has no
  `how` entry and carries no schema reference.
- `tools/release-binary/binaries.json` lines 18-20 list `corvint-mcp`, `corvint-docs-mcp` and
  `corvint-test-validity-mcp`.
- `docs/MCP-SERVER.md` is hand-written.

## What Changes

- One authored table, extending `internal/capcatalog` `MCPTools()`, lists each tool once with: name,
  server (`corvint mcp` or `corvint-docs mcp`), input schema reference, output profile, typed error
  codes, and availability (`stable` or `experimental`). Explicit "no MCP tool" rows name the answers
  that are deliberately CLI only.
- `docs/MCP-SERVER.md` is generated from the table; a drift check compares the table with the tools
  each server registers at start.
- `corvint mcp` serves the testing and flow-report tools: `impact`, `status`, `affected`,
  `test_evidence`, `test_validity`, the flow reports and `help`. The `docs` and `corpus` profiles
  leave `corvint mcp`.
- A new `corvint-docs mcp` subcommand serves the `docs_*` tools, `docs_draft`, `docs_consume` and
  `help`.
- One naming convention for all tools.
- No Tasks server and no tasks tools.
- `CORVINT_SELECTION` with an unknown value makes the `affected` tool select all tests with a reason
  (R13), and `full:<runner>` has the same grammar as the CLI.
- The separate MCP binaries leave `binaries.json` once their replacement ships.

## Capabilities

### New Capabilities

- `mcp-tool-table`: the table, the generated document, the drift check and the server split.

### Modified Capabilities

- `mcp-multi-root` (MMR-V0): whether MMR-V0-016's `corvint-docs-mcp` alias binary stays for one
  release is an owner question; the MMR-V0 IDs that put docs tools on `corvint mcp` are listed in task 1.2 as superseded.
- `capability-catalog` (CAP-V0-8800): the inventory gains schema, server and availability columns.

## Impact

- `internal/mcp/**`, `internal/capcatalog/mcp.go`, `cmd/corvint/mcp.go`, `cmd/corvint-docs`,
  `cmd/corvint-docs-mcp`, `tools/release-binary/binaries.json`, `docs/MCP-SERVER.md`.
- MCP clients configured with `corvint mcp --profile docs` or `corpus` must switch to
  `corvint-docs mcp`; the upgrade message is listed in the change `user-docs`.
- Agent packages and plugin manifests read names from the generated table (change `agent-packages`).
- PROD-DOCS-001 stays PARTIAL until `corvint-docs mcp` ships.

## Non-goals

- No Tasks MCP server or tasks tools (R17).
- No new tool behaviour beyond the renames and the server move; tool outputs keep their profiles.
- No network transport; stdio only, as today.
- The `how` tool's existence is decided by the change `how-rewrite`; this change only hosts its row.
