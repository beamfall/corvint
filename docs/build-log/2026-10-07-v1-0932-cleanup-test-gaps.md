# 2026-10-07: cleanup test gaps for stale step envelopes and event-log rotation

## Intent

Ticket V1-0932 closes two test gaps found during the dispatcher and dogfood cleanup review. Both
behaviours already existed; neither had a test that would fail if it regressed.

## Findings and change

- **Stale `.stderr` envelopes (`script/dogfood-change_test.sh`).** The test seeded
  `coordination-time-impact.stderr` with stale text before a run and then checked the new content.
  That step runs again and rewrites its own envelope, so the check passed even if
  `clearStepEnvelopes` (`internal/dogfoodflow/change.go`) cleared nothing. The test now also
  seeds `orphan-step.stderr`, an envelope of a step that never runs, in both the default-repo and
  the repo evidence directories. It asserts that the next run removes that file, including a run
  that later refuses (status 1), and that a dot-file (`.operator-note.stderr`) is kept.
- **Event-log rotation (CAL-V0-058).** `TestCALV0058_EventLogRotatesOnceAt16MiB`
  (`internal/tasks/dispatch`) seeds `events.jsonl` just past the real `maxEventsBytes` (16 MiB)
  with valid event lines. It then appends through `appendEventLog` and asserts these results:
  - `events.jsonl.1` is byte-identical to the previous log.
  - The live file restarts with exactly the new event, which `ReadEvents` returns.
  - A later append below the threshold only grows the live file.
  - A second rotation replaces `events.jsonl.1`, and no third segment appears.

  No injectable threshold exists. The test uses the 16 MiB constant and writes about 32 MiB to
  `t.TempDir`. No production code changed.

## Evidence

- `bash script/dogfood-change_test.sh`: rc=0. The run printed `dogfood citation transaction
  regressions: PASS` in 35 s.
- `go test -count=1 -run TestCALV0058_EventLogRotatesOnceAt16MiB ./internal/tasks/dispatch`:
  rc=0 in 0.3 s.

## Limits

The rotation test calls `appendEventLog` directly rather than going through a running
dispatcher's `record`. `record` only wraps it with sequence and stderr printing.

## Rollback

Revert the test edits. No production behaviour or stored state changed.
