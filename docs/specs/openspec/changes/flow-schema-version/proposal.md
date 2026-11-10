# Change proposal: versioned flow file, one reader that refuses unknown versions, and a standalone smoke test

Change name: `flow-schema-version`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-INT-016 (and the PROD-DOCS-011 flow reports it exercises) in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/flow-schema-version-v1.md`, a new spec with prefix `FSV-V1`
Requirement IDs this change adds: FSV-V1-001 to FSV-V1-007
Requirement IDs this change modifies: none
Owner answers this change implements: G5
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/flow-schema-version/spec.md`.

## Why

G5 keeps the flow reports that join test evidence native in `corvint`, and requires the flow file to
carry a schema version that every reader checks, plus a release smoke test with `corvint` installed
alone. Once `corvint` and `corvint-docs` ship and update separately (PROD-INT-004), a newer flow file
can meet an older reader. Without one version rule, an older reader either fails with a generic
parse error or, worse, a future reader that relaxes unknown-member checks reads a newer file
partially and plans fewer tests than it should.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- `internal/appflows/flowplan.go` lines 31-32 define `.corvint/flows.json` with profile
  `corvint-flows/0`; `ReadFlowsDeclaration` (line 82) sets `json.RejectUnknownMembers(true)`
  (line 102) and refuses any other profile with "profile must be corvint-flows/0" (lines 105-106).
- `internal/appflows/e2eflows.go` lines 32-33 define `.corvint/e2e-flows.json` with profile
  `corvint-e2e-flows/0`; lines 88-89 refuse a mismatched profile and line 123 sets
  `E2E_FLOWS_INVALID`.
- The affected planner maps a bad flows declaration to `FLOWS_DECLARATION_INVALID`
  (`internal/liveverify/affected/e2eplan.go` line 46) and a bad E2E declaration to
  `E2E_FLOWS_INVALID` (`internal/liveverify/affected/execs.go` line 27). Neither code says that the
  file was valid but newer than the reader.
- `corvint flows` is a `corvint` verb (`cmd/corvint/main.go` line 646).
- No release smoke test installs `corvint` alone.

## What Changes

- The `profile` member `corvint-flows/N` is the schema version. N is a whole number.
- One shared reader in `internal/appflows` is the only code that parses `.corvint/flows.json`; both
  the testing product and the documentation product call it.
- A reader accepts only the versions it was built for. A newer version is refused with a new typed
  reason `FLOWS_SCHEMA_UNSUPPORTED` that names the found and supported versions; the affected plan
  then runs every E2E test with that reason and never narrows on the file.
- Any new member, removed member or changed meaning increments N, because unknown members are
  already refused.
- A release smoke test installs only `corvint` from the candidate archive and runs `corvint affected`
  and the flow reports on a pinned fixture, including a fixture that declares an unsupported version.

## Capabilities

### New Capabilities

- `flow-schema-version`: version rule, shared reader, refusal reason and smoke test.

### Modified Capabilities

- None. The member set of `.corvint/flows.json`, the move of `.corvint/e2e-flows.json` into it and
  the plan kinds belong to the change `affected-plan-kinds`; this change fixes how any version of
  that file is read.

## Impact

- `internal/appflows`, `internal/liveverify/affected` and the `corvint flows` commands.
- `corvint-docs` reads the flow file only through the shared reader (feature map, change
  `feature-map-from-flows`).
- The release workflow of the change `release-process` gains one smoke job.
- The error catalogue of the change `user-docs` gains `FLOWS_SCHEMA_UNSUPPORTED`.

## Non-goals

- No migration of `.corvint/e2e-flows.json` (change `affected-plan-kinds`).
- No reading of older or newer versions by translation; a reader supports a closed list.
- No version field inside other `.corvint/` files in this change.
- No change to how flows select tests.
