# Change proposal: report unassigned E2E tests and broken links in `corvint flows gaps`

Change name: `flows-unlinked-tests-report`
Ticket: V1-5255 (open, P1, milestone `v1-0`; the ticket stays in `.taskman/`)
Product requirement served: PROD-TEST-005 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/application-flow-understanding-v1.md` (prefix `AFU-V1`), inside
its single `## Requirements` section
Requirement IDs this change adds: AFU-V1-155 to AFU-V1-159 (fresh IDs, owner decision R11,
2026-11-09), plus AFU-V1-166 and AFU-V1-167 (owner answers 9 and 15; 160 to 165 are in use at
integ 9ec3a29d0). Retired, never reused: AFU-V1-150 to AFU-V1-154, which keep their integ 9ec3a29d0
meanings for lane `flowrep` (earlier wording: "AFU-V1-150 to AFU-V1-154 (reserved block; meanings
rewritten on 2026-11-09 for G5 and G11)", superseded). `AFU-V1-155` is a string prefix of no
existing ID in `docs/specs/REQUIREMENTS.tsv` at integ 9ec3a29d0 or in the other change folders.
Owner answers and decisions this change implements: G5, G11, owner answers 4, 9 and 15, owner
decision R11
(2026-11-09), owner decision R12 (2026-11-09) and owner decision R30 (2026-11-09)
Intent status: proposed (rewritten 2026-11-09; only the owner can accept it)
Delivery status: experimental code exists on `claude/layers-integ` (lane `flowrep`, head
`a28017c29`, merged compile-only, tests and review not run). It predates G5 and G11.
Written before dispatch: the first version was; this rewrite precedes the rework lane.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/application-flow-understanding/spec.md`.

## Why

A flow is made of variations with tests attached. `flows gaps` reports a variation with no test,
but nothing reports the other half: an E2E test that belongs to no flow. G11 makes that state
explicit: an E2E test linked to no flow always runs and is counted as `unassigned` in every plan and
in `flows gaps`; stale and dangling links are reported.

Verified today on `claude/layers-integ` at `9ec3a29d0`:

- `flows gaps` is registered in the `flows` command group `internal/command/flows/flows.go:124-126`,
  which decision 0595 placed in `corvint-docs`. `corvint` itself serves only `flows test-evidence`
  natively and forwards every other `flows` verb to `corvint-docs`
  (`runFlowsShim` in `cmd/corvint/flows_shim.go` at integ `9ec3a29d0`; its `test-evidence` branch
  is the only native verb).
- The tests layer may not import the `flows` command package
  (`tools/import-ratchet/flows_layer_test.go:28-33`, CMS-V0-916).

G5 moves reports that join flows with tests, including `gaps`, back into `corvint`, built on the
shared readers.

## What Changes

- `corvint flows gaps` runs natively in `corvint`, needs no other binary, and lists every inventory
  E2E test with no accepted, non-stale link as `unassigned` (G5, G11).
- The report also lists stale links (test content hash or routes changed since the pinned commit)
  and dangling links (test or variation no longer exists) (G11).
- An optional CI ratchet, off by default, fails when a change adds new `unassigned` tests (G11).
- `corvint.flows.gaps` on `corvint mcp` returns the same records; its row in the table of change
  `mcp-tool-table` fixes name, schema, profile and error codes (owner decision R30, 2026-11-09).
- Only links a person accepted count as links (owner decision R12, 2026-11-09).
- A variation is covered when it has an accepted link to a test of any kind: unit, integration or
  E2E (owner answer 9). A flow with no linked E2E test gets a `no-e2e-proof` record, which does not
  block (owner answer 15).
- An incomplete inventory is stated, never shown as a clean answer.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `application-flow-understanding`: the gaps report gains `unassigned`, `stale-link`,
  `dangling-link` and `no-e2e-proof` records, counts a linked test of any kind as coverage, and
  moves home to `corvint` (G5).

## Impact

- Code: a gaps query over the shared flow model and link-record reader in `internal/appflows`; a
  native `corvint flows gaps` entry in `cmd/corvint` (tests layer); `internal/mcp/bridge/flows.go`.
  The `corvint-docs` copy of `gaps` is removed so each verb has one home (G5).
- Boundary rule (G5): joins with test evidence or results belong to tests; authoring and discovery
  belong to docs.
- Wire: `application-flow-gaps/1` gains codes as additive values of its existing code field, with
  no new profile version; a reader ignores a code it does not know (decided 2026-11-09 under R42,
  N1). Link records hold no test kind; the kind is read from the inventory by `test_id` (decided
  2026-11-09 under R42, N2).
- Depends on: `.corvint/flows.json` link records (`affected-plan-kinds`), the pinned inventory
  (`e2e-inventory-discovery`), flow-file versioning (`flow-schema-version`) and the MCP table
  (`mcp-tool-table`).

## Non-goals

- Proposing or accepting links (`flows propose --links` in `corvint-docs`, G11).
- Listing unit and integration tests as `unassigned`. Only inventory E2E tests can be
  `unassigned`. A unit or integration test does count toward coverage once it is linked to a
  variation (owner answer 9, AFU-V1-166).
- Blocking on a flow with no E2E test. `no-e2e-proof` is a report record and never changes the exit
  code (owner answer 15, AFU-V1-167).
- Any other change to `no-test`. It keeps its code and now applies only to a variation with no
  accepted link to a test of any kind (owner answer 9).
