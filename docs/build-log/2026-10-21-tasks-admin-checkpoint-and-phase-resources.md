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

`policy.go` `administrativeWriteRound` (policy update and the other admin receipts) never calls it.
That is correct under CAL-V0-060, which makes retention writer-only and best-effort. The previous
checkpoint stays valid because a read replays the admin receipt on top of it. The only cost is one
extra replayed receipt until the next ordinary write.

**Proposed recommendation:** document that administrative rounds are excluded from retention as a
CAL-V0-060 clarification, with no code change. If the feat-tasks-perf checkpoint work makes replay
cost material, it can add the call in its own lane.

## V1-0671: per-phase admission of non-PATH resources

`lease_claim.go:60-68` adds every declared non-PATH resource (`declaredOther`) to each claim scope.
CAL-V0-021 requires this: a claim that holds less than the ticket's full intent could let two
attempts each hold half of a resource set they both need.

**Proposed recommendation:** no per-phase admission for 1.0. Where a phase genuinely needs only part
of the intent, split it into a child ticket with its own narrower declared resources, followed by a
serial parent integration step. Record per-phase admission as a post-1.0 idea. It needs a phase
model in the ticket schema, and today's attempt carries none.
