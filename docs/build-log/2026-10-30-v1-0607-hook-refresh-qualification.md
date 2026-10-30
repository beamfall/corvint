# Hook context delivery and stale-index recovery qualification — V1-0607

Evidence: `docs/experiments/v1-0607-hook-refresh-qualification-20261030.json`, drivers under
`docs/experiments/v1-0607/`. Requirement added: `IDX-SNAP-V0-515` (standing).

## Tuple

Core `1.0.0-rc.3.dev (build 7, commit 93b9f5b08)`, built from this worktree with the Makefile
ldflags; for the binary/index-key case the same source built with `-X main.build=8`, so only the
executable digest (the snapshot engine key) changes. Claude Code plugin 0.3.6, adapter wire 0.1.0,
`hooks.json` UserPromptSubmit timeout 2 s, Core event deadline 1600 ms. Host: Unraid
linux/amd64, 12 CPUs, Linux 6.18.38. Claude Code reports no version to hooks
(`unreported-by-hook-api`); `claude` 2.1.311 is installed but was not driven. Fixture: a clean
linked worktree (`git worktree add`) of a `--no-local` corvint clone, 10,217 tracked files, 0
dirty paths. Each event is the real hook command with a Claude Code payload under `timeout 2`.

## Idle qualification (load average 0.8 to 1.8)

| Step | Hook ms | Outcome |
|---|---|---|
| Cold start, no snapshot | 1671 | `dogfood-event-index-snapshot-stale`, refresh started |
| 4 concurrent hooks during the refresh | 1635 | all `already running`; one worker |
| Refresh | 7330 wait | snapshot and `build-cost.json` published |
| Next hook | 766 | context; packet tree = HEAD tree |
| Commit in the linked worktree | 147 | stale, refresh started (the recorded cost skips the in-memory build) |
| Refresh, next hook | 4548 wait, 621 | context, packet tree = new HEAD tree |
| New binary (engine key) | 159 | stale, refresh started |
| Refresh, next hook | 8698 wait, 731 | context; the old binary is still served from its own snapshot |
| Commit, refresh killed by SIGTERM | 154 | no partial snapshot, no surviving worker, lock free |
| Retry, refresh, next hook | 163, 4779 wait, 640 | context, packet tree = HEAD tree |
| 20 warm hooks | p50 638, p95 764, max 774 | 20/20 context |

At idle each tree or engine change costs exactly one stale fallback. The next hook after the
refresh supplies current-tree context with no manual index command. Refresh is single-flight,
and cancellation leaves neither a partial snapshot nor an unowned process. The cold hook spends
about 1.67 s of its 2 s host limit, because with no recorded build cost it tries the in-memory
build first. Later stale hooks return in about 150 ms.

## Loaded qualification: reproduced avoidable repeated context loss

With 36 busy loops at nice 0 (load average rising to 38), every background refresh of the base
build ran to the hard 60-second bound (waits of 56 to 60 s) and published nothing. Every later
hook was stale again and started a new, equally doomed writer: 0 of 20 warm hooks got context
(18 stale, 2 `adapter-host-kill-deadline`). The stale notice said only `Background index refresh
started`, so the repeated loss looked like the normal cold start.

Cost of a cold refresh measured directly (`index --if-stale`, `GOMAXPROCS=1`, 150 s cap):

| Load | Writer nice | Result |
|---|---|---|
| none | 10 | 8.1 s wall, 8.0 CPU-s |
| 36 loops at nice 0 | 10 | killed at 150 s, unfinished |
| 36 loops at nice 0 | 0 | 88 s (beyond the 60 s bound) |
| 36 loops at nice 15 | 10 | 27 s |

So the loss comes from host load. No bounded writer priority recovers it under sustained 3x
normal-priority oversubscription, and lowering the load's priority (V1-2440) recovers it. A
second 36-loop run finished its refresh in 35 s, so the outcome near the bound depends on
scheduling. With a fresh snapshot, loaded hooks still hit the bounded
`dogfood-event-deadline` refusal at 1.7 s: that is the expected time-bound refusal of
LCP-V0-008, not a defect.

## Decision: name the repeated expiry, keep the bounds

Fork: change the accepted `IDX-SNAP-V0-033` bounds (nice, 60 s wall) so that the writer finishes
under load, or keep them and make the repeated loss visible. Decision: keep the bounds, because
they are owner-accepted (decision 0517) and protect interactive work, and add `IDX-SNAP-V0-515`.
A writer that exits nonzero records `expired` or `failed` in its lock metadata. A killed writer
leaves its running record. The next hook that starts a writer reads that record under the lock
it holds, and its notice says that the previous refresh ended without publishing, its closed
cause, and that later events also get no context until one completes. Each started writer overwrites
the record, so a notice describes only the writer just before it, and a writer that completes
clears the record.

Live witness with this change (72 loops at nice 0): two writers were hard-killed at 61.0 and
60.4 s. Each next hook's notice named the previous writer's start second and `it stopped before
recording an outcome (its 60-second hard limit, ...)`. Under starvation the build never reaches the 55 s
cancel. Once the load was removed, the writer in flight published in 8.2 s, and the next three
hooks supplied context (608 to 749 ms) with no manual index command.

Open for the owner (not implemented): a CPU-time budget in place of the 60 s wall bound, under a
longer hard wall cap. A starved nice +10 writer then costs almost nothing while it waits, and it
converges once CPU frees. It changes `IDX-SNAP-V0-033` and needs an owner decision.

## Acceptance status

- AC1 (reproducible request, versions, durations, size and load): met on linux/amd64 for the
  reproduction. The original incidents' durations and load remain NOT_OBSERVED.
- AC2 (remediation without weakening the deadline, read-only state or fallback honesty): met;
  bounds and the read-only path are unchanged, and the fallback now names repeated expiry.
- AC3 (clean linked worktree after a commit and a binary change): met on linux/amd64; darwin,
  Windows and Codex NOT_RUN on this host.
- AC4 (remediate repeated avoidable fallback): PARTIAL. Recovery is proved once the load ends,
  and the repeated loss is now visible. Recovery during sustained normal-priority 3x load needs
  V1-2440 or the owner amendment above.
- AC5 (bounded deadlines, honest fallback, read-only, independent review, latency acceptance):
  PARTIAL. Latency is recorded above; the independent review is the orchestrator's Codex pass,
  and live Claude Code app dispatch is NOT_RUN.

Focused tests: `go test -run 'Snapshot|Adapter|Claude|Codex|Refresh|Deadline|Dogfood|HostLifecycle|Compaction|Pi' -skip JavaScript ./cmd/corvint`,
`./conformance/host-lifecycle-v1`, and `-run 'Refresh|Snapshot|BuildCost' ./internal/contextindex`
all PASS. The JavaScript host tests are NOT_RUN because this host has no Node. The other 71
`corvint affected` units were selected as unbounded readers (`os.Getwd`) and are NOT_RUN.
