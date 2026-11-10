# Spec delta: `tasks-integration` (change `tasks-integration`)

## ADDED Requirements

### Requirement: TIN-V1-001 The Corvint RC waits for Tasks conformance

The release check MUST refuse to sign the Corvint RC unless a corvint-tasks 1.0 conformance record
exists for the paired Tasks tag and reports every check passed. (G9; PROD-TASK-010)

#### Scenario: Tasks failure

- **GIVEN** a Tasks conformance record with one failed check
- **WHEN** the RC signing check runs
- **THEN** it refuses and names the failed check

### Requirement: TIN-V1-002 Contents of the conformance record

The record MUST contain: the nine Tasks requirements PROD-TASK-001 to 009 each traced to a passing
test on Tasks `main`; the published supported-command list (TIN-V1-013) with each listed command
passing its test; at least 8 concurrent agents with no double claim; recovery after kill -9 during a
write; `receipt audit` with zero failures. The record's file format and criteria are set by change
`tasks-conformance`. (G9; owner decision R17, 2026-11-09)

Earlier wording, superseded by R17 (no Tasks MCP server in 1.0): "MCP parity with the CLI".

#### Scenario: Seven concurrent agents only

- **GIVEN** a record whose concurrency test used 7 agents
- **WHEN** the check reads it
- **THEN** it refuses with `concurrency-below-minimum`

### Requirement: TIN-V1-003 Version pair and tag order

The Corvint release manifest MUST name the compatible Tasks version (Corvint 1.0.0 with
corvint-tasks 1.0.x), and the Tasks tag MUST exist before the Corvint RC is signed. The two products
keep separate semver lines. (G9; PROD-TASK-011)

#### Scenario: Tasks not tagged

- **GIVEN** no corvint-tasks 1.0.x tag
- **WHEN** the Corvint RC is prepared
- **THEN** the manifest check refuses with `tasks-tag-missing`

### Requirement: TIN-V1-004 File contract with a shared fixture

The contract MUST be ticket schema v1 plus receipt reference v1, both defined in files, with one
shared fixture checked byte for byte in both repositories' CI. Neither product MUST import the
other's code. (G9, answer 13; PROD-TASK-011)

The proposed profile names are `taskman-ticket/1` and `taskman-receipt-ref/1`. corvint-tasks owns
and publishes both schemas and the fixture (owner decision R17, 2026-11-09). Corvint MUST keep a
vendored copy whose SHA-256 is pinned in the Corvint repository, and its CI MUST fail when the
vendored bytes do not match the pin or the pinned Tasks release.

#### Scenario: Schema change without fixture

- **GIVEN** a change to the ticket schema in corvint-tasks without a fixture update
- **WHEN** Corvint CI runs
- **THEN** the fixture check fails

### Requirement: TIN-V1-005 Receipt ID for I1 (retired)

Retired by fix release-tasks 6 and replaced by TIN-V1-012: the read-only Corvint resolver it named
is dropped. The ID is not reused. Original text kept for history:

A `corvint test` receipt MUST have an ID defined as the SHA-256 of its canonical bytes (proposed),
printed by `corvint test`, and a read-only resolver MUST return the receipt for an ID or fail with
`receipt-not-found`. (G9; PROD-TASK-012)

#### Scenario: Gate stores and audit resolves

- **GIVEN** a gate that ran `corvint affected` and `corvint test` and stored the printed receipt ID
- **WHEN** `corvint-tasks receipt audit` runs
- **THEN** the ID resolves to the same bytes

### Requirement: TIN-V1-012 Receipt ID stored and audited by Tasks

A `corvint test` receipt ID MUST be the SHA-256 of the receipt file bytes as written. `corvint test`
MUST print the receipt path. The I1 gate MUST store the file with `corvint-tasks receipt put`, which
returns the ID, and audit MUST check it with `corvint-tasks receipt audit --witness-receipts`.
Corvint MUST NOT add a receipt resolver command; Tasks MUST NOT need Corvint installed to audit a
stored receipt. (G9; PROD-TASK-012; fix release-tasks 6)

Failure mode: the stored bytes no longer hash to the ID, or the file is missing from the Tasks
store; `receipt audit --witness-receipts` reports a failure naming the ticket and the ID. A
`corvint test` run that writes no receipt file: the gate fails with a typed reason, never a pass.

Rollback: revert the gate example policy that calls `receipt put`; stored receipts stay in the
Tasks store and still audit.

#### Scenario: Gate stores and audit checks

- **GIVEN** a gate that ran `corvint affected` and `corvint test` and stored the receipt file with
  `corvint-tasks receipt put`
- **WHEN** `corvint-tasks receipt audit --witness-receipts` runs with Corvint uninstalled
- **THEN** the ID matches the SHA-256 of the stored bytes and audit reports zero failures

#### Scenario: Receipt edited after storing

- **GIVEN** a stored receipt whose file bytes were changed afterwards
- **WHEN** audit runs
- **THEN** audit fails and names the ticket and the ID

### Requirement: TIN-V1-006 Corvint stays optional for Tasks

A project MUST be able to use Tasks with no Corvint installed. When a gate declares the I1 step
and `corvint` is absent, the gate MUST fail with `corvint-not-installed`, never pass. (PROD-TASK-008,
PROD-INT-004, G9)

#### Scenario: Missing binary

- **GIVEN** a policy gate with the I1 step and no `corvint` on PATH
- **WHEN** the gate runs
- **THEN** it records a failure with `corvint-not-installed`

### Requirement: TIN-V1-007 Flow and variation citations (I2)

A ticket MAY cite flow or variation IDs. A resolver MUST check each against `.corvint/flows.json`
at the ticket's recorded commit and MUST fail closed on an unknown ID or when both flow files
exist. (G9, G6; PROD-TASK-013)

#### Scenario: Removed variation

- **GIVEN** a ticket citing a variation removed from `.corvint/flows.json`
- **WHEN** the citation check runs
- **THEN** it fails naming the ticket and the ID

### Requirement: TIN-V1-008 Review evidence in the completion gate

`corvint review` MUST emit a versioned JSON report of the G25 findings (changed lines whose selected
tests did not run, changed flows with an untested variation, unlinked E2E tests, stale docs, ticket
acceptance criteria without evidence), and the Tasks completion gate MUST be able to record it as
evidence. The report format is owned by change `review-evidence-check`. The gate MUST treat the
report as advisory unless the queue policy opts in to blocking (owner decision R4, 2026-11-09). The
gate's blocking test input MUST be `corvint affected` plus the stored `corvint test` receipt
(TIN-V1-012), never `review`. When corvint-docs is absent, the report's docs section is
NOT_OBSERVED and is not a failure. (G25; PROD-TASK-003; review docs F3)

#### Scenario: Advisory finding

- **GIVEN** a review report with one unlinked E2E test and a policy without the opt-in
- **WHEN** the completion gate runs
- **THEN** the finding is recorded and the gate result is not changed by it

### Requirement: TIN-V1-009 Takeover trial

Corvint and `beamfall/core` (owner decision R18, 2026-11-09) MUST run Tasks on the frozen RC binary
for 21 days with zero lost tickets, checked by a ticket ID and content hash diff at start and end. A store format change MUST
restart the clock. Each trial repository MUST record at least 50 ticket writes and 5 completions,
read from the store event log, or the trial does not count (R42 (panel pick) C9). (G9, answer 12;
PROD-TASK-009)

The trial starts on 2026-12-08, the day the Tasks RC is tagged, and ends on 2026-12-29; the
dates and what must land before the tag are in TCF-V1-009. Publishing the Tasks RC tag is the
owner's act. (decided 2026-11-09 under R42, C22)

#### Scenario: Format change on day 10

- **GIVEN** a trial at day 10
- **WHEN** a store format version changes
- **THEN** the trial record shows a restart and a new start date

### Requirement: TIN-V1-014 Trial store in `beamfall/core` is initialized by pull request

The Tasks store for the trial in `beamfall/core` MUST be initialized by a pull request from a
worktree off `origin/main` of that repository, on 2026-12-08, with the tagged Tasks RC binary.
The seed file (Beamfall's open roadmap items, taken over per V1-0464 and V1-4860) MUST be
prepared and reviewed by 2026-12-01, and a dry run of the initialization on a copy MUST be done
by 2026-12-04. The pull request is prepared without the owner; merging it is the owner's act,
unless the owner says the standing merge rule covers `beamfall/core`. The conformance record on
corvint-tasks main at the RC tag is produced by the same work. The trial floor of TIN-V1-009
applies. Where Beamfall's current backlog lives is NOT_VERIFIED. (decided 2026-11-09 under R42,
C23; PROD-TASK-009)

#### Scenario: Store written before the freeze

- **GIVEN** a `beamfall/core` store initialized with a build older than the tagged Tasks RC
- **WHEN** the trial start record is checked
- **THEN** the trial does not start on that store

#### Scenario: Pull request not merged

- **GIVEN** the initialization pull request open and not merged by the owner on 2026-12-08
- **WHEN** the trial start is attempted in `beamfall/core`
- **THEN** the trial clock for that repository does not start and a blocker is filed

### Requirement: TIN-V1-010 Tasks specs leave only after PR #4 merges

Tasks specs in this repository MUST stay as retired-in-place files until corvint-tasks PR #4 is
merged, and their deletion MUST cite that merge. PR #4 is due to merge by 2026-11-13. (G27;
decided 2026-11-09 under R42, C22)

#### Scenario: Early deletion

- **GIVEN** PR #4 still open
- **WHEN** a change deletes a retired Tasks spec
- **THEN** review refuses the change for citing no merge

### Requirement: TIN-V1-011 No Tasks tools in Corvint MCP

`corvint mcp` MUST list no Tasks tools. There is no Tasks MCP server in 1.0; agents reach Tasks
through the `corvint-tasks` CLI. (answer 11; PROD-TASK-007; owner decision R17, 2026-11-09)

#### Scenario: Tool list

- **GIVEN** `corvint mcp` started
- **WHEN** its tools are listed
- **THEN** no tool name or description is a Tasks operation

### Requirement: TIN-V1-013 Corvint uses only the Tasks supported commands

corvint-tasks MUST publish a supported-command list with its 1.0 release, and every Corvint-side
caller (the I1 gate example policy, the agent tasks skill of change `agent-packages`, and Corvint
documentation) MUST use only commands on that list. The list's file format is owned by
corvint-tasks and is not yet defined. (owner decision R17, 2026-11-09; release-tasks 7/8)

Failure mode: a Corvint-side caller names a command missing from the list; a Corvint CI check that
compares the callers against the vendored list fails and names the command. The list is not yet
published: the check reports NOT_RUN and TIN-V1-001 refuses the RC.

Rollback: remove the check; callers keep working, but the coupling loses its command contract.

#### Scenario: Unlisted command

- **GIVEN** the I1 gate example policy calls a `corvint-tasks` command that the published list does
  not contain
- **WHEN** Corvint CI runs
- **THEN** the check fails and names the command
