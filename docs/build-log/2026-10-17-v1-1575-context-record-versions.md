# V1-1575: a newer context record is never overwritten

Date: 2026-10-17. Ticket: V1-1575. Spec: `docs/specs/corvint-tasks-attempt-runner-v0.md`,
ATR-V0-054..055, accepted under the owner's standing auto-accept of lane requirements (2026-10-11).
No accepted requirement is amended.

## Problem

`contextFor` treated every unreadable `context.json` the same way: it alerted and replaced it on the
next tick. A record another build wrote with a later `taskman-dispatch-context` version reads as
`UNSUPPORTED_VERSION`, so an older dispatcher destroyed state it could not interpret.

## Decisions

- A record of another profile version sets a per-process `foreign` flag. The worker's record is
  never written, one alert names it, and no crossing is recorded or reported. The hard stop still
  follows the in-memory estimate, because stopping a worker past its hard budget does not depend on
  the record. `dispatch status` already showed `UNKNOWN` for any unreadable record; a test now pins
  that for the newer-version case.
- A damaged record of this version (not JSON, truncated, no profile, non-canonical) keeps the
  ATR-V0-033 behaviour, now specified as ATR-V0-055: one alert, then a fresh record.

## Evidence

- `TestATRV0054_NewerContextRecordIsPreserved` fails without the change: the newer record is
  overwritten with a `/0` record holding three crossings. It passes with the change.
- `TestATRV0055_CorruptContextRecordIsReplacedAfterOneAlert` and
  `TestATRV0054_DispatchStatusShowsNewerRecordUnknown` pass.
