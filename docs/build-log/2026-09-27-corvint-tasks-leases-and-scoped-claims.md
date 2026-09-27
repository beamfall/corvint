## 2026-09-27 Corvint Tasks agent leases S3 and the S8 claim side: external-agent leases with scoped claims

S3 of `docs/specs/corvint-tasks-agent-leases-v0.md` (CAL-V0-007 and 009 to 013) and the claim side
of S8 (CAL-V0-021 to 023 and 025) are built as one change, because scope and collision sit in the
same claim transaction. The verbs are `claim`, `renew`, `release`, `reap`, `widen` and
`attempt show`. `queue status` now lists live attempts. `claim --next` (CAL-V0-008) answers
`UNSUPPORTED` until the S4 plan is wired, and the context-index scope deriver (CAL-V0-022) is an
injected hook that abstains, so an undeclared, unrequested claim holds `WHOLE_REPOSITORY`.

Decisions and spec amendments made while building:

- Generation. A new attempt takes the next queue-wide generation, as TCP-00 TM-V0-011 requires,
  not generation 1 as CAL-V0-007 first said.
- Lease receipts have `ticketId` null and name the attempt. TCP-00 binds a ticket afterimage to
  every completed receipt that names a ticket, and a lease writes no ticket file. Recorded under A9.
- Non-`PATH` resources. The first build put only `PATH` resources in a claim scope, as CAL-V0-021
  was written, so two tickets that declared one database or shared gate could be claimed together.
  Review found it before merge. Every scope short of `WHOLE_REPOSITORY` now also holds the
  ticket's declared non-`PATH` resources, and CAL-V0-021 says so.
- A `PATH` key covers the paths under it only when it ends in `/` (TCP-00 §4.2), which CAL-V0-021
  now states.
- Once a lease has expired, `renew`, `release` and `widen` are all fenced; only `reap` or a
  colliding `claim` closes the attempt. Reap writes one transaction per attempt.
- A clock that steps backward refuses `STORAGE_FAILED` before writing, and writes no receipt.
- `cutover` refuses `QUIESCENCE_UNPROVED` while any reservation entry exists. The journal reports
  semantic coverage `UNKNOWN` once attempts exist, because its semantic replay does not model them.
- The stage descriptor limit for a lease is 6 artifacts and 1677 bytes, and the request cap is 579
  bytes, both measured on the widest descriptor.

Independent review of the change found three defects, fixed before merge:

- The backward-clock check ran only for lease transactions, so a non-lease writer with a clock set
  back could record an earlier head receipt, and a later `renew` compared against it would extend
  an expired lease. Every store writer now hands the model the head receipt, and the model refuses
  any transaction earlier than it (CAL-V0-012).
- `widen` was allowed under an `ADMISSION` barrier, which refuses scope-expand. It now refuses
  `PAUSED` (CAL-V0-025).
- An `ALL` barrier refused `release` and `reap`, although it lets `cancel` through. Both now pass
  it, in the model and in the store's writer guard (CAL-V0-011).

Two lower findings are left as follow-ups: `widen` appends the added resources to the scope
without sorting them, and `queue status` now fails outright when its attempt audit fails instead of
reporting the queue without attempts.

Found in passing: `TestCALV0004_CutoverSwitchesWriterInOneReceipt` compared the barrier time with a
second clock read and failed when a second boundary fell between them. It now bounds the time by
reads taken before and after the switch.

Evidence: `TestCALV0007_*` to `TestCALV0013_*`, `TestCALV0021_*`, `TestCALV0022_*`,
`TestCALV0023_*` and `TestCALV0025_*` in `internal/tasks/store` and `internal/tasks/ticket`. There
are no CLI tests for the lease verbs yet. The lock-hold target (CAL-V0-026) is not measured here.

Rollback: remove the lease verbs, `internal/tasks/store/lease.go`, the `transaction/lease*.go`
files and the lease stage operation. A store holding live `external-agent` attempts must first
`release` or `reap` them, because a rolled-back reader reports them `NOT_OBSERVED`.
