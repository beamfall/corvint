# 2026-11-04 floor: executor build configuration, range privacy and standard input

The review of `84c54e769` found two P1 gaps and one P2 in V1-2744's experimental typed test
narrowing. Each fix lands with a regression test that failed before it. AFP-V0-2371, AFP-V0-2372,
AFP-V0-2374 and AFP-V0-2376 are amended in place; no requirement IDs are added.

## Decisions

- **AFP-V0-2374, range assignment:** `assignedElsewhere` did not count a `range` clause key or
  value as an assignment.
  - Failure: a local path assigned from `t.TempDir()` and then reassigned by `range` to a
    shared path stayed private, so a later reader of the shared file was dropped.
  - Fix: a range key or value ends privacy.
- **AFP-V0-2374, standard input:** `fmt.Scan`, `Scanf` and `Scanln` recorded only an `os.Stdin`
  read.
  - Failure: when `TestMain` points `os.Stdin` at a file that a kept test writes, a dropped
    reader was not kept.
  - Fix: these calls, and the functions taken as values, read the `files` pseudo variable
    (`streamInput`). `Fscan*` on `os.Stdin` was already `unknown` through its `io.Reader`
    argument.
- **AFP-V0-2372, build constraints:** the unconditional refusal of constrained source is replaced.
  - Constraints are now evaluated under the executor's exact build configuration, read with
    `go env GOFLAGS` and `go list`'s `context`.
  - Both commands run under the same environment and `go test` flags the runner uses
    (`sessiontest.GoExecutor`).
  - The configuration identity is bound into every narrowed reason. The executing shard
    recomputes the plan, so a run under another configuration refuses the supplied manifest.
  - A flag the model does not classify, a `GOFLAGS` entry it does not classify (including one
    written by `go env -w`), a non-`gc` compiler or an unreadable probe keeps the package whole.
  - A changed file whose constraint lines differ between base and head refuses with
    `BASE_SOURCE_UNPROVED`.
  - `typedreach.Load` without a configuration still refuses constrained source.
  - Rejected alternative: keep the refusal. It makes every real package whole, because the
    module's own `_unix`/`_windows` files sit in every test binary's closure.

## Evidence

- `TestAFPV02374RangeAssignmentEndsPrivacy` (value, key) and `TestAFPV02374ScanReadsFiles`
  (`fmt.Scan`, `fmt.Scanf`, a `fmt.Scanln` value) failed before the fix and pass after it.
  - The `fmt.Fscan` stdin guard subtest passed before as well: it is existing coverage.
- Before the configuration work, `TestAFPV02372ExecutorConfigSelectsConstrainedSource`,
  `TestAFPV02376BaseConstraintChangeRefuses`, `TestAFPV02372BuildFlagsClassifiesExecutorFlags`
  and `TestAFPV02372ProbeConfigReadsTheExecutorEnvironment` did not compile.
  - The probe test covers `CGO_ENABLED`, `-tags`, `GOFLAGS`, `go env -w` `GOFLAGS`, `-race` and
    `GOAMD64`.
- `TestAFPV02372DryRunBindsExecutorBuildConfiguration` stayed whole with the build-constrained
  refusal at `84c54e769`.
- `TestAFPV02372RunRefusesAnotherConfiguration` fails when the identity is replaced by a constant:
  the shard accepted a plan made without the run's `-tags`.

## V1-2744 acceptance measurement

`corvint test --dry-run` (the product path, not the measurement tool) was swept over 44
first-parent commits from `f811cbfaf` to `6a2c582ef` touching `internal/tasks/**`.

- **One package narrowed:** `internal/tasks/store` at `5c5fe54a4` (`internal/tasks/similar/terms.go`).
  - 669 of 688 test functions kept, 19 dropped.
  - Configuration `sha256:0a64eebf2333ef49`.
  - Kept codes: mostly `STARTS_PROCESS` (620) and `READS_STATE_A_KEPT_TEST_WRITES` (43).
- **Fault oracle at `5c5fe54a4`** (`tools/typed-reach-measure -oracle -oracle-dropped-only`):
  - Faults were injected into `bodyScore`, `fieldScore`, `idf`, `lexicalTermCounts` and
    `stopWord`, and the 19 dropped tests were run.
  - No dropped test observed a fault, so none was missed.
  - Recall over kept observers is 0/0. Observability by a kept test was not measured.
- **Everything else stayed whole.** `internal/tasks/cli` and `cmd/corvint` never narrow. The reasons:
  - unbounded repository reads
  - `IMPORT_GRAPH_READER`
  - `DIRECT_SOURCE_CHANGE`
  - `ALWAYS_RUN_REACHES_CHANGE`
  - `BASE_MAY_WRITE_SHARED_STATE`
  - possible embedding
  - an unindexed source path, at `5c31ad86e`
- **Measurement tool caveat:** the tool also reported store `SLICED` at `50dfb87d9` (19 of 682).
  The product does not select store there, so that run is not evidence.

## V1-2744 acceptance verdict

| Criterion | Verdict | Reason |
| --- | --- | --- |
| AC0 | **NOT MET** as worded | The three V1-2708 repros remain `NOT_OBSERVED`. One real allow-listed narrowing (store at `5c5fe54a4`) dropped 19 tests and the oracle found no missed observer, but cli and cmd/corvint never narrow. |
| AC1 | MET | Always-run reach keeps the package whole, and the reason is named. |
| AC2 | Measured, weak | Recall is retained for the one narrowing, but kept observers were 0/0. |

## Fix-only review: two refusals, disabled for 1.0

The fix-only review of `4ef3e9c88` found two more soundness gaps. Both are closed by refusing, with
no new modelling:
- **Non-identifier assignment targets.** `assignedElsewhere` matched only bare identifiers, so
  `for _, (p) = range ...` kept `p` private. Now any range key or value that is not a bare
  identifier, and any parenthesized assignment target, ends the privacy of every variable in the
  body. Regression: `TestAFPV02374RangeAssignmentEndsPrivacy` subtests parenthesized value, key and
  assignment. All three failed before the fix and pass after it.
- **Explicit switch values and switches set twice.** `BuildFlags` dropped `-race=false` from the
  probe, so `GOFLAGS=-race` with a declared `-race=false` probed with race while the run went
  without it. Now a switch with an explicit value, in the flags or in `GOFLAGS`, is unmodelled. A
  build switch (`-tags`, `-race`, `-msan`, `-asan`) set in both `GOFLAGS` and the flags also
  refuses. Either case keeps the conservative plan, naming that the executor build configuration
  cannot be read. Regression: `TestAFPV02372ExplicitSwitchValueRefuses`. It failed before the fix,
  and fails again with the both-places check mutated off.
  End to end, `TestAFPV02372DryRunBindsExecutorBuildConfiguration` subtests `GOFLAGS -race with
  -race=false` and `-race in both` plan the package whole. Both failed against the pre-fix
  `config.go`.

## Rollback

Revert the lane commit.
- This restores the unconditional constrained-source refusal and the earlier privacy and
  `Scan` handling.
- Emptying `typedReachPackages` still disables typed narrowing entirely.
