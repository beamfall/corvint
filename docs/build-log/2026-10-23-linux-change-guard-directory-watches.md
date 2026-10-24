# Linux change guard watches directories, not files (V1-2482, PSR-V0-222)

Date: 2026-10-23 lane (written 2026-10-24). Ticket: V1-2482. Spec:
`docs/specs/corvint-tasks-pool-safe-reuse-v0.md` PSR-V0-222.

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

Gained: an in-place write or mode change to any singly linked file is reported at any time.
Under PSR-V0-221, a file beyond the ceiling was reported only up to the sweep.

Lost: a non-Tasks actor can create a hard link to a store file outside the observed tree after
registration, then write through that link. Neither step is reported. No Tasks writer does this.
The 45,000-receipt fixture had no file with more than one link.

## Load run

`TestPSRV0055_PoolAcquireLoad`: 45,000 receipts, 8 acquirers, 180 s, 120 s grace. During the run,
`max_user_watches` was lowered to the host's existing use (1,140) plus 4,000. Every call
completed with no error code (`workerFailures` 0), so the peak over all nine processes stayed below
4,000 watches. One PSR-V0-221 guard alone takes about 4,300. The run is UNQUALIFIED: load was 9.35
on 4 CPUs. The 600 s, 45,000-receipt run on the WSL2 quiet host is NEEDS-HOST.
