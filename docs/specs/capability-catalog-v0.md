# Capability catalog V0

Owner: Russell Lewis
Date: 2026-10-20
Requirement prefix: `CAP-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008); CAP-V0-007 to CAP-V0-009 accepted under the same standing auto-accept (V1-2020)
Delivery status: experimental
Authoritative inputs: V1-2008 owner request (2026-10-20) "make sure corvints capabilities are properly documented so agents can fully understand what it can do", and the same-day scope addition for a README capability section; `../../AGENTS.md` invariants 2 and 8; `../AGENT-ROUTES.md`.

## Agent digest
- Claim: `docs/CAPABILITIES.md` lists every command, binary and MCP tool with its use, invocation, maturity and refusals; a Go test fails on drift.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008; V1-2020)/experimental
- Exists: `docs/CAPABILITIES.md`, the README "What Corvint can do" section, `tools/capabilities-check` (its `go test` and `make capabilities-check`), and catalog pointers in `AGENTS.md`, `docs/AGENT-ROUTES.md`, `docs/README.md` and the shipped Claude Code, Codex, Gemini CLI, OpenCode and OpenSpec skills.
- Blocked on: subcommands outside the declared tables, flags, profile membership and prose claims are not gated (see Failure modes); the Pi package text is not routed yet (its qualified adapter tuple pins the package bytes).
- Read next: Requirements; Boundaries and failure modes; Traceability.

## User and baseline

An agent that has never seen Corvint must learn what it can ask, how, and how far to trust the
answer. Before this spec the answers were spread over the 589-line README, 50 command help pages,
four MCP servers, 54 companion binaries and more than 200 specs, and some shipped skill text was
older than the code (V1-1271). The baseline is reading `corvint help`, each `COMMAND --help` and
the spec index by hand. The job is one dense page that is checked against the code, so that a row
cannot silently go missing or outlive its command.

## Requirements

- `CAP-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `docs/CAPABILITIES.md` MUST open with a "Start here" list of 10 to 15 task-to-command workflows,
  then group reference tables whose rows name the command or tool, the question it answers, when
  to use it and when not, a minimal invocation, its output, its maturity, what it refuses or
  abstains on, and its owning spec as a link. It MUST carry a "Language and path support" section
  for impact, affected and context, including what falls back. Every claim MUST come from the
  current code or help text; maturity MUST be `core` only for the verbs `corvint help` lists as
  Core, `internal` for CI and release tooling, and otherwise the owning spec's delivery status
  (`experimental` unless that spec says `implemented`). Nothing experimental is described as
  delivered or stable.
- `CAP-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The
  catalog MUST hold exactly one gated row for each top-level command listed in the root help's
  `Commands:` block of `cmd/corvint/help.go` or the `topLevelCommands` dispatch list in
  `cmd/corvint/main.go`, each directory under `cmd/` holding a `package main` Go file, and each
  `corvint.*` MCP tool name literal (letters, digits, `_`, `.` and `-`) in non-test Go source
  under `internal/mcp/`
  and `cmd/corvint-*mcp/`. Gated rows live between `<!-- capability-gate:commands -->`,
  `<!-- capability-gate:binaries -->` and `<!-- capability-gate:mcp-tools -->` markers and their
  `<!-- /capability-gate:... -->` closers; a row's name is the first backticked span in its first
  cell, with a leading `corvint ` removed for commands.
- `CAP-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `tools/capabilities-check` MUST report a `missing` finding for an inventory item without a row,
  a `stale` finding for a row naming nothing in the inventory, a `duplicate` finding for a second
  row of the same name, a `maturity` finding when a command row's maturity disagrees with the
  root help's Core list, a `dispatch-undocumented` finding for a dispatched verb without a root
  help line, a `dispatch-absent` finding for a root help verb that is not dispatched, and a `broken-link` finding for a relative link in a gated region whose
  target does not exist. Its test MUST run under plain `go test ./...` and MUST prove failure on a
  synthetic omission and a synthetic stale row. `make capabilities-check` runs the same check.
- `CAP-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The root
  `README.md` MUST carry a "What Corvint can do" section near the top whose gated rows
  (`<!-- capability-gate:readme -->`) each give a one-line benefit, a minimal command and a
  maturity label, and link to `docs/CAPABILITIES.md`. Each row's first backticked command MUST
  resolve to a catalog row, and its maturity's first word MUST equal that catalog row's; otherwise
  the check reports `readme-unmapped` or `readme-maturity`.
- `CAP-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `AGENTS.md` (one line), the "What capability exists?" row of `docs/AGENT-ROUTES.md`,
  `docs/README.md`, the root README and the shipped agent skills MUST route capability questions
  to `docs/CAPABILITIES.md`. A shipped package whose bytes change MUST carry its version bump and
  mirrors so the host-package version gate (AHI-020) stays green.
- `CAP-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The check
  MUST read only the paths it is given (the catalog, the README, `cmd/corvint/help.go`,
  `cmd/corvint/main.go`, the `cmd/` tree and `internal/mcp/`, plus `os.Stat` of linked targets;
  since V1-2020 also the non-test Go files of `cmd/corvint/` for CAP-V0-007 and the non-test Go
  source under `cmd/` and `internal/`, outside `testdata/`, for CAP-V0-009), run no command, use no
  network and write nothing.
- `CAP-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) The check
  MUST read the code-declared subcommand tables of `cmd/corvint` (`helpSubcommands`, a nested map
  whose values are literals or a top-level identifier such as `cemHelpActions`;
  `docsSubcommands` for `docs`; `flowSubcommands` for `flows`, plus every
  `flowSubcommands["NAME"] = ...` registration in a non-test file; `leaseActions` for `lease`) and
  report a `subcommand-undocumented` finding when a declared subcommand is not a word inside any
  backticked span of a catalog row for its verb. A verb without a row is left to the `missing`
  finding. A declared table that is no longer found is a check error (exit 2), so renaming a
  table cannot silently disable the check.
- `CAP-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) When the
  root help groups its verbs, a command row whose verb sits under the Experimental header MUST
  have maturity `experimental`; any other maturity is a `maturity` finding, in addition to the
  CAP-V0-003 Core comparison.
- `CAP-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) Each
  backticked span in a gated table's "Abstains or refuses on" cell that has the shape of a code
  (kebab-case or SCREAMING_SNAKE_CASE with at least two parts) MUST be a whole word (letters,
  digits, `_` and `-`) of some string literal in non-test Go source under `cmd/` or `internal/`,
  outside `testdata/`; otherwise the check reports `refusal-unknown`, so a code removed or
  renamed in the code cannot stay listed.

## Boundaries and failure modes

Non-goals: generating the catalog from code; checking subcommands outside the declared tables, a
row naming a subcommand the code no longer declares, flags, output schemas or the prose of a row; running any listed command; ranking capabilities; replacing a spec, a help page or
the deep guides the catalog links to.

Failure modes: a subcommand dispatched outside the CAP-V0-007 tables (for example `flows coverage`
and `flows record`, or any verb whose subcommands live only in a switch) is not detected, and a
subcommand removed from the code can stay listed in its row; a subcommand word that appears in
some other backticked span of the row counts as listed; MCP profile membership and binary class are prose and unchecked;
the MCP scan counts every `"corvint.<name>"` literal in the scanned source, so a non-tool literal
of that shape would demand a row (none exists today) and a tool name built at run time would be
missed; maturity is checked for commands and README rows only, so a binary or tool row's maturity
can drift from its spec; a refusal code stays accepted while any string literal, including help
prose or an unrelated message, still contains it as a word, and a code written in prose without
backticks or without a separator is not checked; a row naming the right thing with a wrong claim
passes. A catalog edit
that the check rejects is fixed by editing the row, never by widening the gate.

## Rollback

Delete `docs/CAPABILITIES.md`, `tools/capabilities-check`, the `capabilities-check` Makefile target
and `.PHONY` entry, the `tools/capabilities-check` entry in `.corvint/test-read-scopes.json`, the
README section and the routing lines, then this spec's README row and INDEX entry. No stored state
depends on them. Reverting a skill text change needs its own version bump, because the host-package
version gate refuses a content change at an unchanged version. To roll back only CAP-V0-007 to
CAP-V0-009, remove `parseSubcommands`, `ScanWords`, `checkSubcommands`, `checkRefusals` and the
Experimental maturity case with their tests, and narrow the test read scope back to `internal/mcp/`;
the catalog row edits stay correct.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| CAP-V0-001 | `docs/CAPABILITIES.md` | `TestCAPV0002RepositoryCatalogMatchesInventory` (structure via gated regions); build log self-check of five rows |
| CAP-V0-002 | `Collect`, `parseCommands`, `parseDispatched`, `scanBinaries`, `scanTools`, `toolNames`, `parseRows` | `TestCAPV0002RepositoryCatalogMatchesInventory`, `TestCAPV0002InventoryCountsMatchCode`, `TestCAPV0002ToolNamesWithDigitsAndHyphens` |
| CAP-V0-003 | `Check`, `checkDispatch`, `tools/capabilities-check/main.go` | `TestCAPV0003SyntheticOmissionAndStaleRowFail`, `TestCAPV0003MaturityAndLinkDriftFail`, `TestCAPV0002DispatchDriftFails` |
| CAP-V0-004 | `checkReadme` | `TestCAPV0004ReadmeRowsMapToCatalog` |
| CAP-V0-005 | routing lines and skill edits | `TestCAPV0005RoutingPointersNameTheCatalog`; `make host-package-versions-check`; `cmd/corvint`'s `TestHostAdapterJavaScriptHosts` (runs `integrations/host-adapters.test.mjs`) |
| CAP-V0-006 | `run`, `Collect`, `Check`, `ScanWords`, `parseSubcommands` | `TestCAPV0006CheckIsReadOnly` (synthetic tree unchanged by path, mode, mtime and digest), `TestCAPV0006CheckImportsNoProcessOrNetwork` |
| CAP-V0-007 | `subcommandTables`, `parseSubcommands`, `literalKeys`, `checkSubcommands` | `TestCAPV0007UndocumentedSubcommandFails`, `TestCAPV0007SubcommandTablesParse`, `TestCAPV0002RepositoryCatalogMatchesInventory` |
| CAP-V0-008 | `parseCommands` experimental set, `Check` maturity switch | `TestCAPV0008ExperimentalMaturityMismatchFails`, `TestCAPV0002GroupedRootHelp` |
| CAP-V0-009 | `ScanWords`, `refusalCode`, `checkRefusals` | `TestCAPV0009RemovedRefusalCodeFails`, `TestCAPV0002RepositoryCatalogMatchesInventory` |
