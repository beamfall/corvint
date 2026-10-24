# 2026-10-24 V1-1091: one tolerant cross-process identity comparison

PSR-V0-204 made the pool runner comparison accept a one-second Linux `btime` difference, because
the kernel recomputes `btime` on every `/proc/stat` read and an identity another process recorded
can differ from a fresh read by one second. Every other site that compared a fresh
`ProcessIdentity` with a recorded one still used exact equality (the residual named in
`2026-10-20-v1-1091-btime-jitter.md`). PSR-V0-227 moves the comparison to the supervisor
(`CompareIdentity`, `SameProcess`, `SplitLinuxIdentity`) and uses it at each such site.

## Decision

`CompareIdentity(fresh, recorded)` is three-valued. Equal non-empty identities, or `linux:`
identities with the same start tick and `btime` within one second, are the same process. Equal
start ticks with a wider `btime` gap are unknown, because a realtime clock step moves `btime` while
the process lives. Anything else, including an empty identity, is different.

Site policy:

| Site class | Sites | Unknown means |
| --- | --- | --- |
| A live process blocks the action | dispatch worker tree refresh (keeps the recorded tree with an error), pool runner, program owner, workflow remover and cleanup owner, dispatch reader leader, supervisor escape anchor, dispatch tree-may-run, CLI detached run | possibly live: `QUIESCENCE_UNPROVED`, uncertain, or may run |
| Liveness permits the action | dispatch signal, session, leader, exemption and detached supervisor; program heartbeat owner and leader; supervisor boot, owned-group anchor and observe, member signal; service pulse `RUNNING`; dispatch adoption message | not the recorded process |
| Status reader with an `UNKNOWN` state | dispatch `RunState`, `OwnerProcess` | `UNKNOWN` |
| Lease detach child | CLI lease child | falls through to the existing startup-window check |

Left exact: comparisons of two identities recorded by the same process (workflow, protocol,
`sameRun`, the escape scan's own snapshots) and executable identities in criterion binding. The
program owner-change check now uses the tolerant comparison too, so the same owner re-read with
one second of jitter is not a takeover.

No identity format, record or receipt changes; `boot_id` stays after 1.0. Rollback: revert the
commit, which restores exact equality at every site.

## AC1 measurement

`TestPSRV0227_BtimeSampleWithinTolerance` (`internal/tasks/supervisor`, Linux) read `/proc/stat`
3,000 times per run. On the WSL2 offload host (PC WSL2 linux/amd64, go1.27.3, through `heavy.sh`)
five runs (`-count=5 -v`) gave n=15,000 reads with one distinct `btime` value (1792768119) and a
spread of 0 s. This host showed no jitter in that sample; the tolerance covers hosts and moments
where the kernel's rounding does move it, and the per-site tests inject the one-second case.

## Review

The one Codex review round found that an unknown member or leader identity in the dispatch tree
refresh still dropped the member, so `supervise` could end and `killTree` report done for a live
worker; the refresh now returns an uncertainty error and keeps the recorded tree, and
`TestPSRV0227_WorkerTreeToleratesJitter` asserts it. It also found that the TCB-V0-046 route named
`submit --tree <merge>^{tree}`, which `submit` refuses as not a full OID; the route now resolves
each OID with `git rev-parse`.
