# Decision 0492 — PGO-V0-009 no process-group signal after the reap, accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("yes, that's the RC3 cut, start on it" and "approve all the things"), covering the RC3
process-group lane: V1-1036, V1-1047, V1-1050 and V1-1056.

## Context

Once a process-group leader is reaped, its numeric group ID can be reused, so a group signal sent
after the reap can reach an unrelated group. `PGO-V0-008` moved the `groupreap` helpers to
signal-before-reap. The V1-1036 and V1-1047 audits found further product sites that still signalled
by number after the reap, and V1-1056 found that `groupreap.Run` groups were not recorded for the
`AHI-048` exit kill. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `PGO-V0-009` in `docs/specs/process-group-owner-v0.md`:

1. Where waitid is available, no product path sends a real numeric group signal once the leader may
   have been reaped. The task-store lease gate, batch command and preflight, the dashboard wait
   paths, the tasks doctor plugin and the work version probe now signal only while the exited
   leader is held unreaped.
2. `groupreap.Run`, `RunRetiring` and `RunContained` record the group they lead for the exit kill
   until the pre-reap release, and refuse a start after `KillLive`.
3. Three post-reap observations are retained. Each sends signal 0 only and fails closed. They are
   the pool's `CAL-V0-188` absence check, the release gate's absence check, and the Owner's absence
   check.

The acceptance evidence is the tests named in the `PGO-V0-009` trace row. See
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-pgo-v0-009-no-group-signal-after-reap.md`.

## Limits

This decision settles intent. The evidence is focused Darwin tests run with `-race`; Linux was
checked by vet only. `make gate` and live host qualification are `NOT_RUN`.

`internal/liveverify/gorunner` still cleans up after the reap. That is a known gap under the
go-live-test-provider contract and is proposed as follow-up work. This decision completes no
ticket.

## Rollback

Revert this decision and the `PGO-V0-009` commits, then regenerate `docs/specs/REQUIREMENTS.tsv`.
No store or wire format changes.
