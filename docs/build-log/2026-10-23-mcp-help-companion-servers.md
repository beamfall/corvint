# `corvint.help` on the companion MCP servers

Date: 2026-10-23. Requirements `MCPV0-101` to `MCPV0-105` in
`docs/specs/mcp-server-2026-07-28-v0.md` (owner request, 2026-10-23); `MCPV0-106` to `MCPV0-109`
are reserved and unassigned. Amends `MTV-V0-001` and `SDD-V0-006`, and the `MCPV0-100` non-goal
that kept the tool off the other servers.

## Problem

`corvint-mcp` serves `corvint.help` (`MCPV0-100`): the binary's own `corvint help --json` catalog
or `corvint help guide`, byte for byte. An agent connected only to `corvint-docs-mcp`,
`corvint-corpus-mcp` or `corvint-test-validity-mcp` had no way to discover the command surface.

## Decision

- All three servers list `corvint.help` with the same descriptor and answer it through one shared
  entry, `bridge.HelpToolResult` in `internal/mcp/bridge/help.go`. That entry reuses
  `callHelp`, the closed receipt and its validator, the budget and the cancellation order, and
  returns the `CallToolResult` that `corvint-mcp` emits for the same call. Nothing is copied:
  the servers call the bridge, and each one only maps the bridge's error codes the way it maps
  its own tools' failures.
- The call reads only the binary. It skips each server's repository identity check, the corpus
  call deadline and `corvint/timing` `_meta`. Each server still needs its usual start arguments.
- The tool counts change: the docs server lists three tools, test-validity two, and the corpus
  server one more in every configuration. The `corvint-mcp-test-validity-conformance/0` tool list,
  the docs server's `tools/list` test, and the companion workflow smoke and installed-evidence
  tool lists were repinned in the same change, as `MCPV0-100` repinned `corvint-mcp`'s lists. The
  profile identity was not bumped: adding a tool is additive for MCP clients, and decision 0147's
  "one tool" is recorded as amended in `MTV-V0-001`.
- The shipped agent guide now names the four servers, so `catalog.json` was regenerated (only
  the guide digest changed).

## Evidence

Per server, a wire test runs the compiled `run` under both protocol versions. It checks that
`tools/list` carries the shared descriptor, that the call's result equals `bridge.HelpToolResult`
apart from the transport's own `_meta` and `resultType`, that both topics carry the embedded bytes
in `structuredContent` and in the enveloped text, and that an unknown topic is `-32602`. A second
test removes the repository root (and corpus artifact) after startup: help still answers, and the
server's own tool does not. `TestMCPV0105CompanionHelpMatchesTheCLI` binds the shared entry's text
to the CLI's stdout.

## Limits

Installed-host qualification of the three binaries was not run. Installed evidence captured before
this change lists the old tool sets, so it no longer matches the core-evidence validator. Fresh
capture is needed.

## Rollback

Revert the change. That removes `HelpDescriptor`, `HelpToolResult`, the three handlers' help
branch, the tests, and the repinned lists and amendments. No stored state depends on the tool.
