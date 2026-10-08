# 2026-10-08 — A failed dispatcher tick ledger save fails the tick (V1-0672, CAL-V0-192)

Ticket: V1-0672 (Dispatcher tick silently ignores a failed private ledger save). Spec:
`docs/specs/corvint-tasks-agent-leases-v0.md`, new proposed `CAL-V0-192`. Base
`ecfff8e063914c9e59c83d1efada9e9b1119a30e`.

## Defect at the base

`Dispatcher.tick` saved the ledger in a deferred function and kept only `tickSaved = save(...) == nil`.
That flag already kept the idle gate unarmed and the controlled boundary unrecorded, but `Tick` and
`Run` returned success. A scratch reproduction at the base (Darwin, Go 1.27.1, program directory
chmod 0500, `CreateTemp` there confirmed failing, ticket status changed, one tick) logged:
`tick err=<nil> tickSaved=false inMemory="IN_PROGRESS|NONE||" stateUnchanged=true
stateEventPublished=true` — the `state` event and in-memory baseline advanced while `state.json`
stayed byte-identical.

## Change

`tick` has a named error result. When the final save fails it joins an error that matches the new
`ErrLedgerUnsaved`, names `state.json` as not confirmed durable (it may still hold the previous
save, or the new bytes when only the directory sync failed; independent review P2) and wraps the original save error (`%w`), so
`errors.Is(err, fs.ErrPermission)` and an injected sync failure's text survive. Nothing else
changes: the in-memory ledger and its workers stay authoritative, no snapshot is reloaded, `Run`
raises its existing `tick failed: ...` alert and keeps supervising, and the next tick's save writes
the current ledger whole (an unchanged ledger is still not rewritten, CAL-V0-139). A bounded run
returns the error, so `dispatch --ticks N` reports `ERROR` with the cause as a warning.

## Evidence

- Failing before (fix reverted, test sentinel shimmed in): both new tests fail at the base,
  `tick with an unwritable ledger returned <nil>, want ErrLedgerUnsaved` and `tick 0 returned <nil>,
  want ErrLedgerUnsaved`.
- Passing after: `TestCALV0192_FailedTickSaveIsReportedAndRetried` (error and cause, in-memory vs
  persisted difference retained, `Run` alert, recovery save equals `ledgerBytes` of the current
  ledger) and `TestCALV0192_UnsavedTickKeepsRunningWorkers` (a live worker stays recorded and
  running across two unsaved ticks, no `killing`, the recovered save records it, a restart adopts it).
- Four existing tests asserted the suppressed success and now require the surfaced error:
  `TestSERVICE500_UnsavedLaunchStaysUnrecorded`, `TestSERVICE500_UnsyncedLedgerRenameStaysUnrecorded`,
  `TestCALV0064_LaterSaveFailureCannotReviveGrantedParking`,
  `TestCALV0064_FirstSeedEndedWorkerAndLaterFailure`.
- `go test ./internal/tasks/dispatch ./internal/tasks/service ./internal/tasks/cli` pass (GOMAXPROCS=3, -p 1).

## Non-goals, failure modes, rollback

Non-goals: power-loss durability beyond the existing write/sync/rename sequence, a new event kind or
wire code, in-tick save retries, holding launches while saves fail. Failure modes: events already
appended are not withdrawn; a dispatcher that ends before a later save succeeds restarts from the
last saved ledger as after a crash between event append and save (re-reported state, possible
re-accounting, event sequence from the saved value); a worker launched while saves fail falls to the
existing unrecorded-tree handling if never saved. Rollback: revert the commit; no ledger, event or
wire shape changed.

NOT_RUN: live dispatcher qualification against a real store and host; Linux; `make gate` (not
requested; host shared).
