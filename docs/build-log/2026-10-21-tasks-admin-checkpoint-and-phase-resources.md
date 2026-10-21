# V1-0673 and V1-0671: admin-writer checkpoint retention, and per-phase non-PATH resources

Spike tickets V1-0673 and V1-0671. Decisions delegated to the orchestrator (decision 0548). Both
touch `internal/tasks`, where `claude/feat-tasks-perf` is active, so this spike changes no code.

## V1-0673: administrative writes do not retain a checkpoint

`retainCheckpoint` is called from these writer rounds:

- `guards.go:124` (the shared helper);
- `writer_route.go:134` and `:320`;
- `writer_checkpoint.go:456`;
- `lease_write.go:679`;
- `mutate.go:154`.

**Observed behaviour.** `policy.go` `administrativeWriteRound` (policy update and the other
administrative receipts) never calls `retainCheckpoint`. After an administrative write, the next
read starts from the previous checkpoint and replays the administrative receipt. Repeated
administrative writes with no ordinary write between them accumulate that replay work: each one adds
a receipt that every later read replays until an ordinary writer round retains a new checkpoint.

**Contract.** CAL-V0-060 makes checkpoint retention writer-side and best-effort. It does not exempt
administrative writers. The observed omission is therefore a gap against the contract as written,
not an intended exclusion. It is safe, because the previous checkpoint stays valid and replay is
correct, but it is not specified.

**Proposed recommendation (policy change for the orchestrator):** choose one of the following.

- **(a)** Have administrative rounds retain a checkpoint like other writer rounds. This bounds
  replay and keeps CAL-V0-060 unchanged. *Preferred.*
- **(b)** Amend CAL-V0-060 to exempt administrative writers explicitly, and record the accumulating
  replay cost as a known limit.

Option (a) is a small change in `policy.go`. It belongs in the feat-tasks-perf lane, which owns the
checkpoint route, so this spike makes no code change.

## V1-0671: per-phase admission of non-PATH resources

`lease_claim.go:60-68` adds every declared non-PATH resource (`declaredOther`) to each claim scope.
CAL-V0-021 requires this: a claim that holds less than the ticket's full intent could let two
attempts each hold half of a resource set they both need.

**Proposed recommendation:** no per-phase admission for 1.0. Where a phase genuinely needs only part
of the intent, split it into a child ticket with its own narrower declared resources, followed by a
serial parent integration step. Record per-phase admission as a post-1.0 idea. It needs a phase
model in the ticket schema, and today's attempt carries none.
