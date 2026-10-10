# V1-1096: test-level slices of oversized packages across CI shards

Date: 2026-10-10

## Problem

AFP-V0-022 partitions whole packages across the six full-CI shards, so the slowest package sets a
floor under the slowest shard. Main run
[38055182050](https://github.com/beamfall/corvint/actions/runs/38055182050) tested
`e0ef9aa891edbd84b19b1f30f12bc6ed13635a96`. Its 320 terminal package outcomes sum to 7,684.8 s,
which gives an ideal share of 1,280.8 s per shard. `internal/tasks/store` alone took 1,871.8 s,
and its shard summed to 2,100.8 s. `cmd/corvint` took 764.6 s, which is below the ideal share.

## Decision

The new requirement is AFP-V0-041 in `docs/specs/affected-plan-v0.md`. It was proposed as
experimental. The owner accepted it in chat on 2026-10-10 ("accept AFP-V0-041", relayed by the
coordinating agent), recorded as decision 0488.

**Exactly once.** A split package runs as K invocations:

- K-1 named slices, each `-run '^(A|B|...)$'` over plain top-level names;
- one catch-all, `-skip` over the anchored union of every named slice.

`go test` splits a pattern on `/`, and on `|` outside parentheses. Grouping the alternation keeps
it one element, and a one-element pattern matches only top-level names, so subtests always run
with their parent. The anchors keep `TestA` from matching `TestAB`. The same flags filter examples
and fuzz seeds. `-list` also prints benchmarks, so the generator drops them.

What happens to tests that change after generation:

- a test added or renamed after generation runs once, in the catch-all;
- a removed name matches nothing.

`TestAFPV0041SlicesRunEveryTestExactlyOnce` runs real slices over a fixture module containing
`TestMain`, subtests, a parallel test, a fuzz seed, an example, a benchmark, and added, removed and
renamed names. With the end anchor removed, it fails with "TestAB ran 2 times across slices".

**Authority and fallbacks.** A package splits only when both of these hold:

- the project-owned allow-list `.github/cishards/test-split-allow.json` names it, with a reason;
- the slice file `.github/cishards/test-slices.json` splits it validly.

The two allowed packages are `cmd/corvint` and `internal/tasks/store`. Their `TestMain` functions
change only their own process (environment, re-exec roles, `sync.Once` roots), and no test is known
to depend on order. Each of the following keeps packages whole:

- an unusable file;
- an unlisted or unknown package;
- an invalid name or cost;
- a pattern over 64 KiB;
- invalid package costs.

None of them drops a test.

**Placement.** Units are placed longest first: whole packages at their AFP-V0-022 cost, and slices
at their recorded cost. Each unit goes to the least-loaded shard, and slices avoid shards that
already hold a sibling. Without a split, the plan equals `Partition`, and
`TestAFPV0041PlanEqualsPartitionWithoutSplits` checks this on 200 random universes.

**Identity.** The slice code, the allow-list and the slice file are copied into the protected helper
module and covered by its digest. The digest domain is now `corvint-ci-partition/2`. The digest is
part of the AFP-V0-014 frozen identity. The package-level PR driver cannot run slices, so it refuses
sharded execution while any package of the runtime universe is split. The workflow's driver path
does not fall back, so the refusal fails the shard closed. A pinned older driver fails on the
digest mismatch.

**Workflow.** The FULL path runs whole packages in one invocation as before. It then runs each
slice from `ci-shards --slices` as its own `go test`, with the same `-exec` confinement and
`-json -p 1 -count=1 -race -timeout 50m`. The shard fails if any invocation fails, before the
AFP-V0-024 tested-tree record. The CI-side change is small:

- the helper `cp` line gains the four slice files;
- the test step gains the slice loop.

A parallel lane wraps the test command in a `run_tests` function with `tee`. On merge, the slice
loop belongs inside that function.

## Generation

The generator is `tools/ci-test-slices generate`. It accepts only a complete, passing run:

- every package that appears ends in a pass or skip;
- every test or subtest that starts ends;
- no test, subtest or package fails.

The revision must be a full commit id of the repository. The generator checks out that commit's
tracked tree into a temporary directory through a temporary index, then reads the allow-list and
enumerates there. Untracked, modified or staged files therefore cannot contribute, and the
repository's index, worktrees and refs are not written. It enumerates with
`go test -race -count=1 -list .`, without workspace or `GOFLAGS` redirection, and splits each
allowed package that is slower than the target. The target defaults to the ideal share. Named
slices minimise the largest slice over sorted names, weighted by observed top-level time; a name
the run lacks takes the package median.

The committed file was generated at `1f68f622e8ee3af9cd39a57696881e2e9b18c7ac` (the implementation
commit), from run 38055182050, on darwin/arm64 with go1.27.1:

- `cmd/corvint` stays whole: 764.6 s is within the 1,280.8 s target.
- `internal/tasks/store` splits in two:
  - named slice: 143 names, 920.5 s planned;
  - catch-all: 951.4 s planned.

Enumeration listed 361 store tests and 7 benchmarks. The run had 359 top-level outcomes. The two
names the run lacks (`TestCALV0026_InPlaceIntentEditAfterCommitSweep`,
`TestCALV0026_LeaseCommitSweepsOutsideWriterLock`) were added after `e0ef9aa8` and took the median
weight.

## Replay of run 38055182050

`tools/ci-test-slices replay` places the run's 320 packages with and without the slice file. Whole
packages count at their observed time. A slice counts at its package's observed time, scaled by its
tests' share of the package's summed top-level test time. That scaling is an inference.
`internal/tasks/store` runs its tests serially (top-level sum about 1,870.7 s against 1,871.8 s
elapsed), so the share is close for it. `cmd/corvint` runs in parallel (2,485.8 s summed against
764.6 s elapsed), so a share of it is less reliable.

| Placement costs | Slices | Per-shard sums (s) | Max (s) |
|---|---|---|---|
| (observed hosted run) | none | 1113.2 1148.7 1145.4 2100.8 1312.8 863.9 | 2100.8 |
| committed `package-costs.json` | none | 2100.8 1113.2 1148.7 863.9 1145.4 1312.8 | 2100.8 |
| committed `package-costs.json` | committed (store x2) | 1266.5 1261.2 1291.7 1236.0 1162.5 1466.9 | 1466.9 |
| observed run as costs | none | 1871.8 1162.8 1162.8 1162.8 1161.8 1162.8 | 1871.8 |
| observed run as costs | committed (store x2) | 1282.3 1280.3 1281.3 1280.3 1280.3 1280.3 | 1282.3 |
| committed `package-costs.json` | `--target 400s` (store x5, cmd/corvint x2) | 1348.9 1169.7 1244.8 1362.8 1342.5 1216.0 | 1362.8 |
| observed run as costs | `--target 400s` | 1280.8 1281.2 1281.2 1281.2 1281.2 1279.3 | 1281.2 |

With the committed cost table and no slices, replay reproduces the observed per-shard sums exactly,
as a permutation. With the committed slices, the predicted maximum falls from 2,100.8 s to
1,466.9 s on the committed table, and to 1,282.3 s once the table matches the run. The residual
above the ideal share on the committed table comes from cost-table drift, not from a package floor.

`--target 400s` gains about 104 s on the committed table and nothing on a current one. It also
multiplies the per-slice overhead and loses `cmd/corvint`'s cross-test parallelism, so the
committed file keeps the default target.

## Local slice execution

The two store slices that the built helper prints for shards 0 and 1 ran through the workflow's
loop on darwin, without the Linux confinement wrapper, as `go test -json -p 1 -count=1 -race`:

| Shard | Slice | Tests | Hosted sum of the same tests (s) | Predicted (s) | Local wall (s) | rc |
|---|---|---|---|---|---|---|
| 0 | `-skip` catch-all | 218 | 951.8 | 952.4 | 1,885 | 1 |
| 1 | `-run` named | 143 | 918.9 (141 with hosted times) | 919.5 | 1,253 | 0 |

`go test -list` printed 361 tests and 7 benchmarks. The two outputs hold exactly one terminal
top-level outcome for each of the 361 tests (350 pass, 10 skip, 1 fail):

- none is missing, duplicated or extra;
- every named test ran in the named slice, and every other test ran in the catch-all;
- no benchmark ran.

The one failure was `TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage` ("turns 5, want 4"), in
the catch-all. It took 185.1 s there, against 42.9 s on the hosted run. Run alone with the same
flags, it passed in 49.2 s. It builds its own fixture, and its assertion counts supervisor turns
against a stage wall clock. The machine had 12 cores at a load average of 35 to 63, with other
lanes running this package's tests at the same time. This failure is therefore read as load-timing
sensitivity, not as a slicing defect; that reading is an inference. Local wall times are not hosted
evidence.

## Review repairs

### Round 1

An independent Codex review of the first two commits found three defects. All three are fixed in
a follow-up commit:

- **(P2) Incomplete or failed logs passed.** Subtest events were dropped before the failure
  check, and a package needed only one terminal outcome. Failures are now checked before any
  filtering. Every started package must reach a terminal pass or skip, and every started test
  must end. `TestAFPV0041ObserveRefusesIncompleteOrFailedLogs` covers a failed subtest, a failed
  package, a truncated package and a test that never ends.
- **(P2) Enumeration was not bound to the revision.** The old clean-tree check ignored untracked
  files. Generation now uses the temporary checkout described above.
  `TestAFPV0041GenerateEnumeratesOnlyTheRevision` adds an untracked test file, a staged
  modification and a working-tree allow-list that drops the package. It checks that:
  - only the committed test is sliced;
  - the checkout lies outside the repository and is removed;
  - status, the staged index, the worktree list and the refs are unchanged.
- **(P3) Spec wording.** The complete-universe sentence now says "every unsplit package".

Each new check was tested by mutation. Seven mutants each fail one of the two tests:

- removing any of the three refusals;
- pointing allow-list reading, enumeration or the index back at the repository;
- dropping the removal of the temporary checkout.

The six hosted logs still pass the new checks, with the same replay sums. Regenerating at
`1f68f622` through the temporary checkout reproduces the committed `test-slices.json` byte for
byte.

### Round 2

A second Codex review, of `afe0f8df`, found three more defects:

- **(P2) Incomplete logs still passed.**
  - The sequence start, pass, start, end of file was accepted, because a second lifecycle was never
    refused.
  - A damaged JSON record was silently skipped.
  - The reader now tracks each package's lifecycle in a log: its first record must be `start`, and
    no record may follow its terminal pass or skip. It also refuses a second start, a test that
    runs twice or ends without running, and a test still running when its package ends.
  - A line that carries a `go test -json` record (`{"Time":`, `{"Action":` or `{"ImportPath":`
    after any log prefix) must decode completely. Other lines are runner, shell or build output.
    That includes a printf format string beginning `{"profile":` in the workflow's shell output.
- **(P3) An empty generation skipped source validation.** When nothing split, a bad or missing
  `--run-url` was written. The file-level check is now `cishards.SliceFileUsable`, shared with
  `SplitPackages`. It runs before the checkout and again before the write.
- **(P3) An interruption leaked the temporary checkout.** `main` now runs under
  `signal.NotifyContext` for SIGINT and SIGTERM.
  - Child processes get an interrupt on cancellation and are killed after ten seconds.
  - A cancelled run refuses the write, because a cancelled enumeration otherwise looks like a
    failed one and would keep packages whole.
  - `TestAFPV0041InterruptRemovesTheCheckout` signals a real helper process blocked in enumeration
    with each signal. It checks exit code 2, an empty temporary directory and an unwritten slice
    file.
  - SIGKILL cannot be handled, so it still leaks the checkout under `TMPDIR`.

The six hosted logs pass the stricter reader:

- every package starts with `start`;
- no record follows a terminal outcome;
- no test ends without running.

Replay sums and the byte-identical regeneration at `1f68f622` are unchanged. Eleven mutants each
fail a test:

- removing the signal context, the cancellation refusal or the early source check;
- removing the after-outcome, before-start, twice-started, damaged-record, ran-twice,
  ended-without-running or running-at-close refusal.

### Main merge and cost aggregation

Main gained the AFP-V0-040 shard-cost capture and drift job (decision 0487) while this lane was
open. The merge at `c8029870` integrates the two:

- **One capture for every invocation.** The tests step now defines `run_tests`, which runs the
  whole packages and then each slice, each with `< /dev/null`, and returns failure when any
  invocation failed. Main's capture block is unchanged: it pipes `run_tests` through `tee`, takes
  `PIPESTATUS[0]`, and sets `costs=1` only on success; without a capture directory it runs
  `run_tests` uncaptured. The cache restore and save steps are main's, byte for byte.
- **Semantic conflict in `go-static`.** Main's copy of the protected helper listed only
  `partition.go`, `order.go` and `package-costs.json`. With this lane's `//go:embed` that build
  fails with `partition.go:23:34: pattern slices.go: no matching files found`. The copy now lists
  the same files as the shard job's. An isolated replica of the step passes `go test`, `go vet` and
  the windows build.
- **Slice outcomes combine (AFP-V0-041 (8)).** `tools/ci-shard-costs` reads the committed
  allow-list and slice file (`--allow`, `--slices`) and calls `cishards.SplitPackages` over the
  logs' packages with the log count as the shard count, so it splits exactly what the partition
  split. Each log may end a package once. A split package must end exactly once per slice; its
  cost is the sum (round 3 removed a single-outcome exception, below). A failed slice, an
  unterminated slice, a slice count that differs from the file, or a repeat of an unsplit package
  refuses the refresh and makes the advisory check abstain.
  `TestAFPV0041RefreshCombinesTestSlices` and `TestAFPV0041AdvisoryCombinesTestSlices` cover both
  paths. Seven mutants each fail a test:
  dropping the per-log repeat check, the slice-count check, the unterminated-slice check or the
  unsplit-repeat check; a shard count of 1; assigning instead of summing; and an empty allow-list
  path in the advisory form.
- **End to end.** The workflow's `run_tests` and capture block, extracted verbatim, ran three
  shards locally with real `go test -json` (a shim drops `-exec`). Two shards ran the two slices
  of `tools/ci-shard-costs` and exited 0 with `costs=1`. A third ran both slices of a fixture
  package whose first slice fails: it exited 1 with no `costs` output, and its second slice still
  ran. Given a fixture allow-list and slice file naming that package, `refresh` over the two
  passing captures wrote 2,695 ms, the sum of 1,231 ms and 1,464 ms, and `check --advisory
  --shards 2` reported normally. Without the slice file the same logs abstained with "two
  terminal outcomes".

### Round 3

The independent review of `0d534a03` found three gaps; each fix is mutation-checked.

- **Exact slice count (P2).** `combine` accepted one terminal outcome for a split package as a
  run made before the split, so a sliced run that lost a slice's log still refreshed costs. A
  package the committed slice file splits must now end exactly once per slice in the run; any
  other count refuses refresh and makes the advisory check abstain. A run made before the split
  therefore cannot refresh a split package's cost (AFP-V0-041 (8), Limits).
  `tools/ci-test-slices` keeps its own whole-run reader and is unaffected. Two cases join each of
  `TestAFPV0041RefreshCombinesTestSlices` and `TestAFPV0041AdvisoryCombinesTestSlices`: a
  pre-split whole run and a run missing its second slice. Restoring the old rule (`n > 1 && n !=
  k`) fails all four.
- **Truncated records (P2).** `tools/ci-test-slices` skipped any line without the
  `{"Time":`, `{"Action":` or `{"ImportPath":` key prefix, so a completed package followed by a
  truncated `{"Time"` or `{"Act` was accepted. The reader now strips the hosted prefix (gh job and
  step columns, the byte-order mark, the runner timestamp and a `##[error]`-style annotation).
  Content that starts with `{` must decode completely and name its `Action`, and a record after
  other text on its line refuses. Only non-JSON lines are skipped. Eight truncation and
  interleaving cases join `TestAFPV0041ObserveRefusesIncompleteOrFailedLogs`, along with a
  positive case that wraps the passing log in every hosted prefix; the old reader accepts seven of
  the eight. Under the strict reader, the six retained hosted shard logs still replay to the
  observed, whole and sliced sums in the table above; one log has 327 `##[error]` record lines.
- **Process-group cancellation (P3).** Cancellation interrupted only the immediate `go` (or
  `git`) process, and Go's WaitDelay kill also reaches only that process. Every subprocess now
  starts in its own process group (`inGroup`, build-tagged `procgroup_unix.go`). Cancel sends
  SIGINT to the group, waits up to `stopGrace` (10 s) for it to empty, sends SIGKILL to the group,
  and returns only once the group is empty, so `Wait` cannot return while a descendant runs.
  `TestAFPV0041CancelStopsTheEnumerationGroup` puts a fake `go` on `PATH` whose `sleep 300` child
  ignores SIGINT and holds the output pipe, in two variants: an interruptible leader and a leader
  that traps SIGINT. It checks that the group and the grandchild are gone. Both variants fail
  under the old leader-only interrupt and under a group interrupt without the SIGKILL step. On
  other platforms `procgroup_other.go` kills only the immediate process; `GOOS=windows go vet`
  and a windows test build pass.

### Round 4

The independent review of `732433c4` found two P2 gaps; each fix is mutation-checked.

- **Declared shard and slice counts (P2).** `refresh` and `check` passed the number of logs
  supplied to `SplitPackages`. With fewer logs than slices, the split package fell back to whole,
  so one surviving slice was accepted as the whole package's cost. Every mode now takes the CI
  shard count as `--shards N` (the go-product-shard matrix, as `check --advisory` already did) and
  refuses unless exactly N logs are given. `combine` splits with N, never the log count. It also
  refuses an allowed package that the slice file splits into more slices than N, so no
  declaration can count one slice as whole (AFP-V0-041 (8), Limits). Four cases join each of
  `TestAFPV0041RefreshCombinesTestSlices` (now run through `refresh` and `check`) and
  `TestAFPV0041AdvisoryCombinesTestSlices`:
  - one log of two shards;
  - one log declared as one shard;
  - two logs of a three-slice package declared as three shards;
  - the same two logs declared as two shards.
  Dropping the log-count refusal, dropping the slice-file refusal, or splitting with a one-shard
  count each fails a case. The AFP-V0-022 calls declare `--shards`. The partial-refresh case
  declares one shard so that it still reaches the stale-package refusal. Against the six retained
  pre-split hosted logs, `check --shards 6` refuses the split store package ("1 terminal outcomes
  for its 2 test slices"), and `check` without `--shards` refuses.
- **Package-less failures (P2).** `tools/ci-test-slices` skipped every record without `Package`,
  so a completed passing package followed by `{"Action":"build-fail","ImportPath":...}` and EOF
  was accepted. Before that skip, the reader now refuses `build-fail` ("build of ... failed"), any
  other action without `Package`, and build output without `ImportPath`, mirroring
  `ci-shard-costs`. Four cases join `TestAFPV0041ObserveRefusesIncompleteOrFailedLogs`, one of
  them that exact truncated stream; the passing case gains a `build-output` record. The pre-fix
  skip accepts all four. The six retained hosted logs hold no build record and still replay
  unchanged.

### Round 5

The independent review of `40bafcea` found one P2; the fix is mutation-checked.

- **Partial run in `ci-test-slices` (P2).** `generate` and `replay` needed only one log, whatever
  `--shards` said. A surviving shard whose package lifecycles were complete passed `observe`, and
  `generate` then set its target from the partial total and rewrote the slice file. Both modes
  now refuse before any work unless exactly `--shards` logs are given ("K of N shard logs
  present", as in `ci-shard-costs`). Each log must also hold a terminal package outcome, so an
  empty placeholder cannot stand in for a missing shard (AFP-V0-041 (5)). `replay` now always
  prints the observed per-shard sums; it no longer places a run under a different shard count.
  The test fixture became a two-shard run, with package `other` in shard 1, so the target, the
  split and the predicted sums are unchanged:
  - `TestAFPV0041GenerateSplitsOnlySlowListablePackages`, which generated from one log under
    `--shards 2`, now first refuses that partial run and leaves the slice file unwritten.
  - `TestAFPV0041GenerateRefusesUnboundInputs` gains cases for a missing log, the default six
    shards with two logs, an extra log, an empty shard log, and the same missing and empty cases
    under `replay`.
  Dropping the count refusal, or the empty-log refusal, each fails a case. The six retained hosted
  logs replay with the same sums under the default `--shards 6`; five of them refuse.

## Limits and integration

- **Lost interleavings.** Splitting loses cross-slice parallel interleavings, including race
  detection between tests in different slices.
- **Per-slice overhead.** Every slice repeats the package's build or link, process start and
  `TestMain`. This overhead is NOT_OBSERVED on hosted runners.
- **Costs and slices need different runs.** `tools/ci-shard-costs` measures a split package only
  from a sliced run and refuses a run made before the split. `tools/ci-test-slices generate` reads
  only whole runs and refuses a sliced one ("two terminal outcomes"), so regenerating the slice
  file needs a complete run in which the package ran whole. Full CI does not make one while the
  package is split; a run of a branch whose slice file is emptied would.
- **Stale slice file.** A stale slice file can only move time between shards.
- **Declared shard count.** `--shards` is declared in both tools, not read from the workflow. In
  `ci-shard-costs`, a count that differs from the matrix refuses a sliced run or measures only the
  logs given, but never counts one slice as a whole package. `ci-test-slices` has no cost table to
  compare, so a partial log set declared as its own shard count (five logs of a six-shard run as
  `--shards 5`) is not detected there.
- **Cancellation reach.** On Windows, cancellation kills only the immediate subprocess, not its
  descendants. On unix, a descendant that leaves its process group is not reached, and a SIGKILL
  of `ci-test-slices` itself stops nothing and leaves the temporary checkout behind.
- **Selective PR pins.** The workflow's selective PR pins (`CORVINT_PR_TOOL_SOURCE` and the
  qualification pins) are empty, so every run takes the FULL path that runs slices. If those pins
  are set while a package is split, the driver's refusal fails every sharded PR run closed. Enabling
  them therefore first needs an empty slice file, or a driver that runs slices.

## Rollback

Choose either:

- set `packages` to `{}` in `.github/cishards/test-slices.json`, after which the plan equals
  AFP-V0-022 and the driver accepts sharding again;
- revert the workflow slice loop and the helper files.

## Not run

- A hosted sliced run.
- Paired hosted timings.
- Per-slice overhead.
- Linux enumeration (darwin was used).
- A hosted run of the merged `ci.yml` (actionlint and the local slice-loop run only).
- `make gate`.
