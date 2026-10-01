# Tasks continuous dispatcher

Owner request [431](https://github.com/beamfall/corvint/issues/431) adds S11 / CAL-V0-052..058:
`corvint-tasks dispatch`, a foreground deterministic roster over native queue state that launches
independent host workers, supervises their whole process trees, and heals through the existing
fenced lease transactions. New package `internal/tasks/dispatch`; CLI in
`internal/tasks/cli/dispatch.go`.

Design decisions:

- No new queue authority. The dispatcher's ledger, events and logs live under the configured
  `stateDir`; the only store writes are `release --reason HANDOFF` (with
  `--evidence dispatch:<worker>` for a no-tree attempt, CAL-V0-046) and `reap`, using
  deterministic request IDs so a repeated heal replays. `status` and `unpark` never touch the store.
- Any live attempt removes its ticket from the roster, including an expired one: a launched worker
  would otherwise fail to claim when reap is off. Reap frees it on the next tick.
- Workers start in their own session. Supervision tracks session, process group and descendants by
  verified start identity, so an exited leader's children are still found (ORPHANED) and a reused
  PID is never signalled. One program has one dispatcher, enforced by a non-blocking `flock`; the
  lock file records the owner's identity so `status` reports liveness without taking the lock.
- Progress is a change in the durable fingerprint (ticket status/revision/work state; attempts with
  a candidate, gates, reviews or a durable phase). A worker's own message is never progress.
- Issue 431 asked for RETRY_EXHAUSTED readmission. CAL-V0-043 makes readmission owner authority, so
  the dispatcher only emits `needs-owner` naming `ticket reopen`; this is retained as an open question
  rather than delegated.

Live qualification (2026-10-01, macOS, opencode 2.0.21, disposable fixture store with one P1
ticket, `globalCap` 1, tick 3 s, deny-list via `OPENCODE_CONFIG_CONTENT` denying push, commit,
`rm -rf`, sudo, curl, release, reap and webfetch): the dispatcher launched `opencode run --auto
--format json` pinned to the ticket. The worker claimed under its worker ID, wrote the requested
file and exited 0. The next tick released the RUNNING no-candidate attempt as HANDOFF with
evidence `dispatch:qual-impl-1-1` (read back via `attempt show`: cause HANDOFF), extracted the
worker's final message from OpenCode JSON into the `finished` event, recorded no progress (no
candidate was submitted), started the cooldown (1 of 2 before parking) and saw the plan return to
SELECTED. SIGINT stopped the dispatcher with `interrupted: true` and zero workers left. Status
afterwards reported NOT_RUNNING and the cooling key. Pre-existing `opencode serve --service`
processes were unaffected. That first run used the pre-review worker ID format (`qual-impl-1-1`). After the review
fixes, the same scenario was rerun with a fresh state directory. Worker `qual.impl.1.e82cd67b-1`
claimed, wrote the file and exited 0. It was handed off as generation 2 (`attempt show`: cause
HANDOFF, handoffEvidence `dispatch:qual.impl.1.e82cd67b-1`). Its summary was extracted and it
cooled down (1 of 2). SIGINT stopped the dispatcher with zero workers, and status reported
NOT_RUNNING.

Not qualified: Codex as a dispatched host (argv-compatible; summary extraction is unit-tested only),
lane roles against a real pool, multi-day runs, Linux, and hostile workers. OpenCode's deny-list
is host-enforced and not containment. Tests: `go test -race -count=3 ./internal/tasks/dispatch/`
and the CLI integration test, which re-executes the test binary as a worker that claims through
`cli.Run`.

Independent review (one reviewer, no blockers) found three major and nine minor issues. All majors
and most minors were fixed before landing; `TestCALV0056_IdentityOutageAndUnknownState` covers them.

- Worker IDs could collide across dashed program and role names, and repeat after a crash or a
  deleted state directory, so heal could release another worker's attempt. They are now
  `<program>.<role>.<slot>.<nonce>-<seq>`. Names cannot contain `.`, the nonce is random per
  start, and the worker is saved to the ledger right after launch.
- An unreadable process identity was read as the process being gone, which could end a running
  worker and hand off its live attempt. The recorded tree is now kept and an alert emitted. A launch
  whose start identity cannot be read kills the new session and fails. The identity is now read
  before the Wait goroutine can reap the leader.
- Minor fixes:
  - SIGTERM is sent once per member, and the kill deadline is kept in the ledger.
  - A process-table error is no longer reported as "survived SIGKILL".
  - Heal re-observes after stopping a tree.
  - An `UNKNOWN` work state is neither progress nor an unpark trigger.
  - The exit code is drained before accounting.
  - `activityPaths` are rendered per worker.
  - Supervision runs during a store outage.
  - Adoption reports a gone leader.
  - `workRoot` must resolve to the dispatcher's store.
  - SIGHUP stops the loop like SIGTERM.
  - Session expansion re-checks the leader identity live.
- Reap scope is documented rather than narrowed: an expired lease is reapable by any operator, so
  the dispatcher reaps any expired lease not held by one of its live workers.

Renumbering (2026-10-01): PR 432 landed CAL-V0-048..051 on main first, so the dispatcher's
requirements moved from CAL-V0-048..054 to CAL-V0-052..058, and their tests were renamed to match.
The behaviour is unchanged. The first sealed CEM still cites the old IDs.
