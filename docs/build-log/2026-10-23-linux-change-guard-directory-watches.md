# Linux change guard watches directories, not files (V1-2482, PSR-V0-222..223)

Date: 2026-10-23 lane (written 2026-10-24). Ticket: V1-2482. Spec:
`docs/specs/corvint-tasks-pool-safe-reuse-v0.md` PSR-V0-222 and PSR-V0-223.

## Cause

The 2026-10-23 WSL2 run (`2026-10-23-wsl2-quiet-host-measurements.md`) measured origin/main
`0641ad2e`, which predates PSR-V0-221 (`0ee8a4b1`). At that sha `authority.WatchChanges` added one
inotify watch per store path, about 90,000 per guard at 45,000 receipts. Ten concurrent guards
exceed WSL2's 524,288 `max_user_watches`. PSR-V0-221 then capped a guard's regular-file watches at a
32nd of the allowance (at most 4,096). That made the count independent of receipts above the cap,
but each guard still held thousands of watches.

## Measurement (4 vCPU Firecracker VM, kernel 6.18.44, allowance 130,016)

Inotify watches held by one live guard, counted from the `inotify wd:` lines in
`/proc/self/fdinfo`, over the `historyStoreAt` store fixture:

| Receipts | Store dirs | Store files | origin/main (PSR-V0-221) | This change |
| --- | --- | --- | --- | --- |
| 1,000 | 259 | 2,047 | 2,312 | 265 |
| 45,000 | 264 | 90,047 | 4,333 | 270 |

The directory count is set by the layout: eight fixed directories plus at most 256 `requests/xx`
shards. The receipts directory is flat.

## Decision

Register no inotify watch for a regular file that has a single link and sits in a directory the
guard already watches with the content mask. On Linux, a directory watch reports `IN_MODIFY`,
`IN_ATTRIB` and `IN_CLOSE_WRITE` for its entries, so a file watch adds nothing for that file. The
one exception is a write through a second link outside the watched directory. A file with
`Nlink > 1`, or one whose parent is not content-watched, is tracked by stat identity and re-read
by `Sweep`, the PSR-V0-221 fallback. Remove the file ceiling and the `finish` hook.

Rejected:
- Keep the ceiling and lower it. The count is still not bounded by directories, and files beyond
  the ceiling still miss in-place writes made after the sweep.
- Watch only head files (journal head, pools). The guard's contract (CAL-V0-026) covers every
  observed path, including intent projections. Narrowing it would change what `SNAPSHOT_MOVED`
  means.

## Change in guarantees

The first version of this change accepted a regression: the kernel reports an entry's events to
its directory only when they go through that entry, so a non-Tasks actor's write or mode change
through a hard link made outside the observed tree after registration, or through a descriptor
opened before registration via a since-removed link or as an `O_TMPFILE`, went unreported. The
independent review confirmed this on Linux 6.18 with a probe: a per-file watch saw it and the
parent watch did not. Writes through `mmap` are reported by neither kind of watch.

The review also found a latent mask downgrade. Re-adding a content-watched directory as an
ancestor would have replaced its mask, and its entries' events would have stopped. A directory
already watched with the content mask is now not registered again.

## Review repair (PR #1657, PSR-V0-223)

The Codex review raised the accepted regression as two P2 findings: a singly linked file had
neither a watch nor a stat record, and a multiply linked file was re-read only by `Sweep`, so a
write through its other link after the sweep and before the writer-lock `CheckEvents` went
unreported. The orchestrator ruled both must be fixed without giving up the bounded watch count.

Decision: record a stat identity (device, inode, mode, link count, size, mtime, ctime) for every
regular file, add none of them as an inotify watch, and re-read every identity in `Check` and
`Sweep`. A link made to a file changes its link count and ctime, and a write through any link or
descriptor changes its size, mtime or ctime, so both findings' cases now report `SNAPSHOT_MOVED`.
Files with another link at registration, or with a parent not content-watched, are also re-read
by every poll, including `CheckEvents` under the writer lock; Tasks makes no such files, so the
lock's cost stays independent of the store (CAL-V0-026). The first repair commit returned from
`poll` on `EAGAIN` before that re-read; the new second-link test caught it.

Rejected: per-file watches for every file (the unbounded count this ticket removes); re-reading
every file under the writer lock (cost grows with receipts, CAL-V0-026); `fstatat` through a
directory descriptor, about 30% cheaper per file (1.6 µs against 2.3 µs on WSL2) but it needs
per-architecture raw syscalls without `golang.org/x/sys`.

Cost, opt-in `TestPSRV0223_ChangeGuardSweepCost` on WSL2 (Linux 6.18.33, 32 vCPU, ext4, medians
of 21 calls):

| Code | Receipts / files | Register | `Check` | `Sweep` | `CheckEvents` (max) |
| --- | --- | --- | --- | --- | --- |
| This repair | 10,000 / 20,057 | 135 ms | 47 ms | 47 ms | 540 ns (13 µs) |
| This repair | 45,000 / 90,057 | 690 ms | 258 ms | 250 ms | 550 ns (15 µs) |
| origin/main `368b80d4` (PSR-V0-221) | 10,000 | 189 ms | 42 ms | 45 ms | 560 ns |
| origin/main `368b80d4` (PSR-V0-221) | 45,000 | 956 ms | 256 ms | 256 ms | 590 ns |
| PSR-V0-222 alone (`0ff27432`) | 10,000 | 96 ms | 0.6 µs | 20 ns | 550 ns |
| PSR-V0-222 alone (`0ff27432`) | 45,000 | 409 ms | 0.6 µs | 20 ns | 550 ns |

So the full re-read costs what PSR-V0-221 already paid to sweep the files beyond its ceiling,
which at 45,000 receipts was nearly every file; the cost under the lock is unchanged. The watch
count is unchanged: 9 watches over 11 files and 9 over 5,001 in
`TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories`. Both new tests
(`TestPSRV0223_ChangeGuardSeesAWriteThroughAnAliasLink`,
`TestPSRV0223_ChangeGuardSeesASecondLinkWriteAfterTheSweep`) fail at `0ff27432` in every mutation
case and pass with the repair (WSL2).

Residual limits, each by an actor outside Tasks: a link made after `Sweep` read the file, and a
write through it before `CheckEvents`, are not seen under the lock (a later `Check` sees them),
the interval Darwin leaves for a file beyond its descriptor budget; a change that restores size and
link count within the filesystem's timestamp granularity is not seen on kernels without multigrain
timestamps (before 6.13); `mmap` writes are not seen. Creating a link alone now reports
`SNAPSHOT_MOVED`, which is conservative. Inference, not measured: a lease command runs several
`Check` calls, each now a full re-read, as under PSR-V0-221.

## Load run

`TestPSRV0055_PoolAcquireLoad`: 45,000 receipts, 8 acquirers, 180 s, 120 s grace. During the run,
`max_user_watches` was lowered to the host's existing use (1,140) plus 4,000. Every call
completed with no error code (`workerFailures` 0), so the peak over all nine processes stayed below
4,000 watches. One PSR-V0-221 guard alone takes about 4,300. The run is UNQUALIFIED: load was 9.35
on 4 CPUs. The 600 s, 45,000-receipt run on the WSL2 quiet host is NEEDS-HOST.
