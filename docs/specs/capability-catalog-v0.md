# Capability catalog V0

Owner: Russell Lewis
Date: 2026-10-20
Requirement prefix: `CAP-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008)
Delivery status: experimental
Authoritative inputs: V1-2008 owner request (2026-10-20) "make sure corvints capabilities are properly documented so agents can fully understand what it can do", and the same-day scope addition for a README capability section; `../../AGENTS.md` invariants 2 and 8; `../AGENT-ROUTES.md`.

## Agent digest
- Claim: `docs/CAPABILITIES.md` lists every command, binary and MCP tool with its use, invocation, maturity and refusals; a Go test fails on drift.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008)/experimental
- Exists: `docs/CAPABILITIES.md`, the README "What Corvint can do" section, `tools/capabilities-check` (its `go test` and `make capabilities-check`), and catalog pointers in `AGENTS.md`, `docs/AGENT-ROUTES.md`, `docs/README.md` and the shipped Claude Code, Codex, Gemini CLI, OpenCode and OpenSpec skills.
- Blocked on: subcommands, flags, profile membership and prose claims are not gated (see Failure modes); the Pi package text is not routed yet (its qualified adapter tuple pins the package bytes).
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
  `Commands:` block of `cmd/corvint/help.go`, each directory under `cmd/` holding a `package main`
  Go file, and each `corvint.*` MCP tool name literal in non-test Go source under `internal/mcp/`
  and `cmd/corvint-*mcp/`. Gated rows live between `<!-- capability-gate:commands -->`,
  `<!-- capability-gate:binaries -->` and `<!-- capability-gate:mcp-tools -->` markers and their
  `<!-- /capability-gate:... -->` closers; a row's name is the first backticked span in its first
  cell, with a leading `corvint ` removed for commands.
- `CAP-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `tools/capabilities-check` MUST report a `missing` finding for an inventory item without a row,
  a `stale` finding for a row naming nothing in the inventory, a `duplicate` finding for a second
  row of the same name, a `maturity` finding when a command row's maturity disagrees with the
  root help's Core list, and a `broken-link` finding for a relative link in a gated region whose
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
  MUST read only the paths it is given (the catalog, the README, `cmd/corvint/help.go`, the `cmd/`
  tree and `internal/mcp/`, plus `os.Stat` of linked targets), run no command, use no network and
  write nothing.

## Boundaries and failure modes

Non-goals: generating the catalog from code; checking subcommands, flags, output schemas or the
prose of a row; running any listed command; ranking capabilities; replacing a spec, a help page or
the deep guides the catalog links to.

Failure modes: a subcommand added or removed under an existing command is not detected, so the
row's subcommand list can drift; MCP profile membership and binary class are prose and unchecked;
the MCP scan counts every `"corvint.<name>"` literal in the scanned source, so a non-tool literal
of that shape would demand a row (none exists today) and a tool name built at run time would be
missed; maturity is checked for commands and README rows only, so a binary or tool row's maturity
can drift from its spec; a row naming the right thing with a wrong claim passes. A catalog edit
that the check rejects is fixed by editing the row, never by widening the gate.

## Rollback

Delete `docs/CAPABILITIES.md`, `tools/capabilities-check`, the `capabilities-check` Makefile target
and `.PHONY` entry, the `tools/capabilities-check` entry in `.corvint/test-read-scopes.json`, the
README section and the routing lines, then this spec's README row and INDEX entry. No stored state
depends on them. Reverting a skill text change needs its own version bump, because the host-package
version gate refuses a content change at an unchanged version.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| CAP-V0-001 | `docs/CAPABILITIES.md` | `TestCAPV0002RepositoryCatalogMatchesInventory` (structure via gated regions); build log self-check of five rows |
| CAP-V0-002 | `Collect`, `parseCommands`, `scanBinaries`, `scanTools`, `parseRows` | `TestCAPV0002RepositoryCatalogMatchesInventory`, `TestCAPV0002InventoryCountsMatchCode` |
| CAP-V0-003 | `Check`, `tools/capabilities-check/main.go` | `TestCAPV0003SyntheticOmissionAndStaleRowFail`, `TestCAPV0003MaturityAndLinkDriftFail` |
| CAP-V0-004 | `checkReadme` | `TestCAPV0004ReadmeRowsMapToCatalog` |
| CAP-V0-005 | routing lines and skill edits | `make host-package-versions-check`; `cmd/corvint`'s `TestHostAdapterJavaScriptHosts` (runs `integrations/host-adapters.test.mjs`) |
| CAP-V0-006 | `Collect`, `Check` | `TestCAPV0002RepositoryCatalogMatchesInventory` (reads only named inputs) |
