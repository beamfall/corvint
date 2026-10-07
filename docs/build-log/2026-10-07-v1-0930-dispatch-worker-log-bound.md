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
supervising tick before the activity size check:

- An oversized stream's newest bytes, starting at a line boundary, go behind a marker line into a
  temporary file.
- The temporary file is renamed over `<stream>.1`.
- The live file is then truncated to zero.

The `finished` summary reads a short live tail as the continuation of `.1`.

Retirement runs only after a tick finishes a worker. It keeps these directories:

- those the ledger records;
- those whose exit is still awaited;
- infrastructure retry launches, whose absence is the CAL-V0-104 no-spawn proof;
- any directory that changed within an hour.

Of the rest it keeps the newest 32. A pass reads at most 4,096 entries.

## Bounds

- A recorded worker keeps at most 8 MiB per stream in `.1`, and at most 8 MiB plus one tick of
  writes in the live file. Two streams give at most 2×8 MiB rotated plus 2×(8 MiB + one tick)
  live.
- At most 32 finished directories older than an hour remain, beside the recorded, protected and
  recent ones.

## Evidence

- `go test -count=1 ./internal/tasks/dispatch`: rc=0 in 100 s. This includes:
  - `TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns`, which uses a 64 KiB segment and about
    270 KiB per stream from a live worker. Both live files read 0 after one tick. Each `.1` was
    within 1 KiB under the cap, began with the marker and held the newest line. The worker's
    later append landed at offset 0, and the summary read `capped done`.
  - `TestCALV0144_FinishedWorkerDirsAreRetired`. Of 40 old directories, the newest 32 stayed. The
    recent directory and the just-finished one stayed. A directory named by an infrastructure retry
    was kept until the retry cleared.
- `go vet ./internal/tasks/dispatch`: clean.

## Limits

- Bytes appended between the size check and the truncation are lost.
- A stopped dispatcher caps nothing.
- On Linux, a child that reopens its output without append mode leaves a sparse file.
- An unrecorded crash-window tree is protected only by the one-hour quiet window.

Linux runs and live dispatcher qualification were NOT_RUN.

## Rollback

Revert the commit. Existing `.1` segments stay, removed directories stay removed, and no ledger or
event shape changes.
