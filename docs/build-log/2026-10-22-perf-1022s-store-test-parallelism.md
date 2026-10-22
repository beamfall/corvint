# Perf lane 1022s: store test parallelism, kernel pool identity, heavy.sh Go pin, wrapped requirement IDs

Lane `perf-1022s`, branch `claude/perf-1022s`, base `6e90476ec`. Toolchain: go1.27.2 from the module cache,
first on `PATH`, with `GOTOOLCHAIN=local`. Requirements RDG-V0-023, PSR-V0-201, CTS-V0-018 and the ASD-V0-014
amendment are accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- **V1-0961 / V1-2402.** `internal/tasks/store` cannot finish under the 10-minute goal. Go runs serial tests
  before top-level parallel ones, so the serial sum is a floor on wall time. CI run 39774798016 supplied
  the per-test durations. Most serial tests were serial only because they shared package-global seams.
- **V1-1008.** Pool runner identity forked `ps -p PID -o lstart=` on every allocation and recovery check.
- **V1-2384.** `heavy.sh` used whichever `go` came first on `PATH` (go1.27.3), not the repository pin.
- **V1-2403.** `requirement-definitions-check` treated a requirement ID that starts a wrapped continuation
  line as a definition.

## Decisions

- **Per-store attempt entropy.** `mintAttemptID` reads a seam from `attemptEntropies sync.Map`, keyed by the
  store state dir, in place of one global. The 19 resume tests, 14 checkpoint tests and TestCALV0232 then run
  in parallel. `TestV10961_AttemptEntropyIsKeyedByStore` proves that a pinned store does not leak its entropy
  into another store.
- **TestJSGV0012 (V1-2402).** It is parallel and builds its 400-event fixture by padding history appends, not
  by sealing per event. The assertions are unchanged.
- **17 seam-free tests are parallel** (batch 4). TestCALV0018_ImportPublicationFailureRestarts and
  TestCALV0027_ActualCompletedStages stay serial. They install `store.SetPublishFaultForTest` from
  `export_test.go`, and the first parallel run showed injected publication failures leaking into peers.
- **Overlap with beamfall/corvint#1543.** That PR already keys `segmentSealStage` and parallelises the JSG
  segment and seal-stop tests. This lane reverted its own copy of that work (62e511276), and `git merge-tree`
  against the PR head reports no conflict.
- **PSR-V0-201 (V1-1008).** On Darwin, runner identity comes from `supervisor.ProcessIdentity`
  (`kern.proc.pid`, `darwin:<sec>:<usec>`). A recorded `darwin:` identity is re-read natively. Legacy `lstart`
  records and kernel failures fall back to the old `ps` listing. Linux keeps the listing.
- **ASD-V0-014 (V1-2384).** `script/heavy.sh` pins the nearest `go.mod` toolchain when the module cache holds
  it. The host `/private/tmp/cem10-build/heavy.sh` got the same minimal edit (backup:
  `/private/tmp/claude-501/heavy.sh.before-v1-2384`).
- **RDG-V0-023 (V1-2403).** The check tracks wrapped list items, so an ID at the start of a continuation line
  is a citation.

## Evidence

All timings are wall time with heavy.sh, `-p 4 -count=1`, on a loaded host (load 9-22):

| Change | Before | After |
|---|---|---|
| Resume set (19) | 255.46 s | 123.98 s |
| Checkpoint set (14) | 98.59 s | 50.17 s |
| TestJSGV0012 alone | 56.20 s | about 23.7 s |
| Kept set (36 tests) | about 430 s CI serial sum | 174.6 s |
| Batch 4 (17) | 68.4 s (61.8 s serial sum) | 30.1 s |
| Pool identity per call | 3.53 ms (`ps`) | 10.9 µs (kernel) |
| heavy.sh `go version` | go1.27.3 | go1.27.2 |
| requirement definition lines | 5,345 | 5,343 |

- The largest CI serial contributors were JSGV0012 (103.3 s), JSGV0001 (86.4 s), JSGV0013 (75.3 s),
  JSGV0008 (75.1 s) and JSGV0011 (63.5 s). JSGV0012 is now off the serial path, and #1543 removes the others.
- The CI serial sum still left after this lane and #1543 is about 508 s. It comes from:
  - program-continuation tests with 4 s stage walls (about 111 s);
  - the CALV0221 process-wide descriptor bound;
  - guarded inventory counters;
  - pool sweep deadlines;
  - users of `SetPublishFaultForTest` and `t.Setenv`.
- Tests added: TestV10961_AttemptEntropyIsKeyedByStore, TestPSRV0201_RunnerIdentityNeedsNoListing (Darwin),
  a `lane-scripts_test.sh` pin case and a wrapped-line case in the requirement-definitions test.
- **V1-1108.** Acceptance 1 is already delivered: commits bcb97983f, 36a791b31 and fc18077a6, plus
  `store_root_exclusion_test.go`.
- **V1-0907.** Partly already delivered: 6a954979a folds once per call in `completionOffers`, and 4c2cbcf02
  makes the receipt audit reuse the audit fold. A cross-call incremental fold would need persisted derived
  state, which is a design decision.
- **V1-0916.** Skipped; it needs a design decision. Segmented stores are already bounded because the seal is
  the refresh (JSG-V0-010). Unsegmented stores still refresh inside the caller after the lock
  (`mutate.go`, `mutate_batch.go`, `lease_write.go`).
- **V1-2401 (CTS-V0-018).** #1543 merged before this lane's single main merge, which unblocked the ticket.
  Import now checks the whole plan against the journaled map before the first batch, through
  `transaction.ImportMapConflicts`. It refuses `DUPLICATE_ID` and names every conflicting source item.
  Before this, a later batch's conflict left the earlier batches committed. TestCTSV0018 fails on the tree
  without the pre-check: the import returns no error and one refused batch.
- **V1-2400.** Skipped. Amending the map after genesis needs a new journaled, reviewed mutation, and its
  review and identity semantics are a design decision.

## NOT_RUN

- A whole-package `go test ./internal/tasks/store` run. The lane cap is 600 s per command, and the CI serial
  floor is still above that.
- A `-race` run for V1-1108.
- Linux `/proc` runner identity, which is out of scope.

## Rollback

- CTS-V0-018: remove `ImportMapConflicts` and its call in `importWith`. No stored state changes.
- Revert the lane commits. Reverting PSR-V0-201 is safe for `lstart` records. Drain pool allocations whose
  recorded identity starts with `darwin:` before reverting, because the old code compares them against `ps`
  output and would mark those runners gone.
- The host heavy.sh restores from the backup above.
