# 2026-10-07: dispatch-state /2 upgrade over proven-gone workers (V1-0980)

## Intent

Issue 668 (ticket V1-0980) records an upgrade from `main-0b45b05` (dispatch state `/2`) to
`main-0b5096c` (`/3`, V1-0966) that left a program down. The dispatcher received SIGTERM before its
last tick reaped one ended worker, and the new build refused the ledger with
`UNSUPPORTED_VERSION` because it still recorded that worker, although the process was gone.
Recovery needed the old binary, run once with launches held. The change adds proposed CAL-V0-187
to `docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance.

## Decisions

- **Same check as the old reap.** A worker is proven gone when `refreshTree`, the process-tree
  refresh the `/2` build reaps by, finds no process. `process_unix.go` is unchanged between
  `0b45b05` and `0b5096c`, so this is the `/2` build's check. It counts a recorded process only
  while it runs with its recorded start identity. It also follows the dead leader's process group
  and session, so an orphaned child keeps the worker live. The old build would stop that child
  first (ORPHANED), not reap.
- **No detached-run exemption at load.** The CAL-V0-145 exemption needs the attempts the worker
  holds, which the ledger load does not read. Running without it can only find more processes,
  so the proof is conservative. A worker the old build would reap beside a live exempt supervisor
  refuses, and the old build clears it.
- **Identities required.** A worker record without a leader PID above 1 and a leader identity, or
  with a member lacking either, is unprovable and refuses. A ledger without a readable workers
  list refuses as before. A process-table or identity read error,
  or a platform without a process table, refuses too.
- **Reap by the ordinary path.** The load keeps the workers as recorded, on a copy for the probe.
  `Open` already holds the single-dispatcher lock and emits `adopted`. Its first tick's `supervise`
  finds each tree empty and reaps through the ordinary path: hand-off, budget usage, the `finished`
  event, and a stall count of `UNKNOWN` because there is no launch seed. No new event kind or
  reap path is added. `dispatch status` reads the same ledger and never reaps or writes.
- **Refusal names the fix.** The `/1` and `/2` refusals name the first unproven worker and why,
  then the clearing step for the build that wrote the ledger: set role and escalate tier caps to
  0 in a copy CONFIG, then run `corvint-tasks dispatch --program P --config CONFIG --once` until
  status lists no worker. The caps matter because the reaping tick would otherwise launch a new
  worker and leave the ledger undrained. `globalCap` cannot be 0. The ledger does not record the
  config path, so `CONFIG` stays a placeholder. That is also correct, since the operator must
  pass the held copy, not the live config.
- **Review.** The independent review found that moving the workers check had let a `/2` ledger
  without a workers list adopt as empty. The presence check is restored for both versions, and the
  migrate test now covers the attempt hand-off and the budget history across a restart.
- **Scope.** `/1` ledgers with workers still refuse; only the message changes. There is no new
  verb (issue option 2), as the coordinator decided.

## Evidence

Focused tests in `internal/tasks/dispatch` with real worker processes the dispatcher launched:

- `TestCALV0187_Version2LedgerWithGoneWorkersMigrates`: two dead workers. Load adopts `/3`.
  `Open`+`Tick` emit `adopted` and `finished` for both, hand off the held attempt once, and charge
  no new session. A restart repeats neither, and the saved ledger is `/3` with no workers.
- `TestCALV0187_Version2LedgerWithLiveOrUnprovenWorkerRefuses`: one live worker, a missing
  identity, a missing workers list, an injected identity read failure, and a `/1` worker each refuse
  `UNSUPPORTED_VERSION` from both `LoadLedger` and `Open`. The ledger is left unchanged, the live
  worker untouched, and the message content is checked. Control: the same ledger once both
  workers are dead adopts.
- `TestCALV0187_Version2LedgerWithOrphanedProcessRefuses`: leader killed with its background
  child alive refuses as still running.

Rollback: revert the code and the spec amendment. Ledgers already adopted are `/3` and follow the
CAL-V0-185 rollback.
