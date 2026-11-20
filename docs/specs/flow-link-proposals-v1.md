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
- Exists: Private proposals, terminal acceptance and the canonical-inventory plan join.
- Blocked on: Owner self-use acceptance and external outcome qualification.
- Read next: Requirements; Delivery and traceability; Compatibility and rollback.

## User and scope

A repository owner reviews proposed links between canonical test IDs and flow variations. Scores
help review; they confer no acceptance or selection authority. This spec preserves the change's
requirement text. The change was accepted under G14; the new capability spec remains proposed as
required by task 1 until its technical contract and integration evidence are complete.

Non-goals: automatic acceptance, a new flow-intent format, source tags as selection authority,
line-evidence narrowing of browser tests, a CI ratchet, and hosted or network services.
Modelling files under testDir that the browser loads by URL is outside scope (TJAA-V0-140).
The inherited cmd/corvint-docs RCF qualification issue belongs to V1-5692.
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
`accepted_at` (UTC), `pinned_commit`, `test_content_hash` and `variation_digest`, the SHA-256 of
the canonical JSON
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
selection made through an accepted link MUST carry tier `flow` (LEV-V1-009). Execution MUST
preserve every Playwright config that selected the file. If the executor cannot represent those
configs, it MUST refuse before launch with an UNKNOWN reason naming the file and configs.
The command and its working directory must bind the config. A declaration without a discovery
config retains the intent join's selected files and flow hits alongside the canonical inventory.
Every selecting variation appears in the plan (APK-V0-003). Status preserves reviewed intent
results alongside canonical rows, counting each test key and project once per variation.

#### Scenario: after acceptance
- **GIVEN** an accepted link to variation V and a change inside the project's resolved testDir
  that the test's closure excludes and that reaches no file or symbol of V
- **WHEN** `corvint affected` plans the change
- **THEN** that test is not selected, and every unassigned E2E test is selected

#### Scenario: one spec file with a linked and an unlinked test
Added from the watch-flows verdicts, finding 10.
- **GIVEN** a spec file holding one test with an accepted link to variation V and one test with no
  accepted link, and a change inside testDir that its closure excludes and that reaches no
  linked variation
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
For a Playwright exclusion, every dirty file MUST be in the test closure's Excludable set and
inside the resolved testDir of the admitting project, including an inherited config testDir.
A path outside that directory or an unresolved testDir MUST retain the test on a static miss
with reason `link-reachability-unknown`.
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
The Go-only CI profile lists that group as a required check with its reason (STR-V0-012).

#### Scenario: pytest-playwright test with a link
- **GIVEN** an accepted link for a pytest-playwright test
- **WHEN** a plan reaches the pytest instance
- **THEN** the group runs whole with its reason, and the link does not remove the test

## Delivery and traceability

The plan joins committed inventory IDs to accepted variation records and checks the candidate
test blob, variation and static graph. Unassigned IDs stay selected inside mixed Playwright files.
An exclusion requires the Playwright closure for the test's config to prove every dirty path
excludable for that file (TJAA-V0-140/141) within the admitting project's resolved testDir.
An open or widened closure retains the test.
Default proposals require every link of a test to bind before treating it as assigned.
Variation gaps count current accepted links at the report commit. Independent full-instance
obligations run the declared language command when one exists.

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| FLP-V1-001 | Unassigned, bulk and per-flow proposals | `TestFLPV1001PrivateBulkProposals`, `TestFLPV1001MixedLinksAndVariationGaps` |
| FLP-V1-002 | Private replacement, isolated from selection | `TestFLPV1001PrivateBulkProposals`, `TestFLPV1007AcceptedOnly` |
| FLP-V1-003 | Versioned weights, pinned citations and absent inputs | `TestFLPV1003ConfidenceInputs`, `TestFLPV1001PrivateBulkProposals` |
| FLP-V1-004 | Per-flow acceptance, absent and stale refusals | `TestFLPV1004AcceptanceCLI`, `TestFLPV1005NoAutomationAccepts` |
| FLP-V1-005 | Terminal check before acceptance | `TestFLPV1004AcceptanceCLI`, `TestFLPV1005NoAutomationAccepts` |
| FLP-V1-006 | Closed record and acceptance fields | `TestFLPV1006LinkRecordRoundTrip`, `TestFLPV1006MissingAcceptance`, `TestFLPV1006ClosedRecord` |
| FLP-V1-007 | Mixed-file selection and flow tier, full fallback without inventory | `TestFLPV1007AcceptedOnly`, `TestFLPV1007PendingIntegrationNeverNarrows`, `TestFLPV1007DeclaredDiscoverySurvivesOtherInventories`, `TestFLPV1007PlaywrightConfigIdentityAtExecution`, `TestFLPV1007IntentOnlySuiteSurvivesOtherInventory`, `TestFLPV1001MixedLinksAndVariationGaps`, `TestFLPV1007StatusKeepsReviewedIntentResults` |
| FLP-V1-008 | Tags affect proposal order only | `TestFLPV1007AcceptedOnly`, `TestFLPV1003ConfidenceInputs` |
| FLP-V1-009 | Test blob and canonical variation bindings | `TestFLPV1009DoubtfulLinksRun`, `TestFLPV1009CanonicalVariationDigest` |
| FLP-V1-010 | Disagreement, unknown reach, missing test or variation | `TestFLPV1009DoubtfulLinksRun`, `TestFLPV1010DanglingTest`, `TestFLPV1010UnresolvedRuntimeRetainsAcceptedTest`, `TestFLPV1010BrowserLoadedApplicationFile`, `TestFLPV1010ProjectTestDirBounds` |
| FLP-V1-011 | Browser pytest instance keeps its whole-group execution obligation | `TestFLPV1007AcceptedOnly`, `TestFLPV1011DeclaredCommandRunsForBrowserGroup`, `TestFLPV1011CIBrowserGroupCheckForGoChange` |

## Storage, trust boundary and limits

G6 resolves the flow directory from `.corvint/flows.json`. The SHA-256 binds the canonical JSON
of the complete variation object in the declared intent file at the reviewed commit. Canonical
JSON ignores member order and whitespace and preserves absent members, nulls and array order.
Test hashes are Git blob object IDs. The planner never opens the private proposal file.

Version 1 of `corvint-flows/N` adds `links`, an array with the FLP-V1-006 fields. Variation IDs
are unique throughout the intent set. The 64 KiB declaration limit, 256 citations per record
and 4096 bytes per text field apply. Unknown members are refused. Missing acceptance fields,
invalid pins or invalid timestamps cannot authorize narrowing.

Formula `flow-link-confidence/1` gives 0.35 for a matching title segment, 0.25 for a delimited
`@flow:ID` token in the test source and 0.40 for static reach to a declared flow source.
Proposal generation reads immutable Git objects. It keeps the highest scores for each eligible
test within the requested flow scope, including ties. It limits comparisons to 16,384 and output
to 5 MiB. Equal scores sort by canonical test ID and then variation ID.

The private profile is `corvint-flow-link-proposals/0`. The writer replaces the worktree's
`<git-dir>/corvint/flow-link-proposals.json` atomically. The terminal-only command rechecks the
reviewed records at HEAD, writes the selected records together and preserves other links.
A missing declaration has zero eligible flow variations. A declaration without readable inventory
cannot produce a verified candidate set.

## Compatibility and rollback

The writer emits version 1. A declared discovery suite retains its whole-file obligations
unless a current canonical Playwright instance binds its exact config. Version 0 repositories
without canonical inventory continue to use their discovery join. With canonical inventory, every
unassigned browser test remains an obligation before the first acceptance. A reader that cannot interpret the
schema widens to FULL.

Revert the implementation commits and remove the private proposal file to roll back. Retain the
FLP-V1 IDs. Task 8 remains open for a person's self-use acceptance.
