# Flow Link Proposals V1

Owner: Russell Lewis
Date: 2026-11-18
Requirement prefix: `FLP-V1`
Intent status: proposed
Delivery status: experimental
Health: KEEP
Authoritative inputs: `openspec/changes/flow-link-proposals/` proposal, design, tasks and spec
delta;
owner G14 acceptance of the change on 2026-11-18, recorded in PR #235.

## Agent digest
- Claim: Propose flow links for human acceptance; only accepted, fresh links may narrow E2E tests.
- Status: proposed / experimental
- Exists: The canonical requirements and the version 1 link-record codec.
- Blocked on: Variation-route contracts, observed-route evidence and the LEV-V1 selection tier.
- Read next: Requirements; Delivery and traceability; Compatibility and rollback.

## User and scope

A repository owner reviews proposed links between canonical test IDs and flow variations. Scores
help review; they confer no acceptance or selection authority. This spec preserves the change's
requirement text. The change was accepted under G14; the new capability spec remains proposed as
required by task 1 until its technical contract and integration evidence are complete.

Non-goals: automatic acceptance, a new flow-intent format, source tags as selection authority,
line-evidence narrowing of browser tests, a CI ratchet, and hosted or network services.
The baseline is running every unassigned E2E test. Corvint cannot prove a person ran acceptance;
review of the commit changing `.corvint/flows.json` is the check.

## Requirements

- `FLP-V1-001`: Propose links, with a bulk bootstrap

`corvint-docs flows propose --links` MUST propose, for each unassigned E2E test in the test
inventory,
the flow variations it most likely exercises, each with cited evidence. `--all` MUST propose for
every
E2E test in the inventory. Proposing MUST NOT run tests and MUST NOT change any file under
`.corvint/`.

#### Scenario: bootstrap on a repository without links
- **GIVEN** a repository with flows and twenty Playwright tests and no accepted links
- **WHEN** `corvint-docs flows propose --links --all` runs
- **THEN** proposals with evidence appear for the tests, and no file under `.corvint/` changes

- `FLP-V1-002`: Proposals are private

Proposals MUST be written to `<git-dir>/corvint/flow-link-proposals.json`, MUST NOT be committed and
MUST NOT be stored under `.corvint/`. No selection path MAY read the proposals file.

#### Scenario: proposals change
- **GIVEN** a new proposal run replaces the proposals file
- **WHEN** `corvint affected` plans the next change
- **THEN** the plan is the same as before the proposal run and no full run is forced

- `FLP-V1-003`: Confidence score

Each proposal MUST carry `confidence`, a number from 0.00 to 1.00 with two decimals, and
`confidenceFormula`, the version of the formula that produced it. The formula MUST be deterministic
over its cited evidence and MUST list any input it lacked. The score MUST NOT accept a link and MUST
NOT be a selection input. Version 1 uses the test title path matching the flow or variation name,
a source tag naming the flow, and static reachability from the test file to declared flow files.
Missing inputs contribute zero and MUST be cited as `absent`. Route visits and entry-symbol
reachability require a later formula version.

#### Scenario: same inputs, same score
- **GIVEN** two proposal runs on the same commit, inventory and static evidence
- **WHEN** their outputs are compared
- **THEN** every proposal has the same score and formula version

- `FLP-V1-004`: Only a person accepts, per flow

`corvint-docs flows accept --flow ID --by NAME` MUST copy the chosen proposals of one flow into
`.corvint/flows.json` as link records. It MUST refuse with `flow-proposals-absent` when no proposals
file exists and with `flow-proposals-stale` when the proposals were made at another commit. No MCP
tool, hook, Tasks gate or GitHub Action step MAY accept a link, whatever its score.

#### Scenario: high score, not accepted
- **GIVEN** a proposal with confidence 0.99 and no accept command run
- **WHEN** `corvint affected` plans a change that reaches the proposed flow
- **THEN** the test runs as `unassigned`

- `FLP-V1-005`: Accept refuses without a terminal

`flows accept` MUST refuse with `flow-accept-needs-person` when standard input is not a terminal.
The
specification MUST state that Corvint cannot prove a person ran the command and that review of the
commit changing `.corvint/flows.json` is the check.

#### Scenario: accept from a script
- **GIVEN** `flows accept` run with standard input redirected from a file
- **WHEN** it starts
- **THEN** it exits non-zero with `flow-accept-needs-person` and writes nothing

- `FLP-V1-006`: Link record fields

A link record MUST hold `test_id` (the canonical test ID, CTI-V1-001, carrying the file path and
title path), `variation_id`, `evidence`, `confidence`, `confidenceFormula`, `accepted_by`,
`accepted_at` (UTC), `pinned_commit`, `test_content_hash` and `variation_digest`, the SHA-256 of the canonical JSON
variation record in `.corvint/flows.json` at `pinned_commit`. A record missing
`accepted_by`, `accepted_at` or `pinned_commit` MUST be treated as unaccepted. A link record MUST
NOT hold a test `kind`; the kind is read from the test inventory by `test_id` (decided 2026-11-09
under R42, N2).

#### Scenario: hand-written record without acceptance
- **GIVEN** a link record in `.corvint/flows.json` with no `accepted_by`
- **WHEN** a plan reads it
- **THEN** the test runs as `unassigned` and the record is reported as unaccepted

- `FLP-V1-007`: Only accepted links narrow

E2E selection MUST read link records only from `.corvint/flows.json` and MUST use only accepted
links. A browser E2E test with no accepted link MUST run and MUST be counted `unassigned`. A
selection made through an accepted link MUST carry tier `flow` (LEV-V1-009).

#### Scenario: after acceptance
- **GIVEN** an accepted link from a Playwright test to variation V, and a change that reaches no
file
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

- `FLP-V1-008`: Source tags are evidence only

A tag in test source that names a flow MUST be used only as proposal evidence and MUST NOT link,
select or remove a test.

#### Scenario: tagged but unlinked test
- **GIVEN** a Playwright test tagged with flow F and no accepted link
- **WHEN** a change reaches no part of F
- **THEN** the test runs as `unassigned`

- `FLP-V1-009`: Stale links return the test to unassigned

A link MUST be stale when the test file's blob digest differs from `test_content_hash` or the
variation's canonical record digest differs from `variation_digest`. Its test MUST run as
`unassigned`
with reason `link-stale`.

#### Scenario: test edited after acceptance
- **GIVEN** an accepted link and an edit to its test file
- **WHEN** a plan is made
- **THEN** the test runs with reason `link-stale`

- `FLP-V1-010`: Reachability agreement and dangling links

A link MUST remove a test only when static reachability at file and symbol level agrees with it.
When flow reachability cannot be computed at the candidate, its test MUST run as `unassigned`
with reason `link-reachability-unknown`. When they disagree, the test MUST run with reason
`link-reachability-disagrees`. A link whose test
ID
is not in the inventory or whose variation does not exist MUST be reported with `link-dangling` and
MUST NOT narrow.

#### Scenario: renamed test
- **GIVEN** an accepted link whose test title was renamed
- **WHEN** `corvint flows status` runs
- **THEN** the link is reported with `link-dangling` and the renamed test runs as `unassigned`

- `FLP-V1-011`: Browser tests outside Playwright

Links for browser E2E tests run by pytest or RSpec MAY be proposed and accepted. In 1.0 such a test
MUST select its whole group with a printed reason, and an accepted link MUST NOT remove it.

#### Scenario: pytest-playwright test with a link
- **GIVEN** an accepted link for a pytest-playwright test
- **WHEN** a plan reaches the pytest instance
- **THEN** the group runs whole with its reason, and the link does not remove the test

## Delivery and traceability

Task 2 stores records only. No propose or accept command is delivered. Version 1 planning remains
FULL while the route, inventory join and tier contracts are unavailable; record presence or a score
cannot enable narrowing. The existing runner marks this unenumerated E2E obligation incomplete;
it does not claim an empty successful run. Version 0 retains its existing behavior.

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| FLP-V1-001 | Blocked: route inputs absent | (untested: proposal integration blocked) |
| FLP-V1-002 | Blocked with proposal command | (untested: no private writer delivered) |
| FLP-V1-003 | Blocked with proposal inputs | (untested: no formula delivered) |
| FLP-V1-004 | Blocked with proposal command | (untested: acceptance not delivered) |
| FLP-V1-005 | Blocked with acceptance command | (untested: terminal refusal not delivered) |
| FLP-V1-006 | `internal/appflows/flowlinks.go` | `TestFLPV1006LinkRecordRoundTrip`, `TestFLPV1006MissingAcceptance`, `TestFLPV1006ClosedRecord` |
| FLP-V1-007 | Conservative version 1 guard only | `TestFLPV1007PendingIntegrationNeverNarrows` (partial; accepted-link narrowing untested) |
| FLP-V1-008 | No proposal or tag selection path added | (untested: tag fixture awaits integration) |
| FLP-V1-009 | Blocked: no variation route contract | (untested: stale fixtures await integration) |
| FLP-V1-010 | Blocked: canonical inventory join | (untested: disagreeing and dangling fixtures await integration) |
| FLP-V1-011 | Blocked: canonical inventory join | (untested: pytest group fixture awaits integration) |

The eventual acceptance matrix is the scenarios above, including the mixed linked/unlinked file,
one fixture per stale/disagreeing/dangling case, formula determinism and each evidence input, and
CLI refusal and automation-reachability tests. Task 8 stays unticked: owner acceptance is required.
The lane must never run acceptance on Corvint itself.

## Storage, trust boundary and limits

Version 1 of `corvint-flows/N` adds `links`, an array of records with exactly the FLP-V1-006 fields.
Variation IDs are already unique across the intent set; they identify the owning flow without a
second flow ID. `evidence` is an array of opaque citation strings retained verbatim. This codec
does not interpret a citation as proof. The variation digest binds the entire canonical variation record. Missing acceptance fields decode as unaccepted. A malformed timestamp, non-UTC offset,
invalid commit ID or blank acceptor also grants no acceptance.

Confidence is a finite number in [0, 1]; the later proposal formula owns scoring and two-decimal
rendering. It never participates in the acceptance predicate. The existing 64 KiB declaration
limit remains; each record has at most 256 evidence items, each at most 4096 bytes, and text fields
have the same 4096-byte bound. Unknown fields, including a test `kind`, are refused.

Future proposals use `<git-dir>/corvint/flow-link-proposals.json` and profile
`corvint-flow-link-proposals/0`. Absence and a different pinned commit refuse acceptance with
`flow-proposals-absent` and `flow-proposals-stale`; nonterminal input refuses with
`flow-accept-needs-person`. Selection never reads that private file. Missing or doubtful evidence
widens selection. The formula weights remain unresolved because proposal inputs are blocked.

## Compatibility and rollback

The G6 version table gains version 1; version 0 and its released member list and golden remain
unchanged. The writer emits version 1, the highest supported version. Unsupported-version details
list both supported versions, as FSV-V1-004 requires. Existing recordless version 0 plans retain
their behavior. Version 1 plans cannot narrow until full integration is available.

Revert this change to withdraw the codec. An older reader refuses version 1 and selects FULL;
it does not silently read it as version 0. Keep the FLP-V1 IDs reserved. Promotion requires the
entire acceptance matrix and a person's self-use acceptance, not just a schema round trip.

## Recorded blockers

At base `a359919975da48761fd263869cc2f000dae1fb79`, `FlowVariation` has no routes member;
`TestRunEvidence`, `RunAttempt` and `runevidence.Row` have no observed-route member. Their closed
contracts cannot be expanded by this lane. No LEV-V1 requirement is registered, and `E2ESelection`
has no tier. The charter requires stopping a step whose design premise is absent. The evidence
and restart conditions are in `../build-log/2026-11-18-flow-link-proposals.md`.
