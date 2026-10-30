# Decision 0554 — `corvint.help` partially supersedes decision 0103

Date: 2026-10-23. Status: accepted. Authority: this file records a call already made, the owner's
directive through the orchestrator on 2026-10-23 (beamfall/corvint#1588) that MCP help ship before
rc.3. `MCPV0-100` in `docs/specs/mcp-server-2026-07-28-v0.md` applied it in code, and the
`MCPV0-008` amendment and `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-capability-catalog-json.md` describe it. No new
owner call is made here.

## Problem

Decision 0103 (`0103-mcp-v0-stays-three-tools-2026-09-12.md`) froze MCP 2026-07-28 V0 at exactly
three tools (`corvint.query`, `corvint.impact`, `corvint.status`) and required any new MCP tool to
use a descendant profile with its own identity. `MCPV0-100` added a fourth tool, `corvint.help`, to
every `cmd/corvint-mcp` profile, including the default one. The spec recorded that this supersedes
part of decision 0103, but no decision record said so, and 0103 itself still read as fully in force.

## Decision

Superseded, for the `corvint.help` tool only:

1. The tool count. The default `corvint-mcp` V0 profile advertises exactly four tools: the three of
   decision 0103 plus `corvint.help`. Every other profile (`task-review`, `task-review-lsp`,
   `flows`) also advertises `corvint.help` (`MCPV0-100`).
2. The descendant-profile rule, as it applies to this tool. `corvint.help` joins the default profile
   without a new profile identity. That is allowed because it is binary-only and repository-free: it
   reads no repository, index, Git or trace state, and returns only the running binary's embedded
   `corvint help --json` or `corvint help guide` bytes.

Still in force from decision 0103:

- Any other new `corvint-mcp` tool still needs a descendant profile with its own tool schema and
  black-box vectors, as decisions 0374 (task-review) and 0147 (test-validity) applied it. This
  exception does not extend to any other tool, however small.
- `LPCV-V0-047` binds only surfaces that present a test-level result. MCP V0, `corvint.help`
  included, presents none, so the clause places no obligation on it. An MCP projection of test
  validity is still a descendant profile (`corvint-test-validity-mcp`, decision 0147).
- The frozen `MCPV0-001` argv (`corvint-mcp --root ABSOLUTE_ROOT`) is unchanged, and the MCP
  non-goals that exclude test execution and test-level results stand.
- The bridge `tool` enum stays closed. It gained exactly `corvint.help`.

Not decided here: whether the other MCP binaries (`corvint-docs-mcp`, `corvint-corpus-mcp`,
`corvint-test-validity-mcp`) gain a help tool. Each is its own profile, and `MCPV0-100` lists that
as a non-goal.

## Applied

Docs only. Decision 0103 gains a one-line "partially superseded by" pointer under its status line,
and its body is otherwise unchanged. The index row in `docs/decisions/README.md` is added, and 0103's
row notes the partial supersession. No code, schema, golden, conformance vector or spec requirement
changes. `MCPV0-008` and `MCPV0-100` already carry the amendment text.

## Rollback

Revert this decision's commit. That removes this file, its index row, 0103's pointer and the README
status note. Rolling back `corvint.help` itself is described in `MCPV0-100`; if that happens, this
decision should be marked withdrawn in the same change, since 0103 would then hold in full again.
