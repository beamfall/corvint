# internal/tasks/store test wall time

Date: 2026-10-10

This is a test-only change: the only edits outside `_test.go` files are two test seams whose
per-store maps are empty in production, so production behaviour is unchanged.

## Problem

`go test ./internal/tasks/store` took 1,872s in CI shard 0 of run 38055182050, against 871s on
2026-10-04. The CI command is `go test -exec test-confine -json -p 1 -count=1 -race -timeout 50m`
on a 4-vCPU ubuntu-24.04 runner. The target was under 700s without removing, skipping or weakening
any assertion and with production changes limited to test seams.

## Cause of the doubling

The growth is new test volume, not a regression in older tests. The package went from 204 to 362
`func Test` declarations since `a62e1917`. Joined against the per-test times of run 38055182050,
the tests that existed then take about 389s in CI and the 152 added since take about 1,482s.
Most of the added time is supervised-workflow tests (CAL-V0-078/086/087/089/197: real lane-leader
subprocesses, fake agent host scripts and git worktrees) and writer-checkpoint tests
(CAL-V0-115..117, 190: stores of 70 to `WriterFullBound+8` receipts).

The cost is system time, not sleeps. A local baseline used 319s user and 560s system CPU over
1,304s wall. A CPU profile put 52% of samples under `readLeaseProof`, dominated by per-component
`openat` from safe-open, inventory and audit on every supervisor step. Fixture construction is
cheap (about 0.16s per store), so shared read-only fixtures would not help.

## Change

Every test ran sequentially. Because the cost is per-test syscalls and subprocess waits on a
mostly idle CPU, the change runs independent tests concurrently.

- 248 top-level tests and the subtests of four tests call `t.Parallel()`. Each was chosen by a
  scratch AST audit of its transitive test-file call graph, including functions passed as
  values, for package-level assignment, `Set*ForTest` hooks, `Setenv`/`Chdir`, rlimits, signals,
  `/proc` reads and process-global counters. Every test that touches one stays sequential.
- Two test seams became keyed by the store's state directory so the tests that use them can run
  in parallel. Both maps are empty in production, so production behaviour is unchanged:
  - `runFault` (`internal/tasks/store/workflow.go`) became `runFaults sync.Map`. `fault` and
    `landRepositories` now take the repository; `SetRunFaultForTest` takes the state directory.
  - `minWriterCheckpointSeq` (`internal/tasks/store/writer_checkpoint.go`) became a constant;
    tests lower the threshold per store through `writerCheckpointFloors`, read by
    `writerCheckpointMin`. In `TestCALV0115_WriterCheckpointFallsBackToCompleteAudit` the
    second store's threshold is now lowered after `historyStore` builds it, where the old
    global was already lowered during that build. No assertion changed.

Kept sequential, with the reason:

- `publishFault`, `gateRecordFault`, pool hooks and other process-global `Set*ForTest` users,
  including the two `TestCALV0027_*` tests that reach `SetPublishFaultForTest` through functions
  passed to `t.Run`. Making those parallel leaked an injected publish fault into CAL-V0-045.
- `Setenv` tests (for example CAL-V0-197), the lease-route writer tests, inventory, lease cache,
  preparation lock, lock-measurement, contention and pool-sweep lifecycle tests.
- Continuation tests that rely on the fixture's default 4s stage wall (CAL-V0-089 Codex,
  OpenCode, bound-then-restart, turn caps, integrate-checkpoint restart, drain) and
  `TestCALV0086_UnprovedStopIsNotFinished`, whose escape process lives only 5s. Under parallel
  load they missed those walls and failed. Widening the walls would change what they prove.

Inherent costs, left as they are:

- `TestCALV0089_ProgramWallExpiryEndsContinuation` waits for a one-minute program wall. One is
  the policy minimum and the unit is `time.Minute` in `internal/tasks/transaction/program.go`,
  with no existing seam. It now overlaps other tests.
- `TestCALV0116_WriterFullBoundDeclines` must build `WriterFullBound+8` receipts.
- `TestCALV0070_MutateRetriesAuditWithoutWatch` runs a child process descriptor loop.

## Measurements

Local, `GOTOOLCHAIN=local GOMAXPROCS=3 go test -count=1 -json ./internal/tasks/store`, darwin
host shared with other agents. Host load varied widely between runs, so the local walls are
noisy and are not a CI prediction by themselves.

| Run | Wall | Sequential phase | Parallel phase | Failures |
|---|---|---|---|---|
| Baseline, no race | 1,303.9s | 1,303.9s | none | 0 |
| First parallel set, no race | 868.6s | 285.5s | 583.1s | 5 (set later corrected) |
| Final set, no race, load 39 to 66 | 1,448.1s | 442.0s | 1,005.7s | 0 |
| Final set, `-race`, load 11 to 28 | 1,620.6s | 927.5s | 693.1s | 0, no data race |

Baseline top 15 (seconds): CAL-V0-078 IntegratorFailureBeforeLeavingSelectionIsRetryable 73.7,
CAL-V0-089 ProgramWallExpiryEndsContinuation 62.1, CAL-V0-078
GateRunContentionAfterExecutionIsReported 30.7, CAL-V0-019 FaultAtEveryArtifactIsAllOrNothing
30.5, CAL-V0-087 InterruptedIntegrationLandsOnce 29.5, CAL-V0-089
CodexContinuationResumesPreservedSession 27.8, CAL-V0-089 TurnCapsBoundContinuation 27.5,
CAL-V0-197 RunRoleSelectsAnsweredWaitOfItsStage 27.1, CAL-V0-078
SupervisedRunAfterCommitIsNotRetryable 26.3, CAL-V0-089 IntegrateCheckpointRestartKeepsGrant 26.3,
CAL-V0-044 HandoffPolicyReceiptInterval 24.4, CAL-V0-087 ExtraWorktreeCleanup 21.3, CAL-V0-087
UnchangedRepositoryNeedsNoDesignation 21.2, CAL-V0-019 KilledWriterRecovers 20.4, CAL-V0-078
SupervisedCheckingFailureIsNotRetryable 18.2.

The 262 changed or seam-touched tests also ran with `-count=5` (no race, `GOMAXPROCS=3`):
1,310 passes, 0 failures, 3,517.5s.

CI estimate from run 38055182050's per-test times: the tests that stay sequential cost about
599s (program_continuation 167, writer_checkpoint 101, pool_sweep_lifecycle 46,
program_role_stage 43, qualification 36, pinned_inventory 35, others below 25). The parallel set
costs about 1,272s solo. Even with ideal overlap at `-parallel 4`, the package should land near
1,100 to 1,250s in CI, so the 700s target is not met by test seams alone.

## Remaining limits

These are production behaviour, so they were reported rather than changed:

- `programWriter` (`internal/tasks/store/program.go`) is one process-global mutex held across
  `readLeaseProof` and the write in every program transition. It serializes the supervised
  tests, the largest parallel group, and inflated their walls under parallel load.
- `leaseAudits` (`internal/tasks/store/lease_write.go`) is a single-entry, process-global cache.
  Parallel tests over different stores evict each other and repeat complete audits.

Other levers: key those two by store; raise CI `-parallel` above GOMAXPROCS, since the tests
mostly wait on syscalls and subprocesses; key `publishFault`, `gateRecordFault` and the pool
hooks by store to move about 69s of CI time into the parallel set; and split the package across
shards.

## Not run

Dogfood bind and seal, `make gate` and the repository-wide test run, and a CI run confirming the
new wall time.
