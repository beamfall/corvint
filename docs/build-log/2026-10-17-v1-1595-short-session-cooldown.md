# V1-1595: short-session cooldown and SESSION_CHURN

Date: 2026-10-17. Base: `072a306f1dbd89c2f29b20a3434a0f25864ebbf7` (origin/main).
Ticket: V1-1595, from GitHub issue 1038. Spec: `docs/specs/corvint-tasks-attempt-runner-v0.md`,
ATR-V0-060..062, accepted under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Decisions

- State. The streaks live in a sidecar record, `session-churn.json` (profile
  `taskman-dispatch-session-churn/0`), not in the ledger. The ledger's sessions carry no end time,
  and a ledger profile bump would need adopt-only migration. The sidecar keeps streaks across a
  restart and lets `dispatch status`, a separate process, read the hold.
- Event. A hold is a `cooldown` event with `reason=SHORT_SESSIONS`. That reuses the closed
  CAL-V0-058 event vocabulary rather than adding a kind.
- Scope. A hold only removes the ticket from launch selection. Running workers are never touched,
  and lane keys are not tracked.
- Doctor. The doctor reads the journal, not the dispatch configuration. SESSION_CHURN therefore uses
  ended attempt generations and a fixed 300-second bound with the doctor's threshold of three. To
  keep one alert per problem, it skips tickets already reported as NO_PROGRESS_HANDOFF. Three
  existing doctor fixtures (rapid hand-offs) would otherwise report both.
- Concurrency with the doctor lane. The doctor hunk is one line in `doctorStoreFindings`. The
  detector and its remedy live in `doctor_churn.go`, so the remedy map is not edited.

## Evidence

- `TestATRV0060_ShortSessionsHoldThenLongSessionResets`: three short sessions (median 15 s) emit one
  hold with the expected detail, no launch until `holdSeconds` pass, then a long session removes
  the streak.
- `TestATRV0060_AbsentSettingIsUnchangedAndRoleOverrides`: no setting means no hold and no record,
  and a role's setting wins over the global one.
- `TestATRV0060_ReloadLiftsHoldAndRunningWorkersContinue`: during a hold another ticket's worker
  launches. A reload that removes the setting removes the record and launches the held ticket,
  with the running worker unchanged.
- `TestATRV0061_RecordVersions`: a newer record is kept, and a malformed one is replaced.
- `TestATRV0060_ShortSessionsValidation`, `TestATRV0061_DispatchStatusShowsShortSessionHold`,
  `TestATRV0062_DoctorSessionChurn`, and the CAL-V0-131 format row.
- Linux: NOT_RUN.
