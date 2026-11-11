# Run evidence V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Authoritative inputs: change folder `docs/specs/openspec/changes/run-evidence/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/run-evidence/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-DOCS-012, PROD-TEST-018, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7, R8 and R38).

## Agent digest
- Claim: A private append-only store under the Git directory keeps each test run, written by test and ingest commands and read by default by flow reports.
- Status: accepted / partial. Change `run-evidence` auto-accepted under G14 (decision 0596); delivered under V1-5448.
- Exists: `internal/runevidence`; `corvint test` appends each receipt; `flows ingest --record`; flows status, map, gaps, impact, `test-evidence` and the MCP flows tools read the store; `--no-store`.
- Blocked on: `corvint watch` writes no receipt, so its append (RVD-V1-004) is not yet built; the Tasks digest check (RVD-V1-003) is NOT_RUN here.
- Read next: Surfaces; Traceability.

## User and boundary

A developer or agent runs `corvint test` and then asks `corvint flows status` what the flows'
tests did. Before this change each run replaced the last receipt at `<git-dir>/corvint/test-run.json`
and every report read run results only from files named with `--evidence`, so the answer needed
a hand-made file.

This spec is the canonical home of requirements RVD-V1-001 to RVD-V1-012, copied from the change's
`specs/run-evidence/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).

## Requirements

- `RVD-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Private append-only store

Corvint MUST keep run results in a private store at `<git-dir>/corvint/evidence/`. A stored record
MUST NOT be edited after it is written. The store MUST NOT be inside the worktree.

**Scenario: two runs**
- GIVEN an empty store
- WHEN `corvint test` runs twice on different trees
- THEN the store holds two records and the first record's bytes are unchanged

- `RVD-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Records named by their hash and listed in an append-only log

Each record MUST be stored at `records/<sha256>.json`, where the name is the lowercase hex SHA-256
of the record bytes, and MUST be listed by one appended line in `log.jsonl` that names the ID,
profile, tree, commit, runner instances and write time. Appends MUST hold an exclusive lock.

**Scenario: concurrent writers**
- GIVEN two `corvint test` processes finishing at the same time
- WHEN both append
- THEN `log.jsonl` has two whole lines and both records exist

- `RVD-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Record ID is the SHA-256 of the file bytes

The ID of a run record MUST be the SHA-256 of its file bytes, so the same file stored as a Tasks
witness receipt (`corvint-tasks receipt put FILE`) has the same digest.

**Scenario: attach a run to a ticket**
- GIVEN a stored record `records/ab12....json`
- WHEN it is passed to `corvint-tasks receipt put`
- THEN the digest Tasks reports equals `ab12...`

- `RVD-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) corvint test appends one record per run

`corvint test` MUST append exactly one record per run after writing its receipt. `corvint watch`
MUST append in the same way when it runs tests. The GitHub Action MUST record runs through
`corvint test`.

**Scenario: sharded run**
- GIVEN `corvint test` runs with two shards on one machine
- WHEN the run completes
- THEN one record per shard receipt is appended

- `RVD-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) flows ingest can append external results

`flows ingest --record` MUST append the `test-run-evidence/0` document it produces as one record.
Without `--record`, `flows ingest` MUST keep printing to stdout and MUST NOT write the store.

**Scenario: JUnit report from CI**
- GIVEN a JUnit XML report from another system
- WHEN `flows ingest --format junit-xml --from report.xml --record` runs with the run header flags
- THEN one record is appended and its tests appear in `corvint flows status`

- `RVD-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) One row per test per run, keyed by canonical ID and tree

A record MUST hold, for each test it ran, exactly one row with the canonical test ID, outcome, runner
instance, tree and commit, and the per-test duration when measured (PTD-V1-001). A runner that
reports only files MUST write one row per file, marked `file`.

**Scenario: test retried inside one run**
- GIVEN a Playwright test that fails and then passes on retry in one run
- WHEN the record is read
- THEN it holds one row for that test with the final outcome and the retry count

- `RVD-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Reports read the store by default

`corvint flows status`, `flows map`, `flows gaps`, `flows impact`, `corvint test-evidence` and the
matching MCP tools MUST read the store by default. `--evidence FILE` MUST add files; `--no-store`
MUST read only named files. The last result for a test MUST come from the newest record containing
it, and reports MUST show that record's tree and whether it equals HEAD.

**Scenario: PROD-TEST-020**
- GIVEN `corvint test` has just run on a fixture with flows
- WHEN `corvint flows status` runs with no `--evidence`
- THEN it shows the outcomes of that run

- `RVD-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) No listing verb

This change MUST NOT add a `corvint runs` verb or any other verb whose only job is to list stored
runs. The store is read through the commands in RVD-V1-007.

**Scenario: removed verb**
- GIVEN this change is delivered
- WHEN a user runs `corvint runs`
- THEN the command is unknown, as after change `source-removal`

- `RVD-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Bounded retention

After each append, the writer MUST keep for every test the newest 20 records containing it and MUST
remove records no test still needs. When the store would exceed its size cap, the writer MUST remove
the oldest records first. Readers MUST NOT prune.

**Scenario: 21st run**
- GIVEN 20 records each containing test T and no other test
- WHEN a 21st run of T is appended
- THEN the oldest of those records is removed

- `RVD-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Corrupt store and failed writes are named

A record whose bytes do not match its name, or that does not parse, MUST be skipped and its tests
shown as `NOT_OBSERVED` with reason `run-evidence-corrupt`. An unreadable `log.jsonl` MUST be rebuilt
by the next writer. A failed append MUST print `run-evidence-write-failed` and MUST NOT change the
exit code of the test run.

**Scenario: edited record**
- GIVEN a record file edited by hand
- WHEN `corvint flows status` runs
- THEN the record is skipped and its tests show `run-evidence-corrupt`

- `RVD-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Run results never narrow selection

No selection path (`corvint affected`, `corvint test` planning, `corvint watch`, MCP affected) MUST
use stored run results to remove a test from a plan. In 1.0 stored results MAY only change the order
of the selected tests; skipping an already-passed test is a post-1.0 proposal (R42 (panel pick) A3).

**Scenario: test passed many times**
- GIVEN a test that passed in every stored record
- WHEN a change reaches it
- THEN it is selected exactly as without the store

- `RVD-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5448) Only writing commands write the store

Only `corvint test`, `corvint watch` while running tests, and `flows ingest --record` MAY write the
store. Every reader in RVD-V1-007 MUST leave it unchanged.

**Scenario: report run**
- GIVEN a store with records
- WHEN `corvint test-evidence` runs
- THEN no file under `<git-dir>/corvint/evidence/` changes

## Surfaces

Delivered by ticket V1-5448 (build-log `docs/build-log/2026-11-11-run-evidence.md`):

- `internal/runevidence` (shared layer, standard library only) owns the store at
  `<git-dir>/corvint/evidence/` (`git rev-parse --absolute-git-dir`, so a linked worktree has its
  own): `records/<sha256>.json` written once through a temporary file and a rename, `log.jsonl`
  appended under an exclusive `flock` on `log.lock`, directories `0700` and files `0600`, and a
  linked `<git-dir>/corvint` or store directory refused before anything is created in it
  (RVD-V1-001 to RVD-V1-003). Each log line carries `id`, `profile`, `tree`, `commit`,
  `instances`, `writtenAt`, and two added members: `tests` (the record's test keys, so a reader can
  name the tests of a record that no longer hashes to its name) and `bytes` (for the cap).
  Retention keeps the newest 20 records per test, the newest record always, then the oldest go
  until the records fit 64 MiB (RVD-V1-009). The append lock needs `flock`; on a host without it
  (Windows) every append reports `run-evidence-write-failed`.
- Rows (RVD-V1-006): a `corvint-test-run/0` receipt gives one row per unit outcome, granularity
  `file` (a receipt unit is a Go package or a test file on this tree), keyed by the outcome's
  `testId` when present (CTI-V1-009) and otherwise by its unit (CTI-V1-012), with the outcome
  mapped to the test-run-evidence/0 vocabulary, the runner instance (`go`, `declared:<command>` or
  `provider:<runner>` until TIV-V1-003 instance IDs exist), tree, commit, clean flag and the
  outcome's `durationMs`, `durationSource` and `durationGranularity` (PTD-V1-008). A
  `test-run-evidence/0` document gives one row per test with its AFU-V1-013 classification and its
  retry count; a STATIC line gives none.
- Writers: `corvint test` appends each shard's receipt bytes after writing the receipt, and prints
  `corvint-test: run-evidence-write-failed CAUSE` without changing the exit status (RVD-V1-004,
  RVD-V1-010); `flows ingest --record` appends the document it prints and names the record on
  stderr, and a failed append exits 2 with `run-evidence-write-failed` (RVD-V1-005). `corvint
  watch` is not yet built: it runs `go test` itself and writes no receipt. No GitHub Action is on
  this tree; one that runs `corvint test` appends through it.
- Readers (RVD-V1-007): `flows status`, `map`, `gaps` and `impact` (and their MCP tools) read the
  store's records, oldest first, then the `--evidence` files; `--no-store` reads only the files
  (the MCP tools take no such argument). A receipt row becomes a LOCALLY_OBSERVED record that never
  carries a verified run header. `flows status` adds `source_tree` to each test, `reason`
  `run-evidence-corrupt` to a NOT_OBSERVED test that only a skipped record held, and a
  `run_evidence` member when the store gave anything; `flows impact` adds `last_results` per flow
  when the store holds a record, so a report with no stored run keeps its bytes. `corvint
  test-evidence` and `flows test-evidence` add `runs`: the newest stored result of each named test
  with its tree and `at_revision`; stored runs never change the answer's status. Up to 8192 stored
  records are read, newest first, with `truncated` set past that.
- No listing verb (RVD-V1-008). `corvint runs` still exists on this tree until change
  `source-removal` deletes it; it does not read the store.
- No selection path reads the store (RVD-V1-011); STR-V0-011 names the store as an accepted
  receipt reader for reports only.

## Traceability

| Requirement | Evidence |
|---|---|
| RVD-V1-001 | `TestRVDV1001HashNamedRecordsAndLog` (`internal/runevidence`), `TestRVDV1004CLITestAppendsOneRecordPerRun` (`cmd/corvint`) |
| RVD-V1-002 | `TestRVDV1001HashNamedRecordsAndLog`, `TestRVDV1002ConcurrentAppends` (`internal/runevidence`) |
| RVD-V1-003 | `TestRVDV1001HashNamedRecordsAndLog` (`internal/runevidence`); the `corvint-tasks receipt put` digest check is NOT_RUN (a Tasks write, refused in the delivering lane) |
| RVD-V1-004 | `TestRVDV1004CLITestAppendsOneRecordPerRun` (`cmd/corvint`); `corvint watch` not yet built |
| RVD-V1-005 | `TestRVDV1005IngestRecord` (`internal/command/flows`) |
| RVD-V1-006 | `TestRVDV1006RowsFromBothProfiles` (`internal/runevidence`) |
| RVD-V1-007 | `TestRVDV1007ProdTest020TestThenFlowsStatus` (`cmd/corvint`), `TestRVDV1007FlowsReadersUseStore`, `TestRVDV1007FlowsToolsReadStore` (`internal/command/flows`) |
| RVD-V1-008 | `TestRVDV1008NoListingVerb` (`cmd/corvint`) |
| RVD-V1-009 | `TestRVDV1009RetentionAt21Runs`, `TestRVDV1009SizeCap` (`internal/runevidence`) |
| RVD-V1-010 | `TestRVDV1010CorruptRecordSkipped`, `TestRVDV1010UnreadableLogRebuilt` (`internal/runevidence`), `TestRVDV1010StatusNamesCorruptRecord` (`internal/command/flows`), `TestRVDV1010CLIWriteFailureKeepsExitCode` (`cmd/corvint`) |
| RVD-V1-011 | `TestRVDV1011StoredResultsDoNotChangePlan` (`cmd/corvint`) |
| RVD-V1-012 | `TestRVDV1012ReadMissingStoreWritesNothing`, `TestRVDV1010CorruptRecordSkipped` (`internal/runevidence`), `TestRVDV1007FlowsReadersUseStore`, `TestRVDV1007FlowsToolsReadStore` (`internal/command/flows`), `TestRVDV1007ProdTest020TestThenFlowsStatus` (`cmd/corvint`) |

## Non-goals

No `corvint runs` verb and no other listing verb (G24); no shared or remote store; no change to
`corvint-test-run/0` beyond add-only fields (settled D25); no use of run results to remove a test
from a plan; no change to the Tasks witness-receipt format.

## Rollback

Revert the change's commits. `corvint test` then writes only its receipt, readers use `--evidence`
again, and `<git-dir>/corvint/evidence/` can be deleted by hand with no effect on any committed
state. RVD-V1-001 to RVD-V1-012 stay listed as retired if the change is withdrawn and are never
reused.
