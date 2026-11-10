# Spec delta: run-evidence

Capability spec: `docs/specs/run-evidence-v1.md` (new). This delta is a proposal only; the canonical
spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: RVD-V1-001 Private append-only store
Source: owner decision R7 (2026-11-09); review T-B2 and NEW-testing-6.
Status: proposed 2026-11-09; delivery not-started.
Corvint MUST keep run results in a private store at `<git-dir>/corvint/evidence/`. A stored record
MUST NOT be edited after it is written. The store MUST NOT be inside the worktree.

#### Scenario: two runs
- **GIVEN** an empty store
- **WHEN** `corvint test` runs twice on different trees
- **THEN** the store holds two records and the first record's bytes are unchanged

### Requirement: RVD-V1-002 Records named by their hash and listed in an append-only log
Source: owner decision R7 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Each record MUST be stored at `records/<sha256>.json`, where the name is the lowercase hex SHA-256
of the record bytes, and MUST be listed by one appended line in `log.jsonl` that names the ID,
profile, tree, commit, runner instances and write time. Appends MUST hold an exclusive lock.

#### Scenario: concurrent writers
- **GIVEN** two `corvint test` processes finishing at the same time
- **WHEN** both append
- **THEN** `log.jsonl` has two whole lines and both records exist

### Requirement: RVD-V1-003 Record ID is the SHA-256 of the file bytes
Source: owner decision R7 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
The ID of a run record MUST be the SHA-256 of its file bytes, so the same file stored as a Tasks
witness receipt (`corvint-tasks receipt put FILE`) has the same digest.

#### Scenario: attach a run to a ticket
- **GIVEN** a stored record `records/ab12....json`
- **WHEN** it is passed to `corvint-tasks receipt put`
- **THEN** the digest Tasks reports equals `ab12...`

### Requirement: RVD-V1-004 corvint test appends one record per run
Source: owner decision R7 (2026-11-09); PROD-TEST-020.
Status: proposed 2026-11-09; delivery not-started.
`corvint test` MUST append exactly one record per run after writing its receipt. `corvint watch`
MUST append in the same way when it runs tests. The GitHub Action MUST record runs through
`corvint test`.

#### Scenario: sharded run
- **GIVEN** `corvint test` runs with two shards on one machine
- **WHEN** the run completes
- **THEN** one record per shard receipt is appended

### Requirement: RVD-V1-005 flows ingest can append external results
Source: review decisions list ("read by `flows ingest`"); review T-B2.
Status: proposed 2026-11-09; delivery not-started.
`flows ingest --record` MUST append the `test-run-evidence/0` document it produces as one record.
Without `--record`, `flows ingest` MUST keep printing to stdout and MUST NOT write the store.

#### Scenario: JUnit report from CI
- **GIVEN** a JUnit XML report from another system
- **WHEN** `flows ingest --format junit-xml --from report.xml --record` runs with the run header flags
- **THEN** one record is appended and its tests appear in `corvint flows status`

### Requirement: RVD-V1-006 One row per test per run, keyed by canonical ID and tree
Source: review T-B2; owner decision R11 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
A record MUST hold, for each test it ran, exactly one row with the canonical test ID, outcome, runner
instance, tree and commit, and the per-test duration when measured (PTD-V1-001). A runner that
reports only files MUST write one row per file, marked `file`.

#### Scenario: test retried inside one run
- **GIVEN** a Playwright test that fails and then passes on retry in one run
- **WHEN** the record is read
- **THEN** it holds one row for that test with the final outcome and the retry count

### Requirement: RVD-V1-007 Reports read the store by default
Source: owner decision R7 (2026-11-09); PROD-TEST-020; G24.
Status: proposed 2026-11-09; delivery not-started.
`corvint flows status`, `flows map`, `flows gaps`, `flows impact`, `corvint test-evidence` and the
matching MCP tools MUST read the store by default. `--evidence FILE` MUST add files; `--no-store`
MUST read only named files. The last result for a test MUST come from the newest record containing
it, and reports MUST show that record's tree and whether it equals HEAD.

#### Scenario: PROD-TEST-020
- **GIVEN** `corvint test` has just run on a fixture with flows
- **WHEN** `corvint flows status` runs with no `--evidence`
- **THEN** it shows the outcomes of that run

### Requirement: RVD-V1-008 No listing verb
Source: G24 (owner answer); owner decision R7 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
This change MUST NOT add a `corvint runs` verb or any other verb whose only job is to list stored
runs. The store is read through the commands in RVD-V1-007.

#### Scenario: removed verb
- **GIVEN** this change is delivered
- **WHEN** a user runs `corvint runs`
- **THEN** the command is unknown, as after change `source-removal`

### Requirement: RVD-V1-009 Bounded retention
Source: review T-B2 and NEW-testing-6.
Status: proposed 2026-11-09; delivery not-started.
After each append, the writer MUST keep for every test the newest 20 records containing it and MUST
remove records no test still needs. When the store would exceed its size cap, the writer MUST remove
the oldest records first. Readers MUST NOT prune.

#### Scenario: 21st run
- **GIVEN** 20 records each containing test T and no other test
- **WHEN** a 21st run of T is appended
- **THEN** the oldest of those records is removed

### Requirement: RVD-V1-010 Corrupt store and failed writes are named
Source: review T-B2.
Status: proposed 2026-11-09; delivery not-started.
A record whose bytes do not match its name, or that does not parse, MUST be skipped and its tests
shown as `NOT_OBSERVED` with reason `run-evidence-corrupt`. An unreadable `log.jsonl` MUST be rebuilt
by the next writer. A failed append MUST print `run-evidence-write-failed` and MUST NOT change the
exit code of the test run.

#### Scenario: edited record
- **GIVEN** a record file edited by hand
- **WHEN** `corvint flows status` runs
- **THEN** the record is skipped and its tests show `run-evidence-corrupt`

### Requirement: RVD-V1-011 Run results never narrow selection
Source: every unknown runs with a named reason; owner decision R8 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
No selection path (`corvint affected`, `corvint test` planning, `corvint watch`, MCP affected) MUST
use stored run results to remove a test from a plan.

#### Scenario: test passed many times
- **GIVEN** a test that passed in every stored record
- **WHEN** a change reaches it
- **THEN** it is selected exactly as without the store

### Requirement: RVD-V1-012 Only writing commands write the store
Source: product invariant 4.
Status: proposed 2026-11-09; delivery not-started.
Only `corvint test`, `corvint watch` while running tests, and `flows ingest --record` MAY write the
store. Every reader in RVD-V1-007 MUST leave it unchanged.

#### Scenario: report run
- **GIVEN** a store with records
- **WHEN** `corvint test-evidence` runs
- **THEN** no file under `<git-dir>/corvint/evidence/` changes

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
