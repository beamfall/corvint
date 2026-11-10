# Spec delta: flow-link-proposals

Capability spec: `docs/specs/flow-link-proposals-v1.md` (new). This delta is a proposal only; the
canonical spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: FLP-V1-001 Propose links, with a bulk bootstrap
Source: PROD-DOCS-012; G11 (owner answer); owner decision R12 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
`corvint-docs flows propose --links` MUST propose, for each unassigned E2E test in the test inventory,
the flow variations it most likely exercises, each with cited evidence. `--all` MUST propose for every
E2E test in the inventory. Proposing MUST NOT run tests and MUST NOT change any file under
`.corvint/`.

#### Scenario: bootstrap on a repository without links
- **GIVEN** a repository with flows and twenty Playwright tests and no accepted links
- **WHEN** `corvint-docs flows propose --links --all` runs
- **THEN** proposals with evidence appear for the tests, and no file under `.corvint/` changes

### Requirement: FLP-V1-002 Proposals are private
Source: owner decision R31 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Proposals MUST be written to `<git-dir>/corvint/flow-link-proposals.json`, MUST NOT be committed and
MUST NOT be stored under `.corvint/`. No selection path MAY read the proposals file.

#### Scenario: proposals change
- **GIVEN** a new proposal run replaces the proposals file
- **WHEN** `corvint affected` plans the next change
- **THEN** the plan is the same as before the proposal run and no full run is forced

### Requirement: FLP-V1-003 Confidence score
Source: owner decision R12 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Each proposal MUST carry `confidence`, a number from 0.00 to 1.00 with two decimals, and
`confidenceFormula`, the version of the formula that produced it. The formula MUST be deterministic
over its cited evidence and MUST list any input it lacked. The score MUST NOT accept a link and MUST
NOT be a selection input.

#### Scenario: same inputs, same score
- **GIVEN** two proposal runs on the same commit, inventory and stored run evidence
- **WHEN** their outputs are compared
- **THEN** every proposal has the same score and formula version

### Requirement: FLP-V1-004 Only a person accepts, per flow
Source: PROD-DOCS-012; G11 (owner answer); owner decision R12 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
`corvint-docs flows accept --flow ID --by NAME` MUST copy the chosen proposals of one flow into
`.corvint/flows.json` as link records. It MUST refuse with `flow-proposals-absent` when no proposals
file exists and with `flow-proposals-stale` when the proposals were made at another commit. No MCP
tool, hook, Tasks gate or GitHub Action step MAY accept a link, whatever its score.

#### Scenario: high score, not accepted
- **GIVEN** a proposal with confidence 0.99 and no accept command run
- **WHEN** `corvint affected` plans a change that reaches the proposed flow
- **THEN** the test runs as `unassigned`

### Requirement: FLP-V1-005 Accept refuses without a terminal
Source: owner decision R12 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
`flows accept` MUST refuse with `flow-accept-needs-person` when standard input is not a terminal. The
specification MUST state that Corvint cannot prove a person ran the command and that review of the
commit changing `.corvint/flows.json` is the check.

#### Scenario: accept from a script
- **GIVEN** `flows accept` run with standard input redirected from a file
- **WHEN** it starts
- **THEN** it exits non-zero with `flow-accept-needs-person` and writes nothing

### Requirement: FLP-V1-006 Link record fields
Source: G11 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
A link record MUST hold `test_id` (the canonical test ID, CTI-V1-001, carrying the file path and
title path), `variation_id`, `evidence`, `confidence`, `confidenceFormula`, `accepted_by`,
`accepted_at` (UTC), `pinned_commit`, `test_content_hash` and `routes_digest`. A record missing
`accepted_by`, `accepted_at` or `pinned_commit` MUST be treated as unaccepted. A link record MUST
NOT hold a test `kind`; the kind is read from the test inventory by `test_id` (decided 2026-11-09
under R42, N2).

#### Scenario: hand-written record without acceptance
- **GIVEN** a link record in `.corvint/flows.json` with no `accepted_by`
- **WHEN** a plan reads it
- **THEN** the test runs as `unassigned` and the record is reported as unaccepted

### Requirement: FLP-V1-007 Only accepted links narrow
Source: G11 and G2 (owner answers); owner decision R12 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
E2E selection MUST read link records only from `.corvint/flows.json` and MUST use only accepted
links. A browser E2E test with no accepted link MUST run and MUST be counted `unassigned`. A
selection made through an accepted link MUST carry tier `flow` (LEV-V1-009).

#### Scenario: after acceptance
- **GIVEN** an accepted link from a Playwright test to variation V, and a change that reaches no file
  or symbol of V
- **WHEN** `corvint affected` plans the change
- **THEN** that test is not selected, and every unassigned E2E test is selected

#### Scenario: one spec file with a linked and an unlinked test
Added from the watch-flows verdicts, finding 10.
- **GIVEN** a spec file holding one test with an accepted link to variation V and one test with no
  accepted link, and a change that reaches no linked variation
- **WHEN** `corvint affected` plans the change
- **THEN** the linked test is not selected, and the unlinked test in the same file still runs and
  is counted `unassigned`

### Requirement: FLP-V1-008 Source tags are evidence only
Source: G11 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
A tag in test source that names a flow MUST be used only as proposal evidence and MUST NOT link,
select or remove a test.

#### Scenario: tagged but unlinked test
- **GIVEN** a Playwright test tagged with flow F and no accepted link
- **WHEN** a change reaches no part of F
- **THEN** the test runs as `unassigned`

### Requirement: FLP-V1-009 Stale links return the test to unassigned
Source: G11 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
A link MUST be stale when the test file's blob digest differs from `test_content_hash` or the
variation's routes digest differs from `routes_digest`. A stale link's test MUST run as `unassigned`
with reason `link-stale`.

#### Scenario: test edited after acceptance
- **GIVEN** an accepted link and an edit to its test file
- **WHEN** a plan is made
- **THEN** the test runs with reason `link-stale`

### Requirement: FLP-V1-010 Reachability agreement and dangling links
Source: G11 and G2 (owner answers).
Status: proposed 2026-11-09; delivery not-started.
A link MUST remove a test only when static reachability at file and symbol level agrees with it.
When they disagree, the test MUST run with reason `link-reachability-disagrees`. A link whose test ID
is not in the inventory or whose variation does not exist MUST be reported with `link-dangling` and
MUST NOT narrow.

#### Scenario: renamed test
- **GIVEN** an accepted link whose test title was renamed
- **WHEN** `corvint flows status` runs
- **THEN** the link is reported with `link-dangling` and the renamed test runs as `unassigned`

### Requirement: FLP-V1-011 Browser tests outside Playwright
Source: G13 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
Links for browser E2E tests run by pytest or RSpec MAY be proposed and accepted. In 1.0 such a test
MUST select its whole group with a printed reason, and an accepted link MUST NOT remove it.

#### Scenario: pytest-playwright test with a link
- **GIVEN** an accepted link for a pytest-playwright test
- **WHEN** a plan reaches the pytest instance
- **THEN** the group runs whole with its reason, and the link does not remove the test

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
