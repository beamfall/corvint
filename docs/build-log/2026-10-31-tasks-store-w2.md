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
