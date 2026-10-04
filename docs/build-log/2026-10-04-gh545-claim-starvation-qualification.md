# Issue 545: claim starvation qualification

Issue 545 reported `corvint-tasks claim` running 13 minutes and exiting SNAPSHOT_MOVED ("store changed during all preparation attempts") at about 7,140 receipts with six to ten concurrent lease writers. The owner identified the reported build as `32aa5a3f2b7f3fe658d208049fb009e2c9e24c13`. Issue 494 admission (merge `4620e3436ceddc7e9b1ccb392a93f157155a7154`) is not an ancestor of it, so the report describes source without the preparation gate: every writer ran a full inventory and audit outside the writer lock, and each commit invalidated every other writer's round until sixteen rounds were spent (`internal/tasks/store/lease_write.go`, `leaseWrite`).

Decision: no product change. The admission gate already on main removes the starvation, and a rewrite of the preparation path was not justified by evidence. This change adds only the opt-in `TestGH545ClaimStarvation` and the canonical specification note, so the comparison is repeatable.

The harness builds a disposable fixture store (never a real queue), appends synthetic receipts to the requested count, audits it, then runs fresh CLI processes: each writer performs claim, heartbeat, renew, heartbeat and release. It records whole-operation elapsed time and outcome per process and fails if any operation is refused or exceeds the bound. Inputs: `GH545_CLI`, `GH545_EVIDENCE_DIR`, optional `GH545_RECEIPTS` (7140), `GH545_WORKERS` (10), `GH545_MAX_OPERATION_SECONDS` (60). It skips without `GH545_CLI`, so the unit suite and CI do not run it.

One wave per row, local macOS 26.6.2 on twelve CPUs, Go 1.27.1, binaries built with `-trimpath` from the named commits, 7,140 receipts:

| Source | Writers | Completed | Refusals | Worst operation | Median operation | Wave |
|---|---|---|---|---|---|---|
| `32aa5a3f` (reported) | 10 | 25/50 | 5 SNAPSHOT_MOVED | 360,063 ms | 70,172 ms | 437,323 ms |
| `a5bb0d8f` (main) | 10 | 50/50 | 0 | 32,959 ms | 28,165 ms | 145,369 ms |
| `a5bb0d8f` (main) | 14 | 55/70 | 3 LOCK_TIMEOUT | 30,959 ms | not computed | 148,428 ms |

The reported-source wave used 667% CPU for 445 seconds; the main ten-writer wave used 184% for 153 seconds. Full audits before and after the main ten-writer wave were CONSISTENT/AGREES. The unchanged issue 494 wave (5,000 receipts, ten writers, thirty operations) was also rerun once on each side for orientation: main 30/30 in 56,506 ms; the installed `build.202` binary, which also lacks the gate, 6/30.

Residual, demonstrated and not repaired here: admission serializes preparations, and each still performs a complete-history inventory and audit, about 2.9 seconds per operation at 7,140 synthetic receipts. Ten queued writers therefore wait about 29 seconds against the 30-second admission budget, and the 14-writer wave refused three writers LOCK_TIMEOUT after about 30 seconds. That refusal is bounded, retryable and spends no further CPU, as issue 494 specified, but the sustainable writer count falls as history grows. Making mutation cost independent of receipt history is the open native ticket V1-0645 and needs its own contract; it is not claimed here.

Limits: synthetic small receipts on one host; one wave per row; no p95, fairness or latency-budget claim; Linux NOT_RUN; mixed old/new binaries NOT_RUN (older binaries bypass admission); non-lease writers, which do not use the preparation gate, were not in the workload; the 32aa5a3f failures in the 5,000-receipt wave surfaced as a recordedAt-before-head refusal rather than SNAPSHOT_MOVED. Repository-wide `make gate` NOT_RUN by standing owner preference. Evidence directories were scratch and are not retained in the repository; the numbers above are the retained record. Rollback removes the test file and the two documentation paragraphs; no store, wire or behaviour change exists to revert.
