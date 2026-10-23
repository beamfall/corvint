# 2026-10-23 Linux process-group grace, shallow dry merge, Linux ratchet pin and / opens (V1-2399, V1-2388, V1-1726, V1-2249, V1-2365)

Host: Linux 6.18 amd64 container, 4 CPUs, uid 0, PID 1 `process_api`. Go: the system Go is
go1.24.7, so the official `golang.org/toolchain@v0.0.1-go1.27.2.linux-amd64` module was fetched
with `GOTOOLCHAIN=go1.27.2` (checksum database `h1:N6Xv/1OLzI0jVVkUwkrxHRZc/GCTARUo6idcngKwlDE=`)
and put first on `PATH`, so every `GOTOOLCHAIN=local` command below ran go1.27.2.

## V1-2399 (PGO-V0-022): the Linux grace waited out its whole length

- Reproduced: before the change, on this host `TestPWPV0030ZeroInterruptGraceKeepsTerminateFirstStop`
  took 10.22 s, `TestPWPV0030InterruptGraceEscalatesToSIGTERM` 10.51 s and
  `TestTerminateGraceLetsSlowTermHandlerFinish` 10.01 s, each the full 10 s `TerminateGrace`
  although the shell exits on SIGTERM at once.
- Cause confirmed: `processGroupQuiescentBy` treats only ESRCH as quiet, and Linux keeps
  `kill(-pgid, 0)` succeeding while the exited leader is held unreaped as a zombie.
- Fix: once run's waitid goroutine has observed the exit successfully (`stopSignalRecorder.exited`
  sets `leaderHeld`), a signal-0 probe that still reaches the group is followed by
  `groupreap.HeldGroupQuiet`, backed off from 5 ms to 50 ms. `graceHeldQuiet` is nil on Darwin
  and elsewhere. The `process.go` edits are line-neutral so its pinned citations hold.
- After: the three tests take 0.21 s, 0.51 s and 0.56 s. `TestPGOV0022LinuxTermObeyingGroupEndsGraceEarly`
  stops a TERM-obeying group with a 5 s grace in 0.02 s and fails at 5.00 s with `graceHeldQuiet`
  forced to nil on Linux (scratch mutation); its TERM-ignoring case is SIGKILLed after the 500 ms
  grace and its member is no longer running. The two unit tests fail to compile on the old code.
  The package takes 44.6 s instead of 74.3 s.
- Unrelated host limit: five `internal/procgroup` tests (`TestObservedDescendantCancellationReapsEscapedChild`,
  `TestRunProcessReapsDescendantBeforeDelayedSideEffect`, `TestRunProcessInterruptionLeavesNoDescendant`,
  `TestSupervisorSignalReapsNestedOwnedGroup`, `TestForcefulAncestorKillReapsJoinedNestedGroup`)
  fail identically on origin/main and on this change. Each reports a killed orphan still present
  after 1 s; this host's PID 1 reaps orphaned zombies after about 1.6 s (measured).

## V1-2388 (PGO-V0-023): trap-marker witnesses under load

- The trapSpec witnesses already set a 10 s `TerminateGrace` and wait for trap readiness
  (V1-2446), so none depends on the 100 ms default.
- Measured with one compiled test binary, `-test.count=100`, while eight busy loops ran on the
  4 CPUs (load average 7 to 13): every one of the 800 runs of the PWP-V0-030, V1-2319 and
  PGO-V0-022 tests passed. 32 of the 100 `TestTerminateGraceLetsSlowTermHandlerFinish` runs took
  the full 10 s. Instrumenting the held-group proof showed a live `sleep 30`, reparented to PID 1,
  still in the group: the fixture forked `sleep 30 &` with its TERM trap installed, and a SIGTERM
  that reached the child before its exec ran the inherited trap instead of ending it. The proof was
  right and SIGKILL ended it, so this was slow, not a failure.
- Fix (test only): the fixture waits in short foreground sleeps. Rerun under the same load: 100 of
  100 passed, median 0.60 s, maximum 0.68 s.

## V1-1726 (SPC-V0-015): shallow dry merge

- `refuseShallowMergeBase` runs in `Check` and `Behind` after base and head resolve. In a shallow
  repository it refuses `changes-shared-shallow-merge-base` when `merge-base --all` finds none or
  one is a boundary commit other than base or head.
- Before (guard disabled): the beyond-boundary fixtures returned a report computed from the
  boundary merge base, or `changes-shared-range-unavailable`; both new cases fail. The
  inside-boundary test matches the full clone and fails with `changes-git-unavailable` when
  `GIT_SHALLOW_FILE` is not passed to the private merge repository, which is the passthrough the
  ticket said had no maintained test.

## V1-2249 (SNR-V0-031): Linux pin row

- Observed toolchain: the official module above at
  `/root/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.2.linux-amd64`; `go` binary sha256
  `548d3c32…e7e0`; receipt tool built with the gate's `run_go` flags `874295bf…6d8c`; GOROOT
  receipt `{"Entries":13474,"SHA256":"5d9a6167…6ecb"}`. The darwin official-module row has the
  same entry count.
- Before: the origin/main gate printed `sql-native-ratchets status=REJECTED reason=TOOLCHAIN_MISMATCH
  detail=unpinned-go1.27.2-548d3c32…e7e0` and exited 78 on this host.
- After: `sh script/check-sql-native-ratchets_test.sh` passes, its new case 9 reporting that the
  host toolchain matches its row; with the origin/main table case 9 fails. Full gate run,
  `make sql-native-ratchets` on the clean tree at d56eb2c9: exit 0, one line with
  `go_version=go1.27.2 goroot_entries=13474 goroot_sha256=5d9a6167…6ecb core_binary=identical
  core_deps=identical complexity=delimiter-pairs-linear-old-parenAt-model-fails
  candidate_max_b_op=93462 candidate_max_allocs_op=393 causal=NOT_RUN-causal-parent-unavailable
  latency=NOT_RUN`, `source_bytes=64763 binary_bytes=2969724`.
- One run of the fast test with the origin/main gate script copied in failed earlier, at case 4,
  with `reason=CONCURRENT_RUN`; it did not recur in 15 runs of the origin/main test or 15 runs of
  this change's test, so it is recorded as an unreproduced observation.
- `NOT_OBSERVED`: the go.dev tarball (what `actions/setup-go` installs) and linux/arm64; go.dev and
  dl.google.com returned 403 through this host's proxy.

## V1-2365 (AFP-V0-361): no / opens

- With `internal/upgradechanges` and `internal/testrunner` declared with empty scopes and run under
  the `.github/testconfine` wrapper (built into scratch, Landlock ABI 7), the old `openRefs` failed
  `TestCompileRangeAndRefs`, `TestCompileBounds`, `TestCompileReferenceAndIssueCaps` and
  `TestCompileIssueStateFile` (`changes-refs-unavailable`); `internal/testrunner` already passed.
  With the new `openRefs` both packages pass confined.
- `make unbounded-readers-check` passes with testrunner removed from the list and fails if its
  scope declaration is removed again. `interop/cem01-go` runs in its own unconfined job
  (AFP-V0-380), so it gets no scope; its `/` open is removed all the same.
- `TestAFPV0361_OpenRefsNeverReadsFilesystemRoot` and
  `TestAFPV0361_OpenStableArtifactNeverReadsFilesystemRoot` fail with the old open (no absolute
  first-component open observed).
- Unrelated host limit: `TestStableCanonicalFullResults/23-optional-attributes-EACCES` in
  `interop/cem01-go` fails identically on origin/main here, because uid 0 reads a mode-000 file.

## Independent review (one round)

- Finding (medium, fixed): with the merge base equal to base, a side branch of the range could
  still be cut by the shallow boundary (feature forked from main c1, six commits, merged main;
  depth 3), so check-shared walked 3 of 7 commits and `diff-tree --root` read the boundary commit
  as adding the whole tree. `refuseShallowMergeBase` now also refuses when a commit of
  `base...head` is a boundary commit; `TestSPCV0015ShallowRangeCutByBoundaryRefuses` fails
  without that check.
- Nit (fixed): the grace proof backoff widened while the leader still ran, adding up to 50 ms
  after the exit; it now lives in `graceHeldQuiet` and starts with the first proof.
- Note (fixed): case 9 failed any host on an official toolchain module of an admitted version
  without a row (for example go1.27.3 on Linux); it now requires a row only for the go1.27.2
  linux/amd64 module SNR-V0-031 names and reports other unmatched hosts `NOT_OBSERVED`.
- Not changed: a repository path containing a newline makes the shallow check's `rev-parse`
  output unparseable (`changes-git-unavailable`), as `privateMergeRepository` already does.

## Independent review of PR #1623 (repair round)

- Finding (P2, fixed): a shallow criss-cross merge could report clean. With main and feature both
  merging A1 and B1 and a depth-3 clone cut at A0 and B0, both visible merge bases keep their
  parents and `main...feature` is only the two merges, yet Git's recursive virtual base differs
  from the full clone's (a delete/restore file: add/add conflict in the full clone, clean in the
  shallow one). `refuseShallowMergeBase` now refuses `changes-shared-shallow-merge-base` whenever a
  shallow repository has more than one merge base, rather than proving recursive ancestry; this
  also refuses criss-cross ranges a full clone would merge identically, deliberately.
  `TestSPCV0015ShallowCrissCrossMergeRefuses` uses a real `--depth 3` clone and fails on the
  previous rule (check-shared returned no error).
- Finding (P3, fixed): the AFP-V0-361 symlink tests linked beneath the temp directory, so they
  exercised later components only. `TestAFPV0361_OpenRefsRefusesSymlinkedFirstComponent` points
  `openRefsTop` at a test-owned symlink with the flags `openRefs` passes, and
  `TestAFPV0361_OpenStableTopRefusesSymlinkAndSwappedFirstComponent` uses a new `stableTopPath`
  seam. Scratch mutations, each restored: dropping `O_NOFOLLOW` from the `openRefsTop` call,
  removing `openStableTop`'s pre-open symlink and directory admission, removing its
  `SameFile(prior, opened)` check, and removing every post-open check each fail the new tests;
  the earlier tests did not catch the `O_NOFOLLOW` removal.
