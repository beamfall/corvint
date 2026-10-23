# 2026-10-23 store test load: V1-2449 stage-slot race, store fixture cost, load flake counts

Lane `claude/optimistic-volta-x5wmgt`, base `7b3a8e55fc909b9692c684e47dc4c6787ac2419a`. Go 1.27.2
(`GOTOOLCHAIN=go1.27.2` fetched it; the host `go` is 1.24.7). Host: Linux, 4 CPUs.

Tickets: V1-2449, V1-0961, V1-1499, V1-1516, V1-1108, V1-1111, V1-1473, V1-1486, V1-1493,
V1-1500, V1-1532, V1-1566, V1-0771, V1-1405. The session stopped early (cloud budget), so the
campaign below is partial; every unfinished item is named as NOT_RUN.

## V1-2449: unlocked audit refuses a live writer's staging slots (CAL-V0-340)

- Reproduction. The failing CI text was not available (the macos-15 artifact host is blocked in
  this session). Plain runs did not fail: 720 of 720 passes at load 35 to 42, with and without
  `-race`, `-cpu 1,4,8`. A macOS-like slow fsync did: `strace -f --seccomp-bpf -e
  inject=fsync,fdatasync:delay_enter=30ms` failed the unchanged test 3 of 36 runs and at 100 ms 2 of
  38, each as `racer 2: MALFORMED: staging/aNN: unassigned stage slot`. Racer 2 is `gate-a`, whose
  pre-read (`readLeaseProof`) is an unlocked journal audit.
- Cause. Eight other writers publish back to back, and each holds descriptor-less staging slots for
  its whole publish. Every observation of the audit sees slots, so the two-second CTS-V0-008 wait
  ends inside that run of writers and the audit refuses `MALFORMED`, the refusal meant for a killed
  writer's orphan. This is the V1-0467 signature at a slower host speed.
- Fix (production, `internal/tasks/journal`). `slotsInFlight` carries the `head.json` its
  observation read. When the wait is spent and that head differs from the head of the first
  observation of the slots, the audit refuses the retryable `SNAPSHOT_MOVED` at `staging`; the lease
  read's rounds already retry it. A head that never moved keeps the unchanged `MALFORMED`, so the
  orphan tests are untouched. The archive export has the same wait and is not changed.
- Evidence. `TestCALV0340_SpentStageWaitOnAnAdvancingHeadIsRetryable` fails on the old reader
  (`got MALFORMED: staging/a00: unassigned stage slot; want SNAPSHOT_MOVED at staging`) and passes
  with the fix; I ran it both ways. The unchanged racing test under injected fsync latency: fixed
  binary 0 of 40 failures (10 at 30 ms, 30 at 100 ms). The old binary failed 5 of 74 across both
  settings, so that count alone does not separate the two; the unit test is the proof.
- Spec: CAL-V0-340 in `corvint-tasks-agent-leases-v0.md`; CTS-V0-008 and its failure-mode row in
  `corvint-tasks-store-init-v0.md` point to it.
- Review repair. Head movement is evidence that warrants another bounded observation, not proof
  that the slots' holders are live: a writer can advance the head and die before removing its slot.
  The requirement, the CTS-V0-008 cross-reference and the code comments now say so, and the test
  runs a second audit with head and slot unchanged and requires `MALFORMED` `staging/a00`.
- Review repair (fixture). The synthesized CREATE requests of the history fixture recorded
  `SHA256(requestId)` as `mutationSha256`, so resubmitting `historyCreate("history-create-N", ...)`
  refused `REQUEST_ID_CONFLICT` where a real CREATE's request replays. They now record the digest of
  that exact envelope. `TestCALV0070_SynthesizedCreateReplays` failed on the previous head with
  `REQUEST_ID_CONFLICT` and passes now; the CAL-V0-190 corruption cases assert the clean replay
  before corrupting and the expected refusal (`JOURNAL_FORKED`, `JOURNAL_FORKED`, `MALFORMED`)
  after.

## V1-0961, V1-1499, V1-1516: package time

- Measure. Two unmodified full runs of `internal/tasks/store` (`-count=1`, `-p` default, quiet host,
  per-test times from `-test.v=test2json`): 834 s and 722 s wall. The CPU profile of the second
  showed `historyStoreAt` at 30% of CPU, of which 25% was its 50 real `Mutate` creates per fixture;
  37 call sites build it, 14 of them at 1,100 receipts.
- Change (test helper only). `historyStoreAt` now creates one ticket with `Mutate` and the other 49
  with the existing synthetic `historyAppendCreates`, which now also stamps the receipt's ticket and
  the request outcome's revisions so a found request still reports its ticket
  (`TestCALV0070_MutateAuditSequenceEquivalence` needed that). Every history-based assertion is
  unchanged.
- Result. The 80 tests that build a history store: 305.9 s to 126.7 s wall, 840 s to 356 s CPU, all
  passing. Whole package, same command: 722 s to 560 s wall, CPU 1,313 s to 869 s. The single
  failure in all three full runs, `TestPSRV0166_TimeoutKillsGroupOutputBounded`, is a sandbox
  artifact: PID 1 here never reaps orphans, so the killed child stays a zombie and `kill(pid, 0)`
  succeeds. It fails alone on the unmodified binary 3 of 3.
- `TestCALV0078_GateRunContentionAfterExecutionIsReported` idled 30 s for the default lock wait. It
  now bounds the recording's lock wait to 5 s with `WithLeaseLockWait` (CAL-V0-111); 30.4 s to 5.8 s
  at load 28, same assertions. That edit came after the 560 s run.
- Left alone, with reasons: `TestCALV0089_ProgramWallExpiryEndsContinuation` (181 s) waits for a
  real three-minute program wall and needs a production clock seam; the CAL-V0-089 continuation
  tests depend on the fixed 4 s stage wall and failed when parallelised earlier
  (`2026-10-10-store-test-speed.md`); `SetPublishFaultForTest` users are 48 s serial and keying it
  per store is a wider change.
- NOT_RUN: three full-package runs at load 60 or more (V1-1499), and a post-change quiet run after
  the GateRun edit. The 30 m hang detector is unchanged.

## V1-1108: already delivered

Per-root `programWriters` and `leaseAudits` are on main with `store_root_exclusion_test.go`.
`go test -race -run TestV11108 ./internal/tasks/store` passes. The whole-package `-race` run named in
its acceptance is NOT_RUN.

## Load counts (24 CPU burners on 4 CPUs, load 26 to 28, `store.new.test`)

| Ticket | Command pattern (`-test.run`) | Count | Result |
|---|---|---|---|
| V1-1473 | `^TestCALV0072_MultiRepositoryGatesFailClosed$` | 30 | 0 failures |
| V1-1500 | `^TestCALV0089_IntegrateCheckpointRestartKeepsGrant$` | 20 | 1 failure |
| V1-0771 | `^(TestCALV0075_ClaudeCodeProgramFakeHost\|TestCALV0077_OpenCodeProgramFakeHost)$` | 20 | 0 of 40 test runs |
| V1-1493 | `^TestCALV0078_(SupervisedRunAfterCommitIsNotRetryable\|IntegratorFailureBeforeLeavingSelectionIsRetryable)$` | 20 | 0 failures |
| V1-1111 | `^TestCALV0197_` | 10 | 0 of 30 test runs |
| V1-1486 | `^TestCALV0232_` | 10 | 0 failures; 93 s for ten runs (quiet host: 9.0 s each) |
| V1-1516 | `TestPSRAllBarrierTerminal`, `TestPSRSweepSuccessReplaySuccessor`, `TestCALV0089_DrainStopsContinuation`, `TestCALV0089_CodexContinuationResumesPreservedSession`, `TestPoolProcessDescendants` | 10 | 0 failures |
| V1-1405 (PSR) | `TestPSRDispatchAllocationFence`, `TestPSRHistoricalReplayDeadline`, `TestPSRV0012PinBoundToLaunch`, `TestPSRV0012ExpectedNonzeroNeedsProof`, `TestPSRSweepOwnerBarrier`, `TestPSRAllBarrierTerminal`, `TestPSRSweepSuccessReplaySuccessor`, `TestPSRCleanupAllowanceAndProbeStreams`, `TestSupervisorWaitRetainsScopeReleasesWorker`, `TestPoolProcessDescendants` | 10 | 0 failures |
| V1-1405 (SURVIVORS) | `^(TestCALV0086_\|TestCALV0089_\|TestCALV0077_\|TestCALV0078_Supervised)` | 3 | NOT_RUN (stopped) |

- V1-1500 reproduced once in 20 at load about 27: after the operator retry `resume-args` held three
  `exec resume integrate-session` lines instead of two, at `program_continuation_test.go:645`. A
  resumed host turn missed the fixed 4 s stage wall and took one more continuation, the class of
  V1-1516's "implement turns 3, want 2". The property the test proves held (resumed session, grant
  kept, one landing). Not fixed: every open takes the same pinned config, so the wall cannot differ
  per open, and a test seam for the wall is production code. I did not relax the count assertion.
  The TestCALV0117 581/582 mismatch was not run.
- V1-1566. The `releaseAttempt` helper, and the `release-under-run` goroutine in
  `attempt_run_test.go`, passed a `time.Now()` sampled before the lock wait with a plain context,
  while a running supervisor heartbeats. `WithClock` exists for that case (CAL-V0-012, pinned by
  `TestCALV0012_WriterBehindNewerHeadSamplesAgain`). Both now use `store.WithClock(.., WallClock)`.
  Not reproduced: `TestATRV0013_` 30 of 30 on the old helper and 30 of 30 on the new at load 27, and
  16 of 16 forced runs (lock held across a second boundary), both forms. This is a hardening by code
  reading, not a confirmed cause. V1-0532 was not compared.
- V1-1532: the writer already runs until five exports overlap it (`read_wait_test.go`, commit
  `56c27a39`); no run here.

## NOT_RUN

`make dogfood-change`, `dogfood-check`, `dogfood-seal`, `heavy.sh`, `corvint-tasks` writes and the
installed `corvint` binaries (not available in this session); `make gate`; the full-package `-race`
run; `make capability-catalog` and `script/repin-use-case-receipts.sh` (no help text or
`cmd/corvint/main.go` change). V1-2449 has no record in `.taskman/tickets` on this base.

## Rollback

Revert the commit. The audit again refuses `MALFORMED` after the stage-slot budget whatever the
head did; the test helpers return to 50 real creates and the 30 s lock wait.
