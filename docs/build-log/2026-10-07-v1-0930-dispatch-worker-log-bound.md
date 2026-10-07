# 2026-10-07: dispatcher worker log cap and finished worker directory retirement (V1-0930)

## Intent

Ticket V1-0930 found two unbounded growths under a dispatcher's `workers/` directory:

- each worker's `stdout.log` and `stderr.log`;
- the finished worker directories themselves, which were never removed.

The owner asked to keep the dispatcher lean and efficient. The change adds proposed CAL-V0-143 and
CAL-V0-144 (`docs/specs/corvint-tasks-agent-leases-v0.md`), pending owner acceptance.

## Decision

A worker is a session leader. It holds its own append-mode descriptors and can outlive the
dispatcher. A pipe relay would tie the worker's output to the dispatcher's life, and a rename
would leave the worker writing to an unlinked file. The cap is therefore applied in place, on each
supervising tick right after the activity size check, so output that is then cut still counts as
activity:

- An oversized stream's newest bytes, starting at a line boundary, go behind a marker line into a
  temporary file.
- The temporary file is renamed over `<stream>.1`.
- The live file is then truncated to zero.

The `finished` summary reads a short live tail as the continuation of `.1`.

Retirement runs only after a tick finishes a worker. It keeps these directories:

- those the ledger records;
- those whose exit is still awaited;
- infrastructure retry launches, whose absence is the CAL-V0-104 no-spawn proof;
- any directory that changed within an hour;
- any directory whose `leader` file names a tree that may still run. Launch writes this file with
  the leader's PID and start identity. The tree may run if the leader still has that identity, or
  if a live process remains in its process group or session.

Of the rest it keeps the newest 32; a protected directory takes none of those slots. When a
process exits between the process-table read and its session read, the table is read once more
and both reads count. A pass reads at most 4,096 entries.

Removal is confirmed across two passes. The first pass that finds a directory removable writes a
mark under `workers/.retiring/` with the time and the directory's age. A pass at least a minute
later, and so in a later tick, removes the directory only if its own reads again find it
removable with the same age. Any other finding clears the mark.

## Bounds

- A recorded worker keeps at most 8 MiB per stream in `.1`, and at most 8 MiB plus one tick of
  writes in the live file. Two streams give at most 2×8 MiB rotated plus 2×(8 MiB + one tick)
  live.
- At most 32 finished directories older than an hour remain after a confirming pass, beside the
  recorded, protected and recent ones. Marked directories also remain until that pass.

## Evidence

- `go test -count=1 ./internal/tasks/dispatch`: rc=0 in 104 s, after the review fixes. This includes:
  - `TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns`, which uses a 64 KiB segment and about
    270 KiB per stream from a live worker. Both live files read 0 after one tick. Each `.1` was
    within 1 KiB under the cap, began with the marker and held the newest line. The worker's
    later append landed at offset 0, and the summary read `capped done`.
  - `TestCALV0143_CappedOutputCountsAsActivity`. A burst that a tick then cut counts as activity,
    and an unchanged log does not. The test fails when the cut precedes the comparison.
  - `TestCALV0144_FinishedWorkerDirsAreRetired`. Of 40 old directories, the newest 32 stayed. The
    recent directory and the just-finished one stayed, and launch wrote a `leader` file. A
    directory named by an infrastructure retry was kept until the retry cleared. Unrecorded
    directories were kept in these cases:
    - the leader is live;
    - the leader exited but a child still runs in its session;
    - the `leader` file is malformed.

    The directory of a gone leader was retired. Dropping the group and session check makes the
    test fail.
- `go vet ./internal/tasks/dispatch`: clean. `GOOS=windows` and `GOOS=linux` vet are also clean.

## Independent review

One Codex review (gpt-6-astra, read-only) of `5808c26c..f34c2386` requested changes. It raised
four major findings and one minor finding. Two of them concern this ticket:

- An idle kill could follow a cut. This is fixed: the comparison now precedes the cut.
- A quiet unrecorded live tree could lose its directory. This is fixed through the `leader` file.
  Two residual cases remain: a crash between the spawn and the `leader` write, and directories
  from older builds.

A third finding, that the bounded scan could starve retirement, is documented with its precise
condition in CAL-V0-144 rather than fixed with a cursor.

The one permitted re-review, of `643e7cf8`, again requested changes. Two findings concerned this
ticket, and both were fixed afterwards without a further review:

- A session member that exited after forking, between the table read and its session read, was
  skipped. The table is now read once more and both reads are joined. Failing closed on every
  such gap was tried and rejected: this host showed five exiting processes in each table read, so
  retirement would never run. A fork-and-exit chain across both reads remains a documented
  residual. `TestCALV0144_IncompleteSessionReadIsRetaken` fails against the previous code.
- Protected directories used up the 32 retention slots. Protection is now decided before a
  directory is counted. `TestCALV0144_ProtectedDirsTakeNoRetentionSlot` fails against the
  previous code.

The coordinator's Codex review of `73a42cf2` found that a chain forking and exiting through both
reads still left the snapshot looking complete (MAJOR), and that the retake test never checked
the union (MINOR). On the coordinator's decision, removal now needs a confirming pass, as above;
failing closed on every gap stays rejected for the reason given. The retake test now asserts that
groups and sessions seen in only the first read, and in only the second, both survive.
`TestCALV0144_RemovalNeedsAConfirmingPass` and `TestCALV0144_LiveMemberOnConfirmingPassKeepsDir`
cover the confirmation. Each of the three new guards was mutated away in turn: removing on first
sight, keeping a mark a pass did not confirm, and resetting the maps before the second read. Each
mutation made its test fail.

## Limits

- Bytes appended between the size check and the truncation are lost.
- A stopped dispatcher caps nothing.
- On Linux, a child that reopens its output without append mode leaves a sparse file.
- A tree with no `leader` file is protected only by the one-hour quiet window. This covers a crash
  between the spawn and the `leader` write, and a directory from an older build.
- A pass sees only the first 4,096 directory entries.
- A process chain that forks and exits through all four process-table reads across the two
  confirming passes, outside the leader's process group, is not seen.

Linux runs and live dispatcher qualification were NOT_RUN.

## Rollback

Revert the commit. Existing `.1` segments stay, removed directories stay removed, and no ledger or
event shape changes.
