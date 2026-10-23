# Nightly release-gates failures on main (#1551)

Date: 2026-10-23. Lane: claude/nightly-gates-gh1551 (base 0641ad2e09a6da042e144fa5ded4861637e550a9).
Issue: GitHub #1551 (`Refs`, not closed: it closes when a scheduled run passes). Requirement
`CAL-V0-360` (`docs/specs/corvint-tasks-agent-leases-v0.md`) is accepted under the owner's standing
auto-accept (2026-10-11). The cross-vet fix lives under the existing `PRS-V1-019`.

## Distinct failures across the runs #1551 names

| Run | Job | Failure | State on this base |
| --- | --- | --- | --- |
| 39773957842 | ubuntu-24.04 | `no-python-runtime-dependency-test`: `script/dogfood-cache_test.sh` used `python3` | fixed earlier by 699a59187 (#1533) |
| 39773957842 | macos-15 | `internal/tasks/store` `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead`: `racer 2: SNAPSHOT_MOVED: .../head.json: file replaced between stat and open` | fixed here, CAL-V0-360 |
| 39797313530 | macos-15 | `internal/procgroup` `TestPWPV0030InterruptGraceEscalatesToSIGTERM` | fixed earlier by V1-2446 (9047a5bb2, a0fd61b72) |
| 39947631092 | both | `cross-vet`: `cmd/corvint/work_materialization_unix_test.go:393:20: undefined: syscall.Flock` for `GOOS=solaris` | fixed here |
| 39947631092 | macos-15 | `internal/tasks/store` `TestPSRRequestConflictAndOrphan/live-unknown-gone`: `unknown process proof not reached <nil> 0` | fixed earlier by 1ffbb44c4 and 7a6ee4348 (PSR-V0-204) |

## Causes and fixes delivered here

### cross-vet on solaris

`PRS-V1-019` type-checks `cmd/corvint` for `freebsd`, `openbsd`, `netbsd` and `solaris` with
`CGO_ENABLED=0`. `TestWorkCloseKeepsScratchWhenRetirementIsUnconfirmed` holds a Git group record
with `syscall.Flock`, which solaris lacks, inside a `//go:build unix` file. The production code it
exercises, the flock-protected retirement path in `internal/worksource/process_unix.go`, is
`//go:build darwin || linux`; the test now lives in
`cmd/corvint/work_materialization_flock_test.go` under the same constraint. The other tests in the
`unix` file use only `syscall.Kill` and signals, which every `unix` target has. Verified:
`GOOS={solaris,freebsd,openbsd,netbsd,windows} GOARCH=amd64 CGO_ENABLED=0 go vet ./cmd/corvint` and
`GOOS=linux GOARCH=arm64` all clean (solaris failed on the base with the CI message), and
`heavy.sh go test -run TestWork ./cmd/corvint` ok on darwin (87.98s).

### Torn unlocked guard read in `leaseWrite`

Each of `leaseWrite`'s sixteen rounds begins with `writerGuards` → `readHead` → `intent.ReadFile`
without the writer lock. `readBoundedInfo` refuses `SNAPSHOT_MOVED` ("file replaced between stat
and open") when the opened inode differs from the stat'ed one, which a writer that commits
`head.json` under the lock causes for any unlocked reader between those two calls. The
`prepareLease` and `commitLease` steps of the same round retry that code; the guard read did not,
so one torn observation ended the write with fifteen rounds left. The `gate run` racer's unlocked
proof read (`readLeaseProof`) and the `settleLease` reads under the lock were ruled out: the first
retries `SNAPSHOT_MOVED` for sixteen rounds, the second cannot see a concurrent commit.

Fix: a `SNAPSHOT_MOVED` result from that guard read spends the round and the next round reads
again (`continue`), with the round counted in `timing.Rounds` as before; every other guard failure
keeps its disposition through `guardFailureAudit`. The seam `leaseGuardFault(stateDir)` and
`SetLeaseGuardFaultForTest` inject the torn read for a chosen store only, so parallel package
tests on other stores are unaffected.

Evidence: `TestCALV0360_TornGuardReadSpendsOneRound` passes (0.36s); with the `continue` removed
it fails with `claim after a torn guard read: SNAPSHOT_MOVED: head.json: file replaced between
stat and open`, the CI message. `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead` ok at
`-count=10` after the change (18.7s) and at `-count=20` on the base (39.0s), so the CI failure was
not reproduced locally; the cause is the code reading above.

## Residual limits

- The store race was diagnosed from code and the CI log, not reproduced; the fix removes the only
  unlocked, unretried `head.json` read on the racer's path.
- `TestPSRRequestConflictAndOrphan/live-unknown-gone` and the procgroup and python3 failures were
  verified fixed by reading the later merges and running the store tests on this base
  (`-count=5 -run 'TestPSRRequestConflictAndOrphan|TestPSRNativeOrphanChild'` ok, 15.9s); the
  next scheduled `release-gates` run is the proof that closes #1551.
- `make gate` was not run in this lane; cross-vet was run per target by hand.
