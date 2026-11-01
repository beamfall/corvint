# Lane internal-wsl (wave 6, WSL), 2026-10-31

Base `origin/main` 8daec98ce4349c1583186692726d2c3a782ac647, branch `claude/lane-internal-wsl-1031`.
Requirement IDs used from the lane block 970-979: AHI-970, AFP-V0-970, CAP-V0-970, SRG-V0-970,
PRB-V0-970 and PRB-V0-971. Load-flake and process-lifetime tickets ran in two forked worktrees of
this lane, and their commits were cherry-picked here.

## Tickets

- **V1-2635 (delivered).** 34 declared packages that spawn git in tests now isolate Git
  configuration in `TestMain`: 31 get an `isolate_git_test.go` calling `testsupport.IsolateGit`;
  `internal/taskman` calls it in its unix `TestMain` and adds a `!(darwin || linux)` one;
  `conformance/go-live-test-v0` imports no Corvint package, so it sets the same list inline.
  Every one passes with `GIT_CONFIG_GLOBAL` set to a directory, one package per command (34/34
  rc 0). `internal/changee2e` is excluded: its `TestMain` is in a Mac lane's files; it needs one
  `testsupport.IsolateGit()` line there. Identity comes from `IsolateGit`, never the account.
  Scan (run from the repository root; prints packages still unisolated):

  ```python
  import json,os,re,glob,sys
  root=sys.argv[1]
  pk=json.load(open(os.path.join(root,'.corvint/test-read-scopes.json')))['packages']
  spawn=re.compile(r'exec\.Command(Context)?\([^)]*"git"')
  for p in sorted(pk):
      n=0; iso=False
      for f in glob.glob(os.path.join(root,p,'*_test.go')):
          s=open(f).read(); n+=len(spawn.findall(s))
          if ('GIT_CONFIG_GLOBAL' in s and 'TestMain' in s) or 'IsolateGit()' in s: iso=True
      if n and not iso: print(n,p)
  ```

  Before this change the scan listed 35 packages (the ticket's 50 were partly isolated on base by
  the "Corvint 1.0" commit); after it, only `internal/changee2e`.
- **V1-3358 (delivered).** New `internal/jsonend.Done` checks that the next token is `io.EOF`.
  Every single-value strict decode that used `More()` now uses it (23 sites in 22 files). The tasks
  packages and `internal/analyzernativebridge` cannot import a Core package, so they check
  `io.EOF` inline. `internal/tasks/store/pool_identity.go` and `internal/releasegate` already had
  an EOF check after `More()`, so they are unchanged. Negative tests: `internal/jsonend`, the two
  `cmd/corvint-corpus-mcp` decoders (fail before the fix), and `decodeJSONObject` and
  `decodeNegateRefusal` in `cmd/corvint`.
- **V1-3255 (delivered, AHI-970).** The dogfood `dogfood-event-unavailable` notice and the
  PostCompact pin notices (`not-emitted`, `not-preserved`, `revision-unavailable`,
  `verification-unavailable`) keep their frame line and append a cause line and a read-only argv,
  `corvint --root ROOT query --task "Corvint hook check"`. Decision: the hook cannot see the
  typed error behind the closed code, so the recovery is the read that shows it, not a guessed
  cause. PreCompact output goes to the compactor, not the user, so the visible warning is the
  PostCompact one.
- **V1-3321 (delivered).** `MigrationAuthority.ProbeBatch` plus an `objectProbe` in
  `internal/tracemigraterepo`: one `git cat-file --batch` session per migration answers object
  types, commit trees and tree listings, and one `git rev-list --stdin ^HEAD <commits>` replaces
  every `merge-base --is-ancestor`. Each answer still reserves one operation of the migration's
  `gitrun.Budget`. A record the session cannot answer replays the one-shot command under the same
  reservation. `TestLTPMV0517StoreAtMaximumBeyondTheWindow` takes 15.35 s on base and 1.47 s after,
  on WSL (32 cores). The Mac was not measured: the ticket's 85 s was a Mac figure, so the under
  15 s criterion on the Mac is inferred from the 10x ratio, not observed.
  `TestBatchedProbeMatchesOneShotProbes` shows identical dry-run and apply results, inside and
  beyond the replay window. `TestBatchedProbeChargesTheOperationBudget` shows the session path
  refuses past the budget.
- **V1-2610 (delivered, AFP-V0-970).** The graph cache directory is capped at 32 entries. After
  each write, the oldest entries are evicted (by mtime, then name) through the directory handle,
  and only regular `affected-graph-*.json` files are removed. `affected cache` reports
  `entryLimit`. Decision: an entry cap, not a byte cap or root-existence pruning, because pruning
  needs each multi-MB entry opened and a cap needs only a listing. Smallest change in the affected
  lane's files: one call in `writeGraphCache` and one status field.
- **V1-3286 (delivered, CAP-V0-970).** `clitree.Synopses` skips `Example:`/`Examples:` blocks
  and lists labelled usage forms first. A strict "usage block only" rule was tried first and
  rejected: it dropped 44 real nodes whose synopses sit in titled groups outside Usage, such as
  `changes check-shared`. The regenerated snapshot drops only example lines, and `repro record`
  loses a wrong `--path` flag and gains its true required inputs.
- **V1-2830 (delivered, SRG-V0-970).** The one rule already existed: SRG-V0-011 and SRG-V0-980
  hold status only in headers, and the digest contains the header value verbatim. It is now
  stated in `docs/SPEC-DRIVEN-DEVELOPMENT.md`. The drift refusal names the expected
  `- Status: INTENT / DELIVERY` line and where a clause goes instead. Existing specs pass
  unchanged.
- **V1-3418 (delivered, PRB-V0-970..971).** The new `--scenario ID` flag can repeat. It judges
  only the selected scenarios. With `--record-baseline` it replaces only those baseline
  measurements, each with PRB-V0-053 provenance. The new `--record-baseline-from REPORT` flag
  records a report written with `--out` on a quiet host. That is the documented path when the
  local host is contended.
- **V1-1498 (ALREADY-DELIVERED, plus survey).** On base, `exitWhenCaptureParentGone` in
  `internal/localcompletion/storage_test.go` already bounds the helper.
  `TestAggregateCaptureSignalHelperExitsWhenParentDies` SIGKILLs the parent and reran PASS. Added:
  shared `testsupport.ExitWhenParentGone`, which the procprofile signal helper now uses. Survey of
  74 self-exec test files: two helpers are still unbounded, both in other lanes' files, so they
  were reported to the orchestrator rather than fixed:
  - `cmd/corvint/local_completion_signal_unix_test.go`
  - `cmd/corvint-test-runner/fifo_unix_test.go`

  The others are bounded by timers or are deliberate reaping targets.
- **V1-3435 (delivered).** Four WaitDelay values were raised to 5 s, following CEM-V1-820:
  procgroup ps snapshot, darwin processidentity, release-artifact archive and beamfall-shadow. The
  testsupport held-pipe 200 ms bound is kept, with a comment saying it must expire. The new
  `TestDescendantSnapshotSurvivesBrieflyHeldPipe` holds the pipe 500 ms; the snapshot succeeds at
  5 s, and a negative control fails at 100 ms.
- **V1-2426 (NOT DELIVERED).** No test-hook analysis exists on base. Searched: `tools/`,
  `internal/testplan`, `internal/testsource`, `internal/gotestrun`, `cmd/corvint/test_*`, and
  every non-test file naming hooks, seams or setters. The perf-1022s build log shows the
  classification was that lane's manual review. There is nothing to fix until the owner names the
  analysis or asks for one to be built.
- **V1-1742 (BLOCKED).** Round-tripping `ID:PATH` link targets needs a repository-binding input
  on `flows export`, because `extevidence.Repository1` requires a revision. That means
  `internal/appflows` and the `cmd/corvint` flows dispatch, which a Mac lane in flight owns,
  plus doccorpus maintenance resolution. Reassign it to the appflows lane or schedule it after
  that lane lands.
- **V1-3427 (delivered, LOD-V0-970).** Reproduced on a store whose only member file holds one
  valid row and one malformed line, so no member is admitted.
  - The verifier emitted source `byteCount` "0", the digest of the empty `members`,
    `NOT_OBSERVED` and `UNKNOWN`, so `data.artifact.bytes` was a measured 0.
  - The producer emitted null bytes and digest, `OBSERVED` and `VALIDATED_AT`.
  - Decision: the producer's form is right. The frozen `four-invalid-legacy-members.json` fixture
    already records it. Invalid input is "never zero", so bytes and digest are null. The
    rejection itself was observed at the scan boundary, so the source stays `OBSERVED` and
    `VALIDATED_AT`.
  - Only the verifier's all-rejected branch changed; the producer is unchanged.
  - `TestProducerTraceConformanceV1InvalidStore_V13427` fails before the change and passes after.
    The decision-0548 authority dispute stays `NOT_PRODUCED`.
- **V1-1481 (ALREADY-DELIVERED).** The 3 s PID-handoff deadline is gone on base:
  `waitForProcessPID` and `requireDescendantGone` in `internal/doccompiler/process_unix_test.go`
  sync on the command returning, citing V1-1481. `-count=30` passed 30/30 at load 61→40, and
  30/30 with 45 busy loops at load 69→68.
- **V1-2255 (NOT REPRODUCED).** Each test ran 200 times with `-count=200`, with and without
  `-race`, plus 25 to 30 busy loops, all PASS:

  | Test | Mode | Load start → end | Result |
  |---|---|---|---|
  | `TestPGOV0019ShadowCancelKillsTheGroupBeforeTheReap` | plain | 103 → 115 | 200/200 |
  | same | `-race` | 118 → 77 | 200/200 |
  | `TestSTRV0060PathDigestRefusesSwap` | plain | 77 → 54 | 200/200 |
  | same | `-race` | 54 → 73 | 200/200 |

  Remaining timing dependency, a hypothesis only: the 250 ms `residueWindow` after a cancel in
  `tools/beamfall-shadow/process_unix.go`. A timing-free fix changes PGO-V0 behaviour, so it
  needs the spec owner. That file's 250 ms WaitDelay was raised to 5 s under V1-3435.
- **V1-1302 (ALREADY-DELIVERED).** `liftedSourceDeadline` in `cmd/corvint/source_handoff_test.go`
  (cites V1-1302) and V1-2258's `git-failed` classification removed the load-dependent
  `blob-budget`. A focused run passed: `-p 1 -count=20 -run 'TestSourceView|...'` with 25 busy
  loops, 318 s, load 24→51. The full `-p 1 ./cmd/corvint` run, about 591 s quiet, does not fit
  the lane's 600 s command limit, so it was not run.
- **V1-1523 (load-robust, no fix).** Every named test passes 5/5 with `-p 1 -count=5` under
  synthetic load. The 1-minute load reached the 2×-cores target of 64 only part of the time; the
  5-minute load was 58 to 85.

  | Test | Load start → end |
  |---|---|
  | postmergehost `TestPairedAuthorSourceConformance` | 71 → 53 |
  | postmergemetrics `TestMetricsGit` | 53 → 55 |
  | supervisor `TestSupervisorDescendantCleanup` and `TestCALV0077_DetachedServerHostCrash` | 50 → 56 |
  | tasks/cli `TestATRV0006_LostLeaseKillsTheRun` and `TestTQDV0010_PluginFindings` | 76 → 88 |
  | tasks/store `TestPoolOutputBoundRetiresGroup` | 104 → 95 |

## Friction

- `~/lanes/heavy.sh` runs `git rev-parse` itself. With `GIT_CONFIG_GLOBAL` pointing at a
  directory in the caller's environment it refuses ("no checkout at ..."). Repro:
  `GIT_CONFIG_GLOBAL=/tmp ~/lanes/heavy.sh true`. Workaround: pass the variable inside, as in
  `heavy.sh env GIT_CONFIG_GLOBAL=DIR go test ...`.
- A heavy.sh refusal piped through `grep -v '^heavy.sh'` under `pipefail` shows only rc 1 and no
  text, because the refusal lines all start with `heavy.sh:`. Repro: a whole-package `go test`
  without `HEAVY_WHOLE_REASON`, piped that way.
- Adding a `TestMain` changes every test in a package, so the V1-2635 acceptance needs
  whole-package runs. heavy.sh demands a `HEAVY_WHOLE_REASON` for each, and `corvint affected`
  gives no whole-package verdict for a `TestMain` change.
- `make windows-vet` failed with `no space left on device`: `/tmp` is a 16 GB tmpfs, and the
  shared `/tmp/corvint-go-build-cache` holds 13 GB of it. It passed with `TMPDIR` and
  `CORVINT_GOCACHE` moved under `~/.cache`. A stray `go build ./cmd/corvint` binary in the worktree
  root made `shell-foreign-callers-check` blame "corvint" for naming scripts, which reads like a
  real caller.
- `corvint affected --base` reported `scope=UNKNOWN selected=210`, so only focused tests were run.
- V1-2426 names an analysis that does not exist in the repository; the ticket body gives no path.
- V1-1742 was assigned to this internal lane, but it reopens a recorded AFU non-goal in
  Mac-lane-owned `internal/appflows`, the `cmd/corvint` flows CLI and doccorpus maintenance.
- heavy.sh's load gate (`HEAVY_MAX_LOAD=40`, waiting up to 120 s) stalls any deliberate load run,
  and its wait shows only in heavy.sh lines. Repro: start 40 busy loops, then run
  `timeout 180 ~/lanes/heavy.sh env GOTOOLCHAIN=local go test -count=5 -run X ./internal/postmergehost`;
  it is killed with no test output. Workaround: set `HEAVY_MAX_LOAD=1000` on that one command.
- There is no `gh` on the WSL box, so CI logs a flake ticket names cannot be read there
  (`gh run list`: command not found).
- `internal/tasks/service` `TestSERVICE500_*` fail on this WSL host on base 8daec98ce as well,
  with `UNSUPPORTED_FILESYSTEM ... an ancestor directory is writable by another user`. `/tmp` is
  world-writable and sticky, and a private `TMPDIR` under it still has `/tmp` as an ancestor.
  This is environmental, and the store's ancestor check gives no test override.
- Two self-exec test helpers still wait without a bound and need filing (V1-1498 survey):
  - `cmd/corvint/local_completion_signal_unix_test.go` (the finish-signal helper waits on
    `ctx.Done()`)
  - `cmd/corvint-test-runner/fifo_unix_test.go` (`TestFIFOHelperProcess`)

  Both can call `testsupport.ExitWhenParentGone`.
- `internal/changee2e` still spawns git without `IsolateGit`. Its `TestMain` in `prove_test.go`
  needs one line, which this lane left to the Mac lane that owns the package.
