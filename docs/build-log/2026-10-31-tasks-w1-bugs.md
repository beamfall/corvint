# Tasks bug lane tasks-w1 — wave 1 (2026-10-31)

Base `780f9e97099aa3ccd52744a0c239a744c4a30adf`. Loaded runs used 44 CPU burners at nice 10 (the
test's own niceness under `heavy.sh`) on the 32-thread WSL2 box, so the 1-minute load average sat
between 35 and 57 while the tests ran. Each command stayed under the lane's 600 s bound.

## V1-2483: ENOSPC is RESOURCE_EXHAUSTED (PSR-V0-901)

Fork: classify only the change guard's inotify refusal, or every untyped `ENOSPC`. Decision: every
`ENOSPC`, through `errors.Is`, like EMFILE/ENFILE under CAL-V0-215. The requirement was first written as CAL-V0-900, but the agent-leases `## Requirements` section is at the 256-obligation OCM bound (the dogfood batch-precheck caught `internal/lrfrepo` failing), so it is defined as PSR-V0-901 beside the change-guard requirements, as CAL-V0-364 was re-homed to PSR-V0-221. Disk space and the per-user
watch allowance are both host resources that can clear. An explicit code and a `WithoutRetry` mark
still take precedence. The change guard's directory-watch refusal moves from
`UNSUPPORTED_FILESYSTEM` to `RESOURCE_EXHAUSTED`, which amends the code PSR-V0-221 named. The
allowance is shared by concurrent guards and frees when they close, so a retry can succeed. The
pool-acquire harness already keeps a 300-byte error detail for each failed sample
(`pool_acquire_load_test.go`, `s.Detail`), so its AC needed no change.

## V1-2623: a corrupt bound manifest refuses JOURNAL_FORKED (LGL-V0-900)

Root cause: the unlocked Logical source reports a manifest whose digest differs from the head's
binding as `SNAPSHOT_MOVED`, because a seal may replace the manifest between two unlocked reads.
`commitLease` returned that preparation failure under the writer lock, and the lease loop retried
it 16 times. Decision: under the lock, after a `SNAPSHOT_MOVED` preparation failure, re-read the
head and its bound manifest (`boundManifestHeld`). No seal is in flight under the lock, so a
mismatch is corruption and is refused once as `JOURNAL_FORKED`. A moved head still retries
(`TestCALV0372_TornGuardReadSpendsOneRound`). Found in passing: a bound *segment* rewritten in place
with its stamps kept still spends 16 rounds and refuses `SNAPSHOT_MOVED`. LGL-V0-900 leaves that
case out, and it is reported to the orchestrator for its own ticket.

## V1-2622: classified not-a-bug

Evidence and pinned blobs are content-addressed (`evidence/<sha256>`, `pinned/<sha256>.json`), so
no legitimate tail write can give a base blob's path other bytes. `ResumeBase.held` already skips a
blob the walked tail removed (`resume.go`, the `l.digest == nil` branch). The test
`TestLGLV0009_TailRemovedBaseBlobKeepsTheResume` shows that a tail-removed blob keeps the resume
and a blob rewritten in place declines it. A mutation check confirmed it: disabling the skip makes
the removed case decline.

## V1-2447: pools.json bound derived from a superset (PSR-V0-900)

The test sets every optional field on all 256 entries at once, makes every Identifier, label and
prose field all-escaping (`"`/`\`) at its maximum length, and fills 512 slots and 512 waiters.
That superset encodes to 2,542,455 B, which is past the 2 MiB PSR-V0-266 bound. Measured shares:
entries ≈1.12 MB, operations ≈0.48 MB, waiters ≈0.37 MB, slots ≈0.35 MB, identities ≈0.21 MB.
Valid-only states can also pass 2 MiB, so tightening the reason cap alone cannot fix it. Decision,
per decision 0552 ("raise MaxPoolStateBytes to the computed worst case"): 3 MiB. Fresh inputs past
any field bound are still refused before a write, and `PoolState.Encode` still refuses an over-bound
encoding with `LIMIT_EXCEEDED`. An N-1 binary refuses a pools.json over 2 MiB, as PSR-V0-266's
change already implied.

## V1-2489: delete-pending lock create is contention (SCL-V0-900)

The deferred-platform `O_EXCL` lock moved to `store_excl.go`, which builds on every platform and has
an open seam. Its permission retry is therefore tested on Linux. A permission-refused create now
retries within `LockWait` and ends as `ErrLockBusy` naming the refusal. A never-writable directory
therefore waits 2 s before reporting, which is the accepted cost. The Windows-host rerun of
`TestStaleLockContentionGrantsAtMostOneLease` is NOT_RUN (no Windows host in this lane; decision
0559 lists it as post-1.0 qualification).

## Load flakes

- V1-1516: `TestPSRAllBarrierTerminal` failed 1 of 50 at load ≈48. Its reset blocks on the test's
  own barrier step and a refused fresh sweep, and that outlasted the fixture's 3 s attempt bound
  (TIMEOUT before the barrier). It now uses the V1-1324 load-tolerant bounds, and its replay uses
  the same sweep bound. `TestCALV0089_CodexContinuationResumesPreservedSession` failed 1 of 5 at
  load 44 (implement or integrate turns 3): the 4 s stage wall also bounds the resumed turn, so a
  loaded resume was walled again. Its wall is now 12 s. After the change all named tests passed
  5 of 5 at load 25–57: barrier, Codex continuation, `TestPoolProcessDescendants`, and
  `TestCALV0089_DrainStopsContinuation`, which passed 5 of 5 unchanged. Separately,
  `TestPSRSweepSuccessReplaySuccessor` passed 50 of 50. The AC's full-package run at load ≥40 is
  NOT_RUN: the package takes about 560 s quiet, which exceeds the lane's 600 s per-command bound
  under load.
- V1-2538: ALREADY-DELIVERED by `afeeefb2b` (V1-1324 load-tolerant bounds in
  `pool_sweep_test.go`; root cause: the fixture's 3 s attempt bound and the product observation
  bounds ran out before the verify class was reached). WSL2 evidence: `TestPSRSweepStderrIsNotStdout`
  passed 50 of 50 at load up to 48. The macOS rerun is NOT_RUN.
- V1-2625: root cause of the released settle: the forged-identity leader's output holder slept a
  fixed 3 s. On a loaded host the identity refusal and the drain began after the holder exited, so
  the drain proved the output closed. The holder now holds until the test writes a stop file or
  removes its directory, bounded at 2 min. Results after the change at load 38–57: 20 runs with
  1 failure in a different mode (phase READY, no leader PID, so nothing was spawned), then 30 of 30
  and 40 of 40. The failure message now carries RunRole's error so that mode can be diagnosed. It
  is reported for its own ticket.
- V1-2638: route identified and reproduced deterministically. A snapshot that moves under the first
  complete-route commit sends the claim to a second round. That round resumes on the writer route,
  which retains the read checkpoint under the lock (CAL-V0-119) after round 1 staged it, and writes
  no writer delta. `TestJSGV0015_MovedClaimResumesOnTheWriterRoute` forces this by touching a ticket
  at `fast.complete.guards`. The staged test now measures a single-round claim, taking a fresh
  fixture up to 3 times. Result: 20 of 20 at load ≈47.
- V1-2664: the parity test compares reports, errors and store bytes exactly and normalizes only a
  trailing `fresh` stage, which is the ChangeGuard's legitimate fallback. Result: 20 of 20 at
  load ≈40.
- V1-2676: not reproduced, 20 of 20 at load 45–51. Each mode runs in its own child process with its
  own `os.MkdirTemp` store, so a sibling cannot move the clean child's snapshot. The corrupt child's
  EMFILE is the test's own `limitDescriptors` (it gets no descriptors while the watch is open), by
  design. Which write moved the clean child's store during registration is not identified, so the
  ticket stays open. V1-2638, V1-2664 and V1-2676 all show an unexplained change event on a store
  only the test touches; that common thread is reported to the orchestrator.
- V1-2558: 50 of 50 loaded `-race` runs of the CI-assembled helper module (`GOENV=off`, temporary
  module, 44 burners), all four AFPV0023 tests passing each time. Not reproduced. The ci-preflight
  static phase is NOT_RUN, because `ci-preflight.sh` lives outside this checkout.
- V1-2559: the witness test now skips when the runner records a not-run witness whose reason is
  `BUDGET_EXCEEDED`, which the AC allows. Any other not-run detail still fails. Result: 3 of 3
  under `-race` at load 44. The ci-preflight test phase is NOT_RUN.
- V1-2250: blocked. The run log for 39416257984 is not reachable from this host (no `gh`), and the
  failure is macOS-only. `TestATRV0004_InterruptStopsTheRun` passed 20 of 20 at load 41–46 on
  Linux.
- V1-2357: blocked. The frozen code2test suite needs the ARB v2 repository corpus (diffusers at
  c7ea7d6f), which is absent on this host, and the fix is a retrieval-ranking change outside
  `internal/tasks`.

## Dogfood intents

`docs/specs/corvint-tasks-agent-leases-v0.md` is omitted from the OCM intents: it holds 263
requirements, over the 256-obligation bound of one map, so `ocm prepare` refuses it on its own
(`too-many-obligations`, DOGFOOD.md, which says that needs an owner-reviewed restructuring of the
spec). Its CAL-V0-215, CAL-V0-070, CAL-V0-074 and CAL-V0-089 lines are cited as `specification`
instead; omitting the spec does not claim its requirements hold. The intents are the
journal-segments, lease-gate-latency, pool-safe-reuse, scope-lease and test-claim-qualification
specs (244 requirements).

## Friction fixes (owner request, 2026-10-31)

- Rewritten bound segment: LGL-V0-900 now also covers a bound segment rewritten in place at a stable
  head. Under the writer lock, when the preparation failure names a bound segment, the commit
  re-hashes that one segment against its manifest entry and refuses `JOURNAL_FORKED` in round 1.
  It previously spent 16 rounds and refused `SNAPSHOT_MOVED`. Only the named segment is read, so
  the locked work stays bounded by one segment.
- Full `/tmp` on the WSL lane host: the shared `/tmp/corvint-go-build-cache` had reached 15 GiB on
  the 16 GiB tmpfs, because the `go-cache-trim` default bound of 20 GiB exceeds that file system
  (ASD-V0-900). Decision: without `--max-gib`, a cache's bound is the smaller of 20 GiB and 40% of
  its file system. I ran the documented trim once (`--apply --max-gib 8`, entries older than 12 h
  only), which freed 4.5 GB; `/tmp` went from 98% to 71% used.
- OCM bound early warning (SRG-V0-900): `spec-requirements-check`, and so `make doc-gates`, now runs
  `TestSpecsStayWithinTheOCMObligationBound`. `affected-plan-v0.md`, already over the bound on
  main, is listed in `overBoundSpecs` and must stay over it. A probe requirement added to the
  agent-leases spec fails the check.
- Closeout order (DCW-V0-900): `dogfood-closeout` runs the input pre-flight, then batch-precheck,
  then `dogfood-finish`, then batch-precheck again for the sealed head's receipt. That costs one
  more precheck run per closeout; the alternative, moving the single run ahead of the bind, would
  leave the receipt naming a pre-bind head.
- Not changed here: the V1-2625 READY mode (phase READY, no leader spawned) has not reproduced since
  the first sighting, in about 130 loaded runs with RunRole's error now in the failure message.
  The truncated ticket text in the lane input file belongs to the orchestrator. The CORVINT_BIN
  refusal and the propose-citations unknowns are existing behaviour; V1-2360 and V1-2423 cover
  propose-citations.
