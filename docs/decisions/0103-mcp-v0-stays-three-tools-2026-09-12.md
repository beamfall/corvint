# Decision 0103 — MCP 2026-07-28 V0 stays at three tools; `LPCV-V0-047` does not bind it

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`LPCV-V0-047` requires every client that presents a test-level result to project it through the
shared `internal/testvalidity` shape, and its traceability row read the clause as open on the
"CLI/MCP surface". Presenting test-level results over MCP would need a fourth advertised tool, which
`MCPV0-008` forbids (exactly `corvint.query`, `corvint.impact`, `corvint.status`), which the closed bridge
`tool` enum and `conformance/mcp-2026-07-28/mcp_blackbox_test.go` (`len(tools) != 3`) enforce, and
whose results would need a receipt location that the frozen `MCPV0-001` argv
(`corvint-mcp --root ABSOLUTE_ROOT`) cannot configure.

The owner call: keep MCP 2026-07-28 V0 frozen at three tools, and scope `LPCV-V0-047` to surfaces
that actually present a test-level result. MCP V0 presents none, so the clause places no obligation
on it. An MCP projection of test validity is a descendant MCP profile with its own tool schema,
receipt-location contract, and black-box vectors.

- The clause is conditional as written ("every client that presents a test-level result"). The
  earlier row read it as an obligation to add a presentation, which the text does not require.
- The MCP surface is a black-box-conformance profile whose non-goals already exclude test
  execution. Adding a tool, widening the bridge enum, and adding an argv option in place would
  change a published interop profile's tool list without a new profile identity, the silent
  broadening AGENTS.md invariant 7 forbids.
- `LPCV` delivery is `not-started`; no CLI command presents a test-level result either, so the
  CLI half of the clause stays unevidenced and open, unchanged by this decision.

Applied: `LPCV-V0-047` gains a scoping sentence; its traceability row names the MCP half as out of
scope under this decision; the MCP spec non-goals name test-level results and their projection. No
code, schema, bridge enum, or conformance vector changes. The comment citation in
`internal/testvalidity/vectors_test.go` is repinned to `LPCV-V0-050`'s shifted line.

Rollback: revert this decision's commit, which restores the three spec passages and the
`questions.md` entry; no wire bytes change.
