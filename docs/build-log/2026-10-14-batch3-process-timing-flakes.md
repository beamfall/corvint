# 2026-10-14 Batch 3: process-timing flakes (V1-1378, V1-1374, V1-1244, V1-1293, V1-1294)

Lane batch3, base ee27168739ec1b65dd5a653d720cf811338c37ae (origin/main). The 1.0 scope is `PRS-V1-018`;
these tickets keep its process-cleanup tests trustworthy on hosted and loaded runners. One new
requirement: `PGO-V0-016` in `docs/specs/process-group-owner-v0.md` (V1-1244), accepted under the
owner's standing auto-accept (2026-10-11). `PGO-V0-012..015` are reserved by another lane and unused.

Rule for every ticket: the ordering a test depends on is forced by a hook, barrier or readiness
signal; no wall-clock bound that decides pass or fail was widened, and no product bound changed.
Original test bounds are kept; the new 30 s waits are hang detectors only. Contention runs used bounded CPU burners
(`perl -e 'alarm 900; 1 while 1'`, killed by an EXIT/INT/TERM trap) on the Darwin arm64 host;
Linux-only behaviour is NOT_RUN locally and comes from this change's PR CI.

## V1-1378: TestV11055NoGroupOperationAfterConcurrentRelease (test timing)

Release-gates nightly run 38559681509 (ubuntu-24.04) failed with "the foreign reap never preceded
the exit observation" after 20 attempts. Diagnosis: the test raced its own fixture. Its first group
probe SIGKILLed and `Wait4`-reaped the leader while the waitid (`WEXITED|WNOWAIT`) goroutine was
already waiting; whenever waitid saw the zombie before `Wait4` reaped it, the attempt was skipped,
and a hosted runner lost all 20 races. Its 200 ms wait for the release inside the probe could never
succeed either, because the probe runs under `groupOwnership.mu`, which the release also takes.
Not a product defect: `groupOwnership` already refuses every numeric group operation after the
release. The BASE test reproduces the nightly message on this host under load: 8 of 50 runs failed
with 72 burners (load 193 with other lanes).

Fix: a test-only `exitWait` seam in `processWaitChannels` replaces the waitid goroutine's
`waitProcessExitUnreaped`. The rewritten test has no retry loop: `BeforeStop` kills and reaps the
leader, then the seam lets waitid run (it now always fails with ECHILD) and `BeforeStop` returns
only after that goroutine released the group; run's own exit-observation channel is withheld, so
run signals the group as if the release had landed just after its non-blocking check. The test
asserts no group signal was sent, `errors.Is(err, errGroupOwnershipReleased)`, cleanup unproved and
exactly one release. New test: 50 of 50 runs pass under the same 72 burners. Mutation: removing the
`if o.released` refusal in `groupOwnership.signal` fails it ("1 numeric group operations sent after
the group was released"). Linux `-count=50` under contention: NOT_RUN locally (PR CI).

## V1-1374: TestObservedDescendantCancellationReapsEscapedChild (test timing)

Diagnosis: the test cancelled on a fixed 100 ms sleep after the escaped (setsid) child wrote its
pid. Under load no 20 ms descendant snapshot containing that child had been applied by then, so
the observer truthfully reported `Absent: true` with no processes, which is its documented limit
("fast detachment and reparenting between snapshots can remain unobserved"). Every failure showed
`Processes: []`. BASE reproduces it: 0 of 20 runs failed with 30 burners (load 46); 14 of 20 failed
with 72 burners (load 50). Not a product defect.

Fix: a test-only `descendantSnap` seam passes a snapshot function to the observer. The test wraps
the real snapshot and, once a snapshot has contained the child with the leader as parent, closes
`expanded` on the next snapshot call (the observer applies each snapshot before taking the next),
and cancels only then; a run that ends first fails the test. The original 5 s run timeout and 3 s
pid-file wait are kept. New test: 20 of 20 pass with 72 burners (load 48), and again 20 of 20 with
the original bounds restored after review (72 burners, load 45). Mutation: dropping the `signalObservedProcess`
call in `descendantObserver.finish` fails it ("observed descendant remains after cleanup").

## V1-1244: /proc quiet proof "no thread count" for an exiting member (product defect)

CI run 38178767381 failed `TestRunProcessReapsDescendantBeforeDelayedSideEffect` with "groupreap:
/proc quiet proof unavailable: no thread count for 66079". Diagnosis (reasoned from the kernel
source, not reproduced: this host has no /proc): Linux `task_sig` prints `Threads:\t0` when
`lock_task_sighand` fails, which happens when the task was released (reaped by its parent) after
`status` was opened. `procThreadLive` treated a count below one as a malformed line and failed the
proof, so a member that was gone made cleanup unproved. This is a real product defect in the
`PGO-V0-006` proof, not a test timing assumption.

Fix: `procThreadLive` reads a count of 0 as "no thread left", the same as a vanished status file
(`PGO-V0-016`). A missing, malformed or negative count stays a probe failure; a count above one
stays live; leader identity re-check and post-reap absence are unchanged. Regression cases in
`TestProcGroupQuietClassification`: `zombie-member-released-during-status-read` (quiet, no error)
and `zombie-member-negative-thread-count` (failure). Mutation: restoring `threads < 1` fails the new
case ("no thread count for 101"). Linux `-count=50` of the original test: NOT_RUN locally (PR CI).

## V1-1293: gitrun descendant-cleanup tests (test timing)

Diagnosis: the timeout test used a 300 ms per-operation timeout and the cancellation tests a 150 ms
cancel. Under load both fired before the shell had forked the grandchild and written its pid file,
so the group kill ended the shell first and the pid file never existed ("no such file", or the
30 s retry expiring). No grandchild existed, so nothing leaked: load delay, not a cleanup defect.
BASE under 48 burners plus a concurrent `go test ./internal/cem/...` failed 57 of 60 runs
(20 per test); 3 passed.

Fix: cancellation now waits for a parseable pid file (`cancelWhenPIDFile`), and the timeout test
replaces a new package seam, `newOperationTimer` (production: `time.NewTimer`), with an hour-long
timer reset to fire once the pid file is ready. Mutation found a second weakness: with `Setpgid`
removed from `containChild` the tests still passed, because the `sleep 30` grandchild exited on its
own inside `waitForDeath`'s 30 s bound. The three fixtures' grandchildren now sleep 300 s; with the
same mutation all three fail ("descendant N survived cleanup after 30s"). New tests: 60 of 60
(20 each) pass under the same load. `TestDescendantCleanupOnOutputOverflow` keeps a `sleep 30`
grandchild (out of this ticket's scope; same weakness).

## V1-1294: TestCALV0188_GroupListingOverrun (test timing)

Diagnosis: the branch that admits the group without a listing is the `CAL-V0-188` ESRCH fast path
in `poolGroupLiveContext`. `groupreap.Wait` SIGKILLs the group before reaping the leader (V1-1036),
so the "survivor" `sleep 60` is already dead; once launchd or init collects it, `kill(-pgid, 0)`
truthfully answers ESRCH and the pool reports the group clean with no listing. The test assumed the
killed member would still be a zombie at the post-reap probe. The product is truthful. Contention
found a second mode in the retired phases ("retired group unproved: EXIT_ZERO clean=false
listings=1"): Darwin briefly answers EPERM to signal 0 for a just-reaped, empty group (a scratch
probe saw 13 of 200 at load 95 with no member in `ps`), so the fast path was skipped and the
listing ran. That is the documented conservative fallback, not a defect.

Fix: the test overrides `poolGroupSignal` for signal 0 only (other signals are real; signal 0 sends
nothing). The retired phases answer ESRCH, the survivor phase answers EPERM (Darwin's answer for a
group with a remaining member), as `PSRTestFirstCleanupProbe` already does, so each phase exercises
exactly its branch and the listing overrun is always reached. Mutation: treating any signal-0 error
as absent (`!= nil` instead of `== syscall.ESRCH`) fails it ("surviving member admitted: clean=true
listings=0"). Under 32 burners plus a concurrent `go test ./cmd/corvint/` (load about 80): new test
20 of 20 pass; BASE 16 of 20 (3 retired-phase `listings=1`, 1 captured-phase UNKNOWN within the 1 s
pipe-drain product bound, left unchanged). Package duration (acceptance 3): not measured on a quiet
host; the shared host ran at load 35 to 186 from other lanes and `go test -count=1 ./internal/tasks/store/`
reached a 560 s detector inside `TestCALV0089_DrainStopsContinuation`; a retry at load 15 to 41
reached it again inside `TestCALV0215_CommittedDispatchExhaustion`. That clause stays open.

## Report-only tests

Same contention (32 burners, shared host load 56 to 209), race builds, run in the package directory:

- `internal/cem/verify` `TestStableS0EPublicCases`: does not reproduce; 5 of 5 pass (91 to 99 s each).
- `internal/cem/workflow` `TestDiscriminateRecordsWitnessAndReportDowngrades`: reproduces; the one
  completed run failed with `discriminates:0 notRun:2` (strong run 55.8 s) before a 170 s detector
  stopped the next. Different class (mutant wall budget under load); not fixed here, no ticket filed
  from this lane (no Tasks mutations).
- `internal/dogfoodflow` `TestAggregateOwnedProcessRetiresDescendants`: reproduces, 19 of 20 runs
  failed (19 `deadline`, 19 `cancel` subtests: "child.pid: no such file"). Same class as V1-1293:
  the 250 ms deadline and cancel fired before the shell forked. Fixed with the same pattern: cancel
  waits for the pid file, and a new `newAggregateWorkTimer` seam (production `time.NewTimer`) fires
  the work timer once the pid file exists. The deadline subtest now stops through the work timer
  while the wall-clock work deadline is still ahead, so it returns `aggregate-worker-invalid`
  rather than `aggregate-operation-deadline`; the test asserts only failure and descendant death,
  as before; the original 5 s work deadline is kept for every mode. New: 20 of 20 pass under the
same burners, and 20 of 20 under 72 burners after review. Mutation: dropping `owner.Stop()` on the
  stop path fails both subtests ("owned descendant N survived deadline/cancel").

## Line citations

The `process.go` seams move later lines by 5 and 9. `line-citations-check` caught 8 citations that
now landed on blank lines; every `internal/procgroup/process.go:N` citation in
`compat-replay-runner-v0.md` (75), `compat-trial-v0.md` (1) and a `benchmarks/selfuse-batch` comment
(1) was remapped by an exact line alignment against BASE, each to the same unchanged source line.
No citation pointed at a modified line. Citations into `gitrun.go` (`docs/DOGFOOD.md` 32-38) sit
above the new seam and are unchanged.

## Review

One Codex round (`codex exec -s read-only`, log retained in the lane scratch). Two P2 findings, both
fixed: the first draft widened test-supplied bounds, the dogfoodflow work deadline (5 s to 1 min for
every mode) and the V1-1374 run timeout and pid wait (5 s to 1 min, 3 s to 30 s). Both are restored;
the V1-1374 test now also fails at once if the run ends before the observer applied the child.
Codex found no race from the seam overrides (the only parallel tests in these packages do not use
them), confirmed the Linux `Threads: 0` reading from `fs/proc/array.c`, and checked that every
remapped citation resolves to its BASE source text.

## Rollback

Revert the change commit. `PGO-V0-016` rollback restores the `threads < 1` refusal (a member released
during the status read is again an unavailable proof and HOLD). The other changes are test seams and
tests; no store or wire format changes.
