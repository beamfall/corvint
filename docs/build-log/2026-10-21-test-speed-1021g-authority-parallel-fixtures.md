## 2026-10-21 V1-0348: internal/tasks/authority fixture tests run in parallel

Lane test-speed-1021g (pre-1.0 ticket sweep). Test-only change. No product behavior, wire contract
or requirement changes, so no requirement ID is added.

**Problem.** `go test ./internal/tasks/authority` ran its 101 top-level tests one after another.
`go test -json` showed the summed test time equal to the package wall time (188 s on the 12-CPU
darwin host at load 7-30). Five tests took 134 s of that:

- the three `TMV0009` fault suites (`ReturnedFaults`, `EveryPublicationPrefix`, `CleanupBoundaries`),
  which replay every publication, fault and cleanup boundary from a fresh fixture;
- `FreshRecovery`;
- `TestCALV0111_CallerWaitOutlastsDefault`, which sleeps 31 s past the 30 s default lock wait.

A CPU profile of `ReturnedFaults` puts 66 % of samples in `fixtureSession.check`. That function
reopens the primary worktree with `safeopen.Root` and checks every retained parent on each operation.
It is the product's own revalidation, not test overhead. Swapping in no-op `syncFile` and
`syncDirectory` did not change the time (18.3 s compared with 18.8 s), so fsync is not the cost.

**Decision.** The cost comes from the product checks, so the change is to scheduling, not to
assertions:

- Every top-level test in `fixture_transaction_test.go`, `fixture_transaction_fault_test.go`,
  `fixture_recovery_test.go`, `fixture_read_integration_test.go` and `fixture_intent_worktree_test.go`
  calls `t.Parallel()`. So does each per-operation and per-boundary case of the three fault suites
  and `FreshRecovery`. Each case builds its own `TempRepo` and harness. None of these files assigns a
  package hook (`syncFile`, `flockNB`, the `admission*`/`preparation*` seams, `vnodeFileBudget`), and
  none uses `Setenv` or `Chdir`. The tests that do replace hooks stay sequential. Go runs parallel
  tests only after every sequential top-level test has finished, so a replaced hook is never seen by
  a parallel case.
- `TestCALV0111_CallerWaitOutlastsDefault` is also parallel, so its sleep overlaps the CPU-bound
  fixture tests. Its hold grows from default+1 s to default+5 s, which still sits inside the 40 s
  caller wait. This makes the refusal window robust to scheduling delay. The default waiters now also
  have to report LOCK_TIMEOUT no earlier than `DefaultLockWait`, a lower bound the test did not check
  before. The assertion is stronger than before.

**Evidence** (same host, `go test -p 4 -count=1 -json ./internal/tasks/authority/` through the lane
semaphore):

| run | load average | package wall | result |
|---|---|---|---|
| before (`ae3b41815`) | 7 to 30 | 188.5 s | 101 tests pass |
| subtests parallel only | 15 | 158.2 s | 101 tests pass |
| after (this change) | 15 to 56 | 96.3 s | 101 tests pass |

The after run had the higher load and was still 49 % faster. After the merge with origin/main, the package passed again in 274.0 s at load average 266 to 290. Even at that load it is well under the 544 to 655 s the ticket recorded under load. The sequential phase is now about 8 s.
The parallel phase is bound by `check` syscalls. A `-race` run of the parallelized fault, recovery,
native-read and CALV0111 tests passed (36 s).

**NOT_RUN.** Linux CI timing; batch CI measures it. `make gate` is not run, per the lane charter. The
`internal/tasks/cli` and `internal/tasks` full packages are NOT_RUN: only `_test.go` files of
`internal/tasks/authority` changed, and no other package's test binary compiles them. The
pre-push traps (`TestImportDirection`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile`) pass.

**Rollback.** Revert the commit. The tests return to sequential execution and the 1 s CALV0111 margin.

### Other tickets checked in this lane (not changed here)

- **V1-0723** was already delivered by `5db4ff66d` (V1-0772, CAL-V0-086). The stage worktree path
  is now carried as PathText. The `TestCALV0086_*` tests pass with a 178-byte TMPDIR, and so does
  `TestCALV0089_IntegrateCheckpointRestartKeepsGrant`.
- **V1-0649** was already delivered (PR #474 and `03b149a5e`). `corvint affected` with only
  `docs/specs/README.md` dirty selects neither `internal/tasks/authority` nor `internal/tasks/store`.
  Both have entries in `.corvint/test-read-scopes.json`.
- **V1-1500**: `TestCALV0089_IntegrateCheckpointRestartKeepsGrant -count=10` passed 9 of 9 runs, each
  36 to 55 s, at load average 48 to 150. The tenth run was stopped by the lane's own 8 m bound, not by a
  test failure. The failure did not reproduce, and the requested -count=20 was not completed.
- **V1-1495**: `conformance/cli-parity-v0` took 74.9 s at load about 20. `TestGPKV0002ManifestReplay`
  took 45.3 s of that, mostly a cold `go build ./cmd/corvint` with a private GOCACHE. No goroutine
  dump was captured under load 65 or above.
