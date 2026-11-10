# Change proposal: proposed flow links with a confidence score, accepted by a person

Change name: `flow-link-proposals`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-DOCS-012 in `docs/specs/PRODUCT-PLAN-1.0.md` (propose flow
declarations and test links with cited evidence; a person accepts per flow), and the E2E basis of
PROD-TEST-004.
Owning capability spec: a new spec `docs/specs/flow-link-proposals-v1.md` (prefix `FLP-V1`). It does
not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: FLP-V1-001 to FLP-V1-011. `FLP-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `canonical-test-id`: a link names its test by canonical test ID.
- `test-inventory`: proposals are made for tests in the inventory, and dangling links are found
  against it.
- `run-evidence`: observed routes and outcomes are proposal evidence.
- `line-evidence`: tier `flow` and the one-line reason come from there.

This change is fifth and last in the build order fixed by owner decision R38 (2026-11-09). The flow
file `.corvint/flows.json` and its schema version belong to change `affected-plan-kinds` and the flow
schema work under G6; this change adds link records to it.

## Why

G11 (owner answer): an E2E test linked to no flow always runs and is counted `unassigned`; links
live in `.corvint/flows.json`; Corvint proposes links with cited evidence (`flows propose --links`,
bulk bootstrap); a person accepts per flow; a link record holds test_id (file path plus title path),
variation_id, evidence, accepted_by, accepted_at and the pinned commit; tags in test source are
proposal evidence only; a link goes stale when the cited test content hash or routes change; static
reachability must still agree; dangling links are reported.

Owner decision R12 (2026-11-09): each proposed link carries a confidence score, and only a person
accepts a link in 1.0. Unaccepted links never narrow.

Owner decision R31 (2026-11-09): only `.corvint` config files force a full run. A proposals file
under `.corvint/` would therefore force full runs whenever proposals change, so proposals are kept
outside it.

### What exists at integ 9ec3a29d0

- `.corvint/e2e-flows.json` (profile `corvint-e2e-flows/0`) maps a consumer file to a list of flow
  IDs; it holds no per-test link records and no acceptance fields
  (`internal/appflows/e2eflows.go`, lines 32-53, integ 9ec3a29d0).
- `flows import` with a Playwright JSON list writes one proposed ui flow per listed test, keyed
  `path > describe > title`; links are inferred and nothing records who accepted them
  (`internal/command/flows/flows.go`, help text at lines 455-465, integ 9ec3a29d0).
- The `flows` subcommands are export, import, map, gaps, impact, ingest, docs, stability,
  ingest-coverage and test-evidence; there is no `propose` and no `accept`
  (`internal/command/flows/flows.go`, lines 124-126, integ 9ec3a29d0).

## What changes

- `corvint-docs flows propose --links` proposes links for unassigned E2E tests, and `--all` runs the
  bulk bootstrap over every E2E test in the inventory (FLP-V1-001).
- Proposals are written to a private file, `<git-dir>/corvint/flow-link-proposals.json`, never under
  `.corvint/` (FLP-V1-002).
- Each proposal carries a confidence score from 0.00 to 1.00, computed deterministically by a
  versioned formula and shown beside its cited evidence (FLP-V1-003).
- `corvint-docs flows accept --flow ID --by NAME` accepts the proposals of one flow and writes link
  records into `.corvint/flows.json`. No MCP tool, hook, Tasks gate or GitHub Action path accepts
  (FLP-V1-004, FLP-V1-005).
- The link record fields (FLP-V1-006).
- Only accepted links are selection inputs; proposals and source tags never narrow (FLP-V1-007,
  FLP-V1-008).
- Stale, disagreeing and dangling links are reported and their tests run (FLP-V1-009, FLP-V1-010).
- Browser E2E tests outside Playwright can be linked, but select their whole group in 1.0 (G13)
  (FLP-V1-011).

## Non-goals

- No automatic acceptance in 1.0, whatever the score (owner decision R12). The G14 auto-accept of
  implementing proposals does not apply to flow links.
- No proposals stored under `.corvint/` (owner decision R31).
- No flow file schema or version rules (G6; change `affected-plan-kinds`).
- No CI ratchet on new unassigned tests; G11 makes it optional and off by default, and it is not in
  this change.
- No line-evidence narrowing of browser tests (G2; change `line-evidence`).

## Impact

- New spec `docs/specs/flow-link-proposals-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- `corvint-docs` gains `flows propose --links` and `flows accept` (authoring stays in
  `corvint-docs`, G5).
- `.corvint/flows.json` gains link records. Accepting links changes a `.corvint` config file, so the
  next plan runs in full once (owner decision R31).
- E2E selection (changes `e2e-inventory-discovery`, `affected-plan-kinds`) reads accepted links only.
- `flows-unlinked-tests-report` and `flows-status-report` report stale and dangling links.
