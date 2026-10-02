# Descendant observer: retry an unavailable snapshot (issue 435)

Human-owned intent: [GitHub #435](https://github.com/beamfall/corvint/issues/435), follow-up to
#387; native ticket V1-0599. AHI-032 governs the OpenCode qualification this gate belongs to. No
qualification record, deadline in a wire contract, or authority changes.

## Finding

`procgroup`'s descendant observer takes a `ps` snapshot every 20 ms, each bounded at 250 ms. The
first periodic snapshot that failed was kept as the observation's failure, so one starved `ps` run
made the whole observation report `descendant snapshot unavailable`, even when the final sweep then
proved every observed identity gone. The final sweep had one second for all of its snapshots and
ran one extra snapshot per live descendant before signalling it, and it stopped at the first
snapshot error.

The #435 evidence shows about twenty children of the interrupted helper, started within a second
and mostly in state `R`. They match the helper's own observer: a nested `Run` spawns one `ps` per
interval, and the outer observer sees each as a descendant. Two nested observers on a host with
about ten agent sessions is the load under which a 250 ms `ps` bound is missed. This reading is an
inference from the evidence shape; the try-1 process table itself was not retained.

## Decision

- A periodic snapshot that is unavailable is a lost sample, not a survivor. It is counted and
  disclosed as a limitation of the observation (`N periodic snapshots were unavailable; ...`).
  The final sweep still has to prove every known identity gone.
- Each snapshot has its own one-second bound. The final sweep retries at the 20 ms interval inside
  a five-second settle window, and reports `descendant snapshot unavailable` only when the last
  round before the window closes still has no snapshot. A descendant that was live in the last
  round that had a snapshot reports `observed descendant remains after cleanup` as well, together
  with that round's signal failure, so a real leak is named even when the closing round had no
  snapshot.
- Tolerance is bounded. If two successful snapshots are ever more than five seconds apart, the
  observation fails with the blind interval and the bound: a run that was not observed is not
  reported clean. No consumer reads `limitations`, so the limitation alone would not protect a
  gate.
- `finish` cancels a periodic snapshot still in flight rather than waiting out its bound.
- The sweep signals a live descendant against the snapshot its round just took, instead of taking
  one more snapshot per process. The PID/start identity check before the signal is unchanged; the
  time between that check and the signal is no longer than before.

The 4096-process bound and the invalid-row failures remain failures.

## Evidence

`TestObservedDescendantSnapshotLossIsRetried` (`internal/procgroup`): two lost periodic snapshots
leave the observation `absent` and are disclosed; the final sweep succeeds on its fourth snapshot
after three unavailable ones; a snapshot unavailable for the whole settle window still fails with
exactly that reason, inside the window plus one snapshot bound; a blind interval beyond the bound
fails although the final sweep finds nothing. The lost-snapshot case observes a real child of the
root and checks it is gone afterwards. Not covered by a test: the combined reason when a live
descendant is followed by an unavailable closing round. The first version of the first case
used the test process as the observed root and failed because the observer's own `ps` runs were
its descendants, which is the nested-observer effect described above.

Independent review found the unbounded tolerance, the lost survivor reason and the in-flight
snapshot wait; the three decisions above that address them were added after it. In the degraded
case `finish` can now take up to about five seconds, past a two-second shutdown deadline; pipe
drains that completed meanwhile are still collected, and ones that did not are reported as before.

`internal/procgroup` passed three repetitions under the race detector before the review repairs
and once after them;
`internal/opencodequalification`, `internal/testacceptance` and `internal/lspstdio` passed. The
natural flake was not reproduced under real multi-agent load. One live `qualify-opencode`
campaign at commit `f6523547` (before the review repairs) against OpenCode 2.0.21 on darwin returned `PASS`; a single passing
run on a lightly loaded host does not measure the flake rate.

Rollback: revert this change's commits.
