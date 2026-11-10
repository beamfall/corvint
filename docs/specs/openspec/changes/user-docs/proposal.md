# Change proposal: user documentation for 1.0 (first run, configuration, upgrade, errors, release notes, policy statements)

Change name: `user-docs`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: product gaps (a) to (k) in `docs/specs/PRODUCT-PLAN-1.0.md` ("Product
gaps (open)"), PROD-INT-004, PROD-INT-006, PROD-INT-015, PROD-INT-017 and PROD-INT-018
Owning capability spec: `docs/specs/user-docs-v1.md`, a new spec with prefix `UDC-V1`
Requirement IDs this change adds: UDC-V1-001 to UDC-V1-012
Requirement IDs this change modifies: none
Owner answers this change implements: answer 16 (Windows statement), owner decision R33
(2026-11-09) for the `corvint-vscode` repository, and R42 (panel pick) B6: all items (a) to (k)
ship as pages in the repository (Q35; NEW-PROC-1 to 3; checklist O30, O64, O27, O28). The content
of the compatibility statement (g) is the table of change `compatibility-promise` (decided
2026-11-09 under R42, D10 and D10a).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/user-docs/spec.md`.

## Why

The plan lists user-facing items with no requirement: first-run behaviour, a configuration reference,
upgrade from pre-1.0, an error and exit-code catalogue, release notes, where the pages live, and a
1.x compatibility promise. 1.0 removes verbs and files (`cem`, `runs`, `test-plan`, `tests lint`, the
analyzers, `.corvint/e2e-flows.json`, CEM files), so a user upgrading from an alpha or RC needs to
know what replaced each one. Without a catalogue checked against the code, the typed reasons that the
products return (for example `FLOWS_DECLARATION_INVALID`) have no user-facing explanation.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- Present: `SECURITY.md`, `docs/SECURITY.md`, `docs/INSTALL.md`, `docs/MCP-SERVER.md`,
  `docs/CAPABILITIES.md`, `docs/RELEASE-NOTES-alpha.md`.
- Absent: `CHANGELOG.md`, `docs/ERRORS.md`, `docs/EXIT-CODES.md`, `docs/CONFIGURATION.md`.
- `docs/INSTALL.md` line 333 starts an uninstall paragraph that tells the user to close MCP children
  and uninstall the Corvint VSIX; it does not mention the `.corvint/` directory as removable state.
- `.corvint/` in Corvint's own checkout holds more than 25 files; most configure Corvint's own
  self-development and are not read by a user repository.
- PROD-INT-006 notes (checklist O27) propose that Corvint sends no telemetry. PROD-INT-018 (checklist
  O28) proposes copying `SECURITY.md` into `beamfall/corvint-action` and `beamfall/corvint-agents`.

## What Changes

- (a) A first-run page per product (testing, documentation, Tasks), the GitHub Action and the agent
  packages: install, the first command, the expected output, and what to do when it abstains.
- (b) `docs/CONFIGURATION.md`: every `.corvint/` file and environment variable a shipped binary reads
  in a user repository, including `CORVINT_SELECTION`, checked against the path and variable
  constants in code.
- (c) `docs/UPGRADING.md` and runtime messages: each removed verb prints its replacement or "removed
  with no replacement" and exits with a stable code; migration steps for `.corvint/e2e-flows.json`
  and CEM files.
- (d) `docs/ERRORS.md`: every exit code and typed reason, checked against the constants.
- (e) `CHANGELOG.md` and per-tag release notes.
- (f) Pages live in the repository under `docs/`; no documentation site (R42 (panel pick) B6).
- (g) A 1.x compatibility statement; its content is the `compatibility-promise` table
  (CMP-V1-001 to 006).
- (h) A no-telemetry statement.
- (i) The uninstall paragraph in `docs/INSTALL.md` is replaced.
- (j) `SECURITY.md` in each public repository: `corvint-action`, `corvint-agents`, `corvint-tasks`
  and `corvint-vscode`.
- (k) A statement that native Windows is unsupported, with the WSL path.

## Capabilities

### New Capabilities

- `user-docs`: user pages and the checks that tie them to code.

### Modified Capabilities

- None.

## Impact

- New files under `docs/`, plus `CHANGELOG.md` at the root.
- `cmd/corvint` and `cmd/corvint-docs`: stub handlers for removed verbs print the upgrade message.
- `doc-gates` gains the configuration and error catalogue checks.
- Other repositories (`corvint-action`, `corvint-agents`, `corvint-tasks`, `corvint-vscode`) each
  get a `SECURITY.md` through their own PRs.
- Other changes add rows: `flow-schema-version` (`FLOWS_SCHEMA_UNSUPPORTED`), `mcp-tool-table` (MCP
  profile refusal), `release-process` (release notes per tag).

## Non-goals

- No documentation site build (f; R42 (panel pick) B6).
- No translated pages.
- No documentation of Corvint's own self-development configuration files as user configuration.
- No content for the compatibility statement beyond the `compatibility-promise` table (g).
