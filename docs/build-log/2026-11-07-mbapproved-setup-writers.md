<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbapproved: typed reach keeps setup writers, and the third CI round

Fix round 3 of the mbapproved batch (entries `2026-11-06-mbapproved` and
`2026-11-07-mbapproved-ci-integration-fixes`) answers one review finding on the merged floor lane's
typed reach (V1-2744) and the CI failures still open at `283e288d1`.

## Typed reach dropped setup tests (AFP-V0-2374)

Independent review (Greptile, thread 4500549557) found that `typedreach.Analyze` kept a dropped test
that reads state a kept test may write, but never one that writes state a kept test reads. The
claim held. Its own fixture is in `TestAFPV02374SetupWritersOfKeptReadsAreKept`: unchanged
`TestASetup` sets the package flag `enabled`, and `TestBCheck` calls `lib.Check` only when
`enabled` is true. The head breaks `lib.Check`. `lib.Check` writes no state, so `BaseEffects`
admits narrowing. Before the fix, `Analyze` kept only `TestBCheck`, in three variants: a direct
write, a helper write, and a transitive `mode` setup. Run with `go test` on the same fixture:

| Run | Result |
|---|---|
| whole package | FAIL at `TestBCheck`, rc 1 |
| `-run '^(TestBCheck)$'` (before the fix) | ok, rc 0: the missed failure |
| `-run '^(TestASetup\|TestBCheck)$'` (after the fix) | FAIL at `TestBCheck`, rc 1 |

Decision: AFP-V0-2374 is amended, and `Analyze` keeps a dropped test with
`WRITES_STATE_A_KEPT_TEST_READS` when its closure may write state any kept test's closure reads.
The fixpoint is the same as for `READS_STATE_A_KEPT_TEST_WRITES` and ignores test order.

- The fixpoint tracks two sets. The *needed* set is the reads of every kept test. The *tainted* set
  is the writes of every test that may observe the change: `REACHES_CHANGE`, `STARTS_PROCESS`, and
  every test whose reads meet tainted state.
- A setup writer's reads join *needed*. Its writes join *tainted* only once its own reads meet
  tainted state; until then it writes at head what it wrote at base. So a dropped test reading
  only what an unaffected writer writes still observes base values. The existing "plain read"
  precision case in `TestAFPV02374ReviewRegressions` (only `B` changed, `TestAliasRead` dropped)
  stays as it was. Counting every kept test's writes as tainted would also have been sound, but
  that version broke this case.
- `processState` setters (`os.Chdir`, `flag.Set`, `runtime.GOMAXPROCS` and the rest) have no
  per-reader pseudo variable. No kept test can be proved not to read them, so while any test is
  kept a dropped setter is kept too, and the existing kept-process rule then keeps the package
  whole. The graph now records them separately as a node's `setters`.
- A file write at a constant path is not private, so it now also writes the file-system pseudo
  variable, as the requirement text already said of every non-private write. Such setup is kept
  only when a kept test may read files, which keeps the package whole. The existing "fixed path
  negative control" (a kept test that reads no files) still narrows.

Soundness: every writer of state a kept test reads is kept, and so are the writers of those
writers' reads. A narrowed run keeps the kept tests' relative order, so each kept test sees the
writes it sees in the whole package. A dropped test's reads meet no tainted state, and
`BaseEffects` still refuses a changed declaration that wrote state at base. So its result is
unchanged.

Rollback: revert the analyzer commit and the spec amendment together, and keep the regression as
a known failure.

## CI items at `283e288d1`

- **d. `TestCALV0131_LiveFormatsCoverEveryDecodedProfile`: fixed.** clicontract declared the
  `internal/tasks/cli` read scope as a list of `internal/tasks/*/` child directories. The test
  walks `..` (`internal/tasks`) for every decoder literal, and Landlock grants no read of the
  parent directory itself. Under `test-confine` it failed with `open ..: permission denied`, and it
  reproduced locally. The scope now declares the `internal/tasks/` subtree in place of the children
  it covers. `TestTasksCLIRecursiveReadSelection` already requires that every production source
  under `internal/tasks` selects the unit. `make test-read-scopes-check` passed both before and
  after: the checker counts a directory above the package as covered, but the sandbox does not
  grant reading it.
- **e. `TestGoImportsDifferentialAgainstScanner`: not a failure.** CI recorded `pass`. The test
  logs the line scanner's spurious imports and fails only on a walk error. At `origin/main`
  `538e823f4` it also passes, with `spurious=356` and `missed=0`. Every per-file count matches the
  PR head's 424 except two files the batch added: `internal/testreuse/review_test.go` (67) and
  `tools/cli-discovery-gen/help_test.go` (1). `internal/analyzerexec/runner_posix_test.go` gives
  145 on both. Its line 41 opens `import (` inside a raw-string Go fixture, which is the scanner's
  documented over-approximation. The scanner is unchanged.
- **g. `internal/analyzerexec`: not a failure. macOS shard 3: fixed.**
  `TestStageFailureRetainsCleanupResidueEvidence` recorded `pass`; its log line contains the word
  "error". The shard failed `TestRunDirectCommandCleansDescendantAfterNormalExit` with `invalid PID
  handoff ""`. The test is identical at the base and at `origin/main`.
  - Cause: the `descendant` helper exited as soon as the ready file existed. `os.WriteFile`
    creates the file before writing it, so the group cleanup could stop the held child in between.
  - It reproduced 1 in 400 locally at load about 30. The helper now waits for a parsed PID, as the
    test's own waiter does, and 1000 of 1000 runs pass.
- **h. `pi-host-linux`: fixed, hosted run pending.** The bugfix lane's
  `.github/workflows/pi-host.yml` is absent from `origin/main`. It failed in
  `npm install --global npm@12.2.1` with `MODULE_NOT_FOUND: promise-retry`, because upgrading the
  runner's npm 10.9.7 in place broke it before any Corvint step ran. The workflow now installs
  with the protected README's route, `npx --yes npm@12.2.1 ci --ignore-scripts`. There is no Node
  at or above 22.22.2 here, and PWV-V0-8300 already requires a remote run before the job counts as
  qualified.
- Round 2 fixed a, b, c, f and the ASD-V0-2421 part of d. This round confirmed those tests again;
  the results are in the handback.
