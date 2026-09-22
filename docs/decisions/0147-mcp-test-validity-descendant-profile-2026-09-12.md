# Decision 0147 — The test-validity projection reaches MCP through a separate descendant profile

Date: 2026-09-12. Status: accepted. Authority: owner instruction 2026-09-12 via delegation.

`PUB-V0-006` item 1 (`docs/plans/pub-v0-006-reach-gap-2026-09-12.md`) found that the
`corvint-test-validity/0` projection `corvint test-validity` computes (`LPCV-V0-051`) never crossed
MCP. Decision 0103 keeps MCP 2026-07-28 V0 at exactly three tools (`MCPV0-008`) and requires a
descendant profile for any test-validity tool.

The owner call: take the smallest descendant shape, the one `corvint-docs-mcp` already uses. A
separate executable, `corvint-test-validity-mcp --root ABS`, reuses the MCP 2026-07-28 stdio
transport and advertises one read-only tool, `corvint.test_validity`
(`docs/specs/mcp-test-validity-profile-v0.md`, `MTV-V0-001..008`, experimental). The caller names
a receipt path confined to the repository worktree; without one every axis is `UNSUPPORTED`. The
projection logic moves into `internal/testvaliditydoc`, which the CLI verb and the tool both call,
so they cannot disagree. `corvint-mcp`, its bridge enum and `conformance/mcp-2026-07-28` are
unchanged; the new vectors live in `conformance/mcp-test-validity-v0`.

Rejected: a fourth tool on `corvint-mcp` (breaks the frozen profile and decision 0103), and a
negotiated profile id inside `corvint-mcp` (larger change, adds an argv/enum surface to a frozen
binary).

Disclosed residue: the caller must name the receipt, so tracking is not automatic and
`PUB-V0-006` stays unmet; Go provider sessions are still not an accepted input; a projection over
256 KiB is refused, not truncated; the path-component race and the linked-network-code limit are
stated in the spec's failure modes; the binary is not in the release archives; official MCP schema
validation is `NOT_RUN`.

Rollback: delete `cmd/corvint-test-validity-mcp`, `internal/mcp/testvaliditybridge` and
`conformance/mcp-test-validity-v0`, and mark the spec superseded. `internal/testvaliditydoc` may
stay; CLI output is unchanged either way.
