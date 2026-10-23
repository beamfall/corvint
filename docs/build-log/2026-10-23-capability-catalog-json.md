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
  `make capability-catalog-check` runs the tests. Wiring a CI step needs owner
  ci-control-plane approval, so `.github` is unchanged.
- Root help keeps its three usage lines (CCF-V1-009). The second line now ends with
  `| corvint help --json|guide`.

## Omitted, with reasons

- MCP help tool or resource. MCPV0-008 freezes the V0 tool list, and byte goldens pin every
  profile's `tools/list`. The shared `internal/capcatalog` package makes the follow-up a small
  change once that contract is amended.
- Guide as a separate release-archive member. `archiveMembers`, `archivewire.MaxMembers` (6), the
  archive verifier, `internal/releasecandidate` (exactly six members), and the deployed
  `corvint-update` allowlist (`unexpected Core archive file`) all refuse a seventh member.
  Adding one now would break installed updaters. The rollout has two steps: first a tolerant
  updater and release-candidate check, then the member and its manifest pin. Until then the
  guide ships inside the binary.

## Evidence

Focused tests: `go test ./internal/capcatalog`, and the `cmd/corvint` help, core-freeze and docs
tests with `-run`. Results are in the PR body.
