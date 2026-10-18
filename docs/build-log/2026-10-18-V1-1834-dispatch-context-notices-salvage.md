# V1-1834: host-neutral context notices and on-demand salvage (issue 1148)

Ticket V1-1834, GitHub issue 1148. Adds ATR-V0-125..127 to
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md). Accepted under the
owner's standing auto-accept of lane requirements (2026-10-11); this is not a new owner decision.

## Decisions

- **A polled file is the host-neutral path.** Every host can read a file, but only claude-code has a
  hook. A host opts in with `contextNotices: "file"`. The dispatcher appends one line per crossed
  WARN or HANDOFF level to `workers/<id>/context-notices.txt`, named as
  `CORVINT_DISPATCH_CONTEXT_NOTICE_FILE`. It reuses the ATR-V0-101 `context-delivered-<LEVEL>`
  markers, so the hook and the file never repeat a level. The append comes before the marker, so a
  failure is retried and a crash repeats a line at most once (at-least-once delivery).
- **The file is opened beneath the worker directory.** The session can write that directory. The
  dispatcher refuses a non-regular notice file and opens it through `os.OpenRoot`, so it never
  follows a link out or blocks on a FIFO.
- **Salvage is a read-side verb.** `dispatch salvage --attempt A` resolves the attempt's lease holder
  (a native attempt keeps it after release) to a worker of the program, running or stopped. It writes
  the note through the same `buildHandoffNote` bounds and secret screen as the hard stop
  (ATR-V0-104). It never writes the store, so it needs no request ID and races no lease.
- **`stopped` is `finished`.** The issue asks the `launched` and `stopped` events to record the
  warnings and the salvage. The worker-stop event is `finished`; the dispatcher's `stopped` event
  names no worker. The fields therefore go on `finished`: `contextDelivery`, `contextDelivered` and
  `salvage` (`HARD_STOP`, `ON_DEMAND` or `NONE`). The `launched` event gains `contextDelivery` and
  `handoffNote`.

## Evidence

`TestATRV0125_NoticeFileDeliversOnAnyHost`, `TestATRV0125_NoticeFileValidationAndRetry` and
`TestATRV0127_FinishedRecordsSalvage` (`internal/tasks/dispatch`). In `internal/tasks/cli`,
`TestATRV0126_SalvageAttemptWritesScreenedNote` runs against a real store, and the CAL-V0-131
output-only row covers the result profile. Live host qualification and Linux are NOT_RUN.
