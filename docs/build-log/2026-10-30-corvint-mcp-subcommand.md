# 2026-10-30 — `corvint mcp`: the plugins' MCP server ships in the core binary (V1-3244, AHI-373)

## Problem

The Claude Code plugin (`AHI-198`) and the Codex plugin (`AHI-207`) launched `corvint-mcp` from
`PATH`, but the core release archive allowlists only the `corvint` executable, and no install path
builds `corvint-mcp`. Every release user therefore saw `plugin:corvint:corvint` as failed in `/mcp`
and `claude mcp list` (observed in this session's own host before the change: "failed to connect").

## Decision

Serve the same server from the core binary as `corvint mcp ARGS` rather than adding a second
release executable (invariant 7: one native-Go binary). `cmd/corvint-mcp` moved to
`internal/mcp/cli` (`Serve`); `cmd/corvint-mcp/main.go` and `cmd/corvint/mcp.go` both call it, so
the wire is byte-identical and `serverInfo.name` stays `corvint-mcp`. Both plugin `.mcp.json` files
now launch `corvint mcp`; plugin versions are Claude Code 0.3.7 and Codex 0.2.12.
`TestAHI373PluginCommandsShipInCoreRelease` fails if any plugin hook or MCP command is not in
`update.CoreExecutables`, which now feeds the core archive allowlist. The core binary grows by
about 0.86 MB (49.39 to 50.25 MB, darwin/arm64).

## Evidence

- Live, Claude Code 2.1.311 with `--plugin-dir integrations/claude-code/plugins/corvint` and the
  branch build first on `PATH`: `plugin:corvint:corvint: corvint mcp --root <worktree>
  --protocol-version 2025-11-25 - ✔ Connected`.
- Stdio against `corvint mcp --root <worktree> --protocol-version 2025-11-25`: `initialize` result,
  `tools/list` = `corvint.help, corvint.impact, corvint.query, corvint.status`, `corvint.status`
  call `isError: false`.
- Focused tests: `internal/mcp/cli`, `internal/update`, `conformance/mcp-2026-07-28` whole package
  (selector FALLBACK: `UNINDEXED_SOURCE_PATH` for the moved files); `cmd/corvint` AHI-373/AHI-207,
  require-clean, help/catalog/command-tree, profile-knob and `TestHostAdapterJavaScriptHosts`
  (AHI-198). `make windows-vet`, doc gates and use-case conformance pass.
- `NOT_RUN`: a live model turn calling the tool (Claude OAuth expired in this shell); Codex live
  launch; the exhaustive repository gate (batch CI covers it).

## Rollback

Revert both `.mcp.json` files to `corvint-mcp` under new plugin versions and remove the verb.
