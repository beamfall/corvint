# Decision 0374 — MCP task-review tools ship as an opt-in descendant profile

Date: 2026-09-24. Status: accepted. Authority: a wire-contract expert decision, made at the
repository owner's request on 2026-09-24, on how ticket V1-0191 may add `corvint.context` and
`corvint.cem.report` to `corvint-mcp`. This decision applies decision 0103 and does not supersede it.

Decision 0103 froze the MCP 2026-07-28 V0 tool list at `corvint.query`, `corvint.impact` and
`corvint.status`. It also said a new MCP tool belongs in a descendant profile with its own tool
schema and black-box vectors. The first V1-0191 candidate added the two tools to the default list
and to conformance profile `corvint-mcp-2026-07-28-conformance/0`. That changes a published interop
profile without a new identity, the silent broadening AGENTS.md invariant 7 forbids.

The decision:

- `corvint-mcp --root ROOT` keeps advertising exactly the three V0 tools (`MCPV0-008`). Profile
  `/0`, its case inventory and its three-tool assertions are unchanged.
- One closed argv selector, `--tool-profile task-review`, additionally advertises `corvint.context`
  and `corvint.cem.report` (`MCPV0-026`). It follows the `MCPV0-021` precedent: at most one selector,
  exactly one accepted value, and failure before repository startup on a missing, unknown or
  duplicate value or when combined with `--version`. It composes with `--protocol-version
  2025-11-25` because the legacy profile reuses the selected registry.
- A call to a tool the selected profile does not advertise fails as `unsupported-tool`, which is
  `-32602` on the wire, the same as any unknown tool.
- The two tools keep their requirement IDs (`MCPV0-024`, `MCPV0-025`) and move into a
  descendant-profile section of the spec. Conformance profile `corvint-mcp-2026-07-28-conformance/1`
  (`cases-task-review.json`) names `/0` as its parent and covers the selector, the default-profile
  refusal, the five-tool catalogue, the legacy composition, and the two tools' read-only, rejection,
  escape, envelope, filter, pinned-Git and official-schema cases.

Why a selector rather than a new binary or a new protocol version: the tools reuse the same root
binding, bridge, envelope and budget, so a second executable would duplicate the whole trust
boundary. The protocol version names the wire lifecycle, not the tool list. A closed argv selector
is the smallest explicit opt-in, and a client that never passes it cannot observe any change. At
least one shipped client depends on that: the VS Code extension rejects any tool list other than
the three V0 descriptors.

## Applied

- `internal/mcp/bridge`: `New` binds the three-tool registry; `NewTaskReview` binds the five-tool
  registry; `Registry.Tools` and `Registry.Call` gate both tools on the selected profile.
- `cmd/corvint-mcp`: `extractToolProfile` parses the closed selector.
- `docs/specs/mcp-server-2026-07-28-v0.md`: `MCPV0-008` stays at three tools; `MCPV0-026` and the
  moved `MCPV0-024`/`025` form the task-review section; the acceptance matrix, non-goals, rollback,
  failure-code table and traceability name the profile.
- `conformance/mcp-2026-07-28`: `/0` restored to the three-tool manifest and assertions;
  `cases-task-review.json` and `task_review_test.go` added; the context and CEM report cases run
  under the selector.
- `docs/MCP-SERVER.md` and `README.md` document the selector.
- `extensions/vscode` needs no change. `src/mcp.ts` spawns `corvint-mcp --root ROOT` with no
  other argument and compares the `tools/list` result exactly against the three V0 descriptors,
  failing with `toolset-mismatch` otherwise. The unchanged default keeps it working; the first
  candidate's five-tool default would have broken it.

## Rollback

Revert this decision's commit, or delete the selector and the profile: remove
`extractToolProfile` and `NewTaskReview`, the two tool descriptors and bridge cases, the `/1`
manifest and tests, and the task-review spec section. The default output never changed, so no
client, stored state or `/0` vector needs migration.
