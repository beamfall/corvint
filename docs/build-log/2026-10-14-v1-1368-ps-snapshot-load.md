# 2026-10-14 procgroup descendant snapshot bound on a loaded host (V1-1368)

Human-owned intent: native ticket V1-1368, reported by the V1-1364 lane (PR #877). On a darwin host
at load 50-60, `/bin/ps` took about 1.5 s, so the 1 s per-snapshot bound in
`internal/procgroup/descendants.go` refused every observed `Run` start. That failed the
`internal/update` version probes (TestUPDV0005, TestUPDV0009) and BBF attempts with `descendant
snapshot unavailable`. Governing contract: `docs/specs/process-group-owner-v0.md` (`PGO-V0-010`
owns the POSIX descendant snapshot). No accepted spec states the 1 s bound: it appears only in
`2026-10-01-descendant-snapshot-retry.md`, so this entry is the record and no requirement ID is
assigned. No wire field, report string or caller mapping changes.

## Measurement

`/bin/ps -axo pid=,ppid=,lstart=,stat=` with `LC_ALL=C TZ=UTC`, timed by zsh `$EPOCHREALTIME`, on
a 12-CPU darwin host that six other lanes were already loading. Contention came from `N x 12`
`/usr/bin/yes` loops, started by a lane script and killed by its EXIT trap. The 1-minute load
average was recorded with each sample.

| Run | Busy loops | Load (1 min) | n | mean ms | p50 | p90 | max | > 1 s | > 2.5 s |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| baseline (not quiet) | 0 | 25-26 | 20 | 67 | 44 | 235 | 274 | 0 | 0 |
| 1x | 12 | 25-41 | 30 | 66 | 65 | 96 | 108 | 0 | 0 |
| 2x | 24 | 97-112 | 40 | 443 | 126 | 1289 | 5546 | 5 | 1 |
| 4x | 48 | 47-132 | 40 | 671 | 199 | 1960 | 4604 | 9 | 3 |
| 1x, exit status recorded | 12 | 52-55 | 150 | 198 | 86 | 399 | 2551 | 6 | 1 |
| 1x, Go `descendantSnapshot` probe | 12 | ~50 | 200 | 164 | 85 | 242 | 2338 | 5 | 0 |

All 350 exit-status-recorded samples succeeded (exit 0, or no error and the caller's own pid
present in the Go probe). Slow samples were mostly isolated: the samples over 1 s were 1, 12, 19,
20 and 37 of 40 at 2x, and 1, 2, 8, 12, 21, 23, 24, 34 and 37 of 40 at 4x. A truly quiet host was not
available, so the baseline row is a host at load about 25. The ticket's 1.5 s figure at load 50-60
is between the 1x and 2x rows.

## Decision

- Per-snapshot bound `descendantSnapshotTimeout = descendantSettle / 2` (2.5 s, was 1 s). This
  covers the measured p90 at every contention level with headroom and leaves room in the 5 s
  settle window for a second attempt, which gets the remainder of the window (just under 2.5 s
  after a full first attempt and the 20 ms retry delay).
- The first snapshot, which binds the leader identity, now retries at the 20 ms interval inside
  the 5 s settle window, as the final sweep already did. Each attempt gets `min(2.5 s, remaining
  window)`. When the window closes it returns the last failure unchanged. Caller cancellation
  (the `Run` context) stops further retries but not the attempt in flight, so a cancelled start
  waits at most one snapshot bound and gets the outcome a single attempt would have given.
- Unchanged: the 5 s settle window, the 5 s blind-interval (`descendantMaxGap`) bound, periodic
  loss accounting, every error string and every caller's mapping. A start that never gets a
  snapshot is still `process-after-start-failed: descendant snapshot unavailable` with no
  `DescendantObservation`, and BBF maps that to `DescendantsGone=false`. A final sweep that never
  gets one is still `Absent=false` with the `descendant snapshot unavailable` failure. Neither is
  ever an empty descendant set.

Worst-case time to the unavailable outcome for a hung `ps` (stated bound, AC 2): the observer
refuses startup at 5 s plus the 100 ms `WaitDelay` (was about 1.1 s); `Run` then performs its
normal shutdown of the started process, bounded by the caller's `ShutdownTimeout`, before it
returns `process-after-start-failed`. The final sweep is 5 s plus 100 ms (unchanged). A periodic
snapshot is bounded at 2.5 s and is cancelled when `finish` starts. A start cancelled by the caller
waits at most 2.5 s plus 100 ms for the snapshot in flight (was 1.1 s).

Rejected options:
- **1 s with retries.** A host where `ps` takes a steady 1.5 s (the ticket's case) never
  completes a snapshot.
- **Fixed 5 s, the whole settle window.** This was tried first in this lane. Under 1x-2x
  contention, BBF failed with `descendant snapshots were unavailable for 5.393s, beyond the 5s
  observation bound`, because one timed-out periodic snapshot then always exceeds the gap bound.
  The final sweep also got a single attempt. The measured tail is isolated, so a second attempt
  helps more than a longer single one.
- **More than 5 s.** It cannot complete inside the settle window or the gap bound, and widening
  those changes the caller-visible cleanup bounds.
- **Adaptive, latency-tracking bound.** It adds state and makes the worst case harder to state.
  The numbers do not need it.

## Failure modes

- A hung or failing `ps` refuses an observed start after about 5 s instead of about 1 s. In a
  sandbox where `/bin/ps` is denied (V1-0568), every observed `Run` start now spends the 5 s
  window before it refuses. This is a retained cost.
- The first-snapshot retries are not counted in the periodic-loss limitation. A descendant that
  forks and detaches before the first successful snapshot falls under the existing "fast
  detachment ... between snapshots" limitation, inside a window no longer than the gap bound.
- Any snapshot error is retried, including an invalid row or an overflow, as the final sweep
  already does. The last error is still returned.

## Acceptance evidence

- `TestV11368SnapshotBoundUnderLoad` (`internal/procgroup`) has five cases: a 1.2 s snapshot
  through the real command path binds the leader (the 1.5 s version failed under the old 1 s
  bound, checked); an unavailable first snapshot is retried; a cancelled start makes exactly one
  attempt and is refused as unavailable; a hung command (`/bin/sleep 60` through
  `descendantSnapshotCommand`) refuses the start with exactly `descendant snapshot unavailable`
  within [5 s, 7 s]; a hung final sweep reports `Absent=false` with that failure within the
  same bound. The `-race` run passed.
- `TestV10689BlockedStartupJoins` now scales its waits from `descendantSnapshotTimeout` (+2 s and
  +3 s, which were 3 s and 4 s at 1 s).
- Without added contention (host shared with other lanes): `internal/procgroup`, `internal/update` and `internal/behaviorfalsify` pass.
- Under 1x contention, A/B against BASE test binaries (load 44-50): `internal/update` at BASE
  failed in both runs (TestUPDV0009 twice, TestUPDV0005, TestUPDV0004, TestUPDV0007,
  TestUPDV0008), while this change passed both runs, including TestUPDV0005 and TestUPDV0009.
  `internal/behaviorfalsify` passed for both (load 47-65).
- Under 2x contention (load 44-77): `internal/update` at BASE failed TestUPDV0002, 0004 (two
  cases), 0005, 0006, 0007 and 0009, while this change passed. `internal/behaviorfalsify` with
  this change passed.
- `internal/procgroup` under contention and under `-race` shows timing flakes in tests that do not
  observe descendants (TestRunProcessLifecycleMatrix, TestV11055NoGroupOperationAfterConcurrentRelease),
  at BASE and here; each passed when rerun alone.
  TestObservedDescendantCancellationReapsEscapedChild failed 4 of 10 at BASE and 4 of 10 here at
  load about 50. Its 100 ms window before cancel is shorter than a loaded `ps`. This is
  pre-existing and is not changed here.
- `go vet ./internal/procgroup/` and `gofmt -l` are clean.

## Independent review

One Codex read-only round over the change found no path from an unavailable snapshot to an empty
descendant set and no timer or goroutine leak. Its three findings were fixed: caller cancellation
could wait through the whole 5 s retry window (now it stops after the attempt in flight); the
stated worst case omitted `Run`'s shutdown allowance and overstated the second attempt; the slow
fixture had only about 1 s of margin under load (it now sleeps 1.2 s).

## Retained limits

- At load near 100 and above (the 2x and 4x rows), isolated `ps` runs over 5 s still fail closed:
  `descendant snapshot unavailable` or the gap bound. That is the intended outcome beyond the
  design point, not a regression. 
- A truly quiet host and Linux were not measured (NOT_RUN). The live `corvint-update check` was
  NOT_RUN.
- The 112 packages that `corvint affected` selected as dependents were not run beyond the three
  focused packages. The bound changes behaviour only when `ps` is slow or failing.

Rollback: revert this change's commits. The constant and the first-snapshot loop are
self-contained.
