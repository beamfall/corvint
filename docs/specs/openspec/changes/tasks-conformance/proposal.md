# Change proposal: corvint-tasks 1.0 conformance pack, schema v1 and supported-command list

Change name: `tasks-conformance`
Ticket: none filed yet (task 1.1 files it in the corvint-tasks repository's `.taskman/` store and
links it from a `v1-0` ticket in this repository)
Product requirement served: PROD-TASK-010 and PROD-TASK-007 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/tasks-conformance-v1.md` in the `beamfall/corvint-tasks`
repository, a new spec with prefix `TCF-V1`
Requirement IDs this change adds: TCF-V1-001 to TCF-V1-010
Requirement IDs this change modifies: none (TIN-V1-002, TIN-V1-004 and TIN-V1-013 in the change
`tasks-integration` already defer the record format, schema files and command list to this change)
Owner answers this change implements: G9, owner decision R17 (2026-11-09), owner decision R18
(2026-11-09), answer 12, answer 13, owner decision H1 (2026-11-10)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/tasks-conformance/spec.md`. The work happens in the corvint-tasks repository; the folder is
recorded here because the Corvint RC check (TIN-V1-001) reads its output.

## Why

G9 couples the releases: Corvint 1.0 does not ship until corvint-tasks 1.0 passes its own conformance
pack, and Tasks is tagged first. Owner decision R17 (2026-11-09) removes the Tasks MCP server and MCP
parity from that pack and requires Tasks to publish schema v1 of its records and a supported-command
list. Owner decision R18 (2026-11-09) makes `beamfall/core` the conformance and trial subject. The
change `tasks-integration` defines what Corvint checks; nothing yet defines the record Tasks produces,
the schema files, the command list or the tests behind them.

## What exists today

Observed on 2026-11-09 (corvint-tasks build `1.0.0-rc.3.dev+build.144`, installed binary):

- `corvint-tasks` declares the formats `taskman-ticket/0`, `taskman-receipt/0`, `taskman-receipt/1`,
  `taskman-policy/0` and `taskman-release/0`. No `/1` ticket profile and no receipt reference
  profile exist.
- `corvint-tasks help` lists no `mcp` command.
- `corvint-tasks qualify --config FILE --spec PATH --base REV --out DIR --request-id ID` exists; its
  output format was not inspected for this proposal (NOT_VERIFIED).
- `receipt audit` and `queue status` exist and are used in this repository's completion workflow.
- corvint-tasks PR #4 (moving the Tasks specs to the Tasks repository) was OPEN on 2026-11-09.
- Whether `beamfall/core` already has an initialized `.taskman/` store is NOT_VERIFIED.

## What Changes

- corvint-tasks publishes schema files `taskman-ticket/1` and `taskman-receipt-ref/1` (proposed
  names, TIN-V1-004) and the shared fixture in its `testdata/`.
- corvint-tasks publishes a supported-command list: the commands that serve PROD-TASK-001 to 009 and
  the integrations I1 and I2. Every other command is marked experimental. No `mcp` command.
- A conformance record `taskman-conformance/1` is produced at the Tasks tag commit, covering: the nine
  requirements traced to passing tests; every listed command checked against schema v1; at least 8
  concurrent processes with overlapping claims and zero double claims; kill -9 during a write
  followed by a clean `receipt audit` and `queue status` with nothing lost or duplicated; `receipt
  audit` with zero failures.
- The 21-day trial on the frozen RC binary runs in Corvint and `beamfall/core`, with the activity
  floor of the change `release-process` (REL-V1-010). Stable waits for the later of the trial's
  end and the Corvint RC publish date plus 21 days (owner decision H1 (2026-11-10)).
- corvint-tasks ships linux/amd64, linux/arm64 and darwin/arm64 from its own repository, and
  `corvint-update` covers it (REL-V1-008).

## Capabilities

### New Capabilities

- `tasks-conformance` (corvint-tasks repository): schema v1, supported-command list, conformance
  record and its tests.

### Modified Capabilities

- None in this repository.

## Impact

- corvint-tasks repository: schemas, fixture, command list, tests, a record producer and release
  workflow.
- This repository: the vendored fixture and command list pinned by SHA-256 (TIN-V1-004,
  TIN-V1-013) and the RC check (TIN-V1-001).
- `beamfall/core`: a Tasks store for the trial, if it does not have one.

## Non-goals

- No Tasks MCP server and no MCP parity check (R17; supersedes the MCP item of G9).
- No Corvint code inside corvint-tasks and no shared library (answer 13).
- No change to the store format during the trial except to fix a blocker, which restarts the trial.
