# 2026-10-31 tasks-store wave-2 lane

Lane `tasks-store` (wave 2) took 15 internal/tasks/store tickets at origin/main
`6c967b21caa2ffb0324a681f873a891996d82e34`, on the Unraid lane host: linux/amd64 native, 12 CPUs,
go1.27.3, `/tmp` on btrfs. Load figures are the 1-minute load average. Commits are local to
`claude/lane-tasksstore-w2-1031`.

## Delivered

### V1-2878: publish sync errors wrap their cause

- Reproduced: with `TestLGLV0016_FaultAtEachPublishSyncStopsTheNextStep` asserting only `errors.Is`,
  `claim-sync-0` and `heartbeat-sync-0` failed. Sync 0 is the session-open directory sync, which
  `Dir.Sync` reported as `UNSUPPORTED_FILESYSTEM ... directory sync failed: %v`.
- Decision: `wire.Errorf` now formats through `fmt.Errorf` and records each `%w` operand, and
  `*wire.Error` gains `Unwrap() []error`. The message text and the code are unchanged, and the
  concrete type stays `*wire.Error`, which seven call sites type-assert. Every directory-sync and
  full-sync refusal in `internal/tasks/authority` uses `%w`. A wrapper type was rejected because it
  would break those type assertions.
- Evidence: the TestLGLV0016 fault test now asserts `errors.Is` for every sync index, and
  `TestErrorfWrapsCause` covers single and multiple `%w` operands, the outer code winning, and `%v`
  not wrapping. The full `./internal/tasks/wire` package passes. In `./internal/tasks/authority`,
  only `TestCALV0095_PreparationQueueObservation*` fails, and it fails at the base too (see
  Friction).

### V1-2743: per-store clock for CALV0089

- Decision: a test-only registry, `testClocks`, keyed by state directory. `storeNow(repo)` returns
  `time.Now()` unless a test registered a clock for that store. The workflow program wall (the
  remaining time and the continuation check), the program `StartedAt` time, the GateRun clock, and
  the supervisor and pool writers' `storeClock(repo)` all read it. A package-level clock was
  rejected because the CALV0089 tests run `t.Parallel` and other stores must keep real time.
- `TestCALV0089_ProgramWallExpiryEndsContinuation` holds the store clock still. It opens the
  program at T0, sets the clock 4 s short of the unchanged 3-minute wall for the stage, then past
  the wall for the retry. Measured: 186 s before (ticket), 7.7 s alone after. 20/20 passed under 12
  `yes` burners (each about 35 s while starved). The production clock and the wall duration are
  unchanged.

### V1-3046: CALV0089 and CALV0111 timing dependence

- CALV0089: the stage wall was one real duration that had to both stop a stalled fake host (4 s)
  and spare a loaded resume (12 s, V1-1516). Decision: add a per-store `stageWalls` seam. The
  continuation fixture ends a stage as if at its own wall (`context.DeadlineExceeded`) when the
  fake host appends to `stalls`, with a 300 s real backstop. The program wall is excluded, so the
  ProgramWall test keeps its real deadline. Drain/while-running keeps a real wall (backstop length)
  and proves the drain by its error, Canceled not DeadlineExceeded. Before this, its 20 s elapsed
  bound failed 4/4 at load about 23. Its two 15 s polling hang guards are now 120 s.
- CALV0111: the same-request retry reused the 200 ms caller lock wait, which that step does not
  test. It now waits the default lock wait.
- Evidence: 20/20 runs of every CALV0089 test and CALV0111 under parallel package load: 12 runs as 4
  concurrent processes at load 20-26, plus 8 more. Drain on its final code: 4 runs, plus 16 runs as
  4 processes with 8 burners at load 16-18.

### V1-2764: PSRV0224 and PSRV0151 count assertions

- Neither flake reproduced locally: 200 runs each across 4 processes and `-cpu 1,4` with burners,
  and two whole-`TestPSRV0` runs. The counts are therefore bounded, as the AC allows.
- PSRV0224: per lease round, the writer route declines a claim at most twice: the deferral, then the
  refusal of the pre-commit base. The test requires `2 <= declines <= 2*LeaseTiming.Rounds`, the
  complete route, and the moved commit as the base. A single round is still exactly 2.
- PSRV0151: a new helper, `movingObserved`, reads the walk split at the moment the token-invalidated
  resume declines. The declined resume's own charge is checked exactly (no resumed walk, nothing
  resumed). A later round on a loaded host may resume and serve. When nothing resumed, the
  complete route must have served the write.
- Evidence: 100/100 runs of each test (`-count=10 -cpu 1,4` in 5 concurrent processes plus 6
  burners, load 23-28), which covers `-count=50 -cpu 1,4`.

### V1-2773: JSGV0020 quiet resume

- Root cause not reproduced. 20/20 runs under 4 concurrent processes plus 8 burners (load 17-26)
  recorded no quiet-mode resume at all. Quiet mode makes no dispatcher write, so a resume needs a
  round that ended SNAPSHOT_MOVED, and resuming after a move is the correct PSR-V0-052 route.
- Decision: the AC's second branch. Quiet no-resume is asserted only when the series qualification
  is `QUALIFIED`; an unqualified series logs it. At least one audit per call is asserted always.

### V1-2593: inotify watch peak in the PSRV0055 harness

- Each worker reads its own `/proc/self/fdinfo` `inotify wd:` lines just after every change-guard
  registration of a call, while the guard is open. There is no external sampler. The report gains
  `inotifyWatches` with `perProcessPeak`, `processPeak`, and `summedPeak`. `summedPeak` is the sum
  of the per-process peaks, an upper bound on the simultaneous user-wide count. Off Linux the
  report says `NOT_OBSERVED`.
- V1-2482 evidence: a Linux run at 45,000 receipts and 8 acquirers (60 s load window, 60 s grace,
  load 3.5 -> 15.3, UNQUALIFIED by the harness's rule) held 270 watches in each acquirer process,
  a summed peak of 2,160. It had 0 errors (no ENOSPC), and every verb completed 8/8. The
  dispatcher's Mutate path registers no instrumented guard and reads 0. A smoke run at 2,000
  receipts also gave 270 per process.

### V1-2530: dirfd-relative fstatat in the Linux sweep (PSR-V0-890)

- Spec amendment first: PSR-V0-890, experimental under the standing lane auto-accept.
- Decision: `sweep` groups tracked files by directory once per watch. It opens each directory only
  for that sweep and reads each file with `fstatat(AT_SYMLINK_NOFOLLOW)` on a precomputed
  NUL-terminated name. linux/amd64 uses raw `SYS_NEWFSTATAT` (the `syscall` package exports no
  `Fstatat` there) and linux/arm64 raw `SYS_FSTATAT`. Other Linux architectures read through
  `/proc/self/fd/N/name`. There is no new dependency.
- Decision: no directory descriptor is held between sweeps. Holding one per directory (270 at 2k
  receipts) would spend the process descriptor budget that CALV0070 measures. A directory that
  cannot be opened, for lack of a descriptor or search permission, is swept by full path, so
  detection never depends on a descriptor. Poll's re-read of uncovered files keeps `lstat`.
- Evidence: `TestPSRV0223_SweepReadsByDirectoryDescriptor` checks fstatat for every file and no
  lstat, parity with lstat, and detection of a later link, chmod, rename-over, and an unopenable
  directory. The existing alias-link tests pass. `TestPSRV0223_ChangeGuardSweepCost`, linux/amd64
  native btrfs, load below 7: Check median 37.4 -> 28.2 ms at 10,000 receipts (20,057 files) and
  171.6 -> 120.9 ms at 45,000 (90,057 files). WSL2 was not measured.

## Not delivered

(First round. The orchestrator decided the design forks of V1-0961, V1-2203, V1-0916, V1-2358 and
V1-2621 on 2026-11-01, and the second round below reworks them.)

- V1-2676 (BLOCKED, needs a CI-runner reproduction): 20/20 `-count=20` runs under load passed. The
  `corrupt` child's `open /: too many open files ... JOURNAL_FORKED` line is its designed passing
  output: it limits the process to zero spare descriptors with the watch open. Each mode already runs
  in its own child process with its own temporary store, so no sibling can move the clean store.
  Probes registering 50 guards on a 70-receipt store while churning sibling entries and top-level
  `/tmp` entries, on btrfs and tmpfs, saw 0 failures. One hypothesis was ruled out: `mkdir` inside an
  ancestor directory raises no `IN_ATTRIB` on it. Which writer moved the store on the CI runner (ext4,
  load 40-80) is still unknown.
- V1-0961: whole-package `go test -v ./internal/tasks/store` at load 5-12 was cut by the lane's
  595 s command bound with 174 of about 550 top-level tests done. The summed top-level durations
  are about 3,640 s over 515 tests. Top contributors in that run: LGLV0016 FaultAtEachPublishSync
  93 s, JSGV0013 84 s, JSGV0012 77 s, CALV0019 KilledWriter 70 s, CALV0087 Interrupted 66 s,
  JSGV0005 63 s, CALV0045 59 s, CALV0018 52 s, CALV0215 51 s, JSGV0011 48 s. CALV0089
  ProgramWall dropped about 178 s here (V1-2743). Reaching under 10 minutes needs restructuring
  (parallelizing tests that share package-global seams such as `publishFault`), beyond this lane's
  budget.
- V1-2203: `TestCALV0118_WriterHoldProfile` on this host at 880 tickets, 11 reps, load about 3:
  heartbeat median 83.4 ms at 13,500 receipts and 89.8 ms at 40,000, of which fsync is 32.7-33.2 ms
  (btrfs), observe 18.2 -> 27.3 ms, and commit 42.0 -> 40.7 ms. The 50 ms target needs fewer syncs
  per heartbeat, a durability-path change, not attempted.
- V1-0916: a throwaway probe at 880 tickets x 13,000 receipts ran 600 sequential Mutate creates. The
  refresh at write 511 cost 2.38 s inside its caller; p50 was 442 ms, p99 513 ms, max 4.20 s.
  `refreshWriterCheckpoint` still runs a complete audit from receipt 1 in the triggering process,
  after the lock is released. Removing that cost needs an incremental refresh, which weakens
  CAL-V0-117 and needs an owner decision, or an off-caller process. Not attempted.
- V1-2358: a throwaway probe measured derived bytes written under the writer lock at 40 -> 400
  tickets. The Mutate writer route writes `checkpoint.json` (6.4 KB -> 53.3 KB). Refresh and the
  complete-route Mutate each write the read checkpoint plus the writer base and delta (14.9 KB ->
  114.7 KB). The complete-route Mutate audits entirely under the lock, and the writer routes derive
  their proof under it, so staging before the lock first needs those routes restructured. Not
  attempted.
- V1-2621: on this base, a widen on a sealed-base store resumes from the sealed base even when a
  writer-route commit lands at its resume precheck (stages `fast.resume.*`, 0 audits). So widen no
  longer reaches the complete route in that state. The remaining trigger is a complete-route write
  while the bound checkpoint is still valid. Keeping a base at S there needs either a journal-level
  base capture at S during the complete walk or a seam to force that route. Not attempted.
- V1-2539 and V1-2592 (BLOCKED): both acceptance criteria are WSL2 `TestCALV0263_LeaseGateLoad`
  before/after reports. This lane has no WSL2 host. Not attempted.

## Second round (2026-11-01)

The orchestrator decided the remaining forks on 2026-11-01. The installed `corvint`,
`corvint-console`, `corvint-mcp`, `corvint-tasks`, `corvint-update` and `corvint-web-flows` were
rebuilt from the lane head `f1680433b` (the old ones kept as `*.pre-r35.bak`), which cleared
`self-binary-stale`. Load figures below come from a host shared with other lanes.

### V1-0961: store package slices

- Decision: the committed slice route (AFP-V0-041, `.github/cishards/test-slices.json`) already
  runs the store package as four slices. Regenerating it needs one complete 8-shard CI log set,
  which this host cannot fetch (no gh auth), so the slice file is unchanged. The catch-all slice is
  trimmed instead.
- The catch-all slice (every test added since the slice file was generated) first ran past 595 s.
  Its sequential phase was 323 s, of which `TestLGLV0016_FaultAtEachPublishSyncStopsTheNextStep`
  was 123 s: 27 subtests, each building its own store. The subtests now run in parallel under a
  still-sequential parent. A per-store publish-fault seam (`publishFaults`) and a test-side router
  hand each directory sync to the recorder whose store's worktree holds the directory. The baseline
  write that counts the syncs runs alone in strict mode and must route every sync to its store, so
  no sync leaves the matrix. The test takes about 9 s, race-clean.
- Per-slice wall, each one `store.test` process with the committed patterns: before the trim,
  395 / 404 / 225 / >595 s; after the trim, 545 s for the catch-all. With every second-round change,
  538 s (load 6-8), 330 s (15-25), 207 s (12-22) and 523 s (7-18). Every slice is under 600 s. The
  only failure is `TestPSRV0224_HeadFilesResolveAsGit` (old Git, V1-3568).

### V1-2203: heartbeat floor (LGL-V0-890)

- Finding: every writer called `authority.Qualify(repo.CommonDir)`, which costs 13 ms median here:
  a fully synced probe file, a flock and a `.git` directory sync, before the gate.
- Decision: `authority.QualifyReused` keeps a successful qualification per process while
  `os.SameFile` holds on a fresh `Lstat`. A failure is never kept, and a replaced root is probed
  again. Every published artifact keeps its file and directory syncs (LGL-V0-008), so crash safety
  of an acknowledged heartbeat is unchanged. `TestLGLV0890_HeartbeatKeepsEveryArtifactSyncWith-
  AReusedQualification` checks that the heartbeat's sync count is unchanged and that no probe file
  is left. Group commit was not added: it cannot lower a single call's median.
- `TestCALV0118_WriterHoldProfile` (880 tickets, 11 reps, load 2-5): heartbeat median 83.4 ->
  73.4 ms at 13,500 receipts and 89.8 -> 75.1 ms at 40,000. NOT MET against 50 ms. The measured
  floor is the lock hold of about 71 ms: four sequential sync rounds of about 34 ms on btrfs
  (staged files, the receipts directory, the post directories, the head directory), observe of
  19.5 -> 26.3 ms that grows with receipts, and about 8 ms of journal writes. Reaching 50 ms would
  mean dropping one of LGL-V0-008's ordering edges.

### V1-0916: incremental refresh (PSR-V0-891)

- Spec: PSR-V0-891 in the pool-safe-reuse companion spec. It extends CAL-V0-265 and amends
  CAL-V0-117 step 2, and CAL-V0-117 step 2 now points to it. The leases spec gains no requirement.
- Decision: the scheduled refresh first resumes the complete validators from the bound checkpoint
  (`AuditForWriter`) on an unsegmented store. `Result.IncrementalWriterCheckpoint` derives the
  complete refresh's checkpoint from it: a new base, with `FullSeq` at the head. Any decline falls
  back to the complete refresh. The complete refresh also runs whenever the head crosses a
  multiple of 4,096 since `FullSeq`. There is no off-caller process.
- Evidence: `TestPSRV0891_IncrementalRefreshMatchesTheCompleteRefresh` compares base and delta
  byte for byte over six randomized create/heartbeat/renew sequences of 10 to 89 writes, longer
  than the 64-receipt re-base. It passed 12/12 under 4 processes plus burners.
  `TestPSRV0891_FullRefreshStaysTheBackstop` covers the schedule. The opt-in harness
  `TestPSRV0891_RefreshBoundaryLatency` (13,000 x 880, 600 creates, load about 5-6) measured the
  refresh write at 568-580 ms (2.38 s with the complete refresh), p99 758-775 ms, and every write
  after the fixture's first complete-route write at or under 1.03 s.
- Tests of what only the complete refresh detects pin their store to it (`completeRefreshOnly`):
  CALV0116 prefix tamper, the CALV0117 invalidation cases and the read-checkpoint retention, and
  JSGV0003 forged refresh. That carried-forward window up to the backstop is PSR-V0-891's stated
  failure mode.

### V1-2358: checkpoints outside the writer lock (JSG-V0-891)

- Decision: a `deferredRetention` collector records, under the lock, only the audit and token a
  writer decided to retain. After release it derives and stages the checkpoints and renames the
  read checkpoint lock-free, because the checkpoint is bound to its own head identity. It renames
  the writer base and writes the bounded delta under a lock re-acquisition of at most 2 s, gated by
  the recorded token. This applies to Mutate (writer and complete route) and the lease writer route.
  The refresh stages before its lock. The complete-route Mutate audit itself still runs under the
  lock: moving it out would need the lease route's optimistic recheck machinery.
- `TestJSGV0891_WritersRetainCheckpointsOutsideWriterLock`, 40 -> 400 tickets, bytes written
  under the lock: the writer-route Mutate goes from 6.4 KB -> 53 KB before to 0 after. Refresh and
  the complete-route Mutate go from 14.9 KB -> 114.7 KB before to only the writer delta, 645 -> 646
  B. The read checkpoint (53 KB) and the base (61 KB) at 400 tickets are now staged outside the lock.
  The verify compare that stays under the lock costs 0.75 ms -> 6.3 ms.
  `TestJSGV0015_MovedClaimResumesOnTheWriterRoute` now requires 0 read-checkpoint bytes under the
  lock.

### V1-2621: base-at-S capture (LGL-V0-891)

- Decision: `WriterCheckpoint.RebaseOnSealedBase` re-bases a complete audit's checkpoint onto the
  bound checkpoint's sealed base at S through the seal's own `rebase`. `rebase` refuses unless
  everything at or before S matches what the complete audit derived from receipt 1, and the base's
  segment digests must be the manifest's. The lease complete route re-bases before its lock, so
  the JSG-V0-015 staging still applies, and drops the re-base when its verify refutes the bound
  checkpoint. The complete-route Mutate does the same in its deferred derivation.
- Evidence: `TestLGLV0891_CompleteRouteWriteKeepsTheSealedBase` forces a Mutate onto the complete
  route with the checkpoint still valid (test-only `mutateWriterOff`). `FullSeq` stays at S, and the
  next pool release resumes (Resumed 1, 0 audits) with the complete route's report and bytes. With
  the re-base disabled it fails. It passed 12/12 under load. The LGL-V0-011 47k real-disk harness
  run is NOT_RUN.

### V1-3596: TestJSGV0015_MovedClaimResumesOnTheWriterRoute (test expectation, not product)

- Reported as failing on every run at origin/main 6c967b21 and at lane heads with "rounds 1 resumed
  0". On this Linux host it passes at 6c967b21 (2/2), at origin/main 8daec98ce (4/4) and at the lane
  head, so the failure depends on the host.
- Cause, inferred from the guard contract and not reproduced here (no macOS on this host): the
  test's outside edit was an in-place `os.WriteFile` of a ticket under the first commit's lock,
  which the locked `CheckEvents` must report for the claim to take a second round. Linux inotify
  reports it. On macOS, PSR-V0-223 and CAL-V0-026 document that an in-place write to a file beyond
  the vnode descriptor budget is not reported until a sweep, so there the claim commits in one
  round. That is the reported output.
- Decision: fix the test, not the product. Store writers change files only by creating, renaming,
  linking or removing entries, which watched directories report on every platform. The edit now
  writes a new file and renames it over the ticket, which keeps the test's intent (an outside change
  under the first commit's lock moves the snapshot). It passes 4/4 here; confirmation on macOS is
  NOT_RUN from this lane.

### Review fixes (Codex review of f1680433b..bd93c2a6f)

1. FIXED, `seal_checkpoint.go` (V1-2621, LGL-V0-891): the re-base copied the retained base's
   historical receipt bytes and archive cost without checking them. Verified: a base forged with
   lowered costs and correct current totals was reused. `Result.SealedBaseCheckpoint` now derives
   the aggregates at S from the complete audit's own reads (receipts 1..S and every request
   afterimage posted at or before S) and refuses unless the base carries exactly those; the raw
   re-base is unexported. `TestLGLV0891_ForgedSealedBaseAggregatesAreNotReused` uses a test-only
   journal seam, `ForgeSealedBaseAggregatesForTest`, and fails with the comparison removed. The
   base's intent aggregate at S is still not re-derived; the checkpoint's live intent totals come
   from the delta, which rebase checks against the audit.
2. FIXED, `deferred_retention.go` and refresh (V1-2358, JSG-V0-891): a fresh base that failed to
   stage came back as nil and was re-encoded under the lock, and the refresh retried a failed read
   checkpoint under the lock. The lease commit had the same fallback, against JSG-V0-015's own rule.
   Verified with an injected staging failure (`stageCreate` seam). `stageWriterCheckpoint` now
   returns a failed result, which `retainStagedWriterCheckpoint` skips, and the refresh dropped its
   locked retry. `TestJSGV0891_FailedStagingRetainsNothingUnderTheLock` writes nothing under the
   lock. With the old code it finds 6.5 KB (read checkpoint), 15.5 KB (refresh base) and 7.4 KB
   (lease commit base).
3. FIXED, `qualify_reuse_test.go`: the race detector confirmed that the hook's plain counters race
   under the overlapped publish syncs. They are now atomics, and the syncs stay concurrent.
   Race-clean 5/5; the other new tests from this round are race-clean too.

## Friction

- `TestCALV0095_PreparationQueueObservation*` (internal/tasks/authority) fails at the base on this
  host, inside a container, with `lock table device mapping is ambiguous`. Repro: in a container
  whose reader is outside the initial PID namespace, run
  `go test -count=1 -run TestCALV0095_PreparationQueueObservation ./internal/tasks/authority`.
- `TestPSRV0224_HeadFilesResolveAsGit/declines` needs `git init --ref-format=reftable`, which this
  host's Git refuses (`unknown option`), so it fails at the base. Repro:
  `go test -run TestPSRV0224_HeadFilesResolveAsGit ./internal/tasks/store` with Git older than 2.45.
- The lane's 600 s per-command bound cannot hold a whole-package `internal/tasks/store` run (more
  than 595 s), so V1-0961's own whole-package measurement cannot be retained from a lane. Repro:
  `timeout 595 go test -count=1 ./internal/tasks/store`.
- `go test -c` binaries cannot run from `TMPDIR=/dev/shm` here (`noexec`), which blocked a tmpfs
  `go test` run. Repro: `TMPDIR=/dev/shm go test -run X ./internal/tasks/store` gives `fork/exec ...
  permission denied`.
- One heavy.sh-wrapped run of 4 concurrent store processes, CALV0089 x3, outlasted 600 s and was
  moved to the background by the harness. That breaks the charter's foreground rule. It happened
  once, by mis-estimate.
- Loaded flake triage needs a whole-package, CI-like process. Running isolated tests under burners
  did not reproduce any of V1-2676, V1-2764 or V1-2773.
- `make host-adapter-node-test` fails at the base in batch-precheck (base-present, not counted).
- Before the rebuild, the installed `corvint` reported `DEGRADED self-binary-stale`, so
  `corvint affected` ran on a stale binary. With `internal/tasks/wire` changed, its scope was
  `UNKNOWN`.
- The store slices can only be measured on a host shared with other lanes (load 6-25 during the
  second round), so the same slice varied 395-538 s between runs. A per-slice figure from a shared
  host is an upper bound.
- `TestJSGV0891_WritersRetainCheckpointsOutsideWriterLock` and the JSG-V0-015 tests share the
  package-global `derivedWritten` hook, so they cannot run in parallel and add to the sequential
  phase of the catch-all slice.
