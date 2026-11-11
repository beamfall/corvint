<!-- SPDX-License-Identifier: Apache-2.0 -->
# MCP affected tool: a stale binary is the `self-binary-stale` tool error

Builder decision while fixing the batch's test failures (branch `claude/batch-1110`, base
`origin/main` 4c2e6b6a2).

## Collision

Two accepted requirements answered a stale `corvint` binary differently.

- `AFP-V0-9730` (`docs/specs/affected-plan-selection-v0.md`): `corvint affected` refuses a binary
  older than the repository's Go source with exit 2 and the code `self-binary-stale`, unless the
  caller passes `--accept-stale-binary`.
- `MMR-V0-1118` (`docs/specs/mcp-multi-root-v0.md`), as written before this change: the MCP
  `affected` tool answers a stale binary as a successful verdict whose `degradations` name
  `self-binary-stale`, with no tool error.

The MCP runner calls `affected --summary` without `--accept-stale-binary`, so the second text
described a verdict the CLI no longer produces. `TestV15254MCPAffectedStaleBinaryDegrades` failed on
the merged tree for that reason.

## Choice

`AFP-V0-9730` wins. A plan computed by a planner older than the source it plans can select fewer
tests than the change needs, and an MCP caller reads `degradations` less reliably than a tool
error. The tool now answers the group's tool error with the CLI's `code` and `error`. The tool
description and `corvint mcp --help` name the recovery: rebuild `corvint` and restart
`corvint mcp`.

## Changed

- `docs/specs/mcp-multi-root-v0.md`: the stale-binary bullet, the failure-mode list and the
  acceptance evidence of `MMR-V0-1118`.
- `internal/mcp/toolgroup/affected.go`, `cmd/corvint/mcp.go`: description and help text.
- `cmd/corvint/mcp_affected_test.go`: the test requires the tool error. Its name keeps the word
  `Degrades` so the requirement index and earlier evidence still resolve.

## Not decided here

Whether the MCP tool should offer an explicit opt-in equal to `--accept-stale-binary`. No caller
asked for it; it would need its own requirement.

## Rollback

Revert the three items above together. No wire format or persisted state depends on them.
