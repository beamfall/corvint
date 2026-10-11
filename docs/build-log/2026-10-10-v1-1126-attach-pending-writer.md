# V1-1126: attach waits for a pending writer

CTS-V0-009 in `docs/specs/corvint-tasks-store-init-v0.md` is **accepted, decision 0513
(V1-1126)**, cross-referencing CTS-V0-006 and ATR-V0-008/011. Owner acceptance
is decision 0513. The starting commit is `71b2eb8fc3e56a9df368b8be8227bf5ad125f8c9`.
The bounded plan was to prove the existing attach fixture, replace only the bare head probe
with the existing pending-writer reader, exercise its publish and timeout boundaries, then
run the requested qualification and doc gates. Builder/reviewer used the inherited Codex
model and effort; this is a small read-path fix with independent concurrency scrutiny.

Attach's initial head read now uses `snapshot.Reader.Read` and its production two-second
`DefaultPatience`. A private pause seam lets the test complete the writer exactly during the
read's first pause. `--wait 0` still bounds run polling separately. A pending redo after the
budget keeps attach's existing exit 1 and `ERROR`/`REDO_PENDING` envelope, adding the existing
reader's wait/retryable warning. No new wire code, journal format, writer, or recovery path.
Rollback restores the bare `snapshot.Probe` in `attemptAttach` and removes the new tests.

## Evidence

- Baseline: race-enabled single `TestATRV0008_DetachedRunSurvivesItsLauncher` passed (6.259s).
- `TestCTSV0009_AttachWaitsForReceiptLinkBeforeHeadRename` first proves a valid finished-run
  replay, then links canonical receipt bytes with `os.Link`, proves `REDO_PENDING`, and holds
  the head at seq 1 until the reader's first 25 ms pause performs the seq 2 head rename. Attach
  replays the exact kept result; state and run-file checks separate the writer's mutation
  from attach's read-only behavior.
- `TestCTSV0009_AttachReportsPendingAfterPatience` leaves the same publish unfinished,
  spends an explicitly shortened 50 ms patience budget, and asserts the unchanged refusal,
  elapsed wait and retryable warning with journal, intent and run bytes unchanged.
- Focused command: `go test -count=1 -race -run '^TestCTSV0009_' ./internal/tasks/cli/`:
  PASS (2.013s). An initial assertion expecting `NOT_RUN` failed because attach already maps
  this error to `ERROR`; the test and proposed requirement were corrected to preserve it.
- Negative control using a Go overlay restoring only the original bare probe: both CTS-V0-009
  tests FAIL as expected (0.560s), with immediate REDO_PENDING and zero pause callbacks.
- Full touched-package command: `go test -count=1 -race -timeout 30m ./internal/tasks/cli/`:
  FAIL (exit 1, 1654.047s), with 12 failing top-level tests; no CTS-V0-009 or ATR-V0-008 failure.
  Pool claims report `fork/exec /bin/ps: operation not permitted`; foreground sweep helpers
  refuse `QUIESCENCE_UNPROVED` because runner identity is unavailable. That failed signal
  fixture also exposes a data race in its unchanged `bytes.Buffer.String` diagnostic at
  `internal/tasks/cli/pool_sweep_test.go` line 137 while the helper pipe writes. Dispatcher
  hand-off/progress assertions and doctor recovery/idle/plugin assertions also fail. These
  are outside the minimal attach fix; broader suite success remains unresolved. Full log:
  `/private/tmp/claude-501/gotmp/v1-1126-cli.log`.
  Failing tests: TestPSRDispatchNativeAdapter, TestPSRForegroundSignals,
  TestCALV0097_DispatchRoutesPoolTicketsAndKeepsLaneFreeProgress,
  TestCALV0064_DispatchCLIFileProgressAndReplay, TestCALV0052_DispatchCLIClaimHandoffAndStatus,
  TestTQDV0007_SlowLaneRecovery, TestTQDV0009_FalseIdle, TestTQDV0010_PluginFindings,
  TestCALV0065_NativeFixture, TestPSRV0013_PoolStatusMembers, TestPSRV0014_PoolStatusFilters,
  TestPSRV0015_PoolStatusLockFreeAndWritesNothing.
- Representative broader-suite failures against an overlay restoring the original attach
  probe, with `go test -count=1 -race -timeout 5m -overlay ... -run
  '^Test(CALV0065_NativeFixture|PSRV0013_PoolStatusMembers|TQDV0007_SlowLaneRecovery|TQDV0010_PluginFindings)$'
  ./internal/tasks/cli/`: 3 FAIL, 1 PASS (exit 1, 14.006s). CALV0065 and PSRV0013 repeat the
  `/bin/ps` refusal, TQDV0007 repeats the recovery assertion failure, and TQDV0010 passes on
  the quieter rerun. The remaining failures and signal-fixture race are retained without
  claiming a full baseline qualification. Log:
  `/private/tmp/claude-501/gotmp/v1-1126-baseline-failures.log`.
- Initial loaded command omitted the repository's 30-minute timeout: 37 repetitions started,
  36 PASS, 0 test FAIL, 0 SKIP, one interrupted; package interrupted at 393.081s (exit 130).
  At its measured pace it would exceed Go's default ten-minute timeout, so it was stopped
  through the owned execution session and retained separately, not counted as qualification.
- Final loaded command: `go test -count=100 -race -timeout 30m
  -run TestATRV0008_DetachedRunSurvivesItsLauncher ./internal/tasks/cli/ -json`: PASS (exit 0),
  **100 repetitions started, 100 PASS, 0 FAIL, 0 SKIP**, package duration **903.361s**
  (JSON package elapsed 903.362s). The live-writer test explicitly restores production patience
  because the fixture package otherwise disables it. The run overlapped the full CLI race suite
  and doc gates on the loaded host; no additional `-cpu` values were requested.
  Raw JSON is retained at `/private/tmp/claude-501/gotmp/v1-1126-loaded.jsonl`; the interrupted
  initial run is `/private/tmp/claude-501/gotmp/v1-1126-loaded-initial.jsonl`.
- `make -s -k spec-requirements-check requirement-definitions-check traceability-tests-check
  decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check
  use-case-receipts-check use-case-receipts-test`: PASS (exit 0); the spec-index test passed
  (initial 0.497s; final isolated-index rerun 0.302s), all 18 receipts were current,
  and five repinning test cases passed. Final log:
  `/private/tmp/claude-501/gotmp/v1-1126-doc-gates-final.log`.
- `sh script/repin-use-case-receipts.sh --check`: PASS, all 18 receipts current;
  `cmd/corvint/main.go` was not changed.
- After staging the proposed spec in the private index, `make -s spec-requirements`: PASS;
  generated one new CTS-V0-009 row in `docs/specs/REQUIREMENTS.tsv`.
- Independent native read-only review: no findings; reviewed the read budget, link/rename
  synchronization, persistent-redo envelope, no-write checks and live-test patience restoration.

All Go commands use `GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`, and
`GOMAXPROCS=3`. No `make gate`, repository-wide `./...`, commit, push, decision file, or
`.taskman` write is part of this builder slice.

## Retained limits

Corvint pre-change query succeeded; its supplied context packet and original source guided
this change. The affected plan succeeded after a first drift refusal, but scope remains
`UNKNOWN`; whole-repository fallback recommendations are NOT_RUN under the owner's focused
test scope. Index refresh failed because the common Git directory is outside writable roots.
`make dogfood-change BASE=71b2eb8fc3e56a9df368b8be8227bf5ad125f8c9` likewise failed creating
its private Git-directory output. CEM binding/seal is NOT_PRODUCED: this builder cannot
commit, and post-commit gates remain coordinator-owned.

Normal spec staging failed on the worktree's external `index.lock`. Spec generation and
index-based doc gates instead use a copied private index and scratch object directory under
the authorized TMPDIR, with existing objects as a read-only alternate. The real repository
index is unchanged. A doc-test Git fixture replaced the first scratch index; final gates
  are rerun after rebuilding it from HEAD, with a scratch Git shim limiting indexed reads
  to this worktree and leaving fixture Git repositories on their own indexes. The final
  staged diff contains only the six task files. A scratch `mktemp` shim routes no-argument calls into TMPDIR because
Darwin's default no-argument `mktemp` otherwise used an unwritable `/var/folders` directory.

Installed Core is `1.0.0-rc.3 (build 407)`; installed Tasks is
`0.0.0-tcp01-unverified+build.407`. Official release listing observed `v1.0.0-rc.2` and
`tasks-dev-20260929.2` as newest listed component releases. Both optional updater checks failed
their version smoke with `process-after-start-failed: descendant snapshot unavailable`;
freshness is UNKNOWN, no verification was bypassed and no tool was replaced. Account usage
and billed cost telemetry are NOT_OBSERVED. Native receipt audit was read-only and reported
`OK`, `CONSISTENT`, `AGREES`; ticket completion and filing tooling blockers are intentionally
left to the coordinator because this slice forbids writing `.taskman`.
