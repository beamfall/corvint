# Run evidence V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/run-evidence/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/run-evidence/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-DOCS-012, PROD-TEST-018, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7, R8 and R38).

## Agent digest
- Claim: A private append-only store under the Git directory keeps each test run, written by test and ingest commands and read by default by flow reports.
- Status: accepted / not-started. Change `run-evidence` auto-accepted under G14 (decision 0596); delivered under V1-5448.
- Exists: nothing yet.
- Blocked on: nothing.
- Read next: Requirements.

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

Not yet built.

## Traceability

Not yet recorded.

## Non-goals

No `corvint runs` verb and no other listing verb (G24); no shared or remote store; no change to
`corvint-test-run/0` beyond add-only fields (settled D25); no use of run results to remove a test
from a plan; no change to the Tasks witness-receipt format.

## Rollback

Revert the change's commits. `corvint test` then writes only its receipt, readers use `--evidence`
again, and `<git-dir>/corvint/evidence/` can be deleted by hand with no effect on any committed
state. RVD-V1-001 to RVD-V1-012 stay listed as retired if the change is withdrawn and are never
reused.
