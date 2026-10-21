# 2026-10-21 perf2-1021: error-code ownership gate and store program-writer cuts

Lane `perf2-1021` on base `c9ca8beff43d8f548b054d3b3d40e3054cecf816`, under the owner's
2026-10-21 rule to speed up slow Corvint processes. Every number was taken on the same 12-CPU
host while other lanes ran; the 1-minute load average is recorded with each measurement.
ECO-V0-008 is accepted under the owner's standing auto-accept (2026-10-11).

## V1-2210: doc gates take 73 s

Problem. Timing each doc gate separately showed `error-code-ownership-check` as the largest
single cost: 27.6 s to 41.0 s at load 60 to 86. Almost all of it was one `git grep -o -E` whose
pattern is the alternation of every declared code's call positions, run over all 19 MB of
non-test Go sources. Apple Git's regex engine pays for the alternation on every line.

Decisions (ECO-V0-008).
- A first `git grep` with the plain kebab literal set finds the candidate lines (every site's
  line must contain a quoted code). Those lines are written, at their own line numbers and with
  every other line empty, into a private scratch tree built only from index content, and the
  unchanged `position` search runs over that tree with `--no-index`. Line numbers, paths and
  `-o` matches are therefore those of the original search.
- Splitting the alternation into separate searches was rejected: POSIX leftmost-longest `-o`
  semantics could change which site a line reports.
- The scratch search keeps ECO-V0-005: an exit status above 1 fails the gate.

Evidence. On the live tree both routes found the same 3,163 sites and 1,506 first sites. The
gate took 6.2 s to 8.3 s at load 60 to 86 after the change. Test case 2 fails if the scratch tree
drops the empty lines (wrong site line number).

## V1-1499: internal/tasks/store package time (partial)

Problem. A full `go test -json ./internal/tasks/store/` at load 29 to 70 did not finish within
this lane's 540 s command bound: the serial phase alone ran 154 tests in 540 s
(`TestPSRV0113_ForgedHeartbeatTailNeverCommits` 33.3 s, `TestPSRV0026PrepareFailureStaysQuarantined`
23.3 s, `TestPSRV0016_ShareCrashReplay` 19.9 s, `TestPSRV0116_TailRewrittenAfterAuditNeverCommits`
17.9 s, `TestCALV0221_CompleteRouteBoundedDescriptors` 17.8 s). The program_* subset the ticket
names (59 tests, 31 parallel) took 485 s at load 53 to 31: a 226 s serial phase and a 259 s
parallel phase. As the 2026-10-10 store-test-speed entry predicted, the process-global
`programWriter` mutex serialized every parallel supervised test's program transitions.

Decision. `programWriter` is now one mutex per store state directory (a `sync.Map`, like
`runFaults`). `intent` canonicalizes `StateDir`, so every transition over one store still takes
the same mutex, and cross-process exclusion is unchanged (the native writer lock and the
`Expected` binding). A production process opens one store, so its behaviour is unchanged.
No test, assertion or `t.Parallel()` set changed.

Evidence (program_* subset, `-p 4 -count=1 -json`):

| run | code | load start, end | package | serial phase | parallel phase | sum of parallel test walls |
|---|---|---|---|---|---|---|
| base | process-global mutex | 53, 31 | 485.2 s | 226.0 s | 258.9 s | 1,991 s |
| a1 | per-store mutex | 33, 139 | 458.1 s | 231.0 s | 226.7 s | 1,542 s |
| a2 | per-store mutex | 84, 80 | 515.6 s | 324.4 s | 190.8 s | 1,442 s |

At load 80 or more, run a2's parallel phase came within 9 s of the 182 s floor named below. Its
serial phase, which the change does not touch, grew with the load.

Per-test walls of the slowest parallel tests (seconds, base then a1):

| test | base | a1 |
|---|---|---|
| `TestCALV0089_ProgramWallExpiryEndsContinuation` | 182.5 | 194.6 |
| `TestCALV0087_InterruptedIntegrationLandsOnce` | 171.8 | 186.2 |
| `TestCALV0087_DesignatedTargetChecked` | 150.5 | 92.8 |
| `TestCALV0087_ExtraWorktreeCleanup` | 146.4 | 136.8 |
| `TestCALV0076_OpenCodeHostRollback` | 145.2 | 121.9 |
| `TestCALV0074_HostSwitchAndRollback` | 133.4 | 129.6 |
| `TestCALV0077_OpenCodeResumeRequiresFork` | 127.0 | 77.5 |
| `TestCALV0087_DesignatedMultiRepositoryIntegration` | 111.2 | 90.6 |
| `TestCALV0087_UnchangedRepositoryNeedsNoDesignation` | 110.0 | 85.0 |
| `TestCALV0077_OpenCodeProgramFakeHost` | 101.5 | 69.6 |
| `TestCALV0075_ClaudeCodeProgramFakeHost` | 100.2 | 66.4 |
| `TestCALV0087_GrantMustNameEveryTarget` | 80.5 | 42.4 |

The parallel phase cannot fall below `TestCALV0089_ProgramWallExpiryEndsContinuation`, which
waits for its deliberate three-minute program wall (V1-1324). The 16 other program-transition
and control tests (`issue502_escalation`, `supervisor`, program-control callers) passed in 24.3 s
at load 133.

Remaining limits, not changed here:
- The serial phase dominates the package. The lease-resume family (about 170 s serial) stays
  serial because it replaces the process-global `attemptEntropy` and counts complete audits,
  which the single-entry process-global `leaseAudits` cache makes depend on other tests. Keying
  both by store would let it run in parallel.
- The continuation tests that rely on the 4 s stage wall stay serial (2026-10-10 entry).
- The ticket's acceptance (three full-package runs under `-timeout 30m` at load 60 or more) needs
  a full-package run longer than this lane's 600 s command bound, so it is NOT_RUN and V1-1499
  stays open.

## V1-2211, V1-2209, V1-2206: not delivered

- V1-2211 (`tools/gate-ledger` tests 42.5 s): the fixtures use `t.Setenv` and `t.Chdir`, which
  block `t.Parallel()`, and each ledger run invokes `go list` and `go run` twice. A
  `-trimpath` experiment was rejected because it lets Go cache the inner test results.
- V1-2209 (344 s slot wait): the wait is in the host semaphore script outside the repository;
  the focused `cmd/corvint` test itself took about 39 s.
- V1-2206 (shared lexer for the dry-run scans) overlaps PR #1416's open double-scan work.

## NOT_RUN

`make gate`, the repository-wide test run, the full `internal/tasks/store` package, and CI.

## Rollback

Revert the lane's commits. The gate returns to the single alternation search; the store returns
to one process-global program mutex. No persisted state or wire format changed.
