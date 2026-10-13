# V1-1352 lease writes settle a held pending receipt (issue 860)

Date: 2026-10-13. Base: `eee3b51ec48ab55daac93387cdff506bed696e15`.
Requirements: `CAL-V0-222`, `CAL-V0-223` (`docs/specs/corvint-tasks-agent-leases-v0.md`), proposed
(V1-1352). No decision file.

## Finding

Issue 860 reports five consecutive `attempt heartbeat` calls, about 360 seconds apart, each refused
`REDO_PENDING` (retryable); the caller fenced and killed a healthy 36-minute command. A `REDO_PENDING`
is a receipt linked in at `head.seq + 1` whose `head.json` rename never happened, the state a writer
killed at §5.2 crash point C2 leaves. Reads wait for it only up to their bounded patience and never
settle it.

Every lease command (`leaseCommand` in `internal/tasks/cli/lease.go`: claim, renew, heartbeat,
release, submit, gate run, complete, the pool verbs, reap, widen, health) ran one `snapshot.Probe`
before writing, only to learn the queue ID, and returned any probe error verbatim. The store lease
writer already settles a validated pending receipt under the lock (`prepareLease` marks it pending,
`commitLease` runs `redoLeaseProof`, then the next round commits the request), but the CLI never
reached it. Only writers that skip that probe, such as ticket mutations (`store.Mutate` takes the
queue from the intent store), settled the receipt, which is the likely way the reported state later
cleared (inference; the reporter's store was not inspected). The issue's guess that a claim settles
it is wrong: claim was refused at the same probe.

Answers to the issue's questions:

1. A writer interrupted between linking its receipt and renaming `head.json` leaves the redo
   pending. Any §5.2 writer that reaches the store lock settles it; before this change lease
   commands could not, so the next ticket mutation was the only settling writer.
2. No. A refused heartbeat records nothing: `lastHeartbeatAt`, the work lease and the generation
   are unchanged, and `holderStatus` is derived on read from the last recorded heartbeat against
   `heartbeatTTLSeconds` (default 600). Five refusals 360 seconds apart therefore left the holder
   STALE.

## Change

`leaseCommand` accepts a `REDO_PENDING` probe whose head is readable and passes the queue to the
store writer, which audits, settles the pending receipt under the lock and then commits the request,
reporting the existing completed-redo warning. Any other probe error, and a pending receipt the audit
does not validate, are refused as before. The `attempt heartbeat` help note states both caller
facts (CAL-V0-223). No store, receipt or wire format changes.

Crash safety is unchanged: the redo is the store's existing §5.2 redo under the lock. A healthy
writer still holding the lock is not preempted; the lease writer waits within `--lock-wait`, sees the
head move and retries its bounded rounds.

## Evidence

`TestCALV0222_HeartbeatSettlesHeldPendingReceipt` holds a heartbeat, then a renew, then a heartbeat
receipt pending (rewinding `head.json`) across two reads, and requires the next heartbeat to settle it
and commit its own signal (head advances by two, `replayed=false`, `FRESH_HOLDER`). Before the fix it
reproduced the issue's exact refusal. `TestCALV0223_HeartbeatHelpNamesRedoAndLiveness` checks the
help note. Both pass, also under `-race -count=3`.

## Limits

The barrier, policy, import, cutover and escalation writers and the initial probe of `attempt run`
keep the same probe-refusal pattern and are out of scope here. Live fleet qualification NOT_RUN.
