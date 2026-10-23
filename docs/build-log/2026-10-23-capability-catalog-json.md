# Machine-readable command catalog and shipped agent guide

Date: 2026-10-23. Lane: claude/capability-catalog-gh1588 (base 0b7d59ba6). Issue: beamfall/corvint#1588.
Requirements `CAP-V0-015` to `CAP-V0-021` in `docs/specs/capability-catalog-v0.md` are accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

An agent could learn Corvint's surface only by reading `corvint help`, about 50 `COMMAND --help`
pages and the hand-written `docs/CAPABILITIES.md`. Nothing machine-readable came with the binary,
and no task-indexed guide shipped with the install.

## Decision

- `corvint help --json` prints `corvint-capability-catalog/0`. The new package
  `internal/capcatalog` builds it from a `Registry`, which `cmd/corvint/help_catalog.go` fills
  from the binary's own command registry: `topLevelCommands`, root help, each help topic, and
  the code-declared operation tables. Each field comes from a named source, and the document
  includes a `derivation` map that names it. Anything the registry cannot answer is `null`,
  with a reason under `unknowns`: inputs and outputs always, the owning spec of Core verbs
  (those carry `freezeContract: CCF-V1` instead), and writes for commands the root Support
  boundary does not classify. No field is hand-written.
- `corvint help guide` prints `internal/capcatalog/AGENT-GUIDE.md`, which is embedded in the
  binary. It maps goals to command sequences and includes the two-step Playwright discovery
  workflow. Its Non-capabilities section gives each item a tracking issue or a "By design"
  reason. The catalog binds the guide by SHA-256.
- Drift checks: `internal/capcatalog/catalog.json` is the committed snapshot of
  `corvint help --json`, and a `cmd/corvint` test compares the two byte for byte.
  `GuideProblems` fails when the guide names a command, operation, option or schema that the
  live catalog lacks. `make capability-catalog` regenerates the snapshot.
  `make capability-catalog-check` runs the tests. The owner accepted (2026-10-23, through the
  orchestrator) that these tests already run in the existing CI Go test shards, so `.github` is
  unchanged and no separate CI step is added.
- Root help keeps its three usage lines (CCF-V1-009). The second line now ends with
  `| corvint help --json|guide`.

## MCP help tool, and the MCPV0-008 amendment

The owner, through the orchestrator, directed on 2026-10-23 that MCP help ship before rc.3,
because every issue is to be fixed first and adding a tool is additive for MCP clients. New
requirement `MCPV0-100` in `docs/specs/mcp-server-2026-07-28-v0.md` adds one read-only tool,
`corvint.help`, to every `corvint-mcp` profile (default, task-review, task-review-lsp and flows).
With no arguments or `{"topic":"catalog"}` its receipt (`corvint-mcp-help/0`) carries the exact
bytes of `corvint help --json`, and with `{"topic":"guide"}` the exact bytes of
`corvint help guide`, plus their byte count and SHA-256. Both come from `internal/capcatalog` in
the binary, so the call reads no repository state and is deterministic. The catalog is 105,424
bytes, well under the 393,216-byte bridge budget; an oversized future catalog abstains with
`OUTPUT_BUDGET_EXCEEDED` rather than truncating.

This explicitly amends `MCPV0-008`, which said "exactly these three tools" and now says four, and
it supersedes, for this one binary-only tool, decision 0103's rule that a new MCP tool needs a
descendant profile identity. Every other new tool still needs one. The four byte-pinned
`tools/list` goldens, the conformance tool lists, and the companion workflow smoke's tool list
were repinned in the same change; the only difference in each golden is the added descriptor.
The docs, corpus and test-validity MCP servers did not gain the tool.

Parity: `TestMCPV0100HelpToolMatchesTheCLI` in `cmd/corvint` compares the bridge receipt text with
the stdout of `corvint help --json` and `corvint help guide`. `TestMCPV0100HelpToolAnswersOnEveryProfile`
in `cmd/corvint-mcp` checks the wire result in both `structuredContent` and the enveloped text
block on each profile.

## Not done, with reasons

- Guide as a separate release-archive member. The owner accepted (2026-10-23) the embedded guide as
  "shipped with the install", so no archive member is added. `archiveMembers`, `archivewire.MaxMembers` (6), the
  archive verifier, `internal/releasecandidate` (exactly six members), and the deployed
  `corvint-update` allowlist (`unexpected Core archive file`) all refuse a seventh member.
  Adding one now would break installed updaters. If a member is ever wanted, the rollout has two
  steps: first a tolerant updater and release-candidate check, then the member and its manifest
  pin.

## Evidence

Focused tests: `go test ./internal/capcatalog ./internal/mcp/... ./cmd/corvint-mcp
./conformance/mcp-2026-07-28 ./internal/companionrelease`, and the `cmd/corvint` help, core-freeze,
docs and `TestMCPV0100` tests with `-run`. Results are in the PR body.
